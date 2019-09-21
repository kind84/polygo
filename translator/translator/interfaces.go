package translator

type Translator interface {
	ReadStreamAndTranslate(streamData StreamData)
	CloseGracefully()
}
