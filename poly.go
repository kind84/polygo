package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"

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

	var txt struct {
		Text string `json:"text"`
	}

	err := json.NewDecoder(req.Body).Decode(&txt)
	if err != nil {
		log.Fatalln(err)
	}

	translation, err := translator.Translate(txt.Text)
	if err != nil {
		log.Fatalln(err)
	}

	story, err := storyblok.NewStories()
	if err != nil {
		log.Fatalln(err)
	}

	resp := struct {
		Translation string           `json:"translation"`
		Story       *storyblok.Story `json:"story"`
	}{
		Translation: translation,
		Story:       story,
	}

	json.NewEncoder(w).Encode(resp)
}
