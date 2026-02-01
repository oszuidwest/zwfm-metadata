package core

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// InputPrefixSuffix holds text to prepend and append to an input's metadata.
type InputPrefixSuffix struct {
	Prefix string
	Suffix string
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
	Available bool           `json:"available"`
	Status    string         `json:"status"` // "available", "expired", or "unavailable"
	UpdatedAt *time.Time     `json:"updatedAt,omitzero"`
	ExpiresAt *time.Time     `json:"expiresAt,omitzero"`
	Metadata  *CleanMetadata `json:"metadata,omitzero"`
}

// ScheduledUpdate holds a pending output update with its execution time and cancellation token.
type ScheduledUpdate struct {
	ExecuteAt   time.Time
	OutputName  string
	Output      Output
	Metadata    *Metadata
	UpdateType  string // "input_change" or "expiration_fallback"
	CancelToken string // unique token to allow cancellation
}

// Timeline maintains a sorted queue of scheduled updates for time-delayed processing.
type Timeline struct {
	updates []ScheduledUpdate
	signal  chan struct{}
	mu      sync.RWMutex
}

// MetadataRouter coordinates metadata flow between inputs and outputs with priority-based fallback and configurable delays.
type MetadataRouter struct {
	inputs               map[string]Input
	outputs              map[string]Output
	inputSubscriptions   map[string]chan *Metadata // input name -> subscription channel
	outputInputs         map[string][]string       // output name -> input names
	outputFormatters     map[string][]Formatter    // output name -> formatters
	outputFormatterNames map[string][]string       // output name -> formatter names
	inputFilters         map[string][]Filter       // input name -> filters
	inputFilterNames     map[string][]string       // input name -> filter type names (for dashboard)
	inputPrefixSuffix    map[string]InputPrefixSuffix
	inputTypes           map[string]string // input name -> input type
	outputTypes          map[string]string // output name -> output type
	lastSentContent      map[string]string // output name -> last sent content
	currentInputs        map[string]string // output name -> current input name
	timeline             *Timeline
	processorStop        chan struct{}
	started              bool // true after Start() is called; config maps become immutable
	mu                   sync.RWMutex
}

// NewMetadataRouter initializes a router with empty input/output registries and a timeline.
func NewMetadataRouter() *MetadataRouter {
	return &MetadataRouter{
		inputs:               make(map[string]Input),
		outputs:              make(map[string]Output),
		inputSubscriptions:   make(map[string]chan *Metadata),
		outputInputs:         make(map[string][]string),
		outputFormatters:     make(map[string][]Formatter),
		outputFormatterNames: make(map[string][]string),
		inputFilters:         make(map[string][]Filter),
		inputFilterNames:     make(map[string][]string),
		inputPrefixSuffix:    make(map[string]InputPrefixSuffix),
		inputTypes:           make(map[string]string),
		outputTypes:          make(map[string]string),
		lastSentContent:      make(map[string]string),
		currentInputs:        make(map[string]string),
		timeline:             &Timeline{updates: make([]ScheduledUpdate, 0), signal: make(chan struct{}, 1)},
		processorStop:        make(chan struct{}),
	}
}

// AddInput registers an input, returning an error if the name is already taken.
func (mr *MetadataRouter) AddInput(input Input) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("AddInput")

	name := input.GetName()
	if _, exists := mr.inputs[name]; exists {
		return fmt.Errorf("input with name %s already exists", name)
	}

	mr.inputs[name] = input
	return nil
}

// AddOutput registers an output, returning an error if the name is already taken.
func (mr *MetadataRouter) AddOutput(output Output) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("AddOutput")

	name := output.GetName()
	if _, exists := mr.outputs[name]; exists {
		return fmt.Errorf("output with name %s already exists", name)
	}

	mr.outputs[name] = output
	return nil
}

// panicIfStarted panics if configuration is attempted after Start() was called.
func (mr *MetadataRouter) panicIfStarted(method string) {
	if mr.started {
		panic("MetadataRouter." + method + " called after Start() - configuration must happen before Start()")
	}
}

