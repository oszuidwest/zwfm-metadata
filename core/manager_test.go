package core

import (
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type mockInput struct {
	*InputBase
	PassiveComponent
}

func newMockInput(name string) *mockInput {
	return &mockInput{InputBase: NewInputBase(name)}
}

type mockOutput struct {
	*OutputBase
	PassiveComponent
	sendChan chan *StructuredText
}

func newMockOutput(name string) *mockOutput {
	return &mockOutput{
		OutputBase: NewOutputBase(name),
		sendChan:   make(chan *StructuredText, 10),
	}
}

func (m *mockOutput) Send(st *StructuredText) {
	select {
	case m.sendChan <- st:
	default:
	}
}

func (m *mockOutput) waitForSend(timeout time.Duration) (*StructuredText, bool) {
	select {
	case st := <-m.sendChan:
		return st, true
	case <-time.After(timeout):
		return nil, false
	}
}

const (
	trackLength     = 3 * time.Minute
	delaySeconds    = 5
	fallbackSeconds = 20
)

// expectSent allows the regular output delay plus the expiration checker's 1s tick,
// so a send that was held back by the fallback delay is reported as missing.
func expectSent(t *testing.T, output *mockOutput, title string) {
	t.Helper()
	st, ok := output.waitForSend((delaySeconds + 1) * time.Second)
	if !ok || st.Title != title {
		t.Fatalf("expected %q to be sent, got %q", title, st.String())
	}
}

func expectNoSend(t *testing.T, output *mockOutput, within time.Duration) {
	t.Helper()
	if st, ok := output.waitForSend(within); ok {
		t.Fatalf("expected nothing within %v, got %q", within, st.String())
	}
}

type mockFilter struct {
	action FilterAction
}

func newMockFilter(action FilterAction) *mockFilter {
	return &mockFilter{action: action}
}

func (f *mockFilter) Decide(_ *StructuredText) FilterAction {
	return f.action
}

type patternFilter struct {
	pattern string
	action  FilterAction
}

func newPatternFilter(pattern string, action FilterAction) *patternFilter {
	return &patternFilter{pattern: pattern, action: action}
}

func (f *patternFilter) Decide(st *StructuredText) FilterAction {
	if f.pattern != "" && strings.Contains(st.Title, f.pattern) {
		return f.action
	}
	return FilterPass
}

type artistDependentFilter struct{}

func (f *artistDependentFilter) Decide(st *StructuredText) FilterAction {
	if st.Artist == "" {
		return FilterClearTitle
	}
	return FilterPass
}

type capturingFilter struct {
	captured *StructuredText
	mu       sync.Mutex
}

func (f *capturingFilter) Decide(st *StructuredText) FilterAction {
	f.mu.Lock()
	f.captured = st.Clone()
	f.mu.Unlock()
	return FilterPass
}

func (f *capturingFilter) getCaptured() *StructuredText {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.captured
}

type contextAwareFilter struct {
	expectedInputName string
	expectedInputType string
	expectedPrefix    string
	expectedSuffix    string
	contextMatched    bool
	mu                sync.Mutex
}

func newContextAwareFilter(inputName, inputType, prefix, suffix string) *contextAwareFilter {
	return &contextAwareFilter{
		expectedInputName: inputName,
		expectedInputType: inputType,
		expectedPrefix:    prefix,
		expectedSuffix:    suffix,
	}
}

func (f *contextAwareFilter) Decide(st *StructuredText) FilterAction {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.contextMatched = st.InputName == f.expectedInputName &&
		st.InputType == f.expectedInputType &&
		st.Prefix == f.expectedPrefix &&
		st.Suffix == f.expectedSuffix

	return FilterPass
}

func (f *contextAwareFilter) wasContextMatched() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.contextMatched
}

func testMetadata(artist, title string) *Metadata {
	return &Metadata{
		Artist:    artist,
		Title:     title,
		UpdatedAt: time.Now(),
	}
}

// startRouter registers the inputs in priority order and starts the router.
// Callers run inside synctest.Test so router timers use fake time.
func startRouter(t *testing.T, router *MetadataRouter, output *mockOutput, inputs ...*mockInput) {
	t.Helper()

	inputNames := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if err := router.AddInput(input); err != nil {
			t.Fatalf("AddInput failed: %v", err)
		}
		inputNames = append(inputNames, input.GetName())
	}

	if err := router.AddOutput(output); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}
	router.SetOutputInputs(output.GetName(), inputNames)

	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
}

