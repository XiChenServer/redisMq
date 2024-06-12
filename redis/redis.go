package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/demdxx/gocast"
	"github.com/gomodule/redigo/redis"
)

// MsgEntity 消息的结构体
type MsgEntity struct {
	MsgID string
	Key   string
	Val   string
}

// ErrNoMsg 没有消息的报错信息
var ErrNoMsg = errors.New("no msg received")

// Client Redis 客户端.
type Client struct {
	opts *ClientOptions //redis的配置信息
	pool *redis.Pool    //redis的连接池
}

// NewClient 连接一个redis的客户端
func NewClient(network, address, password string, opts ...ClientOption) *Client {
	c := Client{
		opts: &ClientOptions{
			network:  network,
			address:  address,
			password: password,
		},
	}
	//对于其他的配置
	for _, opt := range opts {
		opt(c.opts)
	}
	//修复中间的无效值
	repairClient(c.opts)
	//上面的都是进行配置，这里获取
	pool := c.getRedisPool()
	return &Client{
		pool: pool,
	}
}

func NewClientWithPool(pool *redis.Pool, opts ...ClientOption) *Client {
	c := Client{
		pool: pool,
		opts: &ClientOptions{},
	}

	for _, opt := range opts {
		opt(c.opts)
	}
	repairClient(c.opts)

	return &c
}

// 创建并返回一个新的 redis.Pool 实例
func (c *Client) getRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     c.opts.maxIdle,
		IdleTimeout: time.Duration(c.opts.idleTimeoutSeconds) * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := c.getRedisConn()
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		MaxActive: c.opts.maxActive,
		Wait:      c.opts.wait,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func (c *Client) GetConn(ctx context.Context) (redis.Conn, error) {
	return c.pool.GetContext(ctx)
}

