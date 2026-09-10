// Package main implements a metadata router for radio stations that manages
// multiple input sources and distributes formatted metadata to various outputs.
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

	go func() {
		if err := server.Start(ctx); err != nil {
			slog.Error("Web server encountered an error", "error", err)
		}
	}()

	if err := router.Start(ctx); err != nil {
		slog.Error("Failed to start metadata router", "error", err)
		stop()
		os.Exit(1)
	}

	<-ctx.Done()
	stop()
	slog.Info("Shutting down...")
}

// setupInput creates an input and its filters and registers them on the router.
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

	if err := router.AddInput(input, &spec); err != nil {
		return fmt.Errorf("failed to add input %q: %w", inputCfg.Name, err)
	}

	slog.Info("Added input", "name", inputCfg.Name, "type", inputCfg.Type, "prefix", inputCfg.Prefix, "suffix", inputCfg.Suffix)

	return nil
}

// setupOutput creates an output and its formatters and registers them on the router.
func setupOutput(router *core.MetadataRouter, outputCfg *config.OutputConfig) error {
	timing, err := utils.ParseJSONSettings[core.OutputTiming](outputCfg.Settings)
	if err != nil {
		return fmt.Errorf("failed to parse timing for output %q: %w", outputCfg.Name, err)
	}

	output, err := createOutput(outputCfg)
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

	if err := router.AddOutput(output, &spec); err != nil {
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

// createInput instantiates an input based on the configuration type.
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

// createOutput instantiates an output based on the configuration type.
func createOutput(cfg *config.OutputConfig) (core.Output, error) {
	switch cfg.Type {
	case "icecast":
		settings, err := utils.ParseJSONSettings[config.IcecastOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewIcecastOutput(cfg.Name, settings), nil

	case "file":
		settings, err := utils.ParseJSONSettings[config.FileOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewFileOutput(cfg.Name, settings), nil

	case "url":
		settings, err := utils.ParseJSONSettings[config.URLOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewURLOutput(cfg.Name, settings)

	case "dlplus":
		settings, err := utils.ParseJSONSettings[config.DLPlusOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewDLPlusOutput(cfg.Name, settings), nil

	case "websocket":
		settings, err := utils.ParseJSONSettings[config.WebSocketOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewWebSocketOutput(cfg.Name, settings)

	case "http":
		settings, err := utils.ParseJSONSettings[config.HTTPOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewHTTPOutput(cfg.Name, settings)

	case "stereotool":
		settings, err := utils.ParseJSONSettings[config.StereoToolOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewStereoToolOutput(cfg.Name, settings), nil

	default:
		return nil, fmt.Errorf("unknown type: %s", cfg.Type)
	}
}
