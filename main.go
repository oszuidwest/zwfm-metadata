// Package main runs the metadata router.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/filters"
	"zwfm-metadata/formatters"
	"zwfm-metadata/inputs"
	"zwfm-metadata/outputs"
	"zwfm-metadata/utils"
	"zwfm-metadata/web"
)

func main() {
	configFile := flag.String("config", "config.json", "Path to configuration file")
	healthcheckURL := flag.String("healthcheck", "", "URL to check and exit")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *healthcheckURL != "" {
		if err := runHealthcheck(context.Background(), *healthcheckURL); err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("zwfm-metadata %s (commit: %s, built: %s)\n", utils.Version, utils.Commit, utils.BuildTime)
		os.Exit(0)
	}

	appConfig, err := config.LoadConfig(*configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if appConfig.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting metadata router", "station", appConfig.StationName, "version", utils.Version, "commit", utils.Commit)

	router := core.NewMetadataRouter()

	for _, inputCfg := range appConfig.Inputs {
		if err := setupInput(router, &inputCfg); err != nil {
			slog.Error("Input setup failed", "error", err)
			os.Exit(1)
		}
	}

	for _, outputCfg := range appConfig.Outputs {
		if err := setupOutput(router, &outputCfg); err != nil {
			slog.Error("Output setup failed", "error", err)
			os.Exit(1)
		}
	}

	server, err := web.NewServer(appConfig.WebServerPort, router, appConfig.StationName, appConfig.BrandColor)
	if err != nil {
		slog.Error("Failed to initialize web server", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start(ctx)
		stop()
	}()

	if err := router.Start(ctx); err != nil {
		slog.Error("Failed to start metadata router", "error", err)
		stop()
		<-serverDone
		os.Exit(1)
	}

	<-ctx.Done()
	stop()
	if err := <-serverDone; err != nil {
		slog.Error("Web server encountered an error", "error", err)
		os.Exit(1)
	}
	slog.Info("Shutting down...")
}

// setupInput builds and registers an input and its filters.
func setupInput(router *core.MetadataRouter, inputCfg *config.InputConfig) error {
	input, err := createInput(inputCfg)
	if err != nil {
		return fmt.Errorf("failed to create input %q: %w", inputCfg.Name, err)
	}

	spec := core.InputSpec{
		Type:   inputCfg.Type,
		Prefix: inputCfg.Prefix,
		Suffix: inputCfg.Suffix,
	}
	for i, filterCfg := range inputCfg.Filters {
		filter, err := filters.New(&filterCfg)
		if err != nil {
			return fmt.Errorf("failed to create %s filter for input %q (index %d): %w", filterCfg.Type, inputCfg.Name, i, err)
		}
		spec.Filters = append(spec.Filters, filter)
		spec.FilterNames = append(spec.FilterNames, filterCfg.Type)
	}

	if err := router.AddInput(input, spec); err != nil {
		return fmt.Errorf("failed to add input %q: %w", inputCfg.Name, err)
	}

	slog.Info("Added input", "name", inputCfg.Name, "type", inputCfg.Type, "prefix", inputCfg.Prefix, "suffix", inputCfg.Suffix)

	return nil
}

// setupOutput builds and registers an output and its formatters.
func setupOutput(router *core.MetadataRouter, outputCfg *config.OutputConfig) error {
	output, timing, err := createOutput(outputCfg)
	if err != nil {
		return fmt.Errorf("failed to create output %q: %w", outputCfg.Name, err)
	}

	spec := core.OutputSpec{
		Type:           outputCfg.Type,
		Inputs:         outputCfg.Inputs,
		FormatterNames: outputCfg.Formatters,
		Timing:         timing,
	}
	for _, formatterName := range outputCfg.Formatters {
		formatter, err := formatters.New(formatterName)
		if err != nil {
			return fmt.Errorf("failed to get formatter %q: %w", formatterName, err)
		}
		spec.Formatters = append(spec.Formatters, formatter)
	}

	if err := router.AddOutput(output, spec); err != nil {
		return fmt.Errorf("failed to add output %q: %w", outputCfg.Name, err)
	}

	slog.Info("Added output",
		"name", outputCfg.Name,
		"type", outputCfg.Type,
		"delay", timing.Delay,
		"fallbackDelay", timing.FallbackDelay,
	)

	return nil
}

func createInput(cfg *config.InputConfig) (core.Input, error) {
	switch cfg.Type {
	case "dynamic":
		settings, err := utils.ParseJSONSettings[config.DynamicInputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewDynamicInput(cfg.Name, settings)

	case "url":
		settings, err := utils.ParseJSONSettings[config.URLInputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewURLInput(cfg.Name, &settings)

	case "text":
		settings, err := utils.ParseJSONSettings[config.TextInputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return inputs.NewTextInput(cfg.Name, settings), nil

	default:
		return nil, fmt.Errorf("unknown type: %s", cfg.Type)
	}
}

func createOutput(cfg *config.OutputConfig) (core.Output, core.OutputTiming, error) {
	switch cfg.Type {
	case "icecast":
		settings, timing, err := parseOutputSettings[config.IcecastOutputConfig](cfg.Settings)
		if err != nil {
			return nil, core.OutputTiming{}, err
		}
		return outputs.NewIcecastOutput(cfg.Name, settings), timing, nil

	case "file":
		settings, timing, err := parseOutputSettings[config.FileOutputConfig](cfg.Settings)
		if err != nil {
			return nil, core.OutputTiming{}, err
		}
		return outputs.NewFileOutput(cfg.Name, settings), timing, nil

	case "url":
		settings, timing, err := parseOutputSettings[config.URLOutputConfig](cfg.Settings)
		if err != nil {
			return nil, core.OutputTiming{}, err
		}
		output, err := outputs.NewURLOutput(cfg.Name, settings)
		return output, timing, err

	case "dlplus":
		settings, timing, err := parseOutputSettings[config.DLPlusOutputConfig](cfg.Settings)
		if err != nil {
			return nil, core.OutputTiming{}, err
		}
		return outputs.NewDLPlusOutput(cfg.Name, settings), timing, nil

	case "websocket":
		settings, timing, err := parseOutputSettings[config.WebSocketOutputConfig](cfg.Settings)
		if err != nil {
			return nil, core.OutputTiming{}, err
		}
		output, err := outputs.NewWebSocketOutput(cfg.Name, settings)
		return output, timing, err

	case "http":
		settings, timing, err := parseOutputSettings[config.HTTPOutputConfig](cfg.Settings)
		if err != nil {
			return nil, core.OutputTiming{}, err
		}
		output, err := outputs.NewHTTPOutput(cfg.Name, settings)
		return output, timing, err

	case "stereotool":
		settings, timing, err := parseOutputSettings[config.StereoToolOutputConfig](cfg.Settings)
		if err != nil {
			return nil, core.OutputTiming{}, err
		}
		return outputs.NewStereoToolOutput(cfg.Name, settings), timing, nil

	default:
		return nil, core.OutputTiming{}, fmt.Errorf("unknown type: %s", cfg.Type)
	}
}

func parseOutputSettings[T any](settings json.RawMessage) (T, core.OutputTiming, error) {
	var zero T
	fields, err := utils.ParseJSONSettings[map[string]json.RawMessage](settings)
	if err != nil {
		return zero, core.OutputTiming{}, err
	}

	timingFields := make(map[string]json.RawMessage, 2)
	for _, key := range []string{"delay", "fallbackDelay"} {
		if value, exists := fields[key]; exists {
			timingFields[key] = value
			delete(fields, key)
		}
	}

	timingJSON, err := json.Marshal(timingFields)
	if err != nil {
		return zero, core.OutputTiming{}, fmt.Errorf("failed to prepare output timing: %w", err)
	}
	timing, err := utils.ParseJSONSettings[core.OutputTiming](timingJSON)
	if err != nil {
		return zero, core.OutputTiming{}, err
	}

	componentJSON, err := json.Marshal(fields)
	if err != nil {
		return zero, core.OutputTiming{}, fmt.Errorf("failed to prepare output settings: %w", err)
	}
	component, err := utils.ParseJSONSettings[T](componentJSON)
	if err != nil {
		return zero, core.OutputTiming{}, err
	}

	return component, timing, nil
}
