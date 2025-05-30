package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
	"zwfm-metadata/formatters"
	"zwfm-metadata/utils"
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

// Manager manages all inputs and outputs with centralized timeline scheduling
type Manager struct {
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

// NewManager creates a new manager instance
func NewManager() *Manager {
	return &Manager{
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
func (m *Manager) AddInput(input Input) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := input.GetName()
	if _, exists := m.inputs[name]; exists {
		return fmt.Errorf("input with name %s already exists", name)
	}

	m.inputs[name] = input
	return nil
}

// AddOutput adds an output to the manager
func (m *Manager) AddOutput(output Output) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := output.GetName()
	if _, exists := m.outputs[name]; exists {
		return fmt.Errorf("output with name %s already exists", name)
	}

	m.outputs[name] = output
	return nil
}

// SetOutputInputs sets which inputs an output uses
func (m *Manager) SetOutputInputs(outputName string, inputNames []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputInputs[outputName] = inputNames
}

// SetOutputFormatters sets which formatters an output uses
func (m *Manager) SetOutputFormatters(outputName string, formatters []formatters.Formatter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputFormatters[outputName] = formatters
}

// SetOutputFormatterNames sets the formatter names for an output
func (m *Manager) SetOutputFormatterNames(outputName string, formatterNames []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputFormatterNames[outputName] = formatterNames
}

// SetInputPrefixSuffix sets the prefix and suffix for an input
func (m *Manager) SetInputPrefixSuffix(inputName string, prefix, suffix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputPrefixSuffix[inputName] = InputPrefixSuffix{
		Prefix: prefix,
		Suffix: suffix,
	}
}

// SetInputType sets the type for an input (used for status display)
func (m *Manager) SetInputType(inputName string, inputType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputTypes[inputName] = inputType
}

// SetOutputType sets the type for an output (used for status display)
func (m *Manager) SetOutputType(outputName string, outputType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputTypes[outputName] = outputType
}

// GetOutputType returns the type for an output
func (m *Manager) GetOutputType(outputName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if outputType, exists := m.outputTypes[outputName]; exists {
		return outputType
	}
	return "unknown"
}

// GetInputStatus returns the status of all inputs including prefix/suffix
func (m *Manager) GetInputStatus() []InputStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var statuses []InputStatus
	for name, input := range m.inputs {
		metadata := input.GetMetadata()
		prefixSuffix := m.inputPrefixSuffix[name]
		inputType := m.inputTypes[name]

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
func (m *Manager) GetInput(name string) (Input, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	input, exists := m.inputs[name]
	return input, exists
}

// GetInputs returns all inputs
func (m *Manager) GetInputs() []Input {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inputs := make([]Input, 0, len(m.inputs))
	for _, input := range m.inputs {
		inputs = append(inputs, input)
	}
	return inputs
}

// GetOutputs returns all outputs sorted by name
func (m *Manager) GetOutputs() []Output {
	m.mu.RLock()
	defer m.mu.RUnlock()

	outputs := make([]Output, 0, len(m.outputs))
	for _, output := range m.outputs {
		outputs = append(outputs, output)
	}

	// Sort outputs alphabetically by name for consistent ordering
	sort.Slice(outputs, func(i, j int) bool {
		return outputs[i].GetName() < outputs[j].GetName()
	})

	return outputs
}

// GetOutputInputs returns the input names for an output
func (m *Manager) GetOutputInputs(outputName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if inputs, exists := m.outputInputs[outputName]; exists {
		return inputs
	}
	return []string{}
}

// GetOutputFormatterNames returns the formatter names for an output
func (m *Manager) GetOutputFormatterNames(outputName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if formatterNames, exists := m.outputFormatterNames[outputName]; exists {
		return formatterNames
	}
	return []string{}
}

// GetCurrentInputForOutput returns the current active input for an output
func (m *Manager) GetCurrentInputForOutput(outputName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if currentInput, exists := m.currentInputs[outputName]; exists {
		return currentInput
	}
	return ""
}

// Start starts all inputs and outputs with centralized timeline scheduling
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.inputs) == 0 {
		return fmt.Errorf("cannot start: no inputs configured")
	}

	// Start timeline processor
	go m.startTimelineProcessor(ctx)

	// Start expiration checker
	go m.startExpirationChecker(ctx)

	// Start all inputs and subscribe to their metadata updates
	for name, input := range m.inputs {
		// Start the input
		go func(n string, i Input) {
			if err := i.Start(ctx); err != nil {
				utils.LogError("Failed to start input %s: %v", n, err)
			}
		}(name, input)

		// Subscribe to input metadata updates
		ch := make(chan *Metadata, 10)
		m.inputSubscriptions[name] = ch
		input.Subscribe(ch)

		// Handle metadata updates for this input
		go m.handleInputMetadata(ctx, name, ch)
	}

	// Start all outputs
	for name, output := range m.outputs {
		go func(n string, o Output) {
			if err := o.Start(ctx); err != nil {
				utils.LogError("Failed to start output %s: %v", n, err)
			}
		}(name, output)
	}

	utils.LogInfo("Started centralized timeline manager")

	return nil
}

