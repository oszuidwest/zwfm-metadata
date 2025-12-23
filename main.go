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
	"zwfm-metadata/formatters"
	"zwfm-metadata/inputs"
	"zwfm-metadata/outputs"
	"zwfm-metadata/utils"
	"zwfm-metadata/web"
)

func main() {
	configFile := flag.String("config", "config.json", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

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
		input, err := createInput(inputCfg)
		if err != nil {
			slog.Error("Failed to create input", "name", inputCfg.Name, "error", err)
			os.Exit(1)
		}

		if err := router.AddInput(input); err != nil {
			slog.Error("Failed to add input", "name", inputCfg.Name, "error", err)
			os.Exit(1)
		}

		router.SetInputType(inputCfg.Name, inputCfg.Type)

		if inputCfg.Prefix != "" || inputCfg.Suffix != "" {
			router.SetInputPrefixSuffix(inputCfg.Name, inputCfg.Prefix, inputCfg.Suffix)
			slog.Info("Added input", "name", inputCfg.Name, "type", inputCfg.Type, "prefix", inputCfg.Prefix, "suffix", inputCfg.Suffix)
		} else {
			slog.Info("Added input", "name", inputCfg.Name, "type", inputCfg.Type)
		}
	}

	for _, outputCfg := range appConfig.Outputs {
		output, err := createOutput(&outputCfg)
		if err != nil {
			slog.Error("Failed to create output", "name", outputCfg.Name, "error", err)
			os.Exit(1)
		}

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
		router.SetOutputInputs(outputCfg.Name, outputCfg.Inputs)

		var outputFormatters []core.Formatter
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

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Shutdown signal received")
		cancel()
	}()

	server, err := web.NewServer(appConfig.WebServerPort, router, appConfig.StationName, appConfig.BrandColor)
	if err != nil {
		slog.Error("Failed to initialize web server", "error", err)
		os.Exit(1)
	}
	go func() {
		if err := server.Start(ctx); err != nil {
			slog.Error("Web server encountered an error", "error", err)
		}
	}()

	if err := router.Start(ctx); err != nil {
		slog.Error("Failed to start timeline router", "error", err)
		cancel() // Cancel context before exiting
		os.Exit(1)
	}

	<-ctx.Done()
	slog.Info("Shutting down...")
}

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
		return inputs.NewURLInput(cfg.Name, settings), nil

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
		return outputs.NewFileOutput(cfg.Name, *settings), nil

	case "url":
		settings, err := utils.ParseJSONSettings[config.URLOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewURLOutput(cfg.Name, *settings), nil

	case "dlplus":
		settings, err := utils.ParseJSONSettings[config.DLPlusOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewDLPlusOutput(cfg.Name, *settings), nil

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

	case "stereotool":
		settings, err := utils.ParseJSONSettings[config.StereoToolOutputConfig](cfg.Settings)
		if err != nil {
			return nil, err
		}
		return outputs.NewStereoToolOutput(cfg.Name, *settings), nil

	default:
		return nil, &unknownTypeError{Type: cfg.Type}
	}
}

type unknownTypeError struct {
	Type string
}

func (e *unknownTypeError) Error() string {
	return "unknown type: " + e.Type
}
