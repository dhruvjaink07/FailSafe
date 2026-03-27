package monitoring

import (
	"strings"
	"time"
)

type LogEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Thread    string    `json:"thread"`
	Message   string    `json:"message"`
}

func parseLogcatEvents(raw string, now time.Time) []LogEvent {
	lines := strings.Split(raw, "\n")
	events := make([]LogEvent, 0, 4)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		eventType := ""
		thread := ""
		msg := line

		switch {
		case strings.Contains(line, "FATAL EXCEPTION"):
			eventType = "crash"
			parts := strings.Split(line, "FATAL EXCEPTION:")
			if len(parts) > 1 {
				thread = strings.TrimSpace(parts[1])
			}
		case strings.Contains(line, "ANR in"):
			eventType = "anr"
		case strings.Contains(line, "TimeoutException"):
			eventType = "warning"
		}

		if eventType == "" {
			continue
		}

		events = append(events, LogEvent{
			Type:      eventType,
			Timestamp: now,
			Thread:    thread,
			Message:   msg,
		})
	}

	return events
}
