package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smartfactory/simulator/internal/config"
	"github.com/smartfactory/simulator/internal/core"
	"github.com/smartfactory/simulator/internal/adapters/mqtt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "simulator",
	Short: "Smart Factory Protocol Simulator",
	Long:  "A protocol simulator for testing Smart Factory Integration Gateway",
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the simulator",
	Long:  "Start the simulator and begin publishing telemetry data",
	RunE:  runSimulator,
}

var validateConfigCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validate configuration file",
	Long:  "Validate the configuration file without running the simulator",
	RunE:  validateConfig,
}

var dryRunCmd = &cobra.Command{
	Use:   "dry-run",
	Short: "Dry run (generate messages without publishing)",
	Long:  "Generate messages without actually publishing them to test the generator",
	RunE:  dryRun,
}

var (
	configPath string
	adapter    string
	dryRunCount int
)

func init() {
	runCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file (required)")
	runCmd.Flags().StringVarP(&adapter, "adapter", "a", "", "Adapter to use (overrides config)")
	runCmd.MarkFlagRequired("config")

	validateConfigCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file (required)")
	validateConfigCmd.MarkFlagRequired("config")

	dryRunCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file (required)")
	dryRunCmd.Flags().IntVarP(&dryRunCount, "count", "n", 10, "Number of messages to generate")
	dryRunCmd.MarkFlagRequired("config")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateConfigCmd)
	rootCmd.AddCommand(dryRunCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runSimulator(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Log configuration for debugging
	log.Printf("Configuration loaded:")
	log.Printf("  RandomFactory: %v", cfg.Generator.RandomFactory)
	log.Printf("  RandomSource: %v", cfg.Generator.RandomSource)
	log.Printf("  FactoryID: %s", cfg.Generator.FactoryID)
	log.Printf("  SourceID: %s", cfg.Generator.SourceID)
	log.Printf("  Tags: %d", len(cfg.Generator.Tags))

	// Override adapter if specified via command line
	if adapter != "" {
		cfg.Adapter = adapter
	}

	// Create generator
	generator := core.NewGenerator(cfg.Generator)

	// Create adapter
	var adapterInstance core.Adapter
	metrics := core.NewMetrics()

	switch cfg.Adapter {
	case "mqtt":
		adapterInstance = mqtt.NewAdapter(cfg.MQTT, cfg.Generator.SourceID, metrics)
	default:
		return fmt.Errorf("unsupported adapter: %s", cfg.Adapter)
	}

	// Create engine
	engine := core.NewEngine(generator, adapterInstance, cfg.Engine)

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	// Register signals for graceful shutdown
	// Note: On Windows, go run may not properly forward Ctrl+C signals
	// Consider using a compiled executable (go build) for better signal handling
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start engine in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- engine.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v, shutting down gracefully...", sig)
		cancel()
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("engine error: %w", err)
		}
	}

	// Wait a bit for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	select {
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout reached")
	case <-time.After(2 * time.Second):
	}

	// Print final metrics
	finalMetrics := engine.GetMetrics()
	log.Println("\n=== Final Metrics ===")
	log.Printf("Sent Total: %d", finalMetrics.SentTotal)
	log.Printf("Failed Total: %d", finalMetrics.FailedTotal)
	log.Printf("Reconnect Total: %d", finalMetrics.ReconnectTotal)
	log.Printf("Run Duration: %v", finalMetrics.RunDuration)

	return nil
}

func validateConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	fmt.Println("Configuration is valid!")
	fmt.Printf("\nAdapter: %s\n", cfg.Adapter)
	fmt.Printf("Generator Source ID: %s\n", cfg.Generator.SourceID)
	fmt.Printf("Tags: %d\n", len(cfg.Generator.Tags))
	
	if cfg.Generator.SourceID == "" {
		fmt.Println("  source_id: (empty)")
	} else {
		fmt.Printf("  source_id: %s\n", cfg.Generator.SourceID)
	}
	
	if cfg.MQTT.TopicTemplate == "" {
		fmt.Println("  topic_template: (empty)")
	} else {
		fmt.Printf("  topic_template: %s\n", cfg.MQTT.TopicTemplate)
	}

	for i, tag := range cfg.Generator.Tags {
		fmt.Printf("\nTag %d:\n", i+1)
		fmt.Printf("  tag: %s\n", tag.Tag)
		fmt.Printf("  pattern: %s\n", tag.Pattern)
		fmt.Printf("  min: %f, max: %f\n", tag.Min, tag.Max)
		if tag.Unit != "" {
			fmt.Printf("  unit: %s\n", tag.Unit)
		}
	}

	fmt.Printf("\nEngine:\n")
	fmt.Printf("  rate_mode: %s\n", cfg.Engine.RateMode)
	fmt.Printf("  interval_ms: %d\n", cfg.Engine.IntervalMs)
	fmt.Printf("  queue_size: %d\n", cfg.Engine.QueueSize)

	if cfg.Adapter == "mqtt" {
		fmt.Printf("\nMQTT:\n")
		fmt.Printf("  broker: %s\n", cfg.MQTT.Broker)
		fmt.Printf("  client_id: %s\n", cfg.MQTT.ClientID)
		fmt.Printf("  topic_template: %s\n", cfg.MQTT.TopicTemplate)
	}

	return nil
}

func dryRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	generator := core.NewGenerator(cfg.Generator)

	fmt.Printf("Generating %d messages...\n\n", dryRunCount)

	ctx := context.Background()
	for i := 0; i < dryRunCount; i++ {
		msg, err := generator.Next(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate message: %w", err)
		}

		jsonBytes, err := msg.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize message: %w", err)
		}

		fmt.Printf("Message %d:\n", i+1)
		fmt.Println(string(jsonBytes))
		fmt.Println()
	}

	return nil
}