// handleInputMetadata handles metadata updates from inputs
func (m *Manager) handleInputMetadata(ctx context.Context, inputName string, ch chan *Metadata) {
	for {
		select {
		case <-ctx.Done():
			return
		case metadata := <-ch:
			// Schedule updates for all outputs that use this input
			m.scheduleInputChangeUpdates(inputName, metadata)
		}
	}
}

// scheduleInputChangeUpdates schedules updates for outputs when input metadata changes
func (m *Manager) scheduleInputChangeUpdates(inputName string, metadata *Metadata) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for outputName, output := range m.outputs {
		// Check if this output uses this input
		if !m.outputUsesInput(outputName, inputName) {
			continue
		}

		// Get output's inputs in priority order
		inputNames, exists := m.outputInputs[outputName]
		if !exists {
			continue
		}

		// Check if this input is the highest priority available input
		isHighestPriority := false
		for _, name := range inputNames {
			if input, exists := m.inputs[name]; exists {
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
		m.cancelScheduledUpdates(outputName)

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

		m.timeline.addUpdate(update)

		if delay > 0 {
			utils.LogDebug("Scheduled update for output %s at %v (delay: %ds)", outputName, executeAt.Format("15:04:05"), int(delay.Seconds()))
		} else {
			utils.LogDebug("Scheduled immediate update for output %s", outputName)
		}
	}
}

// outputUsesInput checks if an output uses a specific input
func (m *Manager) outputUsesInput(outputName string, inputName string) bool {
	inputNames, exists := m.outputInputs[outputName]
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
func (m *Manager) cancelScheduledUpdates(outputName string) {
	m.timeline.cancelUpdatesForOutput(outputName)
}

// startExpirationChecker checks for expired metadata and schedules fallback updates
func (m *Manager) startExpirationChecker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	utils.LogInfo("Started expiration checker (1 second interval)")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkForExpirations()
		}
	}
}

