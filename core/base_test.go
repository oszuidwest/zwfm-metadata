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
	updates := make(chan *Metadata, 3)
	input.Subscribe(updates)

	input.SetMetadata(&Metadata{Title: "Song", ExpiresAt: new(time.Now().Add(-time.Second))})
	<-updates

	input.SetMetadata(&Metadata{Title: "Song", ExpiresAt: new(time.Now().Add(time.Minute))})
	select {
	case <-updates:
	default:
		t.Fatal("republishing expired content did not notify subscribers")
	}

	// Moving the expiry of available content must stay silent.
	input.SetMetadata(&Metadata{Title: "Song", ExpiresAt: new(time.Now().Add(2 * time.Minute))})
	select {
	case metadata := <-updates:
		t.Fatalf("expiry-only refresh notified subscribers: %#v", metadata)
	default:
	}
	if got := input.GetMetadata().ExpiresAt; got == nil || time.Until(*got) < time.Minute {
		t.Fatalf("stored expiry = %v, want the refreshed deadline", got)
	}
}
