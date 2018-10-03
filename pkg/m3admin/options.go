package m3admin

import (
	retryhttp "github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

// Option configures an m3admin client.
type Option interface {
	execute(*options)
}

type optionFn func(o *options)

func (f optionFn) execute(o *options) {
	f(o)
}

type options struct {
	logger *zap.Logger
	client *retryhttp.Client
}

// WithLogger configures a logger for the client. If not set a noop logger will
// be used.
func WithLogger(l *zap.Logger) Option {
	return optionFn(func(o *options) {
		o.logger = l
	})
}

// WithHTTPClient configures an http client for the m3admin client. If not set,
// go-retryablehttp's default client will be used. Note that the retry client's
// logger will be overriden with the stdlog-wrapped wrapped zap logger.
func WithHTTPClient(cl *retryhttp.Client) Option {
	return optionFn(func(o *options) {
		o.client = cl
	})
}
