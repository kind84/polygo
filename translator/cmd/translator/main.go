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
	"golang.org/x/text/language"

	"github.com/kind84/polygo/translator/translator"
)

// setting stream data for each stream to be listening on
var streams = []translator.StreamData{
	translator.StreamData{
		StreamFrom: "storyblok",
		Group:      "translate_it-en",
		Consumer:   "translator_it-en",
		StreamTo:   "translation_en",
		LangFrom:   language.Italian,
		LangTo:     language.English,
	},
	translator.StreamData{
		StreamFrom: "translation_en",
		Group:      "translate_en-fr",
		Consumer:   "translator_en-fr",
		StreamTo:   "translation_fr",
		LangFrom:   language.English,
		LangTo:     language.French,
	},
}

// start rpc server
func startServer(rdb *redis.Client) {
	t := new(translator.RPCTranslator)
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

	// setting up redis client
	rh := viper.GetString("redis.host")
	rdb := redis.NewClient(&redis.Options{Addr: rh})

	// start jsonrpc server
	fmt.Println("Jsonrpc sever listening on port 8090")
	go startServer(rdb)

	defer rdb.Close()

	t := translator.NewTranslator(rdb)

	// start reading streams
	for _, s := range streams {
		fmt.Printf("Start reading stream %s\n", s.StreamFrom)
		go t.ReadStreamAndTranslate(ctx, s)
	}

	// wait for shutdown
	if <-shutdownCh != nil {
		fmt.Println("\nShutdown signal detected, gracefully shutting down...")
		t.CloseGracefully()
	}
	fmt.Println("bye")
}
