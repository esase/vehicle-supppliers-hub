package factory

import (
	"fmt"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/profitmaxdht"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/rently"
	"bitbucket.org/crgw/supplier-hub/internal/tools/redisfactory"
)

type Factory struct {
	redisFactory *redisfactory.Factory
	platforms    map[string]any
}

func (f *Factory) GetPlatform(name string) (any, error) {
	_, ok := f.platforms[name]

	if !ok {
		switch name {

		// Register all platforms here
		case "hertz":
			f.platforms[name] = hertz.New(f.redisFactory.ResponsesCacheClient())
		case "profitmaxdht":
			f.platforms[name] = profitmaxdht.New(f.redisFactory.ResponsesCacheClient())
		case "bookingcom":
			f.platforms[name] = bookingcom.New(f.redisFactory.ResponsesCacheClient())
		case "anyrent":
			f.platforms[name] = anyrent.New(f.redisFactory.ResponsesCacheClient())
		case "rently":
			f.platforms[name] = rently.New(f.redisFactory.ResponsesCacheClient())
		default:
			return nil, fmt.Errorf("platform %s not found", name)
		}
	}

	return f.platforms[name], nil
}

func NewFactory(redisFactory *redisfactory.Factory) *Factory {
	return &Factory{
		redisFactory: redisFactory,
		platforms:    make(map[string]any),
	}
}
