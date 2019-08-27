package main

import (
	"fmt"
	"log"
	//	"net"
	//	"net/rpc"
	//	"net/rpc/jsonrpc"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"

	"github.com/kind84/polygo/translator/translator"
)

//func startServer(t translator.Translator) {
//	server := rpc.NewServer()
//	server.Register(t)
//
//	l, err := net.Listen("tcp", ":8090")
//	if err != nil {
//		log.Fatalln("listen error:", err)
//	}
//
//	for {
//		conn, err := l.Accept()
//		if err != nil {
//			log.Fatalln(err)
//		}
//
//		go server.ServeCodec(jsonrpc.NewServerCodec(conn))
//	}
//}

func init() {
	fmt.Println("Setting up configuration...")
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
	shutdownCh := make(chan os.Signal, 1)

	// Wire sutdownCh to get events depending on the OS we are running in
	if runtime.GOOS == "windows" {
		println("Listening to Windows OS interrupt signal for graceful shutdown.")
		signal.Notify(shutdownCh, os.Interrupt)

	} else {
		println("Listening to SIGINT or SIGTERM for graceful shutdown.")
		signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	}

	t := translator.NewTranslator()

	fmt.Println("Jsonrpc sever listening on port 8090")
	//go startServer(t)

	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})
	defer rdb.Close()

	// start reading streams
	go t.ReadStoryGroup(rdb)

	// wait for shutdown
	if <-shutdownCh != nil {
		fmt.Println("\nShutdown signal detected, gracefully shutting down...")
		t.CloseGracefully()
		fmt.Println("bye")
	}
}
