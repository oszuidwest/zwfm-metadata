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

// Decide checks if the metadata duration meets the minimum threshold.
func (d *DurationFilter) Decide(st *core.StructuredText) core.FilterAction {
	if st.Original == nil || st.Original.Duration == "" {
		return core.FilterPass
	}

	seconds, ok := utils.ParseDurationToSeconds(st.Original.Duration)
	if !ok {
		return core.FilterPass
	}

	if seconds < d.minSeconds {
		return core.FilterReject
	}

	return core.FilterPass
}
