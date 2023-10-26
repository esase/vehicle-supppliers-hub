package slowlog

import (
	"bytes"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSLowLog(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should log correctly", func(t *testing.T) {
		tests := []struct {
			name          string
			logic         func(slowLog Logger) []time.Duration
			expectedTimes []time.Duration
		}{
			{
				name: "single slowlog",
				logic: func(slowLog Logger) []time.Duration {
					slowLog.Start("task1")
					time.Sleep(1 * time.Millisecond)
					rounded := slowLog.Stop("task1").Round(time.Millisecond)
					return []time.Duration{rounded}
				},
				expectedTimes: []time.Duration{time.Millisecond},
			},
			{
				name: "multiple slowlogs",
				logic: func(slowLog Logger) []time.Duration {
					slowLog.Start("outer")
					time.Sleep(1 * time.Millisecond)

					slowLog.Start("inner")
					time.Sleep(1 * time.Millisecond)
					inner := slowLog.Stop("inner")

					time.Sleep(1 * time.Millisecond)
					outer := slowLog.Stop("outer")

					inner = inner.Round(time.Millisecond)
					outer = outer.Round(time.Millisecond)

					return []time.Duration{inner, outer}
				},
				expectedTimes: []time.Duration{time.Millisecond, 3 * time.Millisecond},
			},
			{
				name: "multiple same named",
				logic: func(slowLog Logger) []time.Duration {
					slowLog.Start("same")
					time.Sleep(3 * time.Millisecond)
					slowLog.Start("same")
					time.Sleep(1 * time.Millisecond)

					duration := slowLog.Stop("same")
					duration = duration.Round(time.Millisecond)

					return []time.Duration{duration}
				},
				expectedTimes: []time.Duration{1 * time.Millisecond},
			},
		}

		slowLog := CreateLogger(&log)

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				times := test.logic(slowLog)
				assert.Equal(t, 0, len(slowLog.ongoingTimers))
				for i, expectedTime := range test.expectedTimes {
					assert.True(t, times[i] >= expectedTime)
				}

			})
		}
	})
}
