package core

import (
	"context"
)

// Adapter is the interface that all protocol adapters must implement
// This allows adding new protocols by simply implementing this interface
type Adapter interface {
	// Name returns the adapter name (e.g., "mqtt", "opcua")
	Name() string

	// Connect establishes a connection to the protocol endpoint
	Connect(ctx context.Context) error

	// Publish sends a telemetry message
	Publish(ctx context.Context, msg TelemetryMessage) error

	// Close closes the connection gracefully
	Close(ctx context.Context) error
}

