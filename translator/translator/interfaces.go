package translator

import (
	"github.com/go-redis/redis"
)

type Translator interface {
	ReadStoryGroup(rdb *redis.Client, streamFrom, group, consumer, streamTo string)
	CloseGracefully()
}
