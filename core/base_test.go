package core

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestInputBaseLogsDroppedSubscriberUpdate(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

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
