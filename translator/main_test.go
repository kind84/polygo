package main

import (
	"fmt"
	"testing"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"
)

func TestRedisConn(t *testing.T) {
	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})

	msg := "Hello"

	c := rdb.Echo(msg)
	if c.Val() != msg {
		t.Errorf("Error echoing from redis: got '%s', want '%s'", c.Val(), msg)
	}
}

func TestRedisConsumerGroup(t *testing.T) {
	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})
	defer rdb.Close()
	stream := "storyblok-test"
	group := "translate-test"
	consumer := "translator-test"
	rdb.XGroupCreate(stream, group, "$")

	// id := "test-1"
	msg := map[string]interface{}{"Hello": "World"}

	argsAdd := &redis.XAddArgs{
		Stream: stream,
		// MaxLen       int64 // MAXLEN N
		// MaxLenApprox int64 // MAXLEN ~ N
		// ID:     id,
		Values: msg,
	}

	// pipe := rdb.Pipeline()
	// defer pipe.Close()

	id, err := rdb.XAdd(argsAdd).Result()
	// c, err := pipe.Exec()
	if err != nil {
		t.Error(err)
	}

	argsRead := &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		// List of streams and ids.
		Streams: []string{stream, ">"},
		// Count   int64
		// Block: time.Millisecond * 2000,
		// NoAck   bool
	}

	items := rdb.XReadGroup(argsRead)
	if items == nil {
		t.Error("Error reading stream")
		// rdb.XDel(stream, id)
		return
	}
	fmt.Println(items.Val())
	if len(items.Val()) == 0 || len(items.Val()[0].Messages) == 0 {
		t.Error("Empty stream")
		// rdb.XDel(stream, id)
		return
	}

	tStream := items.Val()[0]
	v := tStream.Messages[0].Values["Hello"].(string)
	if v != "World" {
		t.Errorf("Error reading from stream: got %s, want %s", v, "World")
		return
	}

	r, err := rdb.XAck(stream, group, id).Result()
	if err != nil {
		t.Error(err)
	}
	if r != 1 {
		t.Errorf("Acknowledge of message id %v failed", id)
	}

	rdb.XDel(stream, id).Result()
}
