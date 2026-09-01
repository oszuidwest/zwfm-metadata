package core

import (
	"context"
	"html"
	"sync"
)

// PassiveComponent provides a no-op Start method for components without background tasks.
type PassiveComponent struct{}

// Start blocks until context cancellation.
func (p *PassiveComponent) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// InputBase provides the base implementation for metadata input sources.
type InputBase struct {
	name        string
	metadata    *Metadata
	subscribers []chan<- *Metadata
	mu          sync.RWMutex
}

// NewInputBase initializes an InputBase with the given name.
func NewInputBase(name string) *InputBase {
	return &InputBase{
		name:        name,
		subscribers: make([]chan<- *Metadata, 0),
	}
}

// GetName returns the name of this input source.
func (b *InputBase) GetName() string {
	return b.name
}

// GetMetadata returns the current metadata, which may be expired.
func (b *InputBase) GetMetadata() *Metadata {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.metadata != nil {
		return b.metadata.Clone()
	}
	return nil
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

	subscribers := make([]chan<- *Metadata, len(b.subscribers))
	copy(subscribers, b.subscribers)
	b.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- metadata:
		default:
		}
	}
}

// OutputBase provides the base implementation for metadata output destinations.
type OutputBase struct {
	name          string
	delay         int
	fallbackDelay *int
}

// NewOutputBase initializes an OutputBase with the given name.
func NewOutputBase(name string) *OutputBase {
	return &OutputBase{
		name: name,
	}
}

// GetName returns the name of this output destination.
func (b *OutputBase) GetName() string {
	return b.name
}

// SetDelay configures the output delay in seconds.
func (b *OutputBase) SetDelay(delay int) {
	b.delay = delay
}

// GetDelay returns the configured delay in seconds before output delivery.
func (b *OutputBase) GetDelay() int {
	return b.delay
}

// SetFallbackDelay configures how many seconds a fallback waits after the current
// input expires. A nil value means the fallback uses the regular output delay.
func (b *OutputBase) SetFallbackDelay(delay *int) {
	b.fallbackDelay = delay
}

// GetFallbackDelay returns the delay in seconds before a fallback replaces expired
// metadata. A new track arriving within this window cancels the fallback, which
// keeps short gaps between tracks (crossfades, jingles) from flapping to fallback text.
func (b *OutputBase) GetFallbackDelay() int {
	if b.fallbackDelay != nil {
		return *b.fallbackDelay
	}
	return b.delay
}
