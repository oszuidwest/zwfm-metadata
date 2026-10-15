package core

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestInputBaseLogsDroppedSubscriberUpdate(t *testing.T) {
	var logs bytes.Buffer
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))

	input := NewInputBase("input")
	updates := make(chan *Metadata, 1)
	input.Subscribe(updates)
	input.SetMetadata(&Metadata{Title: "first"})
	input.SetMetadata(&Metadata{Title: "dropped"})

	logOutput := logs.String()
	if !strings.Contains(logOutput, "Subscriber channel full, dropping metadata update") {
		t.Fatalf("log output = %q, want subscriber-full warning", logOutput)
	}
	if !strings.Contains(logOutput, "input=input") {
		t.Fatalf("log output = %q, want input name", logOutput)
	}
}

func TestInputBaseNotifiesWhenSameContentBecomesAvailableAgain(t *testing.T) {
	input := NewInputBase("input")
	updates := make(chan *Metadata, 1)
	input.Subscribe(updates)
	notified := func() bool {
		select {
		case <-updates:
			return true
		default:
			return false
		}
	}

	now := time.Now()

	input.SetMetadata(&Metadata{Title: "Song", UpdatedAt: now, ExpiresAt: new(now.Add(-time.Second))})
	if !notified() {
		t.Fatal("initial metadata did not notify subscribers")
	}

	input.SetMetadata(&Metadata{Title: "Song", UpdatedAt: now, ExpiresAt: new(now.Add(time.Minute))})
	if !notified() {
		t.Fatal("republishing expired content did not notify subscribers")
	}

	// A poller refresh of available content moves only the timestamps; it must
	// stay silent but still be stored.
	refreshed := &Metadata{Title: "Song", UpdatedAt: now.Add(time.Second), ExpiresAt: new(now.Add(2 * time.Minute))}
	input.SetMetadata(refreshed)
	if notified() {
		t.Fatal("timestamp-only refresh notified subscribers")
	}
	if input.GetMetadata() != refreshed {
		t.Fatal("timestamp-only refresh was not stored")
	}
}
