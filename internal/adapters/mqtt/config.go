package mqtt

import "time"

// Config holds MQTT adapter configuration
type Config struct {
	Broker     string `yaml:"broker" json:"broker"`           // host:port
	Username   string `yaml:"username,omitempty" json:"username,omitempty"`
	Password   string `yaml:"password,omitempty" json:"password,omitempty"`
	ClientID   string `yaml:"client_id" json:"client_id"`
	TLS        bool   `yaml:"tls" json:"tls"`
	KeepAlive  int    `yaml:"keepalive" json:"keepalive"` // seconds
	QoS        int    `yaml:"qos" json:"qos"`             // 0, 1, or 2
	Retain     bool   `yaml:"retain" json:"retain"`
	TopicTemplate string `yaml:"topic_template" json:"topic_template"` // e.g., "factory/{line}/{source_id}/telemetry"
	Line       string `yaml:"line" json:"line"`           // Line identifier for topic template
	
	// Reconnect settings
	ReconnectMaxRetries int           `yaml:"reconnect_max_retries" json:"reconnect_max_retries"`
	ReconnectMaxWait    time.Duration `yaml:"reconnect_max_wait" json:"reconnect_max_wait"` // Maximum wait time
	ReconnectInitialWait time.Duration `yaml:"reconnect_initial_wait" json:"reconnect_initial_wait"` // Initial wait time
}

// DefaultConfig returns a default MQTT configuration
func DefaultConfig() Config {
	return Config{
		Broker:              "localhost:1883",
		ClientID:            "simulator",
		TLS:                 false,
		KeepAlive:           60,
		QoS:                 1,
		Retain:              false,
		TopicTemplate:       "factory/{line}/{source_id}/telemetry",
		Line:                "line-1",
		ReconnectMaxRetries: 10,
		ReconnectMaxWait:    60 * time.Second,
		ReconnectInitialWait: 1 * time.Second,
	}
}

