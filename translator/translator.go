package translator

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"

	"github.com/kind84/polygo/storyblok"
)

type TRequest struct {
	field      string
	sourceText string
}

type TResponse struct {
	field       string
	translation string
}

type Message struct {
	Story       storyblok.Story
	Translation chan (storyblok.Story)
}

type Element struct {
	Slice []DeepElement
}

type DeepElement struct {
	Translation string
	Source      string
	Nil1        struct{ Pippo string }
	Nil2        struct{ Pippo string }
	Number      int
}

func TranslateRecipe(m Message) {
	fields := map[string]string{
		"Extra":       m.Story.Content.Extra,
		"Title":       m.Story.Content.Title,
		"Summary":     m.Story.Content.Summary,
		"Conclusion":  m.Story.Content.Conclusion,
		"Description": m.Story.Content.Description,
	}

	// copy recipe object and do reflection on the copy
	s := m.Story

	resChan := make(chan TResponse)
	defer close(resChan)

	go translateFields(fields, resChan)

	// for _, i := range s.Content.Ingredients.Ingredients {
	// 	igFields := map[string]string{
	// 		"Quantity": i.Quantity,
	// 		"Unit":     i.Unit,
	// 		"Name":     i.Name,
	// 	}

	// 	go translateFields(igFields, resChan)
	// }

	val := reflect.ValueOf(&s.Content).Elem()

	for range fields {
		t := <-resChan
		val.FieldByName(t.field).SetString(t.translation)
	}

	// send translated recipe over the channel
	m.Translation <- s
}

func translateFields(fields map[string]string, resChan chan (TResponse)) {
	for k, v := range fields {
		if v != "" {
			tReq := TRequest{
				field:      k,
				sourceText: v,
			}
			// context ??
			go func() { resChan <- translate(tReq) }()
		} else {
			resChan <- TResponse{
				field:       k,
				translation: "",
			}
		}
	}
}

func translate(tReq TRequest) TResponse {
	req, err := http.NewRequest("GET", "https://translate.googleapis.com/translate_a/single", nil)
	if err != nil {
		log.Fatalln(err)
	}

	q := req.URL.Query()
	q.Add("client", "gtx")
	q.Add("sl", "it")
	q.Add("tl", "en")
	q.Add("dt", "t")
	q.Add("q", tReq.sourceText)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}

	r, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer r.Body.Close()

	var resp []interface{}
	err = json.NewDecoder(r.Body).Decode((&resp))
	if err != nil {
		log.Fatalln(err)
	}

	return TResponse{
		field:       tReq.field,
		translation: resp[0].([]interface{})[0].([]interface{})[0].(string),
	}
	/*
		Google response has this structure:
		[
			[
				[
					"encodeURI (I like cycling)",  <--- TRANSLATION
					"encodeURI(mi piace andare in bici)",
					null,
					null,
					3,
					null,
					null,
					null,
					[
						[
							[
								"2b7b3f17283598f7d49f0be4a58102b7",
								"it_en_2018q4.md"
							]
						]
					]
				]
			],
			null,
			"it"
		]
	*/
}
