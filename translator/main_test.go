package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/translate"
	"github.com/go-redis/redis"
	"github.com/spf13/viper"
	"golang.org/x/text/language"
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
	rdb.XGroupCreate(stream, group, "$").Result()

	msg := map[string]interface{}{"Hello": "World"}

	argsAdd := &redis.XAddArgs{
		Stream: stream,
		// MaxLen       int64 // MAXLEN N
		// MaxLenApprox int64 // MAXLEN ~ N
		// ID:     id,
		Values: msg,
	}

	id, err := rdb.XAdd(argsAdd).Result()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("Added message ID [%s] to stream [%s]\n", id, stream)
	time.Sleep(time.Microsecond * 200)

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

func TestRedisScript(t *testing.T) {
	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})
	defer rdb.Close()

	streamFrom := "storyblok-test"
	streamTo := "translation-test"
	groupFrom := "translate-test"
	groupTo := "storybloks-test"
	consumerFrom := "translator-test"
	consumerTo := "storybloker-test"
	rdb.XGroupCreate(streamFrom, groupFrom, "$").Result()
	rdb.XGroupCreate(streamTo, groupTo, "$").Result()

	msgFrom := map[string]interface{}{"Hello": "World"}

	argsAdd := &redis.XAddArgs{
		Stream: streamFrom,
		// MaxLen       int64 // MAXLEN N
		// MaxLenApprox int64 // MAXLEN ~ N
		// ID:     id,
		Values: msgFrom,
	}

	idFrom, err := rdb.XAdd(argsAdd).Result()
	if err != nil {
		t.Error(err)
	}

	argsReadFrom := &redis.XReadGroupArgs{
		Group:    groupFrom,
		Consumer: consumerFrom,
		// List of streams and ids.
		Streams: []string{streamFrom, ">"},
		// Count   int64
		// Block: time.Millisecond * 2000,
		// NoAck   bool
	}

	itemsFrom := rdb.XReadGroup(argsReadFrom)
	if itemsFrom == nil {
		t.Error("Error reading streamFrom")
		// rdb.XDel(stream, id)
		return
	}
	fmt.Println(itemsFrom.Val())
	if len(itemsFrom.Val()) == 0 || len(itemsFrom.Val()[0].Messages) == 0 {
		t.Error("Empty streamFrom")
		// rdb.XDel(stream, id)
		return
	}

	tStreamFrom := itemsFrom.Val()[0]
	v := tStreamFrom.Messages[0].Values["Hello"].(string)
	if v != "World" {
		t.Errorf("Error reading from stream: got %s, want %s", v, "World")
		return
	}

	ackNaddScript := redis.NewScript(`
		if redis.call("xack", KEYS[1], ARGV[1], ARGV[2]) == 1 then
			return redis.call("xadd", KEYS[2], "*", ARGV[3], ARGV[4])
		end
		return false
	`)

	idTo, err := ackNaddScript.Run(
		rdb,
		[]string{streamFrom, streamTo}, // KEYS
		[]string{groupFrom, idFrom, "Hi", "Redis"}, // ARGV
	).Result()

	if err != nil {
		t.Error(err)
	}

	argsReadTo := &redis.XReadGroupArgs{
		Group:    groupTo,
		Consumer: consumerTo,
		// List of streams and ids.
		Streams: []string{streamTo, ">"},
		// Count   int64
		// Block: time.Millisecond * 2000,
		// NoAck   bool
	}

	itemsTo := rdb.XReadGroup(argsReadTo)
	if itemsTo == nil {
		t.Error("Error reading streamTo")
		// rdb.XDel(stream, id)
		return
	}
	fmt.Println(itemsTo.Val())
	if len(itemsTo.Val()) == 0 || len(itemsTo.Val()[0].Messages) == 0 {
		t.Error("Empty streamTo")
		// rdb.XDel(stream, id)
		return
	}

	tStreamTo := itemsTo.Val()[0]
	v = tStreamTo.Messages[0].Values["Hi"].(string)
	if v != "Redis" {
		t.Errorf("Error reading from stream: got %s, want %s", v, "Redis")
		return
	}

	r, err := rdb.XAck(streamTo, groupTo, idTo.(string)).Result()
	if err != nil {
		t.Error(err)
	}
	if r != 1 {
		t.Errorf("Acknowledge of message id %v failed", idTo)
	}

	rdb.XDel(streamFrom, idFrom).Result()
	rdb.XDel(streamFrom, idTo.(string)).Result()
}

func TestTranslateText(t *testing.T) {
	ctx := context.TODO()
	client, err := translate.NewClient(ctx)
	if err != nil {
		t.Error(err)
	}
	defer client.Close()

	resp, err := client.Translate(ctx, []string{"gatto"}, language.English, nil)
	if err != nil {
		t.Error(err)
		return
	}

	if resp == nil {
		t.Error("Error: missing translation response")
		return
	}
	if resp[0].Text != "cat" {
		t.Errorf("Error translating text: got '%s', want 'cat'", resp[0].Text)
	}
}
