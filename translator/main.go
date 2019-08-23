package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"

	"github.com/kind84/polygo/storyblok/storyblok"
	"github.com/kind84/polygo/translator/translator"
)

func startServer() {
	t := new(translator.Translator)

	server := rpc.NewServer()
	server.Register(t)

	l, err := net.Listen("tcp", ":8090")
	if err != nil {
		log.Fatalln("listen error:", err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalln(err)
		}

		go server.ServeCodec(jsonrpc.NewServerCodec(conn))
	}
}

func init() {
	log.Println("Setting up configuration...")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("polygo")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error config file: %s", err)
	}
}

func main() {
	log.Println("Jsonrpc sever listening on port 8090")
	go startServer()

	streamFrom := "storyblok"
	group := "translate"
	consumer := "translator"

	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})
	defer rdb.Close()

	// create consumer group if not done yet
	rdb.XGroupCreate(streamFrom, group, "$").Result()

	lastID := "0-0"
	checkHistory := true

	for {
		if !checkHistory {
			lastID = ">"
		}

		args := &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			// List of streams and ids.
			Streams: []string{streamFrom, lastID},
			// Count   int64
			Block: time.Millisecond * 2000,
			// NoAck   bool
		}

		items := rdb.XReadGroup(args)
		if items == nil {
			// Timeout
			continue
		}

		if len(items.Val()) == 0 || len(items.Val()[0].Messages) == 0 {
			checkHistory = false
			continue
		}

		// translation channel
		tChan := make(chan translator.TMessage)
		defer close(tChan)

		sbStream := items.Val()[0]
		log.Printf("Consumer %s received %d messages\n", consumer, len(sbStream.Messages))
		for _, msg := range sbStream.Messages {
			// lastID = msg.ID

			log.Printf("Consumer %s reading message ID %s\n", consumer, msg.ID)
			var story storyblok.Story

			storyStr, ok := msg.Values["story"].(string)
			if !ok {
				log.Printf("Error parsing message ID %v into string.", msg.ID)
			}

			err := json.Unmarshal([]byte(storyStr), &story)
			if err != nil {
				// if a message is malformed continue to process other messages
				log.Println(err)
				continue
			}

			ctx := context.Background()
			m := translator.Message{
				ID:          msg.ID,
				Story:       story,
				Translation: tChan,
			}

			go translator.TranslateRecipe(ctx, m)
		}

		for i := 0; i < len(sbStream.Messages); i++ {
			tMsg := <-tChan

			js, err := json.Marshal(tMsg.Story)
			if err != nil {
				// if a story is malformed continue to process other stories
				log.Println(err)
				continue
			}

			ackNaddScript := redis.NewScript(`
				if redis.call("xack", KEYS[1], ARGV[1], ARGV[2]) == 1 then
					return redis.call("xadd", KEYS[2], "*", ARGV[3], ARGV[4])
				end
				return false
			`)

			_, err = ackNaddScript.Run(
				rdb,
				[]string{streamFrom, "translator"}, // KEYS
				[]string{group, tMsg.ID, "translation", string(js)}, // ARGV
			).Result()

			if err != nil {
				// if an error occurred running the script skip to the next story
				log.Println(err)
				continue
			}
		}
	}
}
