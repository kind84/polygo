package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/rpc/jsonrpc"

	"github.com/go-redis/redis"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/kind84/polygo/storyblok/storyblok"
)

// --- Storyblok payload
type sbTask struct {
	Task    task `json:"task"`
	SpaceID int  `json:"space_id"`
}

type task struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ---

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
	mux := httprouter.New()
	mux.GET("/", hello)
	// mux.POST("/translate", translate)
	// mux.POST("/rpc/translate", rpcTranslate)
	mux.POST("/rpc/stories", rpcStories)
	mux.POST("/stream/stories", streamStories)

	log.Println("Listenting on port 8080")
	http.ListenAndServe(":8080", mux)
}

func hello(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.Write([]byte("Hello from polygo\n"))
}

// func translate(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
// 	// var task sbTask

// 	// err := json.NewDecoder(req.Body).Decode(&task)
// 	// if err != nil {
// 	// 	log.Println(err)
// 	// }
// 	ctx := context.Background()

// 	var txt struct {
// 		Text string `json:"text"`
// 	}

// 	err := json.NewDecoder(req.Body).Decode(&txt)
// 	if err != nil {
// 		log.Fatalln(err)
// 	}

// 	sbToken := viper.GetString("storyblok.token")
// 	sbClient := storyblok.NewClient(sbToken)
// 	stories, err := sbClient.NewStories()
// 	if err != nil {
// 		log.Fatalln(err)
// 	}

// 	var resp struct {
// 		Stories []storyblok.Story `json:"stories"`
// 	}

// 	tChan := make(chan storyblok.Story)
// 	defer close(tChan)

// 	for _, story := range stories {
// 		// Message with recipe and translation channel
// 		msg := translator.Message{
// 			Story:       story,
// 			Translation: tChan,
// 		}

// 		go translator.TranslateRecipe(ctx, msg)
// 	}

// 	for range stories {
// 		t := <-tChan
// 		resp.Stories = append(resp.Stories, t)
// 	}

// 	json.NewEncoder(w).Encode(resp)
// }

func streamStories(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	rh := viper.GetString("redis.host")

	rdb := redis.NewClient(&redis.Options{Addr: rh})

	msg := map[string]interface{}{"hello": "world"}

	xargs := redis.XAddArgs{
		Stream: "storyblok",
		// MaxLen       int64 // MAXLEN N
		// MaxLenApprox int64 // MAXLEN ~ N
		// ID           string
		Values: msg,
	}

	pipe := rdb.Pipeline()
	xadd := pipe.XAdd(&xargs)
	_, err := pipe.Exec()
	if err != nil {
		log.Fatalln(err)
	}

	log.Println(xadd.Val())
}

func rpcStories(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	conn, err := net.Dial("tcp", "localhost:8070")
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	request := storyblok.Request{}
	reply := storyblok.Reply{
		ID: "pippo",
	}

	c := jsonrpc.NewClient(conn)

	err = c.Call("StoryBlok.NewStories", &request, &reply)
	if err != nil {
		log.Fatalln(errors.WithStack(err))
	}

	resp := struct {
		Stories []storyblok.Story `json:"stories"`
	}{Stories: reply.Stories}

	json.NewEncoder(w).Encode(resp)
}

// func rpcTranslate(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
// 	conn, err := net.Dial("tcp", "localhost:8090")
// 	if err != nil {
// 		log.Fatalln(err)
// 	}
// 	defer conn.Close()

// 	sbToken := viper.GetString("storyblok.token")
// 	sbClient := storyblok.NewClient(sbToken)
// 	stories, err := sbClient.NewStories()
// 	if err != nil {
// 		log.Fatalln(err)
// 	}

// 	var resp struct {
// 		Stories []storyblok.Story `json:"stories"`
// 	}

// 	c := jsonrpc.NewClient(conn)

// 	for _, story := range stories {
// 		reply := translator.Reply{ID: story.ID}
// 		request := translator.Request{Story: story}

// 		err = c.Call("Translator.Translate", &request, &reply)
// 		if err != nil {
// 			log.Fatalln(err)
// 		}
// 		resp.Stories = append(resp.Stories, reply.Translation)
// 	}

// 	json.NewEncoder(w).Encode(resp)
// }
