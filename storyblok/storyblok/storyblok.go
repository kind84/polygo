package storyblok

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis"
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
}

type Ingredient struct {
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
}

type Step struct {
	UID       string `json:"_uid"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Component string `json:"component"`
	Thumbnail string `json:"thumbnail"`
}

type Reply struct {
	ID      interface{} `json:"id"`
	Stories []Story     `json:"stories"`
}

type Request struct {
	Message string `json:"message"`
}

type StoryBlok struct {
	token string
	rdb   *redis.Client
}

func NewSBClient(token string, r *redis.Client) *StoryBlok {
	return &StoryBlok{
		token: token,
		rdb:   r,
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
	pipe := s.rdb.Pipeline()
	defer pipe.Close()

	var wg sync.WaitGroup
	wg.Add(len(ss))

	for _, story := range ss {
		go func(w *sync.WaitGroup, st Story) {
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
