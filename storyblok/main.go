package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"
	
	"github.com/kind84/polygo/storyblok/storyblok"
)

func startServer() {
	rh := viper.GetString("redis.host")

	rdb := redis.NewClient(&redis.Options{Addr: rh})

	s := storyblok.NewSBClient(viper.GetString("storyblok.token"), rdb)

	server := rpc.NewServer()
	server.Register(s)

	l, err := net.Listen("tcp", ":8070")
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
	log.Println("Listening on port 8070")
	go startServer()

	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})

	// create consumer group if not done yet
	rdb.XGroupCreate("translator", "storybloks", "$")

	lastID := "0-0"
	checkHistory := true

	for {
		if !checkHistory {
			lastID = ">"
		}

		args := redis.XReadGroupArgs{
			Group:    "storybloks",
			Consumer: "storyBloker",
			// List of streams and ids.
			Streams: []string{"translator", lastID},
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

		tStream := items.Val()[0]
		for _, msg := range tStream.Messages {
			lastID = msg.ID
			
			ackNaddScript := redis.NewScript(`
				return redis.call("xack", KEYS[1], ARGV[1], ARGV[2])
			`)

			_, err := ackNaddScript.Run(
				rdb,
				[]string{"translator"}, // KEYS
				[]string{"storybloks", msg.ID}, // ARGV
			).Result()

			if err != nil {
				// if an error occurred running the script skip to the next story-
				log.Println(err)
				continue
			}

			fmt.Println(msg.Values["translation"].(string))
		}
	}
}
