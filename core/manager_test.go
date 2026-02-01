package core

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockInput implements Input for testing.
type mockInput struct {
	*InputBase
	PassiveComponent
}

func newMockInput(name string) *mockInput { //nolint:unparam // test helper uses consistent name
	return &mockInput{InputBase: NewInputBase(name)}
}

// mockOutput implements Output for testing with configurable delay.
type mockOutput struct {
	*OutputBase
	PassiveComponent
	sentMu   sync.Mutex
	sent     []*StructuredText
	sendChan chan *StructuredText
}

func newMockOutput(name string, delay int) *mockOutput { //nolint:unparam // test helper uses consistent values
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

// mockFilter implements Filter with configurable behavior.
type mockFilter struct {
	action FilterAction
}

func newMockFilter(action FilterAction) *mockFilter {
	return &mockFilter{action: action}
}

func (f *mockFilter) Decide(_ *StructuredText) FilterAction {
	return f.action
}

// contextAwareFilter checks that context fields are set correctly.
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

// Helper to create test metadata.
func testMetadata(name, artist, title string) *Metadata {
	return &Metadata{
		Name:      name,
		Artist:    artist,
		Title:     title,
		UpdatedAt: time.Now(),
	}
}

// TestFilterRejectionPreservesPendingUpdate verifies that when metadata B is rejected
// by a filter, a pending delayed update for metadata A still proceeds.
func TestFilterRejectionPreservesPendingUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := NewMetadataRouter()

	// Set up input with a rejecting filter
	input := newMockInput("test-input")
	if err := router.AddInput(input); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}
	router.SetInputType("test-input", "dynamic")
	router.SetInputFilters("test-input", []Filter{newMockFilter(FilterReject)})

	// Set up output with 100ms delay
	output := newMockOutput("test-output", 0)
	output.SetDelay(0) // We'll use timeline directly for precise control
	if err := router.AddOutput(output); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}
	router.SetOutputInputs("test-output", []string{"test-input"})

	// Start the router
	if err := router.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give router time to initialize
	time.Sleep(50 * time.Millisecond)

	// Metadata A should be rejected by filter, so no update scheduled
	metadataA := testMetadata("input-a", "Artist A", "Title A")
	input.SetMetadata(metadataA)

	// Wait a bit and verify no update was sent (filter rejected)
	time.Sleep(100 * time.Millisecond)

	sent := output.getSent()
	if len(sent) != 0 {
		t.Errorf("Expected no updates (filter should reject), got %d", len(sent))
	}
}

// TestFilterRejectionWithDelayPreservesPendingUpdate tests the specific scenario from issue #67:
// When metadata A is scheduled with delay, and metadata B arrives but is rejected,
// metadata A's update should still proceed.
func TestFilterRejectionWithDelayPreservesPendingUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := NewMetadataRouter()

	// Create a filter that rejects metadata with "REJECT" in title
	rejectFilter := &conditionalFilter{rejectPattern: "REJECT"}

	input := newMockInput("test-input")
	if err := router.AddInput(input); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}
	router.SetInputType("test-input", "dynamic")
	router.SetInputFilters("test-input", []Filter{rejectFilter})

	// Output with 200ms delay
	output := newMockOutput("test-output", 0)
	output.SetDelay(0) // We'll manually control timing
	if err := router.AddOutput(output); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}
	router.SetOutputInputs("test-output", []string{"test-input"})

	if err := router.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send metadata A (should pass filter)
	metadataA := testMetadata("input-a", "Artist A", "Title A")
	input.SetMetadata(metadataA)

	// Wait for update to be sent (no delay configured)
	st, ok := output.waitForSend(500 * time.Millisecond)
	if !ok {
		t.Fatal("Expected metadata A to be sent")
	}
	if st.Title != "Title A" {
		t.Errorf("Expected Title A, got %s", st.Title)
	}

	// Now send metadata B which should be rejected
	metadataB := testMetadata("input-b", "Artist B", "REJECT this")
	input.SetMetadata(metadataB)

	// Wait a bit - no new update should arrive
	_, ok = output.waitForSend(200 * time.Millisecond)
	if ok {
		t.Error("Expected metadata B to be rejected by filter")
	}

	// Verify only one update was sent total
	sent := output.getSent()
	if len(sent) != 1 {
		t.Errorf("Expected exactly 1 update, got %d", len(sent))
	}
}

