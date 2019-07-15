package storyblok

import (
	"encoding/json"
	"net/http"
	"time"
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
	UID         string `json:"_uid"`
	Cost        string `json:"cost"`
	Prep        string `json:"prep"`
	Extra       string `json:"extra"`
	Image       string `json:"image"`
	Likes       string `json:"likes"`
	Steps       []Step `json:"steps"`
	Title       string `json:"title"`
	Cooking     string `json:"cooking"`
	Summary     string `json:"summary"`
	Servings    string `json:"servings"`
	Component   string `json:"component"`
	Conclusion  string `json:"conclusion"`
	Difficulty  string `json:"difficulty"`
	Description string `json:"description"`
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

func NewStories() (*Story, error) {
	req, err := http.NewRequest("GET", "https://api.storyblok.com/v1/cdn/stories/recipes/summer-cheesecake", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("token", "BWZ2r0aQR0LCU9PXMtE06Qtt")
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Content-Type", "applcation/json")
	req.Header.Add("Accept", "applcation/json")

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	s := struct {
		Story Story `json:"story"`
	}{}

	err = json.NewDecoder(res.Body).Decode(&s)
	if err != nil {
		return nil, err
	}

	return &s.Story, nil
}
