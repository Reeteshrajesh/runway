package logger

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Format controls the output format for the webhook listener's event log.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// EventLogger emits structured deploy lifecycle events.
// In text mode it writes plain human-readable lines to stderr.
// In JSON mode it emits newline-delimited JSON objects.
type EventLogger struct {
	w      io.Writer
	format Format
}

// NewEventLogger creates an EventLogger writing to w with the given format.
func NewEventLogger(w io.Writer, format Format) *EventLogger {
	return &EventLogger{w: w, format: format}
}

// DefaultEventLogger returns an EventLogger backed by os.Stderr in text format.
func DefaultEventLogger() *EventLogger {
	return NewEventLogger(os.Stderr, FormatText)
}

// jsonEvent is the structured representation emitted in JSON mode.
type jsonEvent struct {
	Time      string  `json:"time"`
	Level     string  `json:"level"`
	Event     string  `json:"event"`
	Commit    string  `json:"commit,omitempty"`
	Triggered string  `json:"triggered,omitempty"`
	DurationS float64 `json:"duration_s,omitempty"`
	Error     string  `json:"error,omitempty"`
}

func (l *EventLogger) emit(level, event, commit, triggered string, durationS float64, errMsg string) {
	now := time.Now().UTC()

	if l.format == FormatJSON {
		e := jsonEvent{
			Time:      now.Format(time.RFC3339),
			Level:     level,
			Event:     event,
			Commit:    commit,
			Triggered: triggered,
			DurationS: durationS,
			Error:     errMsg,
		}
		b, _ := json.Marshal(e)
		b = append(b, '\n')
		_, _ = l.w.Write(b)
		return
	}

	// Plain text format.
	ts := now.Format(time.RFC3339)
	msg := "[" + ts + "] " + event
	if commit != "" {
		msg += " commit=" + commit
	}
	if triggered != "" {
		msg += " triggered=" + triggered
	}
	if durationS > 0 {
		msg += " duration=" + formatDuration(durationS) + "s"
	}
	if errMsg != "" {
		msg += " error=" + errMsg
	}
	_, _ = io.WriteString(l.w, msg+"\n")
}

// DeployStart logs the start of a deployment.
func (l *EventLogger) DeployStart(commit, triggered string) {
	l.emit("info", "deploy_start", commit, triggered, 0, "")
}

// DeploySuccess logs a successful deployment.
func (l *EventLogger) DeploySuccess(commit, triggered string, durationS float64) {
	l.emit("info", "deploy_success", commit, triggered, durationS, "")
}

// DeployFailed logs a failed deployment.
func (l *EventLogger) DeployFailed(commit, triggered string, durationS float64, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	l.emit("error", "deploy_failed", commit, triggered, durationS, errMsg)
}

// DeployRolledBack logs an auto-rollback event.
func (l *EventLogger) DeployRolledBack(commit, rolledBackTo, triggered string, durationS float64) {
	l.emit("warn", "deploy_rolled_back", commit, triggered, durationS, "rolled back to "+rolledBackTo)
}

func formatDuration(s float64) string {
	// Simple fixed-precision formatting without fmt to keep allocations low.
	// e.g. 42.3 → "42.3"
	i := int(s)
	frac := int((s-float64(i))*10) % 10
	return itoa(i) + "." + itoa(frac)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
