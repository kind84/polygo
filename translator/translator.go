package translator

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"

	"github.com/kind84/polygo/storyblok"
)

type TRequest struct {
	ID         string
	field      string
	sourceText string
}

type TResponse struct {
	ID          string
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

type Fields struct {
	ID     string
	Fields map[string]string
}

func TranslateRecipe(m Message) {
	fields := Fields{
		ID: string(m.Story.ID),
		Fields: map[string]string{
			"Extra":       m.Story.Content.Extra,
			"Title":       m.Story.Content.Title,
			"Summary":     m.Story.Content.Summary,
			"Conclusion":  m.Story.Content.Conclusion,
			"Description": m.Story.Content.Description,
		},
	}

	// copy recipe object and do reflection on the copy
	s := m.Story

	resChan := make(chan TResponse)
	stpChan := make(chan TResponse)
	defer close(resChan)
	defer close(stpChan)

	go translateFields(fields, resChan)

	fm := make(map[string]map[string]string, len(s.Content.Steps))
	for _, stp := range s.Content.Steps {
		stpFields := Fields{
			ID: stp.UID,
			Fields: map[string]string{
				"Title":   stp.Title,
				"Content": stp.Content,
			},
		}
		fm[stp.UID] = map[string]string{
			"Title":   "",
			"Content": "",
		}

		go translateFields(stpFields, stpChan)
	}

	val := reflect.ValueOf(&s.Content).Elem()

	for i := 0; i < len(fields.Fields)+(len(s.Content.Steps)*2); i++ {
		select {
		case t := <-resChan:
			val.FieldByName(t.field).SetString(t.translation)
		case stpT := <-stpChan:
			fm[stpT.ID][stpT.field] = stpT.translation
		}
	}

	for i := 0; i < len(s.Content.Steps); i++ {
		sm := fm[s.Content.Steps[i].UID]
		s.Content.Steps[i].Title = sm["Title"]
		s.Content.Steps[i].Content = sm["Content"]
	}

	// send translated recipe over the channel
	m.Translation <- s
}

func translateFields(f Fields, resChan chan (TResponse)) {
	for k, v := range f.Fields {
		if v != "" {
			tReq := TRequest{
				ID:         f.ID,
				field:      k,
				sourceText: v,
			}
			// context ??
			go func() { resChan <- translate(tReq) }()
		} else {
			resChan <- TResponse{
				ID:          f.ID,
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
	q.Add("sl", "en")
	q.Add("tl", "it")
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
		ID:          tReq.ID,
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
