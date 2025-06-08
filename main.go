package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/formatters"
	"zwfm-metadata/inputs"
	"zwfm-metadata/outputs"
	"zwfm-metadata/utils"
	"zwfm-metadata/web"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		utils.LogFatal("Failed to load configuration: %v", err)
	}

	// Set debug logging based on config
	utils.SetDebug(cfg.Debug)

	// Log startup information
	utils.LogInfo("Starting ZuidWest FM Metadata %s (commit: %s)", Version, Commit)

	// Create timeline manager
	manager := core.NewManager()

	// Create inputs
	for _, inputCfg := range cfg.Inputs {
		input, err := createInput(inputCfg)
		if err != nil {
			utils.LogFatal("Failed to create input %s: %v", inputCfg.Name, err)
		}

		if err := manager.AddInput(input); err != nil {
			utils.LogFatal("Failed to add input %s: %v", inputCfg.Name, err)
		}

		// Store input type for status display
		manager.SetInputType(inputCfg.Name, inputCfg.Type)

		// Configure prefix/suffix for this input
		if inputCfg.Prefix != "" || inputCfg.Suffix != "" {
			manager.SetInputPrefixSuffix(inputCfg.Name, inputCfg.Prefix, inputCfg.Suffix)
			utils.LogInfo("Added input: %s (type: %s, prefix: %q, suffix: %q)", inputCfg.Name, inputCfg.Type, inputCfg.Prefix, inputCfg.Suffix)
		} else {
			utils.LogInfo("Added input: %s (type: %s)", inputCfg.Name, inputCfg.Type)
		}
	}

	// Create outputs
	for _, outputCfg := range cfg.Outputs {
		output, err := createOutput(outputCfg)
		if err != nil {
			utils.LogFatal("Failed to create output %s: %v", outputCfg.Name, err)
		}

		// Set inputs for output
		var outputInputs []core.Input
		for _, inputName := range outputCfg.Inputs {
			input, exists := manager.GetInput(inputName)
			if !exists {
				utils.LogFatal("Input %s not found for output %s", inputName, outputCfg.Name)
			}
			outputInputs = append(outputInputs, input)
		}
		output.SetInputs(outputInputs)

		// Register input mapping with timeline manager
		manager.SetOutputInputs(outputCfg.Name, outputCfg.Inputs)

		// Register formatters with timeline manager
		var outputFormatters []formatters.Formatter
		for _, formatterName := range outputCfg.Formatters {
			formatter, err := formatters.GetFormatter(formatterName)
			if err != nil {
				utils.LogFatal("Failed to get formatter %s: %v", formatterName, err)
			}
			outputFormatters = append(outputFormatters, formatter)
		}
		manager.SetOutputFormatters(outputCfg.Name, outputFormatters)
		manager.SetOutputFormatterNames(outputCfg.Name, outputCfg.Formatters)

		if err := manager.AddOutput(output); err != nil {
			utils.LogFatal("Failed to add output %s: %v", outputCfg.Name, err)
		}
		manager.SetOutputType(outputCfg.Name, outputCfg.Type)

		utils.LogInfo("Added output: %s (type: %s, delay: %ds)", outputCfg.Name, outputCfg.Type, output.GetDelay())
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		utils.LogInfo("Shutdown signal received")
		cancel()
	}()

	// Start web server
	server := web.NewServer(cfg.WebServerPort, manager)
	go func() {
		if err := server.Start(ctx); err != nil {
			utils.LogError("Web server encountered an error: %v", err)
		}
	}()

	// Start timeline manager
	if err := manager.Start(ctx); err != nil {
		utils.LogFatal("Failed to start timeline manager: %v", err)
	}

	// Wait for context cancellation
	<-ctx.Done()
	utils.LogInfo("Shutting down...")
}

// createInput creates an input based on configuration
func createInput(cfg config.InputConfig) (core.Input, error) {
	switch cfg.Type {
	case "dynamic":
		settings, err := utils.ParseJSONSettings[config.DynamicInputSettings](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewDynamicInput(cfg.Name, *settings), nil

	case "url":
		settings, err := utils.ParseJSONSettings[config.URLInputSettings](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewURLInput(cfg.Name, *settings), nil

	case "text":
		settings, err := utils.ParseJSONSettings[config.TextInputSettings](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewTextInput(cfg.Name, *settings), nil

	default:
		return nil, &unknownTypeError{Type: cfg.Type}
	}
}

// createOutput creates an output based on configuration
func createOutput(cfg config.OutputConfig) (core.Output, error) {
	switch cfg.Type {
	case "icecast":
		settings, err := utils.ParseJSONSettings[config.IcecastOutputSettings](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewIcecastOutput(cfg.Name, *settings), nil

	case "file":
		settings, err := utils.ParseJSONSettings[config.FileOutputSettings](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewFileOutput(cfg.Name, *settings), nil

	case "post":
		settings, err := utils.ParseJSONSettings[config.PostOutputSettings](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewPostOutput(cfg.Name, *settings), nil

	default:
		return nil, &unknownTypeError{Type: cfg.Type}
	}
}

// unknownTypeError represents an unknown type error
type unknownTypeError struct {
	Type string
}

func (e *unknownTypeError) Error() string {
	return "unknown type: " + e.Type
}
