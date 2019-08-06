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
	log.Println("Listening on port 8090")
	go startServer()

	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})

	// create consumer group if not done yet
	rdb.XGroupCreate("storyblok", "transl8", "$").Result()

	lastID := "0-0"
	checkHistory := true

	for {
		if !checkHistory {
			lastID = ">"
		}

		args := redis.XReadGroupArgs{
			Group:    "transl8",
			Consumer: "translator",
			// List of streams and ids.
			Streams: []string{"storyblok", lastID},
			// Count   int64
			Block: time.Millisecond * 2000,
			// NoAck   bool
		}

		items := rdb.XReadGroup(&args)
		if items == nil {
			// Timeout
			continue
		}

		if len(items.Val()) == 0 || len(items.Val()[0].Messages) == 0 {
			checkHistory = false
			continue
		}

		tChan := make(chan translator.TMessage)
		defer close(tChan)

		sbStream := items.Val()[0]
		log.Printf("Received %d messages\n", len(sbStream.Messages))
		for _, msg := range sbStream.Messages {
			// lastID = msg.ID

			var story storyblok.Story
			err := json.Unmarshal([]byte(msg.Values["story"].(string)), &story)
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
				[]string{"storyblok", "translator"}, // KEYS
				[]string{"transl8", tMsg.ID, "translation", string(js)}, // ARGV
			).Result()

			if err != nil {
				// if an error occurred running the script skip to the next story-
				log.Println(err)
				continue
			}
		}
	}
}
