package redisfactory

import (
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// If one connection needs to be broken up new function should be introduced
// example: TrafficlightHertzClient()

// todo: what if one DB has to be broken up?
// todo: clients can to be created on-demand, but got to figure out how to fail fast if URIs are missing

type Factory struct {
	trafficlightCache *redis.Client
	responsesCache    *redis.Client
}

func New() *Factory {
	opt, err := redis.ParseURL(os.Getenv("TRAFFICLIGHT_REDIS_URI"))
	if err != nil {
		panic(err)
	}

	opt.DialTimeout = 4 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	trafficlightCache := redis.NewClient(opt)

	opt, err = redis.ParseURL(os.Getenv("RESPONSES_CACHE_REDIS_URI"))
	if err != nil {
		panic(err)
	}

	opt.DialTimeout = 4 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	responsesCache := redis.NewClient(opt)

	return &Factory{
		trafficlightCache: trafficlightCache,
		responsesCache:    responsesCache,
	}
}

func (f *Factory) TrafficlightClient() *redis.Client {
	return f.trafficlightCache
}

func (f *Factory) ResponsesCacheClient() *redis.Client {
	return f.responsesCache
}
