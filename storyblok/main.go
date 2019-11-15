package main

import (
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

	log.Println("Jsonrpc server listening on port 8070")
	go startServer()

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

	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})

	s := storyblok.NewSBConsumer(rdb)

	for _, stream := range streams {
		go s.ReadTranslation(stream)
	}

	// wait for shutdown
	if <-shutdownCh != nil {
		fmt.Println("\nShutdown signal detected, gracefully shutting down...")
		s.CloseGracefully()
	}
	fmt.Println("bye")
}
