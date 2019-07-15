package translator

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

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

// func (e *Element) UnmarshalJSON(bs []byte) error {
// 	arr := []interface{}{}
// 	json.Unmarshal(bs, &arr)
// 	e.Slice = arr[0].([]interface{})
// 	e.Nil = arr[1].(struct{ Pippo string })
// 	e.SourceLang = arr[2].(string)
// 	return nil
// }

func Translate(text string) (string, error) {
	req, err := http.NewRequest("GET", "https://translate.googleapis.com/translate_a/single", nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("client", "gtx")
	q.Add("sl", "it")
	q.Add("tl", "en")
	q.Add("dt", "t")
	q.Add("q", text)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}

	r, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	var resp []interface{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		return "", err
	}

	return resp[0].([]interface{})[0].([]interface{})[0].(string), nil
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
