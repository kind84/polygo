package types

import (
	"encoding/json"
	"reflect"
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
