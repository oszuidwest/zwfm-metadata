package core

import (
	"context"
	"errors"
	"runtime"
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
	sendChan   chan *StructuredText
	beforeSend func(*StructuredText)
	sendErr    error
}

func newMockOutput(name string) *mockOutput {
	return &mockOutput{
		OutputBase: NewOutputBase(name),
		sendChan:   make(chan *StructuredText, 10),
	}
}

func (m *mockOutput) Send(st *StructuredText) error {
	if m.beforeSend != nil {
		m.beforeSend(st)
	}
	if m.sendErr != nil {
		return m.sendErr
	}
	select {
	case m.sendChan <- st:
	default:
	}
	return nil
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
	captured := *st
	f.mu.Lock()
	f.captured = &captured
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

// startRouter registers the output with the given inputs in priority order and
// starts the router. Callers run inside synctest.Test so router timers use fake time.
func startRouter(t *testing.T, router *MetadataRouter, output *mockOutput, timing OutputTiming, inputs ...*mockInput) {
	t.Helper()

	inputNames := make([]string, 0, len(inputs))
	for _, input := range inputs {
		inputNames = append(inputNames, input.GetName())
	}

	if err := router.AddOutput(output, OutputSpec{Inputs: inputNames, Timing: timing}); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}

	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
}

//nolint:gocritic // The helper mirrors AddInput's intentional value semantics.
func addInput(t *testing.T, router *MetadataRouter, input *mockInput, spec InputSpec) {
	t.Helper()
	if err := router.AddInput(input, spec); err != nil {
		t.Fatalf("AddInput failed: %v", err)
	}
}

func setupTestRouter(t *testing.T, outputDelay int, filters []Filter) (*mockInput, *mockOutput) {
	t.Helper()

	router := NewMetadataRouter()
	input := newMockInput("test-input")
	addInput(t, router, input, InputSpec{Filters: filters})

	output := newMockOutput("test-output")
	startRouter(t, router, output, OutputTiming{Delay: outputDelay}, input)

	return input, output
}

func TestAddOutputRejectsUnknownInput(t *testing.T) {
	router := NewMetadataRouter()
	addInput(t, router, newMockInput("input"), InputSpec{})

	err := router.AddOutput(newMockOutput("output"), OutputSpec{Inputs: []string{"missing"}})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("AddOutput() error = %v, want unknown input error", err)
	}
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

func TestOutputUpdatesStayOrdered(t *testing.T) {
	input, output := setupTestRouter(t, 0, nil)
	oldStarted, currentStarted := make(chan struct{}), make(chan struct{})
	releaseOld := make(chan struct{})
	output.beforeSend = func(st *StructuredText) {
		switch st.Title {
		case "old":
			close(oldStarted)
			<-releaseOld
		case "current":
			close(currentStarted)
		}
	}

	input.SetMetadata(testMetadata("", "old"))
	select {
	case <-oldStarted:
	case <-time.After(time.Second):
		t.Fatal("old update did not start")
	}
	input.SetMetadata(testMetadata("", "current"))
	startedOutOfOrder := false
	select {
	case <-currentStarted:
		startedOutOfOrder = true
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseOld)
	if startedOutOfOrder {
		t.Fatal("current update started before the old update completed")
	}

	expectSent(t, output, "old")
	expectSent(t, output, "current")
}

func TestBlockedOutputDoesNotDelayOtherOutputs(t *testing.T) {
	router := NewMetadataRouter()
	input := newMockInput("input")
	addInput(t, router, input, InputSpec{})

	blocked := newMockOutput("blocked")
	fast := newMockOutput("fast")
	for _, output := range []*mockOutput{blocked, fast} {
		if err := router.AddOutput(output, OutputSpec{Inputs: []string{input.GetName()}}); err != nil {
			t.Fatalf("AddOutput(%q) failed: %v", output.GetName(), err)
		}
	}

	started := make(chan struct{})
	release := make(chan struct{})
	blocked.beforeSend = func(st *StructuredText) {
		if st.Title == "first" {
			close(started)
			<-release
		}
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	input.SetMetadata(testMetadata("", "first"))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocked output did not start")
	}
	if st, ok := fast.waitForSend(time.Second); !ok || st.Title != "first" {
		t.Fatalf("fast output first send = %v, %v", st, ok)
	}

	input.SetMetadata(testMetadata("", "second"))
	if st, ok := fast.waitForSend(time.Second); !ok || st.Title != "second" {
		t.Fatalf("fast output second send = %v, %v", st, ok)
	}

	close(release)
	expectSent(t, blocked, "first")
	expectSent(t, blocked, "second")
}

func TestBlockedOutputCoalescesBurstWithBoundedGoroutines(t *testing.T) {
	router := NewMetadataRouter()
	input := newMockInput("input")
	addInput(t, router, input, InputSpec{})
	output := newMockOutput("output")
	startRouter(t, router, output, OutputTiming{}, input)

	started := make(chan struct{})
	release := make(chan struct{})
	output.beforeSend = func(st *StructuredText) {
		if st.Title == "initial" {
			close(started)
			<-release
		}
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	input.SetMetadata(testMetadata("", "initial"))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("initial update did not start")
	}

	baseline := runtime.NumGoroutine()
	entry := router.outputs[output.GetName()]
	const replacements = 200
	for i := range replacements {
		metadata := testMetadata("", strings.Repeat("x", i+1))
		router.mu.Lock()
		router.schedule(output.GetName(), entry, input.GetName(), metadata, "test")
		router.mu.Unlock()
	}
	if delta := runtime.NumGoroutine() - baseline; delta > 5 {
		t.Fatalf("goroutine delta after %d replacements = %d, want at most 5", replacements, delta)
	}

	close(release)
	expectSent(t, output, "initial")
	expectSent(t, output, strings.Repeat("x", replacements))
	expectNoSend(t, output, 50*time.Millisecond)
}

func TestCancellationDropsQueuedOutputUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	router := NewMetadataRouter()
	input := newMockInput("input")
	addInput(t, router, input, InputSpec{})
	output := newMockOutput("output")
	if err := router.AddOutput(output, OutputSpec{Inputs: []string{input.GetName()}}); err != nil {
		t.Fatalf("AddOutput failed: %v", err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	output.beforeSend = func(st *StructuredText) {
		if st.Title == "initial" {
			close(started)
			<-release
		}
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	if err := router.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	input.SetMetadata(testMetadata("", "initial"))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("initial update did not start")
	}

	entry := router.outputs[output.GetName()]
	router.mu.Lock()
	router.schedule(output.GetName(), entry, input.GetName(), testMetadata("", "queued"), "test")
	router.mu.Unlock()
	cancel()

	deadline := time.After(time.Second)
	for {
		router.mu.RLock()
		stopped := router.stopped
		pending := entry.pending
		router.mu.RUnlock()
		if stopped {
			if pending != nil {
				t.Fatal("pending update was not cleared on cancellation")
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("router did not observe cancellation")
		default:
			runtime.Gosched()
		}
	}

	close(release)
	expectSent(t, output, "initial")
	expectNoSend(t, output, 50*time.Millisecond)
}

func TestFailedOutputUpdateCanBeRetried(t *testing.T) {
	router := NewMetadataRouter()
	output := newMockOutput("output")
	entry := &outputEntry{output: output}
	metadata := testMetadata("", "retry me")
	sendCalls := 0
	output.beforeSend = func(*StructuredText) { sendCalls++ }

	output.sendErr = errors.New("destination unavailable")
	router.executeUpdate(output.GetName(), entry, "input", metadata, "test")
	if sendCalls != 1 {
		t.Fatalf("Send() calls after failure = %d, want 1", sendCalls)
	}
	if entry.lastSent != "" || entry.currentInput != "" {
		t.Fatalf("failed send updated router state: lastSent=%q currentInput=%q", entry.lastSent, entry.currentInput)
	}

	output.sendErr = nil
	router.executeUpdate(output.GetName(), entry, "input", metadata, "test")
	expectSent(t, output, "retry me")
	if sendCalls != 2 || entry.lastSent != "retry me" || entry.currentInput != "input" {
		t.Fatalf("retry state: calls=%d lastSent=%q currentInput=%q", sendCalls, entry.lastSent, entry.currentInput)
	}
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
			addInput(t, router, newMockInput("test-input"), InputSpec{Filters: tt.filters})

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
		input := newMockInput("test-input")
		addInput(t, router, input, InputSpec{
			Type:    "url",
			Prefix:  "PREFIX:",
			Suffix:  ":SUFFIX",
			Filters: []Filter{contextFilter},
		})

		output := newMockOutput("test-output")
		startRouter(t, router, output, OutputTiming{}, input)

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
	addInput(t, router, newMockInput("test-input"), InputSpec{
		Type:    "dynamic",
		Prefix:  "Hello ",
		Suffix:  " World",
		Filters: []Filter{captureFilter},
	})

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

	router := NewMetadataRouter()
	primary = newMockInput("primary")
	addInput(t, router, primary, InputSpec{})
	fallback = newMockInput("fallback")
	fallback.SetMetadata(testMetadata("", "Station Name"))
	addInput(t, router, fallback, InputSpec{})

	output = newMockOutput("test-output")
	timing := OutputTiming{Delay: delaySeconds, FallbackDelay: fallbackSeconds}
	startRouter(t, router, output, timing, primary, fallback)

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

		synctest.Sleep(trackLength + 5*time.Second)
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
		synctest.Sleep(trackLength + 5*time.Second)
		fallback.SetMetadata(testMetadata("", "New Station Name"))
		expectNoSend(t, output, (delaySeconds+fallbackSeconds)*time.Second-time.Second)
		expectSent(t, output, "New Station Name")
	})
}

func TestFallbackWithDuplicateContentUpdatesCurrentInputWithoutSending(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		router := NewMetadataRouter()
		primary := newMockInput("primary")
		primaryMetadata := testMetadata("", "Station Name")
		primaryMetadata.ExpiresAt = new(time.Now().Add(trackLength))
		primary.SetMetadata(primaryMetadata)
		addInput(t, router, primary, InputSpec{})
		fallback := newMockInput("fallback")
		fallback.SetMetadata(testMetadata("", "Station Name"))
		addInput(t, router, fallback, InputSpec{})

		output := newMockOutput("output")
		startRouter(t, router, output, OutputTiming{}, primary, fallback)
		expectSent(t, output, "Station Name")
		if got := router.GetOutputStatus()[0].CurrentInput; got != "primary" {
			t.Fatalf("initial current input = %q, want %q", got, "primary")
		}

		synctest.Sleep(trackLength + time.Second)
		synctest.Wait()

		if got := router.GetOutputStatus()[0].CurrentInput; got != "fallback" {
			t.Errorf("current input = %q, want %q", got, "fallback")
		}
		expectNoSend(t, output, time.Second)
	})
}

func TestReturningPrimaryIsNotDelayedByFallbackDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		primary, _, output := setupFallbackRouter(t)

		primary.SetMetadata(expiringMetadata("Song"))
		expectSent(t, output, "Song")

		synctest.Sleep(trackLength + fallbackSeconds*time.Second)
		expectSent(t, output, "Station Name")

		primary.SetMetadata(expiringMetadata("Next Song"))
		expectSent(t, output, "Next Song")
	})
}
