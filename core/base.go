package core

import (
	"context"
	"html"
	"slices"
	"sync"
)

// PassiveComponent provides a Start() method for passive components.
//
// In this system, inputs provide metadata and can be "available" or "unavailable".
// Some inputs (like URLInput) need background tasks to fetch data and maintain availability.
// Others are passive:
//   - TextInput: Always available with static metadata
//   - DynamicInput: Available when it receives HTTP updates
//   - All outputs: Just wait to process metadata from the MetadataRouter
//
// Passive components don't need to do anything in Start() except wait for shutdown.
// They embed PassiveComponent to get this behavior.
type PassiveComponent struct{}

// Start waits for context cancellation (shutdown signal).
func (p *PassiveComponent) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// InputBase provides common fields and methods for all input types.
type InputBase struct {
	name        string
	metadata    *Metadata
	subscribers []chan<- *Metadata
	mu          sync.RWMutex
}

// NewInputBase creates a new InputBase.
func NewInputBase(name string) *InputBase {
	return &InputBase{
		name:        name,
		subscribers: make([]chan<- *Metadata, 0),
	}
}

// GetName returns the input name.
func (b *InputBase) GetName() string {
	return b.name
}

// GetMetadata returns the current metadata (including expired metadata).
func (b *InputBase) GetMetadata() *Metadata {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.metadata != nil {
		return b.metadata.Clone()
	}
	return nil
}

// Subscribe adds a channel to receive metadata updates.
func (b *InputBase) Subscribe(ch chan<- *Metadata) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = append(b.subscribers, ch)
}

// Unsubscribe removes a channel from receiving metadata updates.
func (b *InputBase) Unsubscribe(ch chan<- *Metadata) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers = slices.DeleteFunc(b.subscribers, func(sub chan<- *Metadata) bool {
		return sub == ch
	})
}

// SetMetadata updates the metadata and notifies subscribers only when content changes.
func (b *InputBase) SetMetadata(metadata *Metadata) {
	if metadata != nil {
		metadata.Title = html.UnescapeString(metadata.Title)
		metadata.Artist = html.UnescapeString(metadata.Artist)
	}

	b.mu.Lock()

	hasChanged := false
	if b.metadata == nil && metadata != nil {
		hasChanged = true
	} else if b.metadata != nil && metadata == nil {
		hasChanged = true
	} else if b.metadata != nil && metadata != nil {
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

// ChangeDetector handles change detection for outputs.
type ChangeDetector struct {
	lastValue string
	mu        sync.RWMutex
}

// HasChanged reports whether the value has changed and updates the stored value.
func (c *ChangeDetector) HasChanged(newValue string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newValue != c.lastValue {
		c.lastValue = newValue
		return true
	}
	return false
}

// GetCurrentValue returns the current stored value.
func (c *ChangeDetector) GetCurrentValue() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastValue
}

// SetCurrentValue sets the current stored value.
func (c *ChangeDetector) SetCurrentValue(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastValue = value
}

// OutputBase provides common fields for all output types.
type OutputBase struct {
	name           string
	inputs         []Input
	changeDetector ChangeDetector
	delay          int
}

// NewOutputBase creates a new OutputBase.
func NewOutputBase(name string) *OutputBase {
	return &OutputBase{
		name: name,
	}
}

// GetName returns the output name.
func (b *OutputBase) GetName() string {
	return b.name
}

// SetInputs sets the inputs for this output.
func (b *OutputBase) SetInputs(inputs []Input) {
	b.inputs = inputs
}

// HasChanged reports whether the value has changed.
func (b *OutputBase) HasChanged(newValue string) bool {
	return b.changeDetector.HasChanged(newValue)
}

// SetDelay sets the delay for this output.
func (b *OutputBase) SetDelay(delay int) {
	b.delay = delay
}

// GetDelay returns the delay for this output.
func (b *OutputBase) GetDelay() int {
	return b.delay
}
