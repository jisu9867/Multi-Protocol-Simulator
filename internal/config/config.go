package config

import (
	"time"

	"github.com/smartfactory/simulator/internal/adapters/mqtt"
	"github.com/smartfactory/simulator/internal/core"
	"github.com/spf13/viper"
)

// Config represents the complete simulator configuration
type Config struct {
	Generator core.GeneratorConfig `yaml:"generator" json:"generator"`
	Engine    core.EngineConfig     `yaml:"engine" json:"engine"`
	MQTT      mqtt.Config           `yaml:"mqtt" json:"mqtt"`
	Adapter   string                `yaml:"adapter" json:"adapter"` // "mqtt", etc.
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}
	
	// Manually load values that might not unmarshal correctly
	if config.Generator.SourceID == "" {
		config.Generator.SourceID = v.GetString("generator.source_id")
	}
	if config.MQTT.TopicTemplate == "" {
		config.MQTT.TopicTemplate = v.GetString("mqtt.topic_template")
	}
	if config.MQTT.Line == "" {
		config.MQTT.Line = v.GetString("mqtt.line")
	}
	
	// Parse duration strings manually if needed
	if metricsIntervalStr := v.GetString("engine.metrics_interval"); metricsIntervalStr != "" {
		if d, err := time.ParseDuration(metricsIntervalStr); err == nil {
			config.Engine.MetricsInterval = d
		}
	}
	if reconnectMaxWaitStr := v.GetString("mqtt.reconnect_max_wait"); reconnectMaxWaitStr != "" {
		if d, err := time.ParseDuration(reconnectMaxWaitStr); err == nil {
			config.MQTT.ReconnectMaxWait = d
		}
	}
	if reconnectInitialWaitStr := v.GetString("mqtt.reconnect_initial_wait"); reconnectInitialWaitStr != "" {
		if d, err := time.ParseDuration(reconnectInitialWaitStr); err == nil {
			config.MQTT.ReconnectInitialWait = d
		}
	}
	
	// Set defaults
	setDefaults(&config)
	
	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults(cfg *Config) {
	// Engine defaults
	if cfg.Engine.RateMode == "" {
		cfg.Engine.RateMode = core.RateModeInterval
	}
	if cfg.Engine.IntervalMs == 0 && cfg.Engine.Rate == 0 {
		cfg.Engine.IntervalMs = 1000 // 1 second default
	}
	if cfg.Engine.QueueSize == 0 {
		cfg.Engine.QueueSize = 1000
	}
	if cfg.Engine.OverflowPolicy == "" {
		cfg.Engine.OverflowPolicy = core.OverflowDropOldest
	}
	if cfg.Engine.RetryCount == 0 {
		cfg.Engine.RetryCount = 3
	}
	if cfg.Engine.MetricsInterval == 0 {
		cfg.Engine.MetricsInterval = 10 * time.Second
	}
	
	// MQTT defaults
	if cfg.MQTT.Broker == "" {
		cfg.MQTT = mqtt.DefaultConfig()
	}
	if cfg.MQTT.ClientID == "" {
		cfg.MQTT.ClientID = "simulator"
	}
	if cfg.MQTT.ReconnectMaxRetries <= 0 {
		cfg.MQTT.ReconnectMaxRetries = 10
	}
	if cfg.MQTT.ReconnectInitialWait <= 0 {
		cfg.MQTT.ReconnectInitialWait = 1 * time.Second
	}
	if cfg.MQTT.ReconnectMaxWait <= 0 {
		cfg.MQTT.ReconnectMaxWait = 60 * time.Second
	}
	if cfg.MQTT.KeepAlive <= 0 {
		cfg.MQTT.KeepAlive = 60
	}
}

