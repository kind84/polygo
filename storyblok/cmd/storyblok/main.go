package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"

	"github.com/kind84/polygo/storyblok/storyblok"
)

func startServer(rdb *redis.Client, s *storyblok.StoryBlok) {
	server := rpc.NewServer()
	server.Register(s)

	l, err := net.Listen("tcp", ":8070")
	if err != nil {
		log.Fatalln("listen error:", err)
	}

	log.Println("Jsonrpc server listening on port 8070")
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
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.ReadInConfig()
}

func main() {
	shutdownCh := make(chan os.Signal, 1)

	// Wire shutdownCh to get events depending on the OS we are running in
	if runtime.GOOS == "windows" {
		fmt.Println("Listening to Windows OS interrupt signal for graceful shutdown.")
		signal.Notify(shutdownCh, os.Interrupt)

	} else {
		fmt.Println("Listening to SIGINT or SIGTERM for graceful shutdown.")
		signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	}

	ctx := context.Background()

	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})

	token := viper.GetString("storyblok.token")
	oauth := viper.GetString("storyblok.oauth")
	space := viper.GetString("storyblok.space")

	s := storyblok.NewSBClient(token, oauth, space, rdb)

	go startServer(rdb, s)

	streams := []storyblok.StreamData{
		{
			Stream:   "translation_en",
			Group:    "storybloks_en",
			Consumer: "storybloker_en",
			Code:     "en",
		},
		{
			Stream:   "translation_fr",
			Group:    "storybloks_fr",
			Consumer: "storybloker_fr",
			Code:     "fr",
		},
	}

	sc := storyblok.NewSBConsumer(s)

	for _, stream := range streams {
		go sc.ReadTranslation(ctx, stream)
	}

	// wait for shutdown
	if <-shutdownCh != nil {
		fmt.Println("\nShutdown signal detected, gracefully shutting down...")
		sc.CloseGracefully()
	}
	fmt.Println("bye")
}
