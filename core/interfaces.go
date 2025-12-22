// Package core provides the fundamental interfaces and types for the metadata
// router, including input and output abstractions and metadata structures.
package core

import (
	"context"
	"net/http"
	"time"
)

// Metadata holds song information including artist, title, duration, and expiration.
type Metadata struct {
	Name      string
	SongID    string
	Artist    string
	Title     string
	Duration  string
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

// Input defines metadata source behavior for the router.
type Input interface {
	Start(ctx context.Context) error
	GetName() string
	GetMetadata() *Metadata
	Subscribe(ch chan<- *Metadata)
	Unsubscribe(ch chan<- *Metadata)
}

// Output defines metadata destination behavior for the router.
type Output interface {
	Start(ctx context.Context) error
	GetName() string
	GetDelay() int
	SetInputs(inputs []Input)
	SendFormattedMetadata(formattedText string)
}

// EnhancedOutput extends Output for destinations needing full metadata details.
type EnhancedOutput interface {
	Output
	SendEnhancedMetadata(formattedText string, metadata *Metadata, inputName, inputType string)
}

// RouteRegistrar extends Output for destinations exposing HTTP endpoints.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}
