package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// OutputTiming holds the delivery timing for an output, in seconds. FallbackDelay is
// added on top of Delay when the output switches to a lower-priority input.
type OutputTiming struct {
	Delay         int `json:"delay"`
	FallbackDelay int `json:"fallbackDelay"`
}

// InputSpec configures preprocessing for an input.
type InputSpec struct {
	Type        string
	Prefix      string
	Suffix      string
	Filters     []Filter
	FilterNames []string // dashboard labels
}

// OutputSpec configures an output's sources, formatting, and timing.
type OutputSpec struct {
	Type           string
	Inputs         []string
	Formatters     []Formatter
	FormatterNames []string // dashboard labels
	Timing         OutputTiming
}

// CleanMetadata contains only the public-facing metadata fields for API responses.
type CleanMetadata struct {
	SongID   string `json:"songID,omitzero"`
	Artist   string `json:"artist,omitzero"`
	Title    string `json:"title"`
	Duration string `json:"duration,omitzero"`
}

// InputStatus provides a complete snapshot of an input for the dashboard API.
type InputStatus struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Prefix    string         `json:"prefix"`
	Suffix    string         `json:"suffix"`
	Filters   []string       `json:"filters"`
	Status    string         `json:"status"` // "available", "expired", or "unavailable"
	UpdatedAt *time.Time     `json:"updatedAt,omitzero"`
	ExpiresAt *time.Time     `json:"expiresAt,omitzero"`
	Metadata  *CleanMetadata `json:"metadata,omitzero"`
}

// OutputStatus provides a complete snapshot of an output for the dashboard API.
type OutputStatus struct {
	Name string `json:"name"`
	Type string `json:"type"`
	OutputTiming
	Inputs       []string `json:"inputs"`
	Formatters   []string `json:"formatters"`
	CurrentInput string   `json:"currentInput,omitzero"`
}

type inputEntry struct {
	input Input
	spec  InputSpec
}

type outputUpdate struct {
	inputName string
	metadata  *Metadata
	reason    string
	readyAt   time.Time
}

// outputEntry pairs an output with its spec and the router's per-output state.
// Its worker serializes Send calls and consumes at most one coalesced update.
type outputEntry struct {
	output       Output
	spec         OutputSpec
	lastSent     string
	currentInput string
	pending      *outputUpdate
	wake         chan struct{}
}

// MetadataRouter routes metadata by input priority and output timing.
type MetadataRouter struct {
	inputs  map[string]*inputEntry
	outputs map[string]*outputEntry
	started bool // true after Start() is called; inputs and outputs become immutable
	stopped bool // true after context cancellation; no more updates may be scheduled
	mu      sync.RWMutex
}

// NewMetadataRouter returns an empty router.
func NewMetadataRouter() *MetadataRouter {
	return &MetadataRouter{
		inputs:  make(map[string]*inputEntry),
		outputs: make(map[string]*outputEntry),
	}
}

// AddInput registers an input with its spec, returning an error if the name is already taken.
//
//nolint:gocritic // Value semantics make the required registration spec non-nil by construction.
func (mr *MetadataRouter) AddInput(input Input, spec InputSpec) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("AddInput")

	name := input.GetName()
	if _, exists := mr.inputs[name]; exists {
		return fmt.Errorf("input with name %s already exists", name)
	}
	storedSpec := spec
	storedSpec.Filters = slices.Clone(spec.Filters)
	storedSpec.FilterNames = slices.Clone(spec.FilterNames)
	mr.inputs[name] = &inputEntry{input: input, spec: storedSpec}
	return nil
}

// AddOutput registers an output with its spec. It fails when the name is already
// taken, when there are no inputs, when an input is repeated or unknown, or when
// the timing is negative.
//
//nolint:gocritic // Value semantics make the required registration spec non-nil by construction.
func (mr *MetadataRouter) AddOutput(output Output, spec OutputSpec) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("AddOutput")

	name := output.GetName()
	if _, exists := mr.outputs[name]; exists {
		return fmt.Errorf("output with name %s already exists", name)
	}
	if spec.Timing.Delay < 0 || spec.Timing.FallbackDelay < 0 {
		return fmt.Errorf("output %q: delay and fallbackDelay must not be negative", name)
	}
	if len(spec.Inputs) == 0 {
		return fmt.Errorf("output %q: at least one input is required", name)
	}
	for i, inputName := range spec.Inputs {
		if slices.Contains(spec.Inputs[:i], inputName) {
			return fmt.Errorf("input %q is listed more than once for output %q", inputName, name)
		}
		if _, exists := mr.inputs[inputName]; !exists {
			return fmt.Errorf("input %q not found for output %q", inputName, name)
		}
	}

	storedSpec := spec
	storedSpec.Inputs = slices.Clone(spec.Inputs)
	storedSpec.Formatters = slices.Clone(spec.Formatters)
	storedSpec.FormatterNames = slices.Clone(spec.FormatterNames)
	mr.outputs[name] = &outputEntry{
		output: output,
		spec:   storedSpec,
		wake:   make(chan struct{}, 1),
	}
	return nil
}

