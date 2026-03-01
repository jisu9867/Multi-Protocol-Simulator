package core

import (
	"encoding/json"
	"time"
)

// TelemetryMessage represents the standard telemetry message format
// This is protocol-independent and used across all adapters
type TelemetryMessage struct {
	TS            time.Time   `json:"ts"`
	SourceID      string      `json:"sourceId"`
	FactoryID     string      `json:"factoryId,omitempty"`     // Factory ID: Ulsan, Asan, Jeonju, Hwaseong
	EquipmentType string      `json:"equipmentType,omitempty"` // Equipment type (e.g., "Sensor", "Motor")
	EquipmentName string      `json:"equipmentName,omitempty"` // Equipment name (e.g., "Temperature Sensor 1")
	Tag           string      `json:"tag"`
	Value         interface{} `json:"value"` // number, string, or bool
	Unit          string      `json:"unit,omitempty"`
	Quality       string      `json:"quality"`
	Seq           int64       `json:"seq"`
	TraceID       string      `json:"traceId,omitempty"`
}

// ToJSON serializes the message to JSON bytes
func (m TelemetryMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// Quality constants
const (
	QualityGood    = "Good"
	QualityUncertain = "Uncertain"
	QualityBad     = "Bad"
	QualityUnknown = "Unknown"
)