// checkForExpirations finds expired inputs and schedules fallback updates (with delays)
func (m *Manager) checkForExpirations() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for outputName, output := range m.outputs {
		// Skip if there are already pending updates for this output
		if m.timeline.hasScheduledUpdatesForOutput(outputName) {
			continue
		}

		// Get the current input for this output
		currentInputName, hasCurrentInput := m.currentInputs[outputName]
		if !hasCurrentInput {
			// No current input set, skip
			continue
		}

		// Check if the current input has expired
		currentInput, exists := m.inputs[currentInputName]
		if !exists {
			continue
		}

		currentMetadata := currentInput.GetMetadata()
		if currentMetadata != nil && currentMetadata.IsAvailable() {
			// Current input is still available, no need for fallback
			continue
		}

		// Current input has expired, find next available input
		inputNames, exists := m.outputInputs[outputName]
		if !exists {
			continue
		}

		// Find the first available (non-expired) metadata
		var fallbackMetadata *Metadata
		var fallbackInputName string
		for _, inputName := range inputNames {
			if input, exists := m.inputs[inputName]; exists {
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
			formattedText := m.formatMetadataForOutput(outputName, fallbackMetadata, fallbackInputName)
			if formattedText != "" {
				// Only schedule if content is different from what we last sent
				lastSent := m.lastSentContent[outputName]
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

					m.timeline.addUpdate(update)
					utils.LogDebug("Scheduled expiration fallback for output %s at %v (delay: %ds)", outputName, executeAt.Format("15:04:05"), int(delay.Seconds()))
				}
			}
		}
	}
}

// formatMetadataForOutput formats metadata for a specific output using its formatters and input prefix/suffix
func (m *Manager) formatMetadataForOutput(outputName string, metadata *Metadata, inputName string) string {
	if metadata == nil {
		return ""
	}

	// Get the base formatted string
	formattedText := metadata.FormatString()
	if formattedText == "" {
		return ""
	}

	// Apply input-specific prefix/suffix
	if prefixSuffix, exists := m.inputPrefixSuffix[inputName]; exists {
		if prefixSuffix.Prefix != "" {
			formattedText = prefixSuffix.Prefix + formattedText
		}
		if prefixSuffix.Suffix != "" {
			formattedText = formattedText + prefixSuffix.Suffix
		}
	}

	// Apply output-specific formatters
	outputFormatters, exists := m.outputFormatters[outputName]
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
func (m *Manager) startTimelineProcessor(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // Check every 100ms for precision
	defer ticker.Stop()

	utils.LogInfo("Started timeline processor (100ms interval)")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.processReadyUpdates()
		}
	}
}

// processReadyUpdates executes all updates that are ready to be processed
func (m *Manager) processReadyUpdates() {
	now := time.Now()
	readyUpdates := m.timeline.getReadyUpdates(now)

	// Process all ready updates concurrently (async)
	var wg sync.WaitGroup
	for _, update := range readyUpdates {
		wg.Add(1)
		go func(update ScheduledUpdate) {
			defer wg.Done()
			m.executeUpdate(update)
		}(update)
	}
	wg.Wait()
}

// executeUpdate processes a single scheduled update
func (m *Manager) executeUpdate(update ScheduledUpdate) {
	// Find which input this metadata came from
	var inputName string
	m.mu.RLock()
	for name, input := range m.inputs {
		if input.GetMetadata() != nil && input.GetMetadata().Name == update.Metadata.Name {
			inputName = name
			break
		}
	}
	m.mu.RUnlock()

	// Format metadata for this output
	formattedText := m.formatMetadataForOutput(update.OutputName, update.Metadata, inputName)
	if formattedText == "" {
		return
	}

	// Check if content has changed from what we last sent
	m.mu.Lock()
	lastSent := m.lastSentContent[update.OutputName]
	if formattedText == lastSent {
		m.mu.Unlock()
		return // No change, skip update
	}
	// Update tracking before unlocking
	m.lastSentContent[update.OutputName] = formattedText
	m.currentInputs[update.OutputName] = inputName
	m.mu.Unlock()

	// Execute the update
	utils.LogDebug("Executing %s update for output %s: %s", update.UpdateType, update.OutputName, formattedText)

	// Check if output supports enhanced metadata processing
	if enhancedOutput, ok := update.Output.(EnhancedOutput); ok {
		enhancedOutput.ProcessEnhancedMetadata(formattedText, update.Metadata)
	} else {
		update.Output.ProcessFormattedMetadata(formattedText)
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
		utils.LogDebug("Cancelled %d pending updates for output %s", cancelCount, outputName)
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
