package translator

import (
	"github.com/go-redis/redis"
)

type Translator interface {
	ReadStreamAndTranslate(rdb *redis.Client, streamData StreamData)
	CloseGracefully()
}
