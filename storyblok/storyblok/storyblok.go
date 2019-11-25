package storyblok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis"

	"github.com/kind84/polygo/pkg/types"
)

type StreamData struct {
	Stream   string
	Group    string
	Consumer string
	Code     string
}

type StoryBlok struct {
	token string
	oauth string
	space string
	rdb   *redis.Client
}

type sbConsumer struct {
	*StoryBlok
	shutdownCh chan struct{}
}

func NewSBClient(token string, oauth string, space string, r *redis.Client) *StoryBlok {
	return &StoryBlok{
		token: token,
		oauth: oauth,
		space: space,
		rdb:   r,
	}
}

func NewSBConsumer(s *StoryBlok) *sbConsumer {
	return &sbConsumer{
		StoryBlok:  s,
		shutdownCh: make(chan struct{}),
	}
}

// CloseGracefully sends the shutdown signal to start closing all translator processes
func (s *sbConsumer) CloseGracefully() {
	close(s.shutdownCh)
}

func (s *sbConsumer) shouldExit() bool {
	select {
	case _, ok := <-s.shutdownCh:
		return !ok
	default:
	}
	return false
}

func (s *StoryBlok) NewStories(req *types.Request, reply *types.Reply) error {
	// get new stories from Storyblok api
	ss, err := s.newStories()
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
				log.Fatalln(err)
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
				log.Fatalln(err)
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
		log.Fatalln(err)
	}

	return nil
}

func (s *sbConsumer) ReadTranslation(ctx context.Context, sd StreamData) {
	// create consumer group if not done yet
	s.rdb.XGroupCreateMkStream(sd.Stream, sd.Group, "$")

	fmt.Printf("Consumer group %s created\n", sd.Group)

	lastID := "0-0"
	checkHistory := true

	// listen for translations coming from the stream
	for {
		// TODO: use context once storing translation to storyblok is implemented.
		_, cancel := context.WithCancel(ctx)

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
			// Timeout, check if it's time to exit
			if s.shouldExit() {
				cancel()
				return
			}
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

			var story types.Story
			jsn := msg.Values["story"].(string)
			err := json.Unmarshal([]byte(jsn), &story)
			if err != nil {
				log.Println(err)
				continue
			}

			// ensure that translation has not been persisted yet.
			saved, err := s.checkTranslation(&story, sd.Code)
			if err != nil {
				log.Println(err)
				continue
			}

			if !saved {
				log.Println("saving translation")
				err = s.prepareStory(&story, sd.Code)
				if err != nil {
					log.Println(err)
					continue
				}
				err = s.saveStory(&story)
				if err != nil {
					log.Println(err)
					continue
				}
			}

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

func (s *StoryBlok) newStories() ([]types.Story, error) {
	req, err := http.NewRequest("GET", "https://api.storyblok.com/v1/cdn/stories", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("starts_with", "recipes")
	q.Add("filter_query[translated][in]", "false")
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

func (s *StoryBlok) checkTranslation(story *types.Story, code string) (bool, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.storyblok.com/v1/cdn/stories/%d", story.ID), nil)
	if err != nil {
		return false, err
	}

	q := req.URL.Query()
	q.Add("token", s.token)
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "applcation/json")
	req.Header.Add("Accept", "applcation/json")

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	ss := struct {
		Story types.Story `json:"story"`
	}{}

	err = json.NewDecoder(res.Body).Decode(&ss)
	if err != nil {
		return false, err
	}

	for _, lang := range ss.Story.Content.Translations {
		if lang == code {
			return true, nil
		}
	}
	return false, nil
}

func (s *StoryBlok) prepareStory(story *types.Story, code string) error {
	lang := "__i18n__" + code
	story.Content.Lang = lang
	for i, _ := range story.Content.Steps {
		story.Content.Steps[i].Lang = lang
	}
	for i, _ := range story.Content.Ingredients.Ingredients {
		story.Content.Ingredients.Ingredients[i].Lang = lang
	}
	story.Content.Translations = append(story.Content.Translations, code)

	// TODO: improve this check: Translations must contain all translated language codes.
	if len(story.Content.Translations) == 2 {
		log.Println("All translations done.")
		story.Content.Translated = true
	}

	jsty, err := json.Marshal(story)
	if err != nil {
		return err
	}

	var prettyJson bytes.Buffer
	err = json.Indent(&prettyJson, []byte(jsty), "", "\t")
	if err != nil {
		return err
	}
	fmt.Println(string(prettyJson.Bytes()))

	return nil
}

func (s *StoryBlok) saveStory(story *types.Story) error {
	body := struct {
		Story   types.Story `json:"story"`
		publish int
	}{
		Story:   *story,
		publish: 1,
	}

	jbody, err := json.Marshal(body)

	req, err := http.NewRequest("POST", fmt.Sprintf("https://mapi.storyblok.com/v1/spaces/%s/stories", s.space), bytes.NewBuffer(jbody))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "applcation/json")
	req.Header.Add("Authorization", s.oauth)

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	fmt.Println(res)
	return nil
}
