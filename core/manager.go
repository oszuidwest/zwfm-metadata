package core

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
	"zwfm-metadata/formatters"
)

// InputPrefixSuffix represents prefix/suffix configuration for an input
type InputPrefixSuffix struct {
	Prefix string
	Suffix string
}

// CleanMetadata represents user-facing metadata without internal fields
type CleanMetadata struct {
	SongID   string `json:"songID,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Title    string `json:"title"`
	Duration string `json:"duration,omitempty"`
}

// InputStatus represents the status of an input including prefix/suffix
type InputStatus struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Prefix    string         `json:"prefix"`
	Suffix    string         `json:"suffix"`
	Available bool           `json:"available"`
	Status    string         `json:"status"` // "available", "expired", or "unavailable"
	UpdatedAt *time.Time     `json:"updatedAt,omitempty"`
	ExpiresAt *time.Time     `json:"expiresAt,omitempty"`
	Metadata  *CleanMetadata `json:"metadata,omitempty"`
}

// ScheduledUpdate represents a future update to be processed
type ScheduledUpdate struct {
	ExecuteAt   time.Time
	OutputName  string
	Output      Output
	Metadata    *Metadata
	UpdateType  string // "input_change" or "expiration_fallback"
	CancelToken string // unique token to allow cancellation
}

// Timeline manages all scheduled updates in chronological order
type Timeline struct {
	updates []ScheduledUpdate
	mu      sync.RWMutex
}

// MetadataRouter manages all inputs and outputs with centralized timeline scheduling
type MetadataRouter struct {
	inputs               map[string]Input
	outputs              map[string]Output
	inputSubscriptions   map[string]chan *Metadata         // input name -> subscription channel
	outputInputs         map[string][]string               // output name -> input names
	outputFormatters     map[string][]formatters.Formatter // output name -> formatters
	outputFormatterNames map[string][]string               // output name -> formatter names
	inputPrefixSuffix    map[string]InputPrefixSuffix      // input name -> prefix/suffix
	inputTypes           map[string]string                 // input name -> input type
	outputTypes          map[string]string                 // output name -> output type
	lastSentContent      map[string]string                 // output name -> last sent content
	currentInputs        map[string]string                 // output name -> current input name
	timeline             *Timeline
	processorStop        chan struct{}
	mu                   sync.RWMutex
}

// NewMetadataRouter creates a new metadata router instance
func NewMetadataRouter() *MetadataRouter {
	return &MetadataRouter{
		inputs:               make(map[string]Input),
		outputs:              make(map[string]Output),
		inputSubscriptions:   make(map[string]chan *Metadata),
		outputInputs:         make(map[string][]string),
		outputFormatters:     make(map[string][]formatters.Formatter),
		outputFormatterNames: make(map[string][]string),
		inputPrefixSuffix:    make(map[string]InputPrefixSuffix),
		inputTypes:           make(map[string]string),
		outputTypes:          make(map[string]string),
		lastSentContent:      make(map[string]string),
		currentInputs:        make(map[string]string),
		timeline:             &Timeline{updates: make([]ScheduledUpdate, 0)},
		processorStop:        make(chan struct{}),
	}
}

// AddInput adds an input to the manager
func (mr *MetadataRouter) AddInput(input Input) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	name := input.GetName()
	if _, exists := mr.inputs[name]; exists {
		return fmt.Errorf("input with name %s already exists", name)
	}

	mr.inputs[name] = input
	return nil
}

// AddOutput adds an output to the manager
func (mr *MetadataRouter) AddOutput(output Output) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	name := output.GetName()
	if _, exists := mr.outputs[name]; exists {
		return fmt.Errorf("output with name %s already exists", name)
	}

	mr.outputs[name] = output
	return nil
}

// SetOutputInputs sets which inputs an output uses
func (mr *MetadataRouter) SetOutputInputs(outputName string, inputNames []string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputInputs[outputName] = inputNames
}

// SetOutputFormatters sets which formatters an output uses
func (mr *MetadataRouter) SetOutputFormatters(outputName string, formatters []formatters.Formatter) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputFormatters[outputName] = formatters
}

// SetOutputFormatterNames sets the formatter names for an output
func (mr *MetadataRouter) SetOutputFormatterNames(outputName string, formatterNames []string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputFormatterNames[outputName] = formatterNames
}

// SetInputPrefixSuffix sets the prefix and suffix for an input
func (mr *MetadataRouter) SetInputPrefixSuffix(inputName string, prefix, suffix string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.inputPrefixSuffix[inputName] = InputPrefixSuffix{
		Prefix: prefix,
		Suffix: suffix,
	}
}

// SetInputType sets the type for an input (used for status display)
func (mr *MetadataRouter) SetInputType(inputName string, inputType string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.inputTypes[inputName] = inputType
}

// SetOutputType sets the type for an output (used for status display)
func (mr *MetadataRouter) SetOutputType(outputName string, outputType string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputTypes[outputName] = outputType
}

// GetOutputType returns the type for an output
func (mr *MetadataRouter) GetOutputType(outputName string) string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	if outputType, exists := mr.outputTypes[outputName]; exists {
		return outputType
	}
	return "unknown"
}

// GetInputStatus returns the status of all inputs including prefix/suffix
func (mr *MetadataRouter) GetInputStatus() []InputStatus {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	statuses := make([]InputStatus, 0, len(mr.inputs))
	for name, input := range mr.inputs {
		metadata := input.GetMetadata()
		prefixSuffix := mr.inputPrefixSuffix[name]
		inputType := mr.inputTypes[name]

		status := InputStatus{
			Name:      name,
			Type:      inputType,
			Prefix:    prefixSuffix.Prefix,
			Suffix:    prefixSuffix.Suffix,
			Available: metadata != nil && metadata.IsAvailable(),
		}

		// Determine status
		if metadata == nil || metadata.Title == "" {
			status.Status = "unavailable"
		} else if metadata.IsExpired() {
			status.Status = "expired"
			status.Available = false
		} else {
			status.Status = "available"
		}

		// Add clean metadata and timestamps only if metadata exists
		if metadata != nil {
			status.UpdatedAt = &metadata.UpdatedAt
			status.ExpiresAt = metadata.ExpiresAt

			// Only include metadata if there's actual content
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

	// Sort input statuses alphabetically by name
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	return statuses
}

// GetInput retrieves an input by name
func (mr *MetadataRouter) GetInput(name string) (Input, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	input, exists := mr.inputs[name]
	return input, exists
}

// GetInputs returns all inputs
func (mr *MetadataRouter) GetInputs() []Input {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	inputs := make([]Input, 0, len(mr.inputs))
	for _, input := range mr.inputs {
		inputs = append(inputs, input)
	}
	return inputs
}

// GetOutputs returns all outputs sorted by name
func (mr *MetadataRouter) GetOutputs() []Output {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	outputs := make([]Output, 0, len(mr.outputs))
	for _, output := range mr.outputs {
		outputs = append(outputs, output)
	}

	// Sort outputs alphabetically by name for consistent ordering
	sort.Slice(outputs, func(i, j int) bool {
		return outputs[i].GetName() < outputs[j].GetName()
	})

	return outputs
}

// GetOutputInputs returns the input names for an output
func (mr *MetadataRouter) GetOutputInputs(outputName string) []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if inputs, exists := mr.outputInputs[outputName]; exists {
		return inputs
	}
	return []string{}
}

// GetOutputFormatterNames returns the formatter names for an output
func (mr *MetadataRouter) GetOutputFormatterNames(outputName string) []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if formatterNames, exists := mr.outputFormatterNames[outputName]; exists {
		return formatterNames
	}
	return []string{}
}

// GetCurrentInputForOutput returns the current active input for an output
func (mr *MetadataRouter) GetCurrentInputForOutput(outputName string) string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if currentInput, exists := mr.currentInputs[outputName]; exists {
		return currentInput
	}
	return ""
}

// Start starts all inputs and outputs with centralized timeline scheduling
func (mr *MetadataRouter) Start(ctx context.Context) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if len(mr.inputs) == 0 {
		return fmt.Errorf("cannot start: no inputs configured")
	}

	// Start timeline processor
	go mr.startTimelineProcessor(ctx)

	// Start expiration checker
	go mr.startExpirationChecker(ctx)

	// Start all inputs and subscribe to their metadata updates
	for name, input := range mr.inputs {
		// Start the input
		go func(n string, i Input) {
			if err := i.Start(ctx); err != nil {
				slog.Error("Failed to start input", "name", n, "error", err)
			}
		}(name, input)

		// Subscribe to input metadata updates
		metadataChannel := make(chan *Metadata, 10)
		mr.inputSubscriptions[name] = metadataChannel
		input.Subscribe(metadataChannel)

		// Handle metadata updates for this input
		go mr.handleInputMetadata(ctx, name, metadataChannel)
	}

	// Start all outputs
	for name, output := range mr.outputs {
		go func(n string, o Output) {
			if err := o.Start(ctx); err != nil {
				slog.Error("Failed to start output", "name", n, "error", err)
			}
		}(name, output)
	}

	slog.Info("Started centralized metadata router")

	return nil
}

// handleInputMetadata handles metadata updates from inputs
func (mr *MetadataRouter) handleInputMetadata(ctx context.Context, inputName string, metadataChannel chan *Metadata) {
	for {
		select {
		case <-ctx.Done():
			return
		case metadata := <-metadataChannel:
			// Schedule updates for all outputs that use this input
			mr.scheduleInputChangeUpdates(inputName, metadata)
		}
	}
}

// scheduleInputChangeUpdates schedules updates for outputs when input metadata changes
func (mr *MetadataRouter) scheduleInputChangeUpdates(inputName string, metadata *Metadata) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	for outputName, output := range mr.outputs {
		// Check if this output uses this input
		if !mr.outputUsesInput(outputName, inputName) {
			continue
		}

		// Get output's inputs in priority order
		inputNames, exists := mr.outputInputs[outputName]
		if !exists {
			continue
		}

		// Check if this input is the highest priority available input
		isHighestPriority := false
		for _, name := range inputNames {
			if input, exists := mr.inputs[name]; exists {
				inputMetadata := input.GetMetadata()
				if inputMetadata != nil && inputMetadata.IsAvailable() {
					// Found an available input
					if name == inputName {
						// This is the highest priority available input
						isHighestPriority = true
					}
					break // Stop at first available input
				}
			}
		}

		// Only schedule update if this is the highest priority available input
		if !isHighestPriority {
			continue
		}

		// Cancel any existing updates for this output
		cancelToken := fmt.Sprintf("%s_%d", outputName, time.Now().UnixNano())
		mr.cancelScheduledUpdates(outputName)

		// Schedule new update with delay
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

		mr.timeline.addUpdate(update)

		if delay > 0 {
			slog.Debug("Scheduled update for output", "output", outputName, "time", executeAt.Format("15:04:05"), "delay_seconds", int(delay.Seconds()))
		} else {
			slog.Debug("Scheduled immediate update for output", "output", outputName)
		}
	}
}

// outputUsesInput checks if an output uses a specific input
func (mr *MetadataRouter) outputUsesInput(outputName string, inputName string) bool {
	inputNames, exists := mr.outputInputs[outputName]
	if !exists {
		return false
	}

	for _, name := range inputNames {
		if name == inputName {
			return true
		}
	}
	return false
}

// cancelScheduledUpdates cancels all pending updates for an output
func (mr *MetadataRouter) cancelScheduledUpdates(outputName string) {
	mr.timeline.cancelUpdatesForOutput(outputName)
}

// startExpirationChecker checks for expired metadata and schedules fallback updates
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

// checkForExpirations finds expired inputs and schedules fallback updates (with delays)
func (mr *MetadataRouter) checkForExpirations() {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	for outputName, output := range mr.outputs {
		// Skip if there are already pending updates for this output
		if mr.timeline.hasScheduledUpdatesForOutput(outputName) {
			continue
		}

		// Get the current input for this output
		currentInputName, hasCurrentInput := mr.currentInputs[outputName]
		if !hasCurrentInput {
			// No current input set, skip
			continue
		}

		// Check if the current input has expired
		currentInput, exists := mr.inputs[currentInputName]
		if !exists {
			continue
		}

		currentMetadata := currentInput.GetMetadata()
		if currentMetadata != nil && currentMetadata.IsAvailable() {
			// Current input is still available, no need for fallback
			continue
		}

		// Current input has expired, find next available input
		inputNames, exists := mr.outputInputs[outputName]
		if !exists {
			continue
		}

		// Find the first available (non-expired) metadata
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

		// Check if we have fallback metadata and if it would be different from what we last sent
		if fallbackMetadata != nil && fallbackInputName != currentInputName {
			formattedText := mr.formatMetadataForOutput(outputName, fallbackMetadata, fallbackInputName)
			if formattedText != "" {
				// Only schedule if content is different from what we last sent
				lastSent := mr.lastSentContent[outputName]
				if formattedText != lastSent {
					// Schedule fallback with delay
					delay := time.Duration(output.GetDelay()) * time.Second
					executeAt := time.Now().Add(delay)

					update := ScheduledUpdate{
						ExecuteAt:   executeAt,
						OutputName:  outputName,
						Output:      output,
						Metadata:    fallbackMetadata,
						UpdateType:  "expiration_fallback",
						CancelToken: fmt.Sprintf("%s_exp_%d", outputName, time.Now().UnixNano()),
					}

					mr.timeline.addUpdate(update)
					slog.Debug("Scheduled expiration fallback for output", "output", outputName, "time", executeAt.Format("15:04:05"), "delay_seconds", int(delay.Seconds()))
				}
			}
		} else if fallbackMetadata == nil {
			// No fallback available - clear the current input
			mr.currentInputs[outputName] = ""
			slog.Info("Output has no available inputs - cleared current input", "output", outputName)
		}
	}
}

// formatMetadataForOutput formats metadata for a specific output using its formatters and input prefix/suffix
func (mr *MetadataRouter) formatMetadataForOutput(outputName string, metadata *Metadata, inputName string) string {
	if metadata == nil {
		return ""
	}

	// Get the base formatted string
	formattedText := metadata.FormatString()
	if formattedText == "" {
		return ""
	}

	// Apply input-specific prefix/suffix
	if prefixSuffix, exists := mr.inputPrefixSuffix[inputName]; exists {
		if prefixSuffix.Prefix != "" {
			formattedText = prefixSuffix.Prefix + formattedText
		}
		if prefixSuffix.Suffix != "" {
			formattedText += prefixSuffix.Suffix
		}
	}

	// Apply output-specific formatters
	outputFormatters, exists := mr.outputFormatters[outputName]
	if !exists {
		return formattedText
	}

	// Apply each formatter in sequence
	for _, formatter := range outputFormatters {
		formattedText = formatter.Format(formattedText)
	}

	return formattedText
}

// startTimelineProcessor processes scheduled updates from the timeline
func (mr *MetadataRouter) startTimelineProcessor(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // Check every 100ms for precision
	defer ticker.Stop()

	slog.Info("Started timeline processor (100ms interval)")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mr.processReadyUpdates()
		}
	}
}

// processReadyUpdates executes all updates that are ready to be processed
func (mr *MetadataRouter) processReadyUpdates() {
	now := time.Now()
	readyUpdates := mr.timeline.getReadyUpdates(now)

	// Process all ready updates concurrently (async)
	var wg sync.WaitGroup
	for _, update := range readyUpdates {
		wg.Add(1)
		go func(update ScheduledUpdate) {
			defer wg.Done()
			mr.executeUpdate(update)
		}(update)
	}
	wg.Wait()
}

// executeUpdate processes a single scheduled update
func (mr *MetadataRouter) executeUpdate(update ScheduledUpdate) {
	// Find which input this metadata came from
	var inputName string
	mr.mu.RLock()
	for name, input := range mr.inputs {
		if input.GetMetadata() != nil && input.GetMetadata().Name == update.Metadata.Name {
			inputName = name
			break
		}
	}
	mr.mu.RUnlock()

	// Format metadata for this output
	formattedText := mr.formatMetadataForOutput(update.OutputName, update.Metadata, inputName)
	if formattedText == "" {
		return
	}

	// Check if content has changed from what we last sent
	mr.mu.Lock()
	lastSent := mr.lastSentContent[update.OutputName]
	if formattedText == lastSent {
		mr.mu.Unlock()
		return // No change, skip update
	}
	// Update tracking before unlocking
	mr.lastSentContent[update.OutputName] = formattedText
	mr.currentInputs[update.OutputName] = inputName
	mr.mu.Unlock()

	// Execute the update
	slog.Debug("Executing update for output", "update_type", update.UpdateType, "output", update.OutputName, "text", formattedText)

	// Check if output supports enhanced metadata processing
	if enhancedOutput, ok := update.Output.(EnhancedOutput); ok {
		enhancedOutput.SendEnhancedMetadata(formattedText, update.Metadata)
	} else {
		update.Output.SendFormattedMetadata(formattedText)
	}
}

// Timeline methods

// addUpdate adds an update to the timeline in chronological order
func (t *Timeline) addUpdate(update ScheduledUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Insert in chronological order
	insertIndex := sort.Search(len(t.updates), func(i int) bool {
		return t.updates[i].ExecuteAt.After(update.ExecuteAt)
	})

	// Insert at the correct position
	t.updates = append(t.updates, ScheduledUpdate{})
	copy(t.updates[insertIndex+1:], t.updates[insertIndex:])
	t.updates[insertIndex] = update
}

// getReadyUpdates returns and removes all updates that should be executed now
func (t *Timeline) getReadyUpdates(now time.Time) []ScheduledUpdate {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Find all updates that are ready
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

	// Extract ready updates
	ready := make([]ScheduledUpdate, readyCount)
	copy(ready, t.updates[:readyCount])

	// Remove from timeline
	t.updates = t.updates[readyCount:]

	return ready
}

// cancelUpdatesForOutput removes all scheduled updates for a specific output
func (t *Timeline) cancelUpdatesForOutput(outputName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Filter out updates for this output
	filtered := make([]ScheduledUpdate, 0, len(t.updates))
	cancelCount := 0

	for _, update := range t.updates {
		if update.OutputName != outputName {
			filtered = append(filtered, update)
		} else {
			cancelCount++
		}
	}

	t.updates = filtered

	if cancelCount > 0 {
		slog.Debug("Cancelled pending updates for output", "count", cancelCount, "output", outputName)
	}
}

// hasScheduledUpdatesForOutput checks if an output has any pending updates
func (t *Timeline) hasScheduledUpdatesForOutput(outputName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, update := range t.updates {
		if update.OutputName == outputName {
			return true
		}
	}
	return false
}
