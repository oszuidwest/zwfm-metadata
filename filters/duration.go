package filters

import (
	"fmt"

	"zwfm-metadata/config"
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

func init() {
	RegisterFilter("duration", func(cfg *config.FilterConfig) (core.Filter, error) {
		return NewDurationFilter(cfg.MinSeconds)
	})
}

// Type returns the filter type name.
func (d *DurationFilter) Type() string {
	return "duration"
}

// Decide checks if the metadata duration meets the minimum threshold.
func (d *DurationFilter) Decide(st *core.StructuredText) core.FilterResult {
	if st.Original == nil || st.Original.Duration == "" {
		return core.FilterResult{Pass: true} // No duration info, let it through
	}

	seconds, ok := utils.ParseDurationToSeconds(st.Original.Duration)
	if !ok {
		return core.FilterResult{Pass: true} // Unparseable duration, let it through
	}

	if seconds < d.minSeconds {
		// Too short, skip this update
		return core.FilterResult{Pass: false, ClearAll: true}
	}

	return core.FilterResult{Pass: true}
}
