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
	sentMu   sync.Mutex
	sent     []*StructuredText
	sendChan chan *StructuredText
}

func newMockOutput(name string, delay int) *mockOutput {
	o := &mockOutput{
		OutputBase: NewOutputBase(name),
		sent:       make([]*StructuredText, 0),
		sendChan:   make(chan *StructuredText, 10),
	}
	o.SetDelay(delay)
	return o
}

func (m *mockOutput) Send(st *StructuredText) {
	m.sentMu.Lock()
	m.sent = append(m.sent, st.Clone())
	m.sentMu.Unlock()

	select {
	case m.sendChan <- st:
	default:
	}
}

func (m *mockOutput) getSent() []*StructuredText {
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	result := make([]*StructuredText, len(m.sent))
	copy(result, m.sent)
	return result
}

func (m *mockOutput) waitForSend(timeout time.Duration) (*StructuredText, bool) {
	select {
	case st := <-m.sendChan:
		return st, true
	case <-time.After(timeout):
		return nil, false
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

func setupTestRouter(t *testing.T, outputDelay int, filters []Filter) (*mockInput, *mockOutput) {
	t.Helper()

	router := NewMetadataRouter()

	input := newMockInput("test-input")
	if err := router.AddInput(input); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}

	if len(filters) > 0 {
		router.SetInputFilters("test-input", filters)
	}

	output := newMockOutput("test-output", outputDelay)
	if err := router.AddOutput(output); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}
	router.SetOutputInputs("test-output", []string{"test-input"})

	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	return input, output
}

func TestFilterRejectsMetadata(t *testing.T) {
	input, output := setupTestRouter(t, 0, []Filter{newMockFilter(FilterReject)})

	input.SetMetadata(testMetadata("Artist", "Title"))
	time.Sleep(100 * time.Millisecond)

	sent := output.getSent()
	if len(sent) != 0 {
		t.Errorf("Expected no updates (filter should reject), got %d", len(sent))
	}
}

func TestDelayedUpdatePreservedWhenNewMetadataRejected(t *testing.T) {
	rejectFilter := newPatternFilter("REJECT", FilterReject)
	input, output := setupTestRouter(t, 1, []Filter{rejectFilter})

	input.SetMetadata(testMetadata("Artist A", "Title A"))

	time.Sleep(50 * time.Millisecond)
	input.SetMetadata(testMetadata("Artist B", "REJECT this"))

	st, ok := output.waitForSend(2 * time.Second)
	if !ok {
		t.Fatal("Expected metadata A to be sent after delay - pending update was incorrectly canceled")
	}
	if st.Title != "Title A" {
		t.Errorf("Expected Title A, got %s", st.Title)
	}

	_, gotExtra := output.waitForSend(500 * time.Millisecond)
	if gotExtra {
		t.Error("Expected metadata B to be rejected, but received additional update")
	}

	sent := output.getSent()
	if len(sent) != 1 {
		t.Errorf("Expected exactly 1 update (A), got %d", len(sent))
	}
}

func TestDelayedUpdatePreservedWhenNewMetadataCumulativelyCleared(t *testing.T) {
	filters := []Filter{
		newPatternFilter("CLEAR", FilterClearArtist),
		&artistDependentFilter{},
	}
	input, output := setupTestRouter(t, 1, filters)

	input.SetMetadata(testMetadata("Artist A", "Title A"))

	time.Sleep(50 * time.Millisecond)
	input.SetMetadata(testMetadata("Artist B", "CLEAR me"))

	st, ok := output.waitForSend(2 * time.Second)
	if !ok {
		t.Fatal("Expected metadata A to be sent - pending update was incorrectly canceled")
	}
	if st.Title != "Title A" {
		t.Errorf("Expected Title A, got %s", st.Title)
	}

	_, gotExtra := output.waitForSend(500 * time.Millisecond)
	if gotExtra {
		t.Error("Expected metadata B to be rejected (cumulative clearing), but received additional update")
	}

	sent := output.getSent()
	if len(sent) != 1 {
		t.Errorf("Expected exactly 1 update (A), got %d", len(sent))
	}
}

