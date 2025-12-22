package core

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
	"zwfm-metadata/formatters"
)

// InputPrefixSuffix represents prefix/suffix configuration for an input.
type InputPrefixSuffix struct {
	Prefix string
	Suffix string
}

// CleanMetadata represents user-facing metadata without internal fields.
type CleanMetadata struct {
	SongID   string `json:"songID,omitzero"`
	Artist   string `json:"artist,omitzero"`
	Title    string `json:"title"`
	Duration string `json:"duration,omitzero"`
}

// InputStatus represents the status of an input including prefix/suffix.
type InputStatus struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Prefix    string         `json:"prefix"`
	Suffix    string         `json:"suffix"`
	Available bool           `json:"available"`
	Status    string         `json:"status"` // "available", "expired", or "unavailable"
	UpdatedAt *time.Time     `json:"updatedAt,omitzero"`
	ExpiresAt *time.Time     `json:"expiresAt,omitzero"`
	Metadata  *CleanMetadata `json:"metadata,omitzero"`
}

// ScheduledUpdate represents a future update to be processed.
type ScheduledUpdate struct {
	ExecuteAt   time.Time
	OutputName  string
	Output      Output
	Metadata    *Metadata
	UpdateType  string // "input_change" or "expiration_fallback"
	CancelToken string // unique token to allow cancellation
}

// Timeline manages all scheduled updates in chronological order.
type Timeline struct {
	updates []ScheduledUpdate
	mu      sync.RWMutex
}

// MetadataRouter manages all inputs and outputs with centralized timeline scheduling.
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

// NewMetadataRouter creates a new metadata router instance.
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

// AddInput adds an input to the manager.
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

// AddOutput adds an output to the manager.
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

// SetOutputInputs sets which inputs an output uses.
func (mr *MetadataRouter) SetOutputInputs(outputName string, inputNames []string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputInputs[outputName] = inputNames
}

// SetOutputFormatters sets which formatters an output uses.
func (mr *MetadataRouter) SetOutputFormatters(outputName string, formatters []formatters.Formatter) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputFormatters[outputName] = formatters
}

// SetOutputFormatterNames sets the formatter names for an output.
func (mr *MetadataRouter) SetOutputFormatterNames(outputName string, formatterNames []string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputFormatterNames[outputName] = formatterNames
}

// SetInputPrefixSuffix sets the prefix and suffix for an input.
func (mr *MetadataRouter) SetInputPrefixSuffix(inputName string, prefix, suffix string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.inputPrefixSuffix[inputName] = InputPrefixSuffix{
		Prefix: prefix,
		Suffix: suffix,
	}
}

// SetInputType sets the type for an input (used for status display).
func (mr *MetadataRouter) SetInputType(inputName string, inputType string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.inputTypes[inputName] = inputType
}

// SetOutputType sets the type for an output (used for status display).
func (mr *MetadataRouter) SetOutputType(outputName string, outputType string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.outputTypes[outputName] = outputType
}

// GetOutputType returns the type for an output.
func (mr *MetadataRouter) GetOutputType(outputName string) string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	if outputType, exists := mr.outputTypes[outputName]; exists {
		return outputType
	}
	return "unknown"
}

// GetInputStatus returns the status of all inputs including prefix/suffix.
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

		if metadata == nil || metadata.Title == "" {
			status.Status = "unavailable"
		} else if metadata.IsExpired() {
			status.Status = "expired"
			status.Available = false
		} else {
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

// GetInput retrieves an input by name.
func (mr *MetadataRouter) GetInput(name string) (Input, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	input, exists := mr.inputs[name]
	return input, exists
}

// GetOutputs returns all outputs sorted by name.
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

// GetOutputInputs returns the input names for an output.
func (mr *MetadataRouter) GetOutputInputs(outputName string) []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if inputs, exists := mr.outputInputs[outputName]; exists {
		return inputs
	}
	return []string{}
}

// GetOutputFormatterNames returns the formatter names for an output.
func (mr *MetadataRouter) GetOutputFormatterNames(outputName string) []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if formatterNames, exists := mr.outputFormatterNames[outputName]; exists {
		return formatterNames
	}
	return []string{}
}

// GetCurrentInputForOutput returns the current active input for an output.
func (mr *MetadataRouter) GetCurrentInputForOutput(outputName string) string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if currentInput, exists := mr.currentInputs[outputName]; exists {
		return currentInput
	}
	return ""
}

