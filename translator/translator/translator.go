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

// StreamData groups data to establish a connection to an incoming stream and an outgoing stream
type StreamData struct {
	StreamFrom string
	Group      string
	Consumer   string
	StreamTo   string
	LangFrom   language.Tag
	LangTo     language.Tag
}

// The translation request object. Represents a single translation unit.
type tRequest struct {
	ID         string
	field      string
	sourceText string
	sourceLang language.Tag
	destLang   language.Tag
}

// The response object from a single translation request.
type tResponse struct {
	ID          string
	field       string
	translation string
}

// The translation message buid starting from the stream message.
type tMessage struct {
	id          string
	story       storyblok.Story
	translation chan tChannel
	sourceLang  language.Tag
	destLang    language.Tag
}

// The channel to send over translations.
type tChannel struct {
	id    string
	story storyblok.Story
}

type element struct {
	slice []deepElement
}

type deepElement struct {
	translation string
	source      string
	nil1        struct{ Pippo string }
	nil2        struct{ Pippo string }
	number      int
}

// Group of fields to be translated (root level, steps, ingredients, ...).
type translationData struct {
	id         string
	fields     map[string]string
	sourceLang language.Tag
	destLang   language.Tag
}

type Reply struct {
	ID          interface{}     `json:"id"`
	Translation storyblok.Story `json:"translation"`
}

type Request struct {
	Story storyblok.Story
}

type RPCTranslator struct{}

// translator struct implementing Translator interface.
// It is responsible of translating data coming from the redis stream
// and send back translations through another stream.
type translator struct {
	shutdownCh chan struct{}
	rdb        *redis.Client
}

var units map[string]struct{} = map[string]struct{}{
	"gr": struct{}{},
	"kg": struct{}{},
	"ml": struct{}{},
	"lt": struct{}{},
}

// NewTranslator initialize a new Translator and returns it
func NewTranslator(rdb *redis.Client) Translator {
	return &translator{
		shutdownCh: make(chan struct{}),
		rdb:        rdb,
	}
}

// CloseGracefully sends the shutdown signal to start closing all translator processes
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

	tChan := make(chan tChannel)
	defer close(tChan)

	m := tMessage{
		id:          req.Story.UUID,
		story:       req.Story,
		translation: tChan,
	}

	go translateRecipe(ctx, m)

	tm := <-tChan
	reply.Translation = tm.story

	return nil
}

// ReadStreamAndTranslate reads from the incoming stream and sends back the translation through the recipient stream
func (t *translator) ReadStreamAndTranslate(sd StreamData) {
	// create consumer group if not done yet
	t.rdb.XGroupCreate(sd.StreamFrom, sd.Group, "$").Result()

	log.Printf("Consumer group %s created\n", sd.Group)

	lastID := "0-0"
	checkHistory := true

	for {
		if !checkHistory {
			lastID = ">"
		}

		args := &redis.XReadGroupArgs{
			Group:    sd.Group,
			Consumer: sd.Consumer,
			// List of streams and ids.
			Streams: []string{sd.StreamFrom, lastID},
			// Count   int64
			Block: time.Millisecond * 2000,
			// NoAck   bool
		}

		items := t.rdb.XReadGroup(args)
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
		tChan := make(chan tChannel)
		defer close(tChan)

		sbStream := items.Val()[0]
		log.Printf("Consumer %s received %d messages\n", sd.Consumer, len(sbStream.Messages))
		for _, msg := range sbStream.Messages {
			// lastID = msg.ID

			log.Printf("Consumer %s reading message ID %s\n", sd.Consumer, msg.ID)
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
			m := tMessage{
				id:          msg.ID,
				story:       story,
				translation: tChan,
				sourceLang:  sd.LangFrom,
				destLang:    sd.LangTo,
			}

			go translateRecipe(ctx, m)
		}

		for i := 0; i < len(sbStream.Messages); i++ {
			tMsg := <-tChan

			js, err := json.Marshal(tMsg.story)
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
				t.rdb,
				[]string{sd.StreamFrom, sd.StreamTo}, // KEYS
				[]string{sd.Group, tMsg.id, "story", string(js)}, // ARGV
			).Result()

			if err != nil {
				// if an error occurred running the script skip to the next story
				log.Println(err)
				continue
			}
			log.Printf("Translation for message ID %s sent.\n", tMsg.id)
		}
	}
}

