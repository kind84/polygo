package storyblok

type SBConsumer interface {
	ReadTranslation(sd StreamData)
}