func TestCumulativeFieldClearingRejectsMetadata(t *testing.T) {
	filters := []Filter{
		newMockFilter(FilterClearArtist),
		newMockFilter(FilterClearTitle),
	}
	input, output := setupTestRouter(t, 0, filters)

	input.SetMetadata(testMetadata("Artist", "Title"))
	time.Sleep(100 * time.Millisecond)

	sent := output.getSent()
	if len(sent) != 0 {
		t.Errorf("Expected no updates (cumulative clearing should reject), got %d", len(sent))
	}
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
	ctx := t.Context()

	router := NewMetadataRouter()

	contextFilter := newContextAwareFilter("test-input", "url", "PREFIX:", ":SUFFIX")

	input := newMockInput("test-input")
	if err := router.AddInput(input); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}
	router.SetInputType("test-input", "url")
	router.SetInputPrefixSuffix("test-input", "PREFIX:", ":SUFFIX")
	router.SetInputFilters("test-input", []Filter{contextFilter})

	output := newMockOutput("test-output", 0)
	if err := router.AddOutput(output); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}
	router.SetOutputInputs("test-output", []string{"test-input"})

	if err := router.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	input.SetMetadata(testMetadata("Artist", "Title"))
	time.Sleep(100 * time.Millisecond)

	if !contextFilter.wasContextMatched() {
		t.Error("Filter context did not match expected values during pre-check")
	}
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

const (
	trackLength     = 3 * time.Minute
	delaySeconds    = 5
	fallbackSeconds = 20
)

func expiringMetadata(title string) *Metadata {
	m := testMetadata("Artist", title)
	m.ExpiresAt = new(time.Now().Add(trackLength))
	return m
}

// setupFallbackRouter must run inside synctest.Test so router timers use fake time.
func setupFallbackRouter(t *testing.T) (primary, fallback *mockInput, output *mockOutput) {
	t.Helper()

	router := NewMetadataRouter()

	primary = newMockInput("primary")
	if err := router.AddInput(primary); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}

	fallback = newMockInput("fallback")
	fallback.SetMetadata(testMetadata("", "Station Name"))
	if err := router.AddInput(fallback); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}

	output = newMockOutput("test-output", delaySeconds)
	output.SetFallbackDelay(fallbackSeconds)
	if err := router.AddOutput(output); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}
	router.SetOutputInputs("test-output", []string{"primary", "fallback"})

	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Ignore the initial static fallback.
	expectSent(t, output, "Station Name")

	return primary, fallback, output
}

func expectSent(t *testing.T, output *mockOutput, title string) {
	t.Helper()
	st, ok := output.waitForSend((delaySeconds + 1) * time.Second)
	if !ok || st.Title != title {
		t.Fatalf("expected %q to be sent, got %v", title, st)
	}
}

func TestFallbackWaitsForDelayPlusFallbackDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		primary, _, output := setupFallbackRouter(t)

		primary.SetMetadata(expiringMetadata("Song"))
		expectSent(t, output, "Song")

		// The initial output delay cancels out of the end-to-end wait. The expiration tick
		// may add one second.
		fallbackAt := trackLength + fallbackSeconds*time.Second
		if st, ok := output.waitForSend(fallbackAt - 2*time.Second); ok {
			t.Fatalf("fallback sent too early: %q", st.String())
		}
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

		if st, ok := output.waitForSend(time.Minute); ok {
			t.Fatalf("fallback must be cancelled by the new track, but %q was sent", st.String())
		}
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
		if st, ok := output.waitForSend((delaySeconds + fallbackSeconds - 1) * time.Second); ok {
			t.Fatalf("lower-priority change sent too early: %q", st.String())
		}
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
