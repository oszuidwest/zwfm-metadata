package core

import (
	"context"
	"html"
	"log/slog"
	"slices"
	"sync"
)

// PassiveComponent provides a no-op Start method for components without background tasks.
type PassiveComponent struct{}

// Start returns immediately; passive components have nothing to run.
func (p *PassiveComponent) Start(_ context.Context) error {
	return nil
}

// InputBase provides the base implementation for metadata input sources.
type InputBase struct {
	name        string
	metadata    *Metadata
	subscribers []chan<- *Metadata
	mu          sync.RWMutex
}

// NewInputBase returns an input base with the given name.
func NewInputBase(name string) *InputBase {
	return &InputBase{name: name}
}

// GetName returns the name of this input source.
func (b *InputBase) GetName() string {
	return b.name
}

// GetMetadata returns the current metadata, which may be expired. Metadata is
// immutable once published, so callers share the stored value.
func (b *InputBase) GetMetadata() *Metadata {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.metadata
}

// Subscribe registers a channel to receive metadata change notifications.
func (b *InputBase) Subscribe(ch chan<- *Metadata) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = append(b.subscribers, ch)
}

// SetMetadata stores new metadata and notifies subscribers if content changed.
func (b *InputBase) SetMetadata(metadata *Metadata) {
	if metadata != nil {
		metadata.Title = html.UnescapeString(metadata.Title)
		metadata.Artist = html.UnescapeString(metadata.Artist)
	}

	b.mu.Lock()

	hasChanged := (b.metadata == nil) != (metadata == nil)
	if b.metadata != nil && metadata != nil {
		hasChanged = b.metadata.Title != metadata.Title ||
			b.metadata.Artist != metadata.Artist ||
			b.metadata.SongID != metadata.SongID ||
			b.metadata.Duration != metadata.Duration
	}

	b.metadata = metadata

	if !hasChanged {
		b.mu.Unlock()
		return
	}

	subscribers := slices.Clone(b.subscribers)
	b.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- metadata:
		default:
			slog.Warn("Subscriber channel full, dropping metadata update", "input", b.name)
		}
	}
}

// OutputBase provides the base implementation for metadata output destinations.
type OutputBase struct {
	name string
}

// NewOutputBase returns an output base with the given name.
func NewOutputBase(name string) *OutputBase {
	return &OutputBase{name: name}
}

// GetName returns the name of this output destination.
func (b *OutputBase) GetName() string {
	return b.name
}
