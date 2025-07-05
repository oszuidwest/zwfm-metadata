package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
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

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.json", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version if requested
	if *showVersion {
		fmt.Printf("zwfm-metadata %s (commit: %s, built: %s)\n", utils.Version, utils.Commit, utils.BuildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Configure slog based on debug setting
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	// Log startup information
	slog.Info("Starting metadata router", "station", cfg.StationName, "version", utils.Version, "commit", utils.Commit)

	// Create metadata router
	router := core.NewMetadataRouter()

	// Create inputs
	for _, inputCfg := range cfg.Inputs {
		input, err := createInput(inputCfg)
		if err != nil {
			slog.Error("Failed to create input", "name", inputCfg.Name, "error", err)
			os.Exit(1)
		}

		if err := router.AddInput(input); err != nil {
			slog.Error("Failed to add input", "name", inputCfg.Name, "error", err)
			os.Exit(1)
		}

		// Store input type for status display
		router.SetInputType(inputCfg.Name, inputCfg.Type)

		// Configure prefix/suffix for this input
		if inputCfg.Prefix != "" || inputCfg.Suffix != "" {
			router.SetInputPrefixSuffix(inputCfg.Name, inputCfg.Prefix, inputCfg.Suffix)
			slog.Info("Added input", "name", inputCfg.Name, "type", inputCfg.Type, "prefix", inputCfg.Prefix, "suffix", inputCfg.Suffix)
		} else {
			slog.Info("Added input", "name", inputCfg.Name, "type", inputCfg.Type)
		}
	}

	// Create outputs
	for _, outputCfg := range cfg.Outputs {
		output, err := createOutput(outputCfg)
		if err != nil {
			slog.Error("Failed to create output", "name", outputCfg.Name, "error", err)
			os.Exit(1)
		}

		// Set inputs for output
		var outputInputs []core.Input
		for _, inputName := range outputCfg.Inputs {
			input, exists := router.GetInput(inputName)
			if !exists {
				slog.Error("Input not found for output", "input", inputName, "output", outputCfg.Name)
				os.Exit(1)
			}
			outputInputs = append(outputInputs, input)
		}
		output.SetInputs(outputInputs)

		// Register input mapping with timeline router
		router.SetOutputInputs(outputCfg.Name, outputCfg.Inputs)

		// Register formatters with timeline router
		var outputFormatters []formatters.Formatter
		for _, formatterName := range outputCfg.Formatters {
			formatter, err := formatters.GetFormatter(formatterName)
			if err != nil {
				slog.Error("Failed to get formatter", "formatter", formatterName, "error", err)
				os.Exit(1)
			}
			outputFormatters = append(outputFormatters, formatter)
		}
		router.SetOutputFormatters(outputCfg.Name, outputFormatters)
		router.SetOutputFormatterNames(outputCfg.Name, outputCfg.Formatters)

		if err := router.AddOutput(output); err != nil {
			slog.Error("Failed to add output", "name", outputCfg.Name, "error", err)
			os.Exit(1)
		}
		router.SetOutputType(outputCfg.Name, outputCfg.Type)

		slog.Info("Added output", "name", outputCfg.Name, "type", outputCfg.Type, "delay", output.GetDelay())
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Shutdown signal received")
		cancel()
	}()

	// Start web server
	server := web.NewServer(cfg.WebServerPort, router, cfg.StationName, cfg.BrandColor)
	go func() {
		if err := server.Start(ctx); err != nil {
			slog.Error("Web server encountered an error", "error", err)
		}
	}()

	// Start timeline router
	if err := router.Start(ctx); err != nil {
		slog.Error("Failed to start timeline router", "error", err)
		os.Exit(1)
	}

	// Wait for context cancellation
	<-ctx.Done()
	slog.Info("Shutting down...")
}

// createInput creates an input based on configuration
func createInput(cfg config.InputConfig) (core.Input, error) {
	switch cfg.Type {
	case "dynamic":
		settings, err := utils.ParseJSONSettings[config.DynamicInputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewDynamicInput(cfg.Name, *settings), nil

	case "url":
		settings, err := utils.ParseJSONSettings[config.URLInputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewURLInput(cfg.Name, *settings), nil

	case "text":
		settings, err := utils.ParseJSONSettings[config.TextInputConfig](cfg.Settings)
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
		settings, err := utils.ParseJSONSettings[config.IcecastOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewIcecastOutput(cfg.Name, *settings), nil

	case "file":
		settings, err := utils.ParseJSONSettings[config.FileOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewFileOutput(cfg.Name, *settings), nil

	case "post":
		settings, err := utils.ParseJSONSettings[config.PostOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewPostOutput(cfg.Name, *settings), nil

	case "dlsplus":
		settings, err := utils.ParseJSONSettings[config.DLSPlusOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewDLSPlusOutput(cfg.Name, *settings), nil

	case "websocket":
		settings, err := utils.ParseJSONSettings[config.WebSocketOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewWebSocketOutput(cfg.Name, *settings), nil

	case "http":
		settings, err := utils.ParseJSONSettings[config.HTTPOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewHTTPOutput(cfg.Name, *settings), nil

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