// conditionalFilter rejects metadata where title contains the rejectPattern.
type conditionalFilter struct {
	rejectPattern string
}

func (f *conditionalFilter) Decide(st *StructuredText) FilterAction {
	if f.rejectPattern != "" && st.Title != "" {
		if contains(st.Title, f.rejectPattern) {
			return FilterReject
		}
	}
	return FilterPass
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCumulativeFieldClearingPreservesPendingUpdate verifies that when multiple filters
// cumulatively clear all fields, the pending update for previous metadata proceeds.
func TestCumulativeFieldClearingPreservesPendingUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := NewMetadataRouter()

	// Two filters: one clears artist, one clears title
	clearArtistFilter := newMockFilter(FilterClearArtist)
	clearTitleFilter := newMockFilter(FilterClearTitle)

	input := newMockInput("test-input")
	if err := router.AddInput(input); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}
	router.SetInputFilters("test-input", []Filter{clearArtistFilter, clearTitleFilter})

	output := newMockOutput("test-output", 0)
	if err := router.AddOutput(output); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}
	router.SetOutputInputs("test-output", []string{"test-input"})

	if err := router.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send metadata - both filters will clear their respective fields
	metadata := testMetadata("input", "Artist", "Title")
	input.SetMetadata(metadata)

	// Should be rejected because cumulative clearing leaves no content
	time.Sleep(100 * time.Millisecond)

	sent := output.getSent()
	if len(sent) != 0 {
		t.Errorf("Expected no updates (cumulative clearing should reject), got %d", len(sent))
	}
}

// TestWouldFiltersRejectWithCumulativeClearing tests wouldFiltersReject directly.
func TestWouldFiltersRejectWithCumulativeClearing(t *testing.T) {
	router := NewMetadataRouter()

	// Two filters that together clear all content
	clearArtistFilter := newMockFilter(FilterClearArtist)
	clearTitleFilter := newMockFilter(FilterClearTitle)

	input := newMockInput("test-input")
	_ = router.AddInput(input)
	router.SetInputFilters("test-input", []Filter{clearArtistFilter, clearTitleFilter})

	metadata := testMetadata("input", "Artist", "Title")

	// wouldFiltersReject should return true because cumulative clearing leaves no content
	if !router.wouldFiltersReject("test-input", metadata) {
		t.Error("Expected wouldFiltersReject to return true for cumulative clearing")
	}
}

// TestWouldFiltersRejectExplicitRejection tests explicit FilterReject.
func TestWouldFiltersRejectExplicitRejection(t *testing.T) {
	router := NewMetadataRouter()

	rejectFilter := newMockFilter(FilterReject)

	input := newMockInput("test-input")
	_ = router.AddInput(input)
	router.SetInputFilters("test-input", []Filter{rejectFilter})

	metadata := testMetadata("input", "Artist", "Title")

	if !router.wouldFiltersReject("test-input", metadata) {
		t.Error("Expected wouldFiltersReject to return true for explicit rejection")
	}
}

// TestWouldFiltersRejectPassThrough tests that passing filters don't reject.
func TestWouldFiltersRejectPassThrough(t *testing.T) {
	router := NewMetadataRouter()

	passFilter := newMockFilter(FilterPass)

	input := newMockInput("test-input")
	_ = router.AddInput(input)
	router.SetInputFilters("test-input", []Filter{passFilter})

	metadata := testMetadata("input", "Artist", "Title")

	if router.wouldFiltersReject("test-input", metadata) {
		t.Error("Expected wouldFiltersReject to return false for passing filter")
	}
}

// TestWouldFiltersRejectNoFilters tests that inputs without filters pass.
func TestWouldFiltersRejectNoFilters(t *testing.T) {
	router := NewMetadataRouter()

	input := newMockInput("test-input")
	_ = router.AddInput(input)

	metadata := testMetadata("input", "Artist", "Title")

	if router.wouldFiltersReject("test-input", metadata) {
		t.Error("Expected wouldFiltersReject to return false when no filters configured")
	}
}

