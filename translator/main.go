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
	"syscall"

	"github.com/go-redis/redis"
	"github.com/spf13/viper"

	"github.com/kind84/polygo/translator/translator"
)

// setting stream data for each stream to be listening on
var streams = []translator.StreamData{
	translator.StreamData{
		StreamFrom: "storyblok",
		Group:      "translate",
		Consumer:   "translator",
		StreamTo:   "translator",
	},
}

// start rpc server
func startServer(t *translator.RPCTranslator) {
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

	// Wire shutdownCh to get events depending on the OS we are running in
	if runtime.GOOS == "windows" {
		fmt.Println("Listening to Windows OS interrupt signal for graceful shutdown.")
		signal.Notify(shutdownCh, os.Interrupt)

	} else {
		fmt.Println("Listening to SIGINT or SIGTERM for graceful shutdown.")
		signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	}

	rpcT := new(translator.RPCTranslator)

	// start jsonrpc server
	fmt.Println("Jsonrpc sever listening on port 8090")
	go startServer(rpcT)

	// setting up redis client
	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})
	defer rdb.Close()

	t := translator.NewTranslator()

	// start reading streams
	for _, s := range streams {
		go t.ReadStreamAndTranslate(rdb, s)
	}

	// wait for shutdown
	if <-shutdownCh != nil {
		fmt.Println("\nShutdown signal detected, gracefully shutting down...")
		t.CloseGracefully()
	}
	fmt.Println("bye")
}