// SetOutputInputs configures the priority-ordered list of inputs for an output.
func (mr *MetadataRouter) SetOutputInputs(outputName string, inputNames []string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetOutputInputs")
	mr.outputInputs[outputName] = inputNames
}

// SetOutputFormatters configures the formatter chain applied to an output's metadata.
func (mr *MetadataRouter) SetOutputFormatters(outputName string, formatters []Formatter) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetOutputFormatters")
	mr.outputFormatters[outputName] = formatters
}

// SetOutputFormatterNames stores formatter names for dashboard display.
func (mr *MetadataRouter) SetOutputFormatterNames(outputName string, formatterNames []string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetOutputFormatterNames")
	mr.outputFormatterNames[outputName] = formatterNames
}

// SetInputFilters configures the filter chain applied to an input's metadata.
func (mr *MetadataRouter) SetInputFilters(inputName string, filters []Filter) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetInputFilters")
	mr.inputFilters[inputName] = filters
}

// SetInputFilterNames stores filter type names for dashboard display.
func (mr *MetadataRouter) SetInputFilterNames(inputName string, filterNames []string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetInputFilterNames")
	mr.inputFilterNames[inputName] = filterNames
}

// SetInputPrefixSuffix configures text to prepend and append to an input's metadata.
func (mr *MetadataRouter) SetInputPrefixSuffix(inputName, prefix, suffix string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetInputPrefixSuffix")
	mr.inputPrefixSuffix[inputName] = InputPrefixSuffix{
		Prefix: prefix,
		Suffix: suffix,
	}
}

// SetInputType stores the input type identifier for dashboard display.
func (mr *MetadataRouter) SetInputType(inputName, inputType string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetInputType")
	mr.inputTypes[inputName] = inputType
}

// SetOutputType stores the output type identifier for dashboard display.
func (mr *MetadataRouter) SetOutputType(outputName, outputType string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.panicIfStarted("SetOutputType")
	mr.outputTypes[outputName] = outputType
}

// GetOutputType retrieves the output type identifier, or "unknown" if not set.
func (mr *MetadataRouter) GetOutputType(outputName string) string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	if outputType, exists := mr.outputTypes[outputName]; exists {
		return outputType
	}
	return "unknown"
}

// GetInputStatus builds a sorted snapshot of all inputs for the dashboard API.
func (mr *MetadataRouter) GetInputStatus() []InputStatus {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	statuses := make([]InputStatus, 0, len(mr.inputs))
	for name, input := range mr.inputs {
		metadata := input.GetMetadata()
		prefixSuffix := mr.inputPrefixSuffix[name]
		inputType := mr.inputTypes[name]
		filterNames := mr.inputFilterNames[name]
		if filterNames == nil {
			filterNames = []string{}
		}

		status := InputStatus{
			Name:      name,
			Type:      inputType,
			Prefix:    prefixSuffix.Prefix,
			Suffix:    prefixSuffix.Suffix,
			Filters:   filterNames,
			Available: metadata != nil && metadata.IsAvailable(),
		}

		switch {
		case metadata == nil || metadata.Title == "":
			status.Status = "unavailable"
		case metadata.IsExpired():
			status.Status = "expired"
			status.Available = false
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

// GetInput looks up an input by name, returning false if not found.
func (mr *MetadataRouter) GetInput(name string) (Input, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	input, exists := mr.inputs[name]
	return input, exists
}

// GetOutputs retrieves all registered outputs sorted alphabetically by name.
func (mr *MetadataRouter) GetOutputs() []Output {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	outputs := make([]Output, 0, len(mr.outputs))
	for _, output := range mr.outputs {
		outputs = append(outputs, output)
	}

	slices.SortFunc(outputs, func(a, b Output) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})

	return outputs
}

// GetOutputInputs retrieves the priority-ordered input names configured for an output.
func (mr *MetadataRouter) GetOutputInputs(outputName string) []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if inputs, exists := mr.outputInputs[outputName]; exists {
		return inputs
	}
	return []string{}
}

// GetOutputFormatterNames retrieves the formatter names configured for an output.
func (mr *MetadataRouter) GetOutputFormatterNames(outputName string) []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if formatterNames, exists := mr.outputFormatterNames[outputName]; exists {
		return formatterNames
	}
	return []string{}
}