func (c *Client) getRedisConn() (redis.Conn, error) {
	if c.opts.address == "" {
		panic("Cannot get redis address from config")
	}

	var dialOpts []redis.DialOption
	if len(c.opts.password) > 0 {
		dialOpts = append(dialOpts, redis.DialPassword(c.opts.password))
	}
	conn, err := redis.DialContext(context.Background(),
		c.opts.network, c.opts.address, dialOpts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// XADD 进行添加到消息队列中
func (c *Client) XADD(ctx context.Context, topic string, maxLen int, key, val string) (string, error) {
	if topic == "" {
		return "", errors.New("redis XADD topic can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	return redis.String(conn.Do("XADD", topic, "MAXLEN", maxLen, "*", key, val))
}

// XACK 消费之后返回ack的响应信息
func (c *Client) XACK(ctx context.Context, topic, groupID, msgID string) error {
	//需要必要的信息
	if topic == "" || groupID == "" || msgID == "" {
		return errors.New("redis XACK topic | group_id | msg_ id can't be empty")
	}
	//从redis连接池中获取
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	//返回响应
	reply, err := redis.Int64(conn.Do("XACK", topic, groupID, msgID))
	if err != nil {
		return err
	}
	if reply != 1 {
		return fmt.Errorf("invalid reply: %d", reply)
	}

	return nil
}

// XReadGroupPending 指定的消费者组和消费者中待确认（pending）的消息,有坏的ack回应，需要再走一遍
func (c *Client) XReadGroupPending(ctx context.Context, groupID, consumerID, topic string) ([]*MsgEntity, error) {
	return c.xReadGroup(ctx, groupID, consumerID, topic, 0, true)
}

// XReadGroup 从指定的消费者组和消费者中读取消息，获取新消息
func (c *Client) XReadGroup(ctx context.Context, groupID, consumerID, topic string, timeoutMiliSeconds int) ([]*MsgEntity, error) {
	return c.xReadGroup(ctx, groupID, consumerID, topic, timeoutMiliSeconds, false)
}

func (c *Client) xReadGroup(ctx context.Context, groupID, consumerID, topic string, timeoutMiliSeconds int, pending bool) ([]*MsgEntity, error) {
	if groupID == "" || consumerID == "" || topic == "" {
		return nil, errors.New("redis XREADGROUP groupID/consumerID/topic can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// consumer 刚启动时，批量获取一次分配给本节点，但是还没 ack 的消息进行处理
	// consumer 处理消息之后，如果想给一个坏的 ack，那则是再获取一次 pending 重新走一次流程
	// 分配给本节点，但是尚未 ack 的消息 0-0
	// 拿到尚未分配过的新消息 >
	var rawReply interface{}
	if pending {
		rawReply, err = conn.Do("XREADGROUP", "GROUP", groupID, consumerID, "STREAMS", topic, "0-0")
	} else {
		rawReply, err = conn.Do("XREADGROUP", "GROUP", groupID, consumerID, "BLOCK", timeoutMiliSeconds, "STREAMS", topic, ">")
	}

	if err != nil {
		return nil, err
	}
	reply, _ := rawReply.([]interface{})
	if len(reply) == 0 {
		return nil, ErrNoMsg
	}

	replyElement, _ := reply[0].([]interface{})
	if len(replyElement) != 2 {
		return nil, errors.New("invalid msg format")
	}

	var msgs []*MsgEntity
	rawMsgs, _ := replyElement[1].([]interface{})
	for _, rawMsg := range rawMsgs {
		_msg, _ := rawMsg.([]interface{})
		if len(_msg) != 2 {
			return nil, errors.New("invalid msg format")
		}
		msgID := gocast.ToString(_msg[0])
		msgBody, _ := _msg[1].([]interface{})
		if len(msgBody) != 2 {
			return nil, errors.New("invalid msg format")
		}
		msgKey := gocast.ToString(msgBody[0])
		msgVal := gocast.ToString(msgBody[1])
		msgs = append(msgs, &MsgEntity{
			MsgID: msgID,
			Key:   msgKey,
			Val:   msgVal,
		})
	}

	return msgs, nil
}

// Get 获取key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("redis GET key can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	return redis.String(conn.Do("GET", key))
}

func (c *Client) Set(ctx context.Context, key, value string) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key or value can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	resp, err := conn.Do("SET", key, value)
	if err != nil {
		return -1, err
	}

	if respStr, ok := resp.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}

	return redis.Int64(resp, err)
}

// SetNEX 设置键值对，如果键不存在，则设置键值对并返回成功；如果键已存在，则不执行任何操作。此方法还允许设置键的过期时间（以秒为单位）。
func (c *Client) SetNEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET keyNX or value can't be empty")
	}

	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	reply, err := conn.Do("SET", key, value, "EX", expireSeconds, "NX")
	if err != nil {
		return -1, err
	}

	if respStr, ok := reply.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}

	return redis.Int64(reply, err)
}

// SetNX 不设置过期时间，仅在键不存在时设置键值对
func (c *Client) SetNX(ctx context.Context, key, value string) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key NX or value can't be empty")
	}

	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	reply, err := conn.Do("SET", key, value, "NX")
	if err != nil {
		return -1, err
	}

	if respStr, ok := reply.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}

	return redis.Int64(reply, err)
}

func (c *Client) Del(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("redis DEL key can't be empty")
	}

	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Do("DEL", key)
	return err
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return -1, errors.New("redis INCR key can't be empty")
	}

	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	return redis.Int64(conn.Do("INCR", key))
}

// Eval 支持使用 lua 脚本.
func (c *Client) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error) {
	args := make([]interface{}, 2+len(keysAndArgs))
	args[0] = src
	args[1] = keyCount
	copy(args[2:], keysAndArgs)

	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	return conn.Do("EVAL", args...)
}

// XGroupCreate 用于创建一个新的消费者组
func (c *Client) XGroupCreate(ctx context.Context, topic, group string) (string, error) {
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	return redis.String(conn.Do("XGROUP", "CREATE", topic, group, "0-0"))
}
