package client

import (
	"fmt"
	"os"
	"time"
)

const DefaultTimeout = 3 * time.Second

type OptionFunc func(o *Options)

type Options struct {
	// Name of the caller service, used for logging
	name string

	// Defaults to CRG_SERVICE_DOMAIN
	serviceDomain string

	// ServiceHost - defaults to user-service
	serviceHost string

	// pathPrefix - added before openapi provided path
	pathPrefix string

	// UseHttps - defaults to false, used only with CRG_SERVICE_DOMAIN
	useHTTPS bool

	// BaseURL - full URL to the service (including protocol) - overrides ServiceDomain and ServiceHost
	baseURL string

	// Timeout - if not set, then default timeout is used
	timeout time.Duration
}

func WithServiceDomain(serviceDomain string) OptionFunc {
	return func(o *Options) {
		o.serviceDomain = serviceDomain
	}
}

func WithServiceHost(serviceHost string) OptionFunc {
	return func(o *Options) {
		o.serviceHost = serviceHost
	}
}

func WithPathPrefix(pathPrefix string) OptionFunc {
	return func(o *Options) {
		o.pathPrefix = pathPrefix
	}
}

func WithUseHTTPS(useHTTPS bool) OptionFunc {
	return func(o *Options) {
		o.useHTTPS = useHTTPS
	}
}

func WithBaseURL(baseURL string) OptionFunc {
	return func(o *Options) {
		o.baseURL = baseURL
	}
}

func WithTimeout(timeout time.Duration) OptionFunc {
	return func(o *Options) {
		o.timeout = timeout
	}
}

func NewOptions(optionFuncs ...OptionFunc) (*Options, error) {
	options := &Options{
		name: "supplier-hub",
	}

	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	return options, nil
}

func (o *Options) Name() string {
	return o.name
}

func (o *Options) BaseURL(subDomain string, pathPrefix string) string {
	if o.baseURL != "" {
		return o.baseURL
	}

	var serviceDomain string
	if o.serviceDomain == "" {
		serviceDomain = os.Getenv("CRG_SERVICE_DOMAIN")
	}

	var serviceHost string
	if o.serviceHost == "" {
		serviceHost = subDomain
	}

	var prefix string
	if o.pathPrefix == "" {
		prefix = pathPrefix
	}

	protocol := "http"
	if o.useHTTPS {
		protocol = "https"
	}

	return fmt.Sprintf("%s://%s.%s%s", protocol, serviceHost, serviceDomain, prefix)
}

func (o *Options) Timeout() time.Duration {
	if o.timeout != 0 {
		return o.timeout
	}
	return DefaultTimeout
}