// GetCurrentInputForOutput retrieves which input is currently providing metadata to an output.
func (mr *MetadataRouter) GetCurrentInputForOutput(outputName string) string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if currentInput, exists := mr.currentInputs[outputName]; exists {
		return currentInput
	}
	return ""
}

// Start launches all inputs, outputs, and background processors until context cancellation.
func (mr *MetadataRouter) Start(ctx context.Context) error {
	mr.mu.Lock()

	if mr.started {
		mr.mu.Unlock()
		return fmt.Errorf("router already started")
	}

	if len(mr.inputs) == 0 {
		mr.mu.Unlock()
		return fmt.Errorf("cannot start: no inputs configured")
	}

	mr.started = true

	go mr.startTimelineProcessor(ctx)
	go mr.startExpirationChecker(ctx)

	for name, input := range mr.inputs {
		go func(n string, i Input) {
			if err := i.Start(ctx); err != nil {
				slog.Error("Failed to start input", "name", n, "error", err)
			}
		}(name, input)

		metadataChannel := make(chan *Metadata, 10)
		mr.inputSubscriptions[name] = metadataChannel
		input.Subscribe(metadataChannel)
		go mr.handleInputMetadata(ctx, name, metadataChannel)
	}

	for name, output := range mr.outputs {
		go func(n string, o Output) {
			if err := o.Start(ctx); err != nil {
				slog.Error("Failed to start output", "name", n, "error", err)
			}
		}(name, output)
	}

	mr.mu.Unlock()
	mr.processInitialMetadata()

	slog.Info("Started centralized metadata router")

	return nil
}

// processInitialMetadata triggers updates for inputs with pre-existing metadata like static text.
func (mr *MetadataRouter) processInitialMetadata() {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	for inputName, input := range mr.inputs {
		metadata := input.GetMetadata()
		if metadata != nil && metadata.IsAvailable() {
			mr.scheduleInputChangeUpdates(inputName, metadata)
			slog.Debug("Processed initial metadata for input", "input", inputName, "title", metadata.Title)
		}
	}
}

// handleInputMetadata processes incoming metadata changes from an input's subscription channel.
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
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	for outputName, output := range mr.outputs {
		if !mr.outputUsesInput(outputName, inputName) {
			continue
		}

		inputNames, exists := mr.outputInputs[outputName]
		if !exists {
			continue
		}

		isHighestPriority := false
		for _, name := range inputNames {
			if input, exists := mr.inputs[name]; exists {
				inputMetadata := input.GetMetadata()
				if inputMetadata != nil && inputMetadata.IsAvailable() {
					if name == inputName {
						isHighestPriority = true
					}
					break
				}
			}
		}

		if !isHighestPriority {
			continue
		}

		// Check if filters would reject this metadata BEFORE canceling pending updates.
		// This preserves valid pending updates when new metadata is filtered out.
		if mr.wouldFiltersReject(inputName, metadata) {
			slog.Debug("Skipping update due to filter rejection", "input", inputName, "output", outputName)
			continue
		}

		cancelToken := fmt.Sprintf("%s_%d", outputName, time.Now().UnixNano())
		mr.cancelScheduledUpdates(outputName)

		delay := time.Duration(output.GetDelay()) * time.Second
		executeAt := time.Now().Add(delay)

		update := ScheduledUpdate{
			ExecuteAt:   executeAt,
			OutputName:  outputName,
			Output:      output,
			Metadata:    metadata,
			UpdateType:  "input_change",
			CancelToken: cancelToken,
		}

		mr.timeline.addUpdate(&update)

		if delay > 0 {
			slog.Debug("Scheduled update for output", "output", outputName, "time", executeAt.Format("15:04:05"), "delay_seconds", int(delay.Seconds()))
		} else {
			slog.Debug("Scheduled immediate update for output", "output", outputName)
		}
	}
}

