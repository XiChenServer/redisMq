package redis

import (
	"context"
	"testing"
)

const (
	network  = "tcp"
	address  = "127.0.0.1:6379"
	password = ""
)

func Test_redis_xadd(t *testing.T) {
	client := NewClient(network, address, password)
	ctx := context.Background()
	res, err := client.XADD(ctx, "test_stream_topic", 3, "12357", "12367")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(res)
}

func Test_redis_xreadergroup(t *testing.T) {
	client := NewClient(network, address, password)
	ctx := context.Background()
	res, err := client.XReadGroupPending(ctx, "123", "0", "test_stream_topic")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(res)
}

func Test_redis_xgroupcreate(t *testing.T) {
	client := NewClient(network, address, password)
	ctx := context.Background()
	res, err := client.XGroupCreate(ctx, "test_stream_topic", "1234")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(res)
}

func Test_redis_xack(t *testing.T) {
	client := NewClient(network, address, password)
	ctx := context.Background()
	err := client.XACK(ctx, "test_stream_topic", "123", "1718191087935-0")
	if err != nil {
		t.Error(err)
		return
	}
}

func Test_redis_xreadgroup(t *testing.T) {
	client := NewClient(network, address, password)
	ctx := context.Background()
	res, err := client.XReadGroup(ctx, "1234", "0", "test_stream_topic", 2)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(res)
}
