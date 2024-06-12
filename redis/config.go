package redis

const (
	// DefaultIdleTimeoutSeconds 默认连接池超过 10 秒释放连接
	DefaultIdleTimeoutSeconds = 10
	// DefaultMaxActive 默认最大激活连接数
	DefaultMaxActive = 100
	// DefaultMaxIdle 默认最大空闲连接数
	DefaultMaxIdle = 20
)

// 对于封装的redis进行
type ClientOptions struct {
	maxIdle            int  //最大空闲连接数
	idleTimeoutSeconds int  //最大空闲超时时间
	maxActive          int  //最大激活连接数
	wait               bool //连接池等待模式
	// 必填参数
	// 下面就是普通平常时候对于redis的一些连接选项
	network  string
	address  string
	password string
}

// ClientOption 是一个用于配置 ClientOptions 的函数类型
type ClientOption func(c *ClientOptions)

// WithMaxIdle 设置最大空闲连接数的选项
func WithMaxIdle(maxIdle int) ClientOption {
	return func(c *ClientOptions) {
		c.maxIdle = maxIdle
	}
}

// WithIdleTimeoutSeconds 设置空闲超时时间的选项（单位：秒）
func WithIdleTimeoutSeconds(idleTimeoutSeconds int) ClientOption {
	return func(c *ClientOptions) {
		c.idleTimeoutSeconds = idleTimeoutSeconds
	}
}

// WithMaxActive 设置最大激活连接数的选项
func WithMaxActive(maxActive int) ClientOption {
	return func(c *ClientOptions) {
		c.maxActive = maxActive
	}
}

// WithWaitMode 设置连接池等待模式的选项
func WithWaitMode() ClientOption {
	return func(c *ClientOptions) {
		c.wait = true
	}
}

// repairClient 修复 ClientOptions 中的无效值，设置为默认值
func repairClient(c *ClientOptions) {
	if c.maxIdle < 0 {
		c.maxIdle = DefaultMaxIdle
	}

	if c.idleTimeoutSeconds < 0 {
		c.idleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}

	if c.maxActive < 0 {
		c.maxActive = DefaultMaxActive
	}
}