// outputUsesInput reports whether the given output has the specified input in its priority list.
func (mr *MetadataRouter) outputUsesInput(outputName, inputName string) bool {
	inputNames, exists := mr.outputInputs[outputName]
	return exists && slices.Contains(inputNames, inputName)
}

// cancelScheduledUpdates removes all pending updates for an output from the timeline.
func (mr *MetadataRouter) cancelScheduledUpdates(outputName string) {
	mr.timeline.cancelUpdatesForOutput(outputName)
}

// startExpirationChecker monitors inputs for expiration and triggers fallback to lower-priority sources.
func (mr *MetadataRouter) startExpirationChecker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	slog.Info("Started expiration checker (1 second interval)")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mr.checkForExpirations()
		}
	}
}

// expirationAction holds information about an action to take for an expired input.
type expirationAction struct {
	outputName       string
	output           Output
	fallbackMetadata *Metadata
	shouldClear      bool // true if we should clear currentInputs (no fallback available)
}

// checkForExpirations scans for expired inputs and schedules fallback updates when needed.
// Uses two-phase locking: Phase 1 collects actions under RLock, Phase 2 executes writes.
// NOTE: lastSentContent may change between phases, potentially causing extra scheduled updates.
// This is harmless because executeUpdate deduplicates before sending (see lines 683-688).
func (mr *MetadataRouter) checkForExpirations() {
	// Phase 1: Collect actions under read lock
	var actions []expirationAction

	mr.mu.RLock()
	for outputName, output := range mr.outputs {
		if mr.timeline.hasScheduledUpdatesForOutput(outputName) {
			continue
		}

		currentInputName, hasCurrentInput := mr.currentInputs[outputName]
		if !hasCurrentInput {
			continue
		}

		currentInput, exists := mr.inputs[currentInputName]
		if !exists {
			continue
		}

		currentMetadata := currentInput.GetMetadata()
		if currentMetadata != nil && currentMetadata.IsAvailable() {
			continue
		}

		inputNames, exists := mr.outputInputs[outputName]
		if !exists {
			continue
		}

		var fallbackMetadata *Metadata
		var fallbackInputName string
		for _, inputName := range inputNames {
			if input, exists := mr.inputs[inputName]; exists {
				metadata := input.GetMetadata()
				if metadata != nil && metadata.IsAvailable() {
					fallbackMetadata = metadata
					fallbackInputName = inputName
					break
				}
			}
		}

		if fallbackMetadata != nil && fallbackInputName != currentInputName {
			st := mr.transformMetadataForOutput(outputName, fallbackMetadata, fallbackInputName)
			if st.HasContent() {
				formattedText := st.String()
				lastSent := mr.lastSentContent[outputName]
				if formattedText != lastSent {
					actions = append(actions, expirationAction{
						outputName:       outputName,
						output:           output,
						fallbackMetadata: fallbackMetadata,
					})
				}
			}
		} else if fallbackMetadata == nil {
			actions = append(actions, expirationAction{
				outputName:  outputName,
				shouldClear: true,
			})
		}
	}
	mr.mu.RUnlock()

	// Phase 2: Process actions (writes require full lock)
	for _, action := range actions {
		if action.shouldClear {
			mr.mu.Lock()
			mr.currentInputs[action.outputName] = ""
			mr.mu.Unlock()
			slog.Info("Output has no available inputs - cleared current input", "output", action.outputName)
			continue
		}

		delay := time.Duration(action.output.GetDelay()) * time.Second
		executeAt := time.Now().Add(delay)

		update := ScheduledUpdate{
			ExecuteAt:   executeAt,
			OutputName:  action.outputName,
			Output:      action.output,
			Metadata:    action.fallbackMetadata,
			UpdateType:  "expiration_fallback",
			CancelToken: fmt.Sprintf("%s_exp_%d", action.outputName, time.Now().UnixNano()),
		}

		mr.timeline.addUpdate(&update)
		slog.Debug("Scheduled expiration fallback for output", "output", action.outputName, "time", executeAt.Format("15:04:05"), "delay_seconds", int(delay.Seconds()))
	}
}