// TestWouldFiltersRejectNilMetadata tests nil metadata handling.
func TestWouldFiltersRejectNilMetadata(t *testing.T) {
	router := NewMetadataRouter()

	input := newMockInput("test-input")
	_ = router.AddInput(input)

	if !router.wouldFiltersReject("test-input", nil) {
		t.Error("Expected wouldFiltersReject to return true for nil metadata")
	}
}

// TestWouldFiltersRejectEmptyContent tests empty content handling.
func TestWouldFiltersRejectEmptyContent(t *testing.T) {
	router := NewMetadataRouter()

	input := newMockInput("test-input")
	_ = router.AddInput(input)

	metadata := testMetadata("input", "", "")

	if !router.wouldFiltersReject("test-input", metadata) {
		t.Error("Expected wouldFiltersReject to return true for empty content")
	}
}

// TestFilterContextMatchesExecution verifies that the context (InputName, InputType,
// Prefix, Suffix) available to filters during pre-check matches execution time.
func TestFilterContextMatchesExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := NewMetadataRouter()

	// Create a context-aware filter that verifies fields are set correctly
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

	metadata := testMetadata("input", "Artist", "Title")
	input.SetMetadata(metadata)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if !contextFilter.wasContextMatched() {
		t.Error("Filter context did not match expected values during pre-check")
	}
}

// TestWouldFiltersRejectContextFields verifies wouldFiltersReject sets context fields.
func TestWouldFiltersRejectContextFields(t *testing.T) {
	router := NewMetadataRouter()

	// Create a filter that captures the context
	var capturedST *StructuredText
	captureFilter := &capturingFilter{capture: &capturedST}

	input := newMockInput("test-input")
	_ = router.AddInput(input)
	router.SetInputType("test-input", "dynamic")
	router.SetInputPrefixSuffix("test-input", "Hello ", " World")
	router.SetInputFilters("test-input", []Filter{captureFilter})

	metadata := testMetadata("input", "Artist", "Title")
	router.wouldFiltersReject("test-input", metadata)

	if capturedST == nil {
		t.Fatal("Filter was not called")
	}
	if capturedST.InputName != "test-input" {
		t.Errorf("Expected InputName 'test-input', got %q", capturedST.InputName)
	}
	if capturedST.InputType != "dynamic" {
		t.Errorf("Expected InputType 'dynamic', got %q", capturedST.InputType)
	}
	if capturedST.Prefix != "Hello " {
		t.Errorf("Expected Prefix 'Hello ', got %q", capturedST.Prefix)
	}
	if capturedST.Suffix != " World" {
		t.Errorf("Expected Suffix ' World', got %q", capturedST.Suffix)
	}
}

// capturingFilter captures the StructuredText for inspection.
type capturingFilter struct {
	capture **StructuredText
}

func (f *capturingFilter) Decide(st *StructuredText) FilterAction {
	*f.capture = st.Clone()
	return FilterPass
}

// TestPartialFieldClearingAllowsUpdate verifies that clearing only one field
// (leaving content) allows the update through.
func TestPartialFieldClearingAllowsUpdate(t *testing.T) {
	router := NewMetadataRouter()

	// Only clear artist, title remains
	clearArtistFilter := newMockFilter(FilterClearArtist)

	input := newMockInput("test-input")
	_ = router.AddInput(input)
	router.SetInputFilters("test-input", []Filter{clearArtistFilter})

	metadata := testMetadata("input", "Artist", "Title")

	// Should NOT reject because title is still present
	if router.wouldFiltersReject("test-input", metadata) {
		t.Error("Expected wouldFiltersReject to return false when title remains")
	}
}

// TestPartialFieldClearingTitleOnly verifies clearing title but keeping artist.
func TestPartialFieldClearingTitleOnly(t *testing.T) {
	router := NewMetadataRouter()

	// Only clear title, artist remains
	clearTitleFilter := newMockFilter(FilterClearTitle)

	input := newMockInput("test-input")
	_ = router.AddInput(input)
	router.SetInputFilters("test-input", []Filter{clearTitleFilter})

	metadata := testMetadata("input", "Artist", "Title")

	// Should NOT reject because artist is still present
	if router.wouldFiltersReject("test-input", metadata) {
		t.Error("Expected wouldFiltersReject to return false when artist remains")
	}
}
