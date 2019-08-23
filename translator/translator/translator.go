package translator

import (
	"context"
	"log"
	"reflect"
	"strconv"

	"cloud.google.com/go/translate"
	"golang.org/x/text/language"

	"github.com/kind84/polygo/storyblok/storyblok"
)

// const apiKey = "YOUR_TRANSLATE_API_KEY"

type Translator struct{}

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
	ID          string
	Story       storyblok.Story
	Translation chan TMessage
}

type TMessage struct {
	ID    string
	Story storyblok.Story
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

type Reply struct {
	ID          interface{}     `json:"id"`
	Translation storyblok.Story `json:"translation"`
}

type Request struct {
	Story storyblok.Story
}

func (t *Translator) Translate(req *Request, reply *Reply) error {
	ctx := context.Background()

	tChan := make(chan TMessage)
	defer close(tChan)

	m := Message{
		ID:          req.Story.UUID,
		Story:       req.Story,
		Translation: tChan,
	}

	go TranslateRecipe(ctx, m)

	tm := <-tChan
	reply.Translation = tm.Story

	return nil
}

func TranslateRecipe(ctx context.Context, m Message) {
	// m.Translation <- TMessage{
	// 	ID:    m.ID,
	// 	Story: m.Story,
	// }
	// return

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
	igrChan := make(chan TResponse)
	defer close(resChan)
	defer close(stpChan)
	defer close(igrChan)

	go translateFields(ctx, fields, resChan)

	sfm := make(map[string]map[string]string, len(s.Content.Steps))
	ifm := make(map[string]map[string]string, len(s.Content.Ingredients.Ingredients))
	for _, stp := range s.Content.Steps {
		stpFields := Fields{
			ID: stp.UID,
			Fields: map[string]string{
				"Title":   stp.Title,
				"Content": stp.Content,
			},
		}
		sfm[stp.UID] = map[string]string{
			"Title":   "",
			"Content": "",
		}

		go translateFields(ctx, stpFields, stpChan)
	}

	for i, igr := range s.Content.Ingredients.Ingredients {
		igrFields := Fields{
			ID: strconv.Itoa(i),
			Fields: map[string]string{
				"Name": igr.Name,
			},
		}
		ifm[strconv.Itoa(i)] = map[string]string{
			"Name": "",
		}

		go translateFields(ctx, igrFields, igrChan)
	}

	val := reflect.ValueOf(&s.Content).Elem()

	totFields := len(fields.Fields) + (len(s.Content.Steps) * 2) + len(s.Content.Ingredients.Ingredients)
	for i := 0; i < totFields; i++ { // TODO: use steps fields count instead of the hardcoded number
		select {
		case t := <-resChan:
			val.FieldByName(t.field).SetString(t.translation)
		case stpT := <-stpChan:
			sfm[stpT.ID][stpT.field] = stpT.translation
		case igrT := <-igrChan:
			ifm[igrT.ID][igrT.field] = igrT.translation
		}
	}

	for i := 0; i < len(s.Content.Steps); i++ {
		sm := sfm[s.Content.Steps[i].UID]
		s.Content.Steps[i].Title = sm["Title"]
		s.Content.Steps[i].Content = sm["Content"]
	}

	for i := 0; i < len(s.Content.Ingredients.Ingredients); i++ {
		im := ifm[strconv.Itoa(i)]
		s.Content.Ingredients.Ingredients[i].Name = im["Name"]
	}

	// send translated recipe over the channel
	log.Printf("Translated message ID %s\n", m.ID)
	tm := TMessage{
		ID:    m.ID,
		Story: s,
	}
	m.Translation <- tm
}

func translateFields(ctx context.Context, f Fields, resChan chan (TResponse)) {
	for k, v := range f.Fields {
		if v != "" {
			tReq := TRequest{
				ID:         f.ID,
				field:      k,
				sourceText: v,
			}

			go func() { resChan <- translateText(ctx, tReq) }()

		} else {
			resChan <- TResponse{
				ID:          f.ID,
				field:       k,
				translation: "",
			}
		}
	}
}

func translateText(ctx context.Context, tReq TRequest) TResponse {
	client, err := translate.NewClient(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	// lang, err := language.Parse(targetLang)
	// if err != nil {
	// 	return tResp, err
	// }

	resp, err := client.Translate(ctx, []string{tReq.sourceText}, language.English, nil)
	if err != nil {
		log.Fatalln(err)
	}

	return TResponse{
		ID:          tReq.ID,
		field:       tReq.field,
		translation: resp[0].Text,
	}
}

// func translateText(ctx context.Context, tReq TRequest) TResponse {
// req, err := http.NewRequest("GET", "https://translate.googleapis.com/translate_a/single", nil)
// if err != nil {
// 	log.Fatalln(err)
// }

// q := req.URL.Query()
// q.Add("client", "gtx")
// q.Add("sl", "en")
// q.Add("tl", "it")
// q.Add("dt", "t")
// q.Add("q", tReq.sourceText)
// req.URL.RawQuery = q.Encode()

// client := &http.Client{}

// r, err := client.Do(req)
// if err != nil {
// 	log.Fatalln(err)
// }
// defer r.Body.Close()

// var resp []interface{}
// err = json.NewDecoder(r.Body).Decode((&resp))
// if err != nil {
// 	log.Fatalln(err)
// }

// return TResponse{
// 	ID:          tReq.ID,
// 	field:       tReq.field,
// 	translation: resp[0].([]interface{})[0].([]interface{})[0].(string),
// }
// }
