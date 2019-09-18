package translator

import (
	"context"
	"encoding/json"
	"log"
	"reflect"
	"strconv"
	"time"

	"cloud.google.com/go/translate"
	"github.com/go-redis/redis"
	"golang.org/x/text/language"

	"github.com/kind84/polygo/storyblok/storyblok"
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

type RPCTranslator struct{}

type translator struct {
	shutdownCh chan struct{}
}

var units map[string]struct{} = map[string]struct{}{
	"gr": struct{}{},
	"kg": struct{}{},
	"ml": struct{}{},
	"lt": struct{}{},
}

func NewTranslator() Translator {
	return &translator{make(chan struct{})}
}

func (t *translator) CloseGracefully() {
	close(t.shutdownCh)
}

func (t *translator) shouldExit() bool {
	select {
	case _, ok := <-t.shutdownCh:
		return !ok
	default:
	}
	return false
}

func (t *RPCTranslator) Translate(req *Request, reply *Reply) error {
	ctx := context.Background()

	tChan := make(chan TMessage)
	defer close(tChan)

	m := Message{
		ID:          req.Story.UUID,
		Story:       req.Story,
		Translation: tChan,
	}

	go translateRecipe(ctx, m)

	tm := <-tChan
	reply.Translation = tm.Story

	return nil
}

func (t *translator) ReadStoryGroup(rdb *redis.Client, streamFrom, group, consumer, streamTo string) {
	// create consumer group if not done yet
	rdb.XGroupCreate(streamFrom, group, "$").Result()

	lastID := "0-0"
	checkHistory := true

	for {
		if !checkHistory {
			lastID = ">"
		}

		args := &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			// List of streams and ids.
			Streams: []string{streamFrom, lastID},
			// Count   int64
			Block: time.Millisecond * 2000,
			// NoAck   bool
		}

		items := rdb.XReadGroup(args)
		if items == nil {
			// Timeout, check if it's time to exit
			if t.shouldExit() {
				return
			}
			continue
		}

		if len(items.Val()) == 0 || len(items.Val()[0].Messages) == 0 {
			checkHistory = false
			continue
		}

		// translation channel
		tChan := make(chan TMessage)
		defer close(tChan)

		sbStream := items.Val()[0]
		log.Printf("Consumer %s received %d messages\n", consumer, len(sbStream.Messages))
		for _, msg := range sbStream.Messages {
			// lastID = msg.ID

			log.Printf("Consumer %s reading message ID %s\n", consumer, msg.ID)
			var story storyblok.Story

			storyStr, ok := msg.Values["story"].(string)
			if !ok {
				log.Printf("Error parsing message ID %v into string.", msg.ID)
			}

			err := json.Unmarshal([]byte(storyStr), &story)
			if err != nil {
				// if a message is malformed continue to process other messages
				log.Println(err)
				continue
			}

			ctx := context.Background()
			m := Message{
				ID:          msg.ID,
				Story:       story,
				Translation: tChan,
			}

			go translateRecipe(ctx, m)
		}

		for i := 0; i < len(sbStream.Messages); i++ {
			tMsg := <-tChan

			js, err := json.Marshal(tMsg.Story)
			if err != nil {
				// if a story is malformed continue to process other stories
				log.Println(err)
				continue
			}

			ackNaddScript := redis.NewScript(`
				if redis.call("xack", KEYS[1], ARGV[1], ARGV[2]) == 1 then
					return redis.call("xadd", KEYS[2], "*", ARGV[3], ARGV[4])
				end
				return false
			`)

			_, err = ackNaddScript.Run(
				rdb,
				[]string{streamFrom, streamTo}, // KEYS
				[]string{group, tMsg.ID, "translation", string(js)}, // ARGV
			).Result()

			if err != nil {
				// if an error occurred running the script skip to the next story
				log.Println(err)
				continue
			}
		}
	}
}

func translateRecipe(ctx context.Context, m Message) {
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

	// initialize steps and ingredients maps to group translations
	sfm := make(map[string]map[string]string, len(s.Content.Steps))
	ifm := make(map[string]map[string]string, len(s.Content.Ingredients.Ingredients))

	// launch goroutine for each step
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

	// launch goroutine for each ingredient
	for i, igr := range s.Content.Ingredients.Ingredients {
		igrFields := Fields{
			ID: strconv.Itoa(i),
			Fields: map[string]string{
				"Name": igr.Name,
				"Unit": igr.Unit,
			},
		}
		ifm[strconv.Itoa(i)] = map[string]string{
			"Name": "",
			"Unit": "",
		}

		go translateFields(ctx, igrFields, igrChan)
	}

	// get the reflection Value for the story Content to search for its fields
	val := reflect.ValueOf(&s.Content).Elem()

	// wait for all channels to return the translation
	totFields := len(fields.Fields) + (len(s.Content.Steps) * 2) + (len(s.Content.Ingredients.Ingredients) * 2)
	for i := 0; i < totFields; i++ { // TODO: use steps fields count instead of the hardcoded number
		select {
		case t := <-resChan:
			// search the field name with reflection
			val.FieldByName(t.field).SetString(t.translation)
		case stpT := <-stpChan:
			sfm[stpT.ID][stpT.field] = stpT.translation
		case igrT := <-igrChan:
			ifm[igrT.ID][igrT.field] = igrT.translation
		}
	}

	// retrieve steps translations from map
	for i := 0; i < len(s.Content.Steps); i++ {
		sm := sfm[s.Content.Steps[i].UID]
		s.Content.Steps[i].Title = sm["Title"]
		s.Content.Steps[i].Content = sm["Content"]
	}

	// retrieve ingredients translations from map
	for i := 0; i < len(s.Content.Ingredients.Ingredients); i++ {
		im := ifm[strconv.Itoa(i)]
		s.Content.Ingredients.Ingredients[i].Name = im["Name"]
		s.Content.Ingredients.Ingredients[i].Unit = im["Unit"]
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
		// numbers, units and empty fields don't need translation
		_, unit := units[v]
		if _, err := strconv.ParseFloat(v, 64); err != nil && v != "" && !unit {
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
				translation: v,
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

	opts := &translate.Options{
		Source: language.Italian,
	}

	resp, err := client.Translate(ctx, []string{tReq.sourceText}, language.English, opts)
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