func setupTestRouter(t *testing.T, outputDelay int, filters []Filter) (*mockInput, *mockOutput) {
	t.Helper()

	router := NewMetadataRouter()
	router.SetInputFilters("test-input", filters)

	input := newMockInput("test-input")
	output := newMockOutput("test-output")
	router.SetOutputTiming(output.GetName(), OutputTiming{Delay: outputDelay})
	startRouter(t, router, output, input)

	return input, output
}

func TestFilterRejectsMetadata(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		input, output := setupTestRouter(t, 0, []Filter{newMockFilter(FilterReject)})

		input.SetMetadata(testMetadata("Artist", "Title"))
		expectNoSend(t, output, time.Second)
	})
}

func TestDelayedUpdatePreservedWhenNewMetadataRejected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rejectFilter := newPatternFilter("REJECT", FilterReject)
		input, output := setupTestRouter(t, 1, []Filter{rejectFilter})

		input.SetMetadata(testMetadata("Artist A", "Title A"))
		synctest.Wait()
		input.SetMetadata(testMetadata("Artist B", "REJECT this"))

		expectSent(t, output, "Title A")
		expectNoSend(t, output, time.Second)
	})
}

func TestDelayedUpdatePreservedWhenNewMetadataCumulativelyCleared(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		filters := []Filter{
			newPatternFilter("CLEAR", FilterClearArtist),
			&artistDependentFilter{},
		}
		input, output := setupTestRouter(t, 1, filters)

		input.SetMetadata(testMetadata("Artist A", "Title A"))
		synctest.Wait()
		input.SetMetadata(testMetadata("Artist B", "CLEAR me"))

		expectSent(t, output, "Title A")
		expectNoSend(t, output, time.Second)
	})
}

func TestCumulativeFieldClearingRejectsMetadata(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		filters := []Filter{
			newMockFilter(FilterClearArtist),
			newMockFilter(FilterClearTitle),
		}
		input, output := setupTestRouter(t, 0, filters)

		input.SetMetadata(testMetadata("Artist", "Title"))
		expectNoSend(t, output, time.Second)
	})
}

func TestWouldFiltersReject(t *testing.T) {
	tests := []struct {
		name           string
		filters        []Filter
		metadata       *Metadata
		expectedReject bool
	}{
		{
			name:           "cumulative clearing rejects",
			filters:        []Filter{newMockFilter(FilterClearArtist), newMockFilter(FilterClearTitle)},
			metadata:       testMetadata("Artist", "Title"),
			expectedReject: true,
		},
		{
			name:           "explicit rejection",
			filters:        []Filter{newMockFilter(FilterReject)},
			metadata:       testMetadata("Artist", "Title"),
			expectedReject: true,
		},
		{
			name:           "pass through",
			filters:        []Filter{newMockFilter(FilterPass)},
			metadata:       testMetadata("Artist", "Title"),
			expectedReject: false,
		},
		{
			name:           "no filters passes",
			filters:        nil,
			metadata:       testMetadata("Artist", "Title"),
			expectedReject: false,
		},
		{
			name:           "nil metadata rejects",
			filters:        nil,
			metadata:       nil,
			expectedReject: true,
		},
		{
			name:           "empty content rejects",
			filters:        nil,
			metadata:       testMetadata("", ""),
			expectedReject: true,
		},
		{
			name:           "partial clear artist allows",
			filters:        []Filter{newMockFilter(FilterClearArtist)},
			metadata:       testMetadata("Artist", "Title"),
			expectedReject: false,
		},
		{
			name:           "partial clear title allows",
			filters:        []Filter{newMockFilter(FilterClearTitle)},
			metadata:       testMetadata("Artist", "Title"),
			expectedReject: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewMetadataRouter()
			input := newMockInput("test-input")
			_ = router.AddInput(input)

			if len(tt.filters) > 0 {
				router.SetInputFilters("test-input", tt.filters)
			}

			result := router.wouldFiltersReject("test-input", tt.metadata)
			if result != tt.expectedReject {
				t.Errorf("wouldFiltersReject() = %v, expected %v", result, tt.expectedReject)
			}
		})
	}
}

