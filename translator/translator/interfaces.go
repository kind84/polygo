package translator

import (
	"context"
)

type Translator interface {
	ReadStreamAndTranslate(context.Context, StreamData)
	CloseGracefully()
}