// translateRecipe receives a translation message to translate a single recipe.
// It is responsible to group fields homogeneously, send them to be translated
// and collect translations.
func translateRecipe(ctx context.Context, m tMessage) {
	// m.Translation <- TMessage{
	// 	ID:    m.ID,
	// 	Story: m.Story,
	// }
	// return

	td := translationData{
		id: string(m.story.ID),
		fields: map[string]string{
			"Extra":       m.story.Content.Extra,
			"Title":       m.story.Content.Title,
			"Summary":     m.story.Content.Summary,
			"Conclusion":  m.story.Content.Conclusion,
			"Description": m.story.Content.Description,
		},
		sourceLang: m.sourceLang,
		destLang:   m.destLang,
	}

	// copy recipe object and do reflection on the copy
	s := m.story

	resChan := make(chan tResponse)
	stpChan := make(chan tResponse)
	igrChan := make(chan tResponse)
	defer close(resChan)
	defer close(stpChan)
	defer close(igrChan)

	go translateFields(ctx, td, resChan)

	// initialize steps and ingredients maps to group translations
	sfm := make(map[string]map[string]string, len(s.Content.Steps))
	ifm := make(map[string]map[string]string, len(s.Content.Ingredients.Ingredients))

	// launch goroutine for each step
	for _, stp := range s.Content.Steps {
		stpFields := translationData{
			id: stp.UID,
			fields: map[string]string{
				"Title":   stp.Title,
				"Content": stp.Content,
			},
			sourceLang: m.sourceLang,
			destLang:   m.destLang,
		}
		sfm[stp.UID] = map[string]string{
			"Title":   "",
			"Content": "",
		}

		go translateFields(ctx, stpFields, stpChan)
	}

	// launch goroutine for each ingredient
	for i, igr := range s.Content.Ingredients.Ingredients {
		igrFields := translationData{
			id: strconv.Itoa(i),
			fields: map[string]string{
				"Name": igr.Name,
				"Unit": igr.Unit,
			},
			sourceLang: m.sourceLang,
			destLang:   m.destLang,
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
	totFields := len(td.fields) + (len(s.Content.Steps) * 2) + (len(s.Content.Ingredients.Ingredients) * 2)
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
	log.Printf("Translated message ID %s\n", m.id)
	tm := tChannel{
		id:    m.id,
		story: s,
	}
	m.translation <- tm
}

// translateFields receives a block of fields to be translated, filters those that need
// translation and sends each of them to be translated. Once translated it sends translation
// back through the response channel.
func translateFields(ctx context.Context, td translationData, resChan chan (tResponse)) {
	for k, v := range td.fields {
		// numbers, units and empty fields don't need translation
		_, unit := units[v]
		if _, err := strconv.ParseFloat(v, 64); err != nil && v != "" && !unit {
			tReq := tRequest{
				ID:         td.id,
				field:      k,
				sourceText: v,
				sourceLang: td.sourceLang,
				destLang:   td.destLang,
			}

			go func() { resChan <- translateText(ctx, tReq) }()

		} else {
			resChan <- tResponse{
				ID:          td.id,
				field:       k,
				translation: v,
			}
		}
	}
}

// translateText is responsible to call the translation service (Google Cloud)
// asking for the translation of a single field and send back a translation response object.
func translateText(ctx context.Context, tReq tRequest) tResponse {
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
		Source: tReq.sourceLang,
	}

	resp, err := client.Translate(ctx, []string{tReq.sourceText}, tReq.destLang, opts)
	if err != nil {
		log.Fatalf("Translation service error translating [%s]: %s", tReq.sourceText, err)
	}

	return tResponse{
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
