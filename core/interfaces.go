package core

import (
	"context"
	"time"
)

// Metadata represents the metadata for a song
type Metadata struct {
	Name      string
	SongID    string
	Artist    string
	Title     string
	Duration  string
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

// Input interface for all input types
type Input interface {
	// Start begins processing the input
	Start(ctx context.Context) error
	// GetName returns the name of the input
	GetName() string
	// GetMetadata returns the current metadata
	GetMetadata() *Metadata
	// Subscribe allows manager to subscribe to updates
	Subscribe(ch chan<- *Metadata)
	// Unsubscribe removes a subscription
	Unsubscribe(ch chan<- *Metadata)
}

// Output interface for all output types
type Output interface {
	// Start begins processing the output
	Start(ctx context.Context) error
	// GetName returns the name of the output
	GetName() string
	// GetDelay returns the delay in seconds for this output
	GetDelay() int
	// SetInputs sets the prioritized list of inputs
	SetInputs(inputs []Input)
	// ProcessFormattedMetadata processes pre-formatted metadata string (async safe)
	ProcessFormattedMetadata(formattedText string)
}

// EnhancedOutput interface for outputs that need access to full metadata
type EnhancedOutput interface {
	Output
	// ProcessFormattedMetadataWithDetails processes metadata with full details
	ProcessFormattedMetadataWithDetails(formattedText string, metadata *Metadata)
}
