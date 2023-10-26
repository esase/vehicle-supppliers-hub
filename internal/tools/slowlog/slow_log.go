package slowlog

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Logger interface {
	Start(name string)
	Stop(name string) time.Duration
}

type slowLogger struct {
	log           *zerolog.Logger
	ongoingTimers map[string]time.Time
	sync.Mutex
}

func (s *slowLogger) Start(name string) {
	s.Lock()
	s.ongoingTimers[name] = time.Now()
	s.Unlock()
}

func (s *slowLogger) Stop(name string) time.Duration {
	s.Lock()
	defer s.Unlock()

	start := s.ongoingTimers[name]
	duration := time.Since(start)

	s.log.Debug().
		Float64("duration", duration.Seconds()).
		Str("breakpoint_name", name).
		Msg("")

	delete(s.ongoingTimers, name)

	return time.Since(start)
}

func CreateLogger(log *zerolog.Logger) *slowLogger {
	logger := log.With().Str("label", "slowlog").Logger()
	return &slowLogger{
		log:           &logger,
		ongoingTimers: make(map[string]time.Time),
	}
}
