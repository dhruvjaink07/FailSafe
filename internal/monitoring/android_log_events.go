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

		lower := strings.ToLower(line)

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
		case strings.Contains(line, "OkHttp") || strings.Contains(line, "Retrofit") || strings.Contains(line, "Dio") || strings.Contains(line, " --> GET ") || strings.Contains(line, " --> POST ") || strings.Contains(line, "HTTP/"):
			eventType = "request"
		case strings.Contains(lower, "timeoutexception") || strings.Contains(lower, "unknownhostexception") || strings.Contains(lower, "connectexception") || strings.Contains(lower, "sslhandshakeexception") || strings.Contains(lower, "unable to resolve host") || strings.Contains(lower, "failed to connect") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "http/1.1 5") || strings.Contains(lower, "http 5"):
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
