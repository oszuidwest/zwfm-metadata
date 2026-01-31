// Package core provides the fundamental interfaces and types for the metadata
// router, including input and output abstractions and metadata structures.
package core

import (
	"context"
	"net/http"
	"time"
)

// Metadata carries song information with optional expiration for time-sensitive sources.
type Metadata struct {
	Name      string
	SongID    string
	Artist    string
	Title     string
	Duration  string
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

// Input provides metadata from a source and notifies subscribers of changes.
type Input interface {
	Start(ctx context.Context) error
	GetName() string
	GetMetadata() *Metadata
	Subscribe(ch chan<- *Metadata)
	Unsubscribe(ch chan<- *Metadata)
}

// Output receives formatted metadata and delivers it to a destination.
type Output interface {
	Start(ctx context.Context) error
	GetName() string
	GetDelay() int
	SetInputs(inputs []Input)
	Send(st *StructuredText)
}

// RouteRegistrar allows outputs to register HTTP handlers on the web server.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

// Formatter modifies StructuredText fields before output delivery.
type Formatter interface {
	Format(st *StructuredText)
}

// Filter examines metadata and decides whether it should proceed to outputs.
// Unlike Formatter which transforms text, Filter determines if metadata passes through.
type Filter interface {
	// Filter examines the StructuredText and returns whether processing should continue.
	// It may modify st (e.g., clearing specific fields) regardless of the return value.
	// Returns true to continue processing, false to reject the metadata entirely.
	Filter(st *StructuredText) (pass bool)
}