func (mr *MetadataRouter) panicIfStarted(method string) {
	if mr.started {
		panic("MetadataRouter." + method + " called after Start() - configuration must happen before Start()")
	}
}

// GetInput looks up an input by name, returning false if not found.
func (mr *MetadataRouter) GetInput(name string) (Input, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	entry, exists := mr.inputs[name]
	if !exists {
		return nil, false
	}
	return entry.input, true
}

// GetOutputs retrieves all registered outputs sorted alphabetically by name.
func (mr *MetadataRouter) GetOutputs() []Output {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	outputs := make([]Output, 0, len(mr.outputs))
	for _, entry := range mr.outputs {
		outputs = append(outputs, entry.output)
	}

	slices.SortFunc(outputs, func(a, b Output) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})

	return outputs
}

// GetInputStatus builds a sorted snapshot of all inputs for the dashboard API.
func (mr *MetadataRouter) GetInputStatus() []InputStatus {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	statuses := make([]InputStatus, 0, len(mr.inputs))
	for name, entry := range mr.inputs {
		metadata := entry.input.GetMetadata()

		status := InputStatus{
			Name:    name,
			Type:    entry.spec.Type,
			Prefix:  entry.spec.Prefix,
			Suffix:  entry.spec.Suffix,
			Filters: slices.Clone(entry.spec.FilterNames),
		}

		switch {
		case metadata == nil || metadata.Title == "":
			status.Status = "unavailable"
		case metadata.IsExpired():
			status.Status = "expired"
		default:
			status.Status = "available"
		}

		if metadata != nil {
			status.UpdatedAt = &metadata.UpdatedAt
			status.ExpiresAt = metadata.ExpiresAt
			if metadata.Title != "" {
				status.Metadata = &CleanMetadata{
					SongID:   metadata.SongID,
					Artist:   metadata.Artist,
					Title:    metadata.Title,
					Duration: metadata.Duration,
				}
			}
		}

		statuses = append(statuses, status)
	}

	slices.SortFunc(statuses, func(a, b InputStatus) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return statuses
}

// GetOutputStatus builds a sorted snapshot of all outputs for the dashboard API.
func (mr *MetadataRouter) GetOutputStatus() []OutputStatus {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	statuses := make([]OutputStatus, 0, len(mr.outputs))
	for name, entry := range mr.outputs {
		statuses = append(statuses, OutputStatus{
			Name:         name,
			Type:         entry.spec.Type,
			OutputTiming: entry.spec.Timing,
			Inputs:       slices.Clone(entry.spec.Inputs),
			Formatters:   slices.Clone(entry.spec.FormatterNames),
			CurrentInput: entry.currentInput,
		})
	}

	slices.SortFunc(statuses, func(a, b OutputStatus) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return statuses
}

// Start launches router workers, which stop when ctx is canceled.
func (mr *MetadataRouter) Start(ctx context.Context) error {
	mr.mu.Lock()

	if mr.started {
		mr.mu.Unlock()
		return errors.New("router already started")
	}

	if len(mr.inputs) == 0 {
		mr.mu.Unlock()
		return errors.New("cannot start: no inputs configured")
	}

	mr.started = true

	go mr.startExpirationChecker(ctx)

	for name, entry := range mr.inputs {
		go func() {
			if err := entry.input.Start(ctx); err != nil {
				slog.Error("Failed to start input", "name", name, "error", err)
			}
		}()

		metadataChannel := make(chan *Metadata, 10)
		entry.input.Subscribe(metadataChannel)
		go mr.handleInputMetadata(ctx, name, metadataChannel)
	}

	for name, entry := range mr.outputs {
		go func() {
			if err := entry.output.Start(ctx); err != nil {
				slog.Error("Failed to start output", "name", name, "error", err)
			}
		}()
		go mr.runOutputWorker(ctx, name, entry)
	}

	mr.mu.Unlock()
	mr.processInitialMetadata()

	slog.Info("Started centralized metadata router")

	return nil
}

// processInitialMetadata schedules preloaded inputs after configuration becomes immutable.
func (mr *MetadataRouter) processInitialMetadata() {
	for inputName, entry := range mr.inputs {
		metadata := entry.input.GetMetadata()
		if metadata.IsAvailable() {
			mr.scheduleInputChangeUpdates(inputName, metadata)
			slog.Debug("Processed initial metadata for input", "input", inputName, "title", metadata.Title)
		}
	}
}

func (mr *MetadataRouter) handleInputMetadata(ctx context.Context, inputName string, metadataChannel chan *Metadata) {
	for {
		select {
		case <-ctx.Done():
			return
		case metadata := <-metadataChannel:
			mr.scheduleInputChangeUpdates(inputName, metadata)
		}
	}
}

// scheduleInputChangeUpdates queues delayed updates for outputs using this input as their highest priority source.
func (mr *MetadataRouter) scheduleInputChangeUpdates(inputName string, metadata *Metadata) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	rejected := mr.wouldFiltersReject(inputName, metadata)

	for outputName, entry := range mr.outputs {
		if !slices.Contains(entry.spec.Inputs, inputName) {
			continue
		}

		if highestPriorityInput, _ := mr.findHighestPriorityInput(entry); highestPriorityInput != inputName {
			continue
		}

		// Rejected metadata leaves any pending update in place.
		if rejected {
			slog.Debug("Skipping update due to filter rejection", "input", inputName, "output", outputName)
			continue
		}

		mr.schedule(outputName, entry, inputName, metadata, "input_change")
	}
}

// findHighestPriorityInput returns empty values when no configured input is available.
func (mr *MetadataRouter) findHighestPriorityInput(entry *outputEntry) (string, *Metadata) {
	for _, inputName := range entry.spec.Inputs {
		metadata := mr.inputs[inputName].input.GetMetadata()
		if metadata.IsAvailable() {
			return inputName, metadata
		}
	}
	return "", nil
}

func (mr *MetadataRouter) startExpirationChecker(ctx context.Context) {
	ticks := time.NewTicker(time.Second)
	defer ticks.Stop()

	slog.Info("Started expiration checker (1 second interval)")

	for {
		select {
		case <-ctx.Done():
			mr.cancelPendingUpdates()
			return
		case <-ticks.C:
			mr.checkForExpirations()
		}
	}
}

// cancelPendingUpdates stops new scheduling and drops every pending update.
func (mr *MetadataRouter) cancelPendingUpdates() {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.stopped = true
	for outputName, entry := range mr.outputs {
		if entry.pending != nil {
			entry.pending = nil
			slog.Debug("Cancelled pending output update", "output", outputName)
		}
	}
}

func (mr *MetadataRouter) checkForExpirations() {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	for outputName, entry := range mr.outputs {
		if entry.pending != nil || !mr.currentInputNeedsFallback(entry) {
			continue
		}

		fallbackInputName, fallbackMetadata := mr.findHighestPriorityInput(entry)

		if fallbackMetadata == nil {
			entry.currentInput = ""
			slog.Info("Output has no available inputs - cleared current input", "output", outputName)
			continue
		}

		if fallbackInputName == entry.currentInput {
			continue
		}

		mr.scheduleFallbackUpdate(outputName, entry, fallbackInputName, fallbackMetadata)
	}
}

func (mr *MetadataRouter) currentInputNeedsFallback(entry *outputEntry) bool {
	if entry.currentInput == "" {
		return false
	}
	return !mr.inputs[entry.currentInput].input.GetMetadata().IsAvailable()
}

func (mr *MetadataRouter) scheduleFallbackUpdate(
	outputName string, entry *outputEntry, inputName string, metadata *Metadata,
) {
	st := mr.transformMetadataForOutput(entry, metadata, inputName)
	if !st.HasContent() {
		return
	}

	mr.schedule(outputName, entry, inputName, metadata, "expiration_fallback")
}

// schedule replaces the output's pending update and wakes its worker. Callers
// must hold mr.mu for writing.
func (mr *MetadataRouter) schedule(
	outputName string, entry *outputEntry, inputName string, metadata *Metadata, reason string,
) {
	if mr.stopped {
		return
	}

	delay := mr.updateDelay(entry, inputName)
	entry.pending = &outputUpdate{
		inputName: inputName,
		metadata:  metadata,
		reason:    reason,
		readyAt:   time.Now().Add(delay),
	}
	select {
	case entry.wake <- struct{}{}:
	default:
	}

	slog.Debug("Scheduled update for output",
		"update_type", reason,
		"output", outputName,
		"delay", delay,
	)
}

func (mr *MetadataRouter) runOutputWorker(ctx context.Context, outputName string, entry *outputEntry) {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		mr.mu.Lock()
		pending := entry.pending
		mr.mu.Unlock()

		if pending == nil {
			select {
			case <-ctx.Done():
				return
			case <-entry.wake:
				continue
			}
		}

		delay := max(time.Until(pending.readyAt), 0)
		timer.Reset(delay)

		select {
		case <-ctx.Done():
			stopTimer(timer)
			return
		case <-entry.wake:
			stopTimer(timer)
			continue
		case <-timer.C:
		}

		mr.mu.Lock()
		if entry.pending != pending {
			mr.mu.Unlock()
			continue
		}
		entry.pending = nil
		mr.mu.Unlock()

		mr.executeUpdate(
			outputName,
			entry,
			pending.inputName,
			pending.metadata,
			pending.reason,
		)
	}
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

// updateDelay returns Delay, plus FallbackDelay when inputName ranks below the input
// the output currently shows. That covers both the expiration checker's fallback and
// a lower-priority input changing while the switch is pending. A return to a higher
// priority, or the first send at startup when nothing is current, gets Delay only.
func (mr *MetadataRouter) updateDelay(entry *outputEntry, inputName string) time.Duration {
	seconds := entry.spec.Timing.Delay
	current := slices.Index(entry.spec.Inputs, entry.currentInput)
	if current >= 0 && slices.Index(entry.spec.Inputs, inputName) > current {
		seconds += entry.spec.Timing.FallbackDelay
	}
	return time.Duration(seconds) * time.Second
}

func applyFilterAction(st *StructuredText, action FilterAction) {
	switch action {
	case FilterClearArtist:
		st.Artist = ""
	case FilterClearTitle:
		st.Title = ""
	case FilterReject:
		st.Artist = ""
		st.Title = ""
	case FilterPass:
	}
}

// applyInputStage applies prefix/suffix and the input filters. The result has no
// content when the filters rejected the metadata. It reads only specs, which are
// immutable after Start.
func (mr *MetadataRouter) applyInputStage(inputName string, metadata *Metadata) *StructuredText {
	st := NewStructuredText(metadata)
	if !st.HasContent() {
		return st
	}

	var filters []Filter
	if entry, exists := mr.inputs[inputName]; exists {
		st.Prefix = entry.spec.Prefix
		st.Suffix = entry.spec.Suffix
		st.InputType = entry.spec.Type
		filters = entry.spec.Filters
	}
	st.InputName = inputName

	for _, filter := range filters {
		applyFilterAction(st, filter.Decide(st))
		if !st.HasContent() {
			break
		}
	}

	return st
}

// wouldFiltersReject accounts for cumulative field clearing across filters.
func (mr *MetadataRouter) wouldFiltersReject(inputName string, metadata *Metadata) bool {
	return metadata == nil || !mr.applyInputStage(inputName, metadata).HasContent()
}

func (mr *MetadataRouter) transformMetadataForOutput(
	entry *outputEntry, metadata *Metadata, inputName string,
) *StructuredText {
	st := mr.applyInputStage(inputName, metadata)
	if !st.HasContent() {
		return st
	}

	for _, formatter := range entry.spec.Formatters {
		formatter.Format(st)
	}

	return st
}

// executeUpdate records state only after changed content is delivered successfully.
func (mr *MetadataRouter) executeUpdate(
	outputName string, entry *outputEntry, inputName string, metadata *Metadata, reason string,
) {
	st := mr.transformMetadataForOutput(entry, metadata, inputName)
	if !st.HasContent() {
		return
	}

	formattedText := st.String()

	mr.mu.Lock()
	if formattedText == entry.lastSent {
		entry.currentInput = inputName
		mr.mu.Unlock()
		return
	}
	mr.mu.Unlock()

	slog.Debug("Executing update for output",
		"update_type", reason,
		"output", outputName,
		"text", formattedText,
	)

	if err := entry.output.Send(st); err != nil {
		slog.Error("Failed to send output update", "output", outputName, "error", err)
		return
	}

	mr.mu.Lock()
	entry.lastSent = formattedText
	entry.currentInput = inputName
	mr.mu.Unlock()
}
