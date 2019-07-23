package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"

	"github.com/kind84/polygo/storyblok"
	"github.com/kind84/polygo/translator"
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
	mux.POST("/translate", translate)

	log.Println("Listenting on port 8080")
	http.ListenAndServe(":8080", mux)
}

func hello(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.Write([]byte("Hello from polygo\n"))
}

func translate(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// var task sbTask

	// err := json.NewDecoder(req.Body).Decode(&task)
	// if err != nil {
	// 	log.Println(err)
	// }
	ctx := context.Background()

	var txt struct {
		Text string `json:"text"`
	}

	err := json.NewDecoder(req.Body).Decode(&txt)
	if err != nil {
		log.Fatalln(err)
	}

	sbToken := viper.GetString("storyblok.token")
	sbClient := storyblok.NewClient(sbToken)
	stories, err := sbClient.NewStories()
	if err != nil {
		log.Fatalln(err)
	}

	var resp struct {
		Stories []storyblok.Story `json:"stories"`
	}

	tChan := make(chan storyblok.Story)
	defer close(tChan)

	for _, story := range stories {
		// Message with recipe and translation channel
		msg := translator.Message{
			Story:       story,
			Translation: tChan,
		}

		go translator.TranslateRecipe(ctx, msg)
	}

	for range stories {
		t := <-tChan
		resp.Stories = append(resp.Stories, t)
	}

	json.NewEncoder(w).Encode(resp)
}