// applyFilterAction applies the action specified by a filter to a StructuredText.
// Returns true if processing should continue, false if metadata was rejected.
func applyFilterAction(st *StructuredText, action FilterAction) bool {
	switch action {
	case FilterPass:
		return true
	case FilterClearArtist:
		st.Artist = ""
		return true
	case FilterClearTitle:
		st.Title = ""
		return true
	case FilterReject:
		st.Artist = ""
		st.Title = ""
		st.Prefix = ""
		st.Suffix = ""
		return false
	default:
		return true
	}
}

// wouldFiltersReject checks if filters would reject metadata or clear all content.
// Used to avoid canceling valid pending updates when new metadata would be rejected.
// This mirrors the filter evaluation in transformMetadataForOutput to catch both explicit
// rejections and cumulative field clearing (e.g., one filter clears artist, another title).
func (mr *MetadataRouter) wouldFiltersReject(inputName string, metadata *Metadata) bool {
	if metadata == nil {
		return true
	}

	// Create temporary StructuredText with same context as execution time
	st := NewStructuredText(metadata)
	if !st.HasContent() {
		return true
	}

	// Set prefix/suffix and input type to match execution-time context
	if prefixSuffix, exists := mr.inputPrefixSuffix[inputName]; exists {
		st.Prefix = prefixSuffix.Prefix
		st.Suffix = prefixSuffix.Suffix
	}
	st.InputName = inputName
	st.InputType = mr.inputTypes[inputName]

	// Apply all filters, checking for both explicit rejection and cumulative clearing
	inputFilters, exists := mr.inputFilters[inputName]
	if !exists || len(inputFilters) == 0 {
		return false
	}

	for _, filter := range inputFilters {
		action := filter.Decide(st)
		if !applyFilterAction(st, action) {
			return true // Explicit rejection
		}
	}

	// Check if filters cumulatively cleared all content
	return !st.HasContent()
}

// transformMetadataForOutput builds a StructuredText from metadata with prefix/suffix and formatters applied.
// NOTE: This method reads inputPrefixSuffix, inputTypes, inputFilters, and outputFormatters
// without locks. These maps are immutable after Start() - enforced by panicIfStarted in Set* methods.
func (mr *MetadataRouter) transformMetadataForOutput(outputName string, metadata *Metadata, inputName string) *StructuredText {
	if metadata == nil {
		return nil
	}

	st := NewStructuredText(metadata)
	if !st.HasContent() {
		return st
	}

	if prefixSuffix, exists := mr.inputPrefixSuffix[inputName]; exists {
		st.Prefix = prefixSuffix.Prefix
		st.Suffix = prefixSuffix.Suffix
	}

	st.InputName = inputName
	st.InputType = mr.inputTypes[inputName]

	// Apply input filters first - filters can reject metadata entirely
	if inputFilters, exists := mr.inputFilters[inputName]; exists {
		for _, filter := range inputFilters {
			action := filter.Decide(st)
			if !applyFilterAction(st, action) {
				// Filter rejected the metadata - return StructuredText with cleared fields
				return st
			}
		}
	}

	// Apply output formatters
	if outputFormatters, exists := mr.outputFormatters[outputName]; exists {
		for _, formatter := range outputFormatters {
			formatter.Format(st)
		}
	}

	return st
}

// startTimelineProcessor waits for scheduled updates and executes them when their time arrives.
func (mr *MetadataRouter) startTimelineProcessor(ctx context.Context) {
	slog.Info("Started timeline processor (event-based)")

	for {
		nextTime := mr.timeline.nextExecutionTime()

		if nextTime.IsZero() {
			// No updates scheduled, wait for signal
			select {
			case <-ctx.Done():
				return
			case <-mr.timeline.signal:
				continue
			}
		}

		waitDuration := time.Until(nextTime)
		if waitDuration <= 0 {
			// Update is ready now
			mr.processReadyUpdates()
			continue
		}

		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-mr.timeline.signal:
			timer.Stop()
			continue
		case <-timer.C:
			mr.processReadyUpdates()
		}
	}
}

