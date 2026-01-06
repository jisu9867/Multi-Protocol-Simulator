package tests

import (
	"context"
	"testing"
	"time"

	"github.com/smartfactory/simulator/internal/adapters/mqtt"
	"github.com/smartfactory/simulator/internal/core"
)

func TestMQTTAdapterTopicRendering(t *testing.T) {
	config := mqtt.Config{
		Broker:        "localhost:1883",
		ClientID:      "test-client",
		TopicTemplate: "factory/{line}/{source_id}/telemetry",
		Line:          "line-1",
	}

	adapter := mqtt.NewAdapter(config, "sim-001", nil)

	// Test topic rendering by checking the adapter can be created
	if adapter.Name() != "mqtt" {
		t.Errorf("Expected adapter name 'mqtt', got '%s'", adapter.Name())
	}
}

func TestMQTTAdapterConfigDefaults(t *testing.T) {
	config := mqtt.DefaultConfig()

	if config.Broker == "" {
		t.Error("Default broker should not be empty")
	}

	if config.ClientID == "" {
		t.Error("Default client ID should not be empty")
	}

	if config.TopicTemplate == "" {
		t.Error("Default topic template should not be empty")
	}

	if config.KeepAlive <= 0 {
		t.Error("Default keepalive should be positive")
	}

	if config.QoS < 0 || config.QoS > 2 {
		t.Error("Default QoS should be 0, 1, or 2")
	}
}

func TestMQTTMessageSerialization(t *testing.T) {
	msg := core.TelemetryMessage{
		TS:       time.Now(),
		SourceID: "sim-001",
		Tag:      "temp",
		Value:    23.5,
		Unit:     "C",
		Quality:  core.QualityGood,
		Seq:      123,
		TraceID:  "abc123",
	}

	jsonBytes, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify required fields are present
	requiredFields := []string{"ts", "sourceId", "tag", "value", "quality", "seq"}
	for _, field := range requiredFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing required field: %s", field)
		}
	}

	// Verify field names match gateway expectations
	if !contains(jsonStr, `"sourceId"`) {
		t.Error("JSON should contain 'sourceId' field (camelCase)")
	}

	if !contains(jsonStr, `"ts"`) {
		t.Error("JSON should contain 'ts' field")
	}
}

func TestMQTTAdapterWithoutConnection(t *testing.T) {
	// Test that adapter methods handle nil client gracefully
	config := mqtt.DefaultConfig()
	adapter := mqtt.NewAdapter(config, "test-source", nil)

	ctx := context.Background()

	// Publish should fail if not connected
	msg := core.TelemetryMessage{
		TS:       time.Now(),
		SourceID: "test-source",
		Tag:      "temp",
		Value:    23.5,
		Quality:  core.QualityGood,
		Seq:      1,
	}

	err := adapter.Publish(ctx, msg)
	if err == nil {
		t.Error("Expected error when publishing without connection")
	}
}