func TestFilterContextMatchesExecution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		router := NewMetadataRouter()
		contextFilter := newContextAwareFilter("test-input", "url", "PREFIX:", ":SUFFIX")
		router.SetInputType("test-input", "url")
		router.SetInputPrefixSuffix("test-input", "PREFIX:", ":SUFFIX")
		router.SetInputFilters("test-input", []Filter{contextFilter})

		input := newMockInput("test-input")
		output := newMockOutput("test-output")
		startRouter(t, router, output, input)

		input.SetMetadata(testMetadata("Artist", "Title"))
		synctest.Wait()

		if !contextFilter.wasContextMatched() {
			t.Error("Filter context did not match expected values during pre-check")
		}
	})
}

func TestWouldFiltersRejectContextFields(t *testing.T) {
	router := NewMetadataRouter()

	captureFilter := &capturingFilter{}

	input := newMockInput("test-input")
	_ = router.AddInput(input)
	router.SetInputType("test-input", "dynamic")
	router.SetInputPrefixSuffix("test-input", "Hello ", " World")
	router.SetInputFilters("test-input", []Filter{captureFilter})

	router.wouldFiltersReject("test-input", testMetadata("Artist", "Title"))

	captured := captureFilter.getCaptured()
	if captured == nil {
		t.Fatal("Filter was not called")
	}

	checks := []struct {
		field    string
		got      string
		expected string
	}{
		{"InputName", captured.InputName, "test-input"},
		{"InputType", captured.InputType, "dynamic"},
		{"Prefix", captured.Prefix, "Hello "},
		{"Suffix", captured.Suffix, " World"},
	}

	for _, check := range checks {
		if check.got != check.expected {
			t.Errorf("Expected %s %q, got %q", check.field, check.expected, check.got)
		}
	}
}

func expiringMetadata(title string) *Metadata {
	m := testMetadata("Artist", title)
	m.ExpiresAt = new(time.Now().Add(trackLength))
	return m
}

func setupFallbackRouter(t *testing.T) (primary, fallback *mockInput, output *mockOutput) {
	t.Helper()

	primary = newMockInput("primary")
	fallback = newMockInput("fallback")
	fallback.SetMetadata(testMetadata("", "Station Name"))
	output = newMockOutput("test-output")
	router := NewMetadataRouter()
	router.SetOutputTiming(output.GetName(), OutputTiming{
		Delay:         delaySeconds,
		FallbackDelay: fallbackSeconds,
	})
	startRouter(t, router, output, primary, fallback)

	// Drain the initial static fallback.
	expectSent(t, output, "Station Name")

	return primary, fallback, output
}

func TestFallbackWaitsForDelayPlusFallbackDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		primary, _, output := setupFallbackRouter(t)

		primary.SetMetadata(expiringMetadata("Song"))
		expectSent(t, output, "Song")

		// Measured from the "Song" send, so the output delay has already elapsed.
		// The expiration checker ticks once a second, hence the 1s margin.
		expectNoSend(t, output, trackLength+fallbackSeconds*time.Second-time.Second)
		expectSent(t, output, "Station Name")
	})
}

func TestNewTrackWithinFallbackDelayCancelsFallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		primary, _, output := setupFallbackRouter(t)

		primary.SetMetadata(expiringMetadata("First Song"))
		expectSent(t, output, "First Song")

		time.Sleep(trackLength + 5*time.Second)
		primary.SetMetadata(expiringMetadata("Second Song"))
		expectSent(t, output, "Second Song")

		expectNoSend(t, output, time.Minute)
	})
}

func TestFallbackInputChangeWithinFallbackDelayStillWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		primary, fallback, output := setupFallbackRouter(t)

		primary.SetMetadata(expiringMetadata("Song"))
		expectSent(t, output, "Song")

		// Replacing a pending fallback restarts its full delay.
		time.Sleep(trackLength + 5*time.Second)
		fallback.SetMetadata(testMetadata("", "New Station Name"))
		expectNoSend(t, output, (delaySeconds+fallbackSeconds)*time.Second-time.Second)
		expectSent(t, output, "New Station Name")
	})
}

func TestReturningPrimaryIsNotDelayedByFallbackDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		primary, _, output := setupFallbackRouter(t)

		primary.SetMetadata(expiringMetadata("Song"))
		expectSent(t, output, "Song")

		time.Sleep(trackLength + fallbackSeconds*time.Second)
		expectSent(t, output, "Station Name")

		primary.SetMetadata(expiringMetadata("Next Song"))
		expectSent(t, output, "Next Song")
	})
}