// processReadyUpdates dequeues and executes all updates scheduled for the current time.
func (mr *MetadataRouter) processReadyUpdates() {
	now := time.Now()
	readyUpdates := mr.timeline.getReadyUpdates(now)

	var wg sync.WaitGroup
	for _, update := range readyUpdates {
		wg.Go(func() {
			mr.executeUpdate(&update)
		})
	}
	wg.Wait()
}

// executeUpdate sends metadata to an output, skipping if content matches the last sent value.
func (mr *MetadataRouter) executeUpdate(update *ScheduledUpdate) {
	var inputName string
	mr.mu.RLock()
	for name, input := range mr.inputs {
		if input.GetMetadata() != nil && input.GetMetadata().Name == update.Metadata.Name {
			inputName = name
			break
		}
	}
	mr.mu.RUnlock()

	st := mr.transformMetadataForOutput(update.OutputName, update.Metadata, inputName)
	if st == nil || !st.HasContent() {
		return
	}

	formattedText := st.String()

	mr.mu.Lock()
	lastSent := mr.lastSentContent[update.OutputName]
	if formattedText == lastSent {
		mr.mu.Unlock()
		return
	}
	mr.lastSentContent[update.OutputName] = formattedText
	mr.currentInputs[update.OutputName] = inputName
	mr.mu.Unlock()

	slog.Debug("Executing update for output", "update_type", update.UpdateType, "output", update.OutputName, "text", formattedText)

	update.Output.Send(st)
}

// addUpdate inserts an update into the timeline, maintaining chronological order.
func (t *Timeline) addUpdate(update *ScheduledUpdate) {
	t.mu.Lock()
	insertIndex, _ := slices.BinarySearchFunc(t.updates, *update, func(a, b ScheduledUpdate) int {
		return a.ExecuteAt.Compare(b.ExecuteAt)
	})
	t.updates = slices.Insert(t.updates, insertIndex, *update)
	t.mu.Unlock()

	// Non-blocking signal to wake up the processor
	select {
	case t.signal <- struct{}{}:
	default:
	}
}

// nextExecutionTime returns the time of the earliest scheduled update, or zero if none.
func (t *Timeline) nextExecutionTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.updates) == 0 {
		return time.Time{}
	}
	return t.updates[0].ExecuteAt
}

// getReadyUpdates removes and returns all updates scheduled at or before the given time.
func (t *Timeline) getReadyUpdates(now time.Time) []ScheduledUpdate {
	t.mu.Lock()
	defer t.mu.Unlock()

	readyCount := 0
	for i, update := range t.updates {
		if update.ExecuteAt.After(now) {
			break
		}
		readyCount = i + 1
	}

	if readyCount == 0 {
		return nil
	}

	ready := make([]ScheduledUpdate, readyCount)
	copy(ready, t.updates[:readyCount])
	t.updates = t.updates[readyCount:]

	return ready
}

// cancelUpdatesForOutput removes all pending updates for the specified output.
func (t *Timeline) cancelUpdatesForOutput(outputName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	initialLen := len(t.updates)
	t.updates = slices.DeleteFunc(t.updates, func(update ScheduledUpdate) bool {
		return update.OutputName == outputName
	})
	cancelCount := initialLen - len(t.updates)

	if cancelCount > 0 {
		slog.Debug("Cancelled pending updates for output", "count", cancelCount, "output", outputName)
	}
}

// hasScheduledUpdatesForOutput reports whether any updates are pending for the specified output.
func (t *Timeline) hasScheduledUpdatesForOutput(outputName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return slices.ContainsFunc(t.updates, func(u ScheduledUpdate) bool {
		return u.OutputName == outputName
	})
}
