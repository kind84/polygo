package storyblok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"

	"github.com/kind84/polygo/pkg/types"
)

type Reply struct {
	ID      interface{}   `json:"id"`
	Stories []types.Story `json:"stories"`
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
		go func(w *sync.WaitGroup, st types.Story) {
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

			var sty types.Story
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

func (s *StoryBlok) newSBStories() ([]types.Story, error) {
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
		Stories []types.Story `json:"stories"`
	}{}

	err = json.NewDecoder(res.Body).Decode(&ss)
	if err != nil {
		return nil, err
	}
	return ss.Stories, nil
}
