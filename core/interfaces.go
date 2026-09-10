// Package core defines metadata routing primitives.
package core

import (
	"context"
	"net/http"
	"time"
)

// Metadata carries song information with optional expiration for time-sensitive sources.
// It is immutable once handed to InputBase.SetMetadata.
type Metadata struct {
	SongID    string
	Artist    string
	Title     string
	Duration  string
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

// MetadataRequest holds the fields for an incoming metadata update request via the HTTP API.
// Title is required; Secret is checked when configured for the receiving input.
type MetadataRequest struct {
	SongID   string
	Artist   string
	Title    string
	Duration string
	Secret   string
}

// Input provides metadata from a source and notifies subscribers of changes.
type Input interface {
	// Start runs background work, if any, until ctx is canceled.
	Start(ctx context.Context) error
	GetName() string
	GetMetadata() *Metadata
	Subscribe(ch chan<- *Metadata)
}

// Output receives formatted metadata and delivers it to a destination.
type Output interface {
	// Start runs background work, if any, until ctx is canceled.
	Start(ctx context.Context) error
	GetName() string
	// Send must not mutate st.Original.
	Send(st *StructuredText) error
}

// RouteRegistrar allows outputs to register HTTP handlers on the web server.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

// Formatter modifies StructuredText fields before output delivery.
type Formatter interface {
	// Format may modify st but must not mutate st.Original.
	Format(st *StructuredText)
}

// FilterAction specifies what a filter decides to do with metadata.
type FilterAction int

const (
	// FilterPass allows metadata through unchanged.
	FilterPass FilterAction = iota
	// FilterClearArtist clears only the Artist field, allowing metadata through.
	FilterClearArtist
	// FilterClearTitle clears only the Title field, allowing metadata through.
	FilterClearTitle
	// FilterReject rejects the metadata entirely, clearing all fields.
	FilterReject
)

// Filter accepts, rejects, or partially clears metadata before formatting.
type Filter interface {
	Decide(st *StructuredText) FilterAction
}
