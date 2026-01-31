package filters

import (
	"fmt"

	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// DurationFilter skips metadata updates where the duration is below a minimum threshold.
type DurationFilter struct {
	minSeconds int
}

// NewDurationFilter creates a filter that skips tracks shorter than minSeconds.
func NewDurationFilter(minSeconds int) (*DurationFilter, error) {
	if minSeconds < 0 {
		return nil, fmt.Errorf("minSeconds must be >= 0, got %d", minSeconds)
	}

	return &DurationFilter{
		minSeconds: minSeconds,
	}, nil
}

// Filter checks if the metadata duration meets the minimum threshold.
// Returns false to skip updates with duration below the threshold.
// If duration is missing or unparseable, the update passes through.
func (d *DurationFilter) Filter(st *core.StructuredText) bool {
	if st.Original == nil || st.Original.Duration == "" {
		return true // No duration info, let it through
	}

	seconds, ok := utils.ParseDurationToSeconds(st.Original.Duration)
	if !ok {
		return true // Unparseable duration, let it through
	}

	if seconds < d.minSeconds {
		// Too short, skip this update
		st.Artist = ""
		st.Title = ""
		st.Prefix = ""
		st.Suffix = ""
		return false
	}

	return true
}
