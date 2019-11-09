package storyblok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

type Story struct {
	Name             string        `json:"name"`
	CreatedAt        time.Time     `json:"created_at"`
	PublishedAt      time.Time     `json:"published_at"`
	Alternates       []interface{} `json:"alternates"`
	ID               int           `json:"id"`
	UUID             string        `json:"uuid"`
	Content          Recipe        `json:"content"`
	Slug             string        `json:"slug"`
	FullSlug         string        `json:"full_slug"`
	SortByDate       interface{}   `json:"sort_by_date"`
	Position         int           `json:"position"`
	TagList          []string      `json:"tag_list"`
	IsStartpage      bool          `json:"is_startpage"`
	ParentID         int           `json:"parent_id"`
	MetaData         interface{}   `json:"meta_data"`
	GroupID          string        `json:"group_id"`
	FirstPublishedAt time.Time     `json:"first_published_at"`
	ReleaseID        interface{}   `json:"release_id"`
	Lang             string        `json:"lang"`
	Path             interface{}   `json:"path"`
}

type Recipe struct {
	UID         string      `json:"_uid"`
	Cost        string      `json:"cost"`
	Prep        string      `json:"prep"`
	Extra       string      `json:"extra"`
	Image       string      `json:"image"`
	Likes       interface{} `json:"likes"`
	Steps       []Step      `json:"steps"`
	Title       string      `json:"title"`
	Cooking     string      `json:"cooking"`
	Summary     string      `json:"summary"`
	Servings    string      `json:"servings"`
	Component   string      `json:"component"`
	Conclusion  string      `json:"conclusion"`
	Difficulty  string      `json:"difficulty"`
	Description string      `json:"description"`
	Translated  bool        `json:"translated"`
	Ingredients struct {
		UID         string       `json:"_uid"`
		Plugin      string       `json:"plugin"`
		Ingredients []Ingredient `json:"ingredients"`
	} `json:"ingredients"`
	Lang string `json:"-"`
}

type Ingredient struct {
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
	Lang     string `json:"-"`
}

