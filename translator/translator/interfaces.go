package translator

import (
	"github.com/go-redis/redis"
)

type Translator interface {
	ReadStoryGroup(rdb *redis.Client)
	CloseGracefully()
}
