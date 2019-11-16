package storyblok

import (
	"context"
)

type SBConsumer interface {
	ReadTranslation(context.Context, StreamData)
	CloseGracefully()
}