type Step struct {
	UID       string `json:"_uid"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Component string `json:"component"`
	Thumbnail string `json:"thumbnail"`
	Lang      string `json:"-"`
}

type Reply struct {
	ID      interface{} `json:"id"`
	Stories []Story     `json:"stories"`
}

type Request struct {
	Message string `json:"message"`
}

type StreamData struct {
	Stream   string
	Group    string
	Consumer string
	Code     string
}

type StoryBlok struct {
	token string
	rdb   *redis.Client
}

type sbConsumer struct {
	rdb *redis.Client
}

func NewSBClient(token string, r *redis.Client) *StoryBlok {
	return &StoryBlok{
		token: token,
		rdb:   r,
	}
}

func NewSBConsumer(r *redis.Client) SBConsumer {
	return &sbConsumer{
		rdb: r,
	}
}

func (s *StoryBlok) NewStories(req *Request, reply *Reply) error {
	// get new stories from Storyblok api
	ss, err := s.newSBStories()
	if err != nil {
		return err
	}
	reply.Stories = ss

	// create a pipeline to add messages to the stream in a single transaction
	// TODO transform in a transaction instead of pipe?
	pipe := s.rdb.Pipeline()
	defer pipe.Close()

	var wg sync.WaitGroup
	wg.Add(len(ss))

	for _, story := range ss {
		go func(w *sync.WaitGroup, st Story) {
			js, err := json.Marshal(st)
			if err != nil {
				log.Fatalln(errors.WithStack(err))
			}

			msg := map[string]interface{}{"story": js}

			args := &redis.XAddArgs{
				Stream: "storyblok",
				// MaxLen       int64 // MAXLEN N
				// MaxLenApprox int64 // MAXLEN ~ N
				// ID           string
				Values: msg,
			}

			// add message to the pipeline
			id, err := pipe.XAdd(args).Result()
			if err != nil {
				log.Fatalln(errors.WithStack(err))
			}

			log.Printf("Sending message ID %s for story ID %d", id, st.ID)

			w.Done()
		}(&wg, story)
	}

	wg.Wait()
	log.Println("Wait group done")

	// commit the transaction to the stream
	_, err = pipe.Exec()
	if err != nil {
		log.Fatalln(errors.WithStack(err))
	}

	return nil
}

func (s *sbConsumer) ReadTranslation(sd StreamData) {
	// create consumer group if not done yet
	s.rdb.XGroupCreate(sd.Stream, sd.Group, "$")

	fmt.Printf("Consumer group %s created\n", sd.Group)

	lastID := "0-0"
	checkHistory := true

	// listen for translations coming from the stream
	for {
		if !checkHistory {
			lastID = ">"
		}

		args := redis.XReadGroupArgs{
			Group:    sd.Group,
			Consumer: sd.Consumer,
			// List of streams and ids.
			Streams: []string{sd.Stream, lastID},
			Count:   10,
			Block:   time.Millisecond * 2000,
			// NoAck   bool
		}

		items := s.rdb.XReadGroup(&args)
		if items == nil {
			// Timeout
			continue
		}

		if len(items.Val()) == 0 || len(items.Val()[0].Messages) == 0 {
			checkHistory = false
			continue
		}

		tStream := items.Val()[0]
		log.Printf("Consumer %s received %d messages\n", sd.Consumer, len(tStream.Messages))
		for _, msg := range tStream.Messages {
			log.Printf("Consumer %s reading message ID %s\n", sd.Consumer, msg.ID)
			lastID = msg.ID

			// TODO add here a check to ensure translation has been persisted.

			var sty Story
			jsn := msg.Values["story"].(string)
			err := json.Unmarshal([]byte(jsn), &sty)
			if err != nil {
				log.Println(errors.WithStack(err))
				continue
			}
			sty.Content.Lang = "_i18n_" + sd.Code
			for i, _ := range sty.Content.Steps {
				sty.Content.Steps[i].Lang = "_i18n_" + sd.Code
			}
			for i, _ := range sty.Content.Ingredients.Ingredients {
				sty.Content.Ingredients.Ingredients[i].Lang = "_i18n_" + sd.Code
			}

			jsty, err := json.Marshal(sty)
			if err != nil {
				log.Println(errors.WithStack(err))
				continue
			}

			var prettyJson bytes.Buffer
			err = json.Indent(&prettyJson, []byte(jsty), "", "\t")
			if err != nil {
				log.Println(errors.WithStack(err))
				continue
			}
			fmt.Println(string(prettyJson.Bytes()))

			ackScript := redis.NewScript(`
				return redis.call("xack", KEYS[1], ARGV[1], ARGV[2])
			`)

			_, err = ackScript.Run(
				s.rdb,
				[]string{sd.Stream},        // KEYS
				[]string{sd.Group, msg.ID}, // ARGV
			).Result()

			if err != nil {
				// if an error occurred running the script skip to the next story
				log.Println(err)
				continue
			}
		}
	}
}

func (s *StoryBlok) newSBStories() ([]Story, error) {
	req, err := http.NewRequest("GET", "https://api.storyblok.com/v1/cdn/stories", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("starts_with", "recipes")
	q.Add("filter_query[translated][in]", "true")
	q.Add("token", s.token)
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "applcation/json")
	req.Header.Add("Accept", "applcation/json")

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	ss := struct {
		Stories []Story `json:"stories"`
	}{}

	err = json.NewDecoder(res.Body).Decode(&ss)
	if err != nil {
		return nil, err
	}
	return ss.Stories, nil
}

// define the naming strategy
func (r Recipe) SetJSONname(jsonTag string) string {
	return jsonTag + r.Lang
}

// implement MarshalJSON for type Recipe
func (r Recipe) MarshalJSON() ([]byte, error) {

	// specify the naming strategy here
	return marshalJSON("SetJSONname", r)
}

// define the naming strategy
func (s Step) SetJSONname(jsonTag string) string {
	return jsonTag + s.Lang
}

// implement MarshalJSON for type Step
func (s Step) MarshalJSON() ([]byte, error) {

	// specify the naming strategy here
	return marshalJSON("SetJSONname", s)
}

// define the naming strategy
func (i Ingredient) SetJSONname(jsonTag string) string {
	return jsonTag + i.Lang
}

// implement MarshalJSON for type Ingredient
func (i Ingredient) MarshalJSON() ([]byte, error) {

	// specify the naming strategy here
	return marshalJSON("SetJSONname", i)
}

// implement a general marshaler that takes a naming strategy
func marshalJSON(namingStrategy string, that interface{}) ([]byte, error) {
	out := map[string]interface{}{}
	t := reflect.TypeOf(that)
	v := reflect.ValueOf(that)

	fnctn := v.MethodByName(namingStrategy)
	fname := func(params ...interface{}) string {
		in := make([]reflect.Value, len(params))
		for k, param := range params {
			in[k] = reflect.ValueOf(param)
		}
		return fnctn.Call(in)[0].String()
	}
	outName := ""
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		switch n := f.Tag.Get("json"); n {
		//set cases
		case "":
			outName = f.Name
		case "-":
			outName = ""
		case "title":
			outName = fname(n)
		case "summary":
			outName = fname(n)
		case "description":
			outName = fname(n)
		case "conclusion":
			outName = fname(n)
		case "name":
			outName = fname(n)
		case "unit":
			outName = fname(n)
		case "content":
			outName = fname(n)
		default:
			outName = f.Name
		}
		if outName != "" {
			out[outName] = v.Field(i).Interface()
		}
	}
	return json.Marshal(out)
}
