package types

type Reply struct {
	ID      interface{} `json:"id"`
	Stories []Story     `json:"stories"`
}

type Request struct {
	Message string `json:"message"`
}