// Start starts all inputs and outputs with centralized timeline scheduling.
func (mr *MetadataRouter) Start(ctx context.Context) error {
	mr.mu.Lock()

	if len(mr.inputs) == 0 {
		mr.mu.Unlock()
		return fmt.Errorf("cannot start: no inputs configured")
	}

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

// processInitialMetadata schedules updates for inputs that already have metadata (e.g., text inputs).
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

// handleInputMetadata listens for metadata updates and schedules output updates.
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

// scheduleInputChangeUpdates schedules delayed updates for outputs affected by input changes.
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

		mr.timeline.addUpdate(update)

		if delay > 0 {
			slog.Debug("Scheduled update for output", "output", outputName, "time", executeAt.Format("15:04:05"), "delay_seconds", int(delay.Seconds()))
		} else {
			slog.Debug("Scheduled immediate update for output", "output", outputName)
		}
	}
}

func (mr *MetadataRouter) outputUsesInput(outputName string, inputName string) bool {
	inputNames, exists := mr.outputInputs[outputName]
	return exists && slices.Contains(inputNames, inputName)
}

func (mr *MetadataRouter) cancelScheduledUpdates(outputName string) {
	mr.timeline.cancelUpdatesForOutput(outputName)
}

// startExpirationChecker checks for expired metadata and schedules fallback updates.
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

// checkForExpirations schedules fallback updates for outputs whose current input has expired.
func (mr *MetadataRouter) checkForExpirations() {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

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
			formattedText := mr.formatMetadataForOutput(outputName, fallbackMetadata, fallbackInputName)
			if formattedText != "" {
				lastSent := mr.lastSentContent[outputName]
				if formattedText != lastSent {
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
			mr.currentInputs[outputName] = ""
			slog.Info("Output has no available inputs - cleared current input", "output", outputName)
		}
	}
}

// formatMetadataForOutput applies prefix/suffix and formatters to produce final output text.
func (mr *MetadataRouter) formatMetadataForOutput(outputName string, metadata *Metadata, inputName string) string {
	if metadata == nil {
		return ""
	}

	formattedText := metadata.FormatString()
	if formattedText == "" {
		return ""
	}

	if prefixSuffix, exists := mr.inputPrefixSuffix[inputName]; exists {
		if prefixSuffix.Prefix != "" {
			formattedText = prefixSuffix.Prefix + formattedText
		}
		if prefixSuffix.Suffix != "" {
			formattedText += prefixSuffix.Suffix
		}
	}

	outputFormatters, exists := mr.outputFormatters[outputName]
	if !exists {
		return formattedText
	}

	for _, formatter := range outputFormatters {
		formattedText = formatter.Format(formattedText)
	}

	return formattedText
}

// startTimelineProcessor runs a background loop that executes scheduled updates.
func (mr *MetadataRouter) startTimelineProcessor(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
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

// processReadyUpdates executes all scheduled updates whose time has arrived.
func (mr *MetadataRouter) processReadyUpdates() {
	now := time.Now()
	readyUpdates := mr.timeline.getReadyUpdates(now)

	var wg sync.WaitGroup
	for _, update := range readyUpdates {
		wg.Go(func() {
			mr.executeUpdate(update)
		})
	}
	wg.Wait()
}

// executeUpdate sends formatted metadata to an output if content has changed.
func (mr *MetadataRouter) executeUpdate(update ScheduledUpdate) {
	var inputName string
	var inputType string
	mr.mu.RLock()
	for name, input := range mr.inputs {
		if input.GetMetadata() != nil && input.GetMetadata().Name == update.Metadata.Name {
			inputName = name
			inputType = mr.inputTypes[name]
			break
		}
	}
	mr.mu.RUnlock()

	formattedText := mr.formatMetadataForOutput(update.OutputName, update.Metadata, inputName)
	if formattedText == "" {
		return
	}

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

	if enhancedOutput, ok := update.Output.(EnhancedOutput); ok {
		enhancedOutput.SendEnhancedMetadata(formattedText, update.Metadata, inputName, inputType)
	} else {
		update.Output.SendFormattedMetadata(formattedText)
	}
}

func (t *Timeline) addUpdate(update ScheduledUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	insertIndex, _ := slices.BinarySearchFunc(t.updates, update, func(a, b ScheduledUpdate) int {
		return a.ExecuteAt.Compare(b.ExecuteAt)
	})
	t.updates = slices.Insert(t.updates, insertIndex, update)
}

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
