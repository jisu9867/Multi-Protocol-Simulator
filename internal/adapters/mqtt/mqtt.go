package mqtt

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/smartfactory/simulator/internal/core"
)

// Adapter implements the core.Adapter interface for MQTT
type Adapter struct {
	config   Config
	client   mqtt.Client
	clientMu sync.RWMutex
	metrics  *core.Metrics
	sourceID string
}

// NewAdapter creates a new MQTT adapter
func NewAdapter(config Config, sourceID string, metrics *core.Metrics) *Adapter {
	return &Adapter{
		config:   config,
		metrics:  metrics,
		sourceID: sourceID,
	}
}

// Name returns the adapter name
func (a *Adapter) Name() string {
	return "mqtt"
}

// Connect establishes connection to MQTT broker
func (a *Adapter) Connect(ctx context.Context) error {
	// Ensure we have valid retry settings
	maxRetries := a.config.ReconnectMaxRetries
	if maxRetries <= 0 {
		maxRetries = 10 // Default to 10 retries
	}
	
	opts := mqtt.NewClientOptions()
	
	// Parse broker address
	broker := a.config.Broker
	if !strings.Contains(broker, ":") {
		if a.config.TLS {
			broker += ":8883"
		} else {
			broker += ":1883"
		}
	}
	
	brokerURL := fmt.Sprintf("tcp://%s", broker)
	opts.AddBroker(brokerURL)
	
	clientID := a.config.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("simulator-%d", time.Now().UnixNano())
	}
	opts.SetClientID(clientID)
	
	if a.config.Username != "" {
		opts.SetUsername(a.config.Username)
	}
	if a.config.Password != "" {
		opts.SetPassword(a.config.Password)
	}
	
	opts.SetKeepAlive(time.Duration(a.config.KeepAlive) * time.Second)
	opts.SetAutoReconnect(false) // We handle reconnection manually
	opts.SetConnectRetry(false)   // We handle retry manually
	opts.SetCleanSession(true)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetConnectTimeout(10 * time.Second)
	opts.SetWriteTimeout(10 * time.Second)
	
	// Connection lost handler
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		if metrics := a.metrics; metrics != nil {
			metrics.RecordReconnect()
		}
	})
	
	// Connected handler
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		// Connection successful
	})
	
	client := mqtt.NewClient(opts)
	
	// Connect with retry logic
	var lastErr error
	waitTime := a.config.ReconnectInitialWait
	if waitTime <= 0 {
		waitTime = 1 * time.Second
	}
	maxWait := a.config.ReconnectMaxWait
	if maxWait <= 0 {
		maxWait = 60 * time.Second
	}
	
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		log.Printf("[MQTT] Attempting connection %d/%d to %s...", i+1, maxRetries, brokerURL)
		token := client.Connect()
		connected := token.WaitTimeout(10 * time.Second) // Increase timeout
		
		if connected && token.Error() == nil {
			// Wait a bit for connection to fully establish
			time.Sleep(500 * time.Millisecond)
			
			// Verify connection is actually established
			if client.IsConnected() {
				// Successfully connected
				log.Printf("[MQTT] Successfully connected to %s", brokerURL)
				a.clientMu.Lock()
				a.client = client
				a.clientMu.Unlock()
				return nil
			} else {
				// Connection token succeeded but client is not actually connected
				lastErr = fmt.Errorf("MQTT connection token succeeded but client is not connected")
				log.Printf("[MQTT] Connection token succeeded but IsConnected() returned false")
			}
		} else {
			// Connection failed
			if token.Error() != nil {
				lastErr = fmt.Errorf("MQTT connection error: %v", token.Error())
				log.Printf("[MQTT] Connection error: %v", token.Error())
			} else {
				lastErr = fmt.Errorf("MQTT connection timeout to %s", brokerURL)
				log.Printf("[MQTT] Connection timeout to %s", brokerURL)
			}
		}
		
		// Don't wait after the last attempt
		if i < maxRetries-1 {
			// Exponential backoff
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
			
			if waitTime < maxWait {
				waitTime *= 2
				if waitTime > maxWait {
					waitTime = maxWait
				}
			}
		}
	}
	
	// Format error message properly
	if lastErr != nil {
		return fmt.Errorf("failed to connect to MQTT broker %s after %d retries: %v", brokerURL, maxRetries, lastErr)
	}
	return fmt.Errorf("failed to connect to MQTT broker %s after %d retries", brokerURL, maxRetries)
}

// Publish sends a telemetry message to MQTT broker
func (a *Adapter) Publish(ctx context.Context, msg core.TelemetryMessage) error {
	a.clientMu.RLock()
	client := a.client
	a.clientMu.RUnlock()
	
	if client == nil {
		return fmt.Errorf("MQTT client is nil")
	}
	
	if !client.IsConnected() {
		// Try to reconnect once
		a.clientMu.Lock()
		if a.client != nil && !a.client.IsConnected() {
			// Connection lost, try to reconnect
			log.Printf("[MQTT] Connection lost, attempting reconnect...")
			reconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := a.Connect(reconnectCtx); err != nil {
				a.clientMu.Unlock()
				return fmt.Errorf("MQTT client not connected and reconnect failed: %v", err)
			}
			client = a.client
		}
		a.clientMu.Unlock()
		
		if !client.IsConnected() {
			return fmt.Errorf("MQTT client not connected")
		}
	}
	
	// Render topic template
	topic := a.renderTopic(msg.SourceID)
	
	// Serialize message to JSON
	payload, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}
	
	// Double-check connection before publishing
	if !client.IsConnected() {
		return fmt.Errorf("MQTT client disconnected before publish")
	}
	
	// Publish
	log.Printf("[MQTT] Publishing to topic: %s", topic)
	token := client.Publish(topic, byte(a.config.QoS), a.config.Retain, payload)
	
	// Wait with context timeout
	done := make(chan bool, 1)
	go func() {
		token.Wait()
		done <- true
	}()
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		if token.Error() != nil {
			log.Printf("[MQTT] Publish error: %v", token.Error())
			return fmt.Errorf("publish failed: %w", token.Error())
		}
		log.Printf("[MQTT] Successfully published to %s", topic)
		return nil
	case <-time.After(5 * time.Second):
		log.Printf("[MQTT] Publish timeout")
		return fmt.Errorf("publish timeout")
	}
}

// Close closes the MQTT connection
func (a *Adapter) Close(ctx context.Context) error {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()
	
	if a.client != nil && a.client.IsConnected() {
		a.client.Disconnect(250) // 250ms wait
		a.client = nil
	}
	
	return nil
}

// renderTopic renders the topic template with actual values
func (a *Adapter) renderTopic(sourceID string) string {
	topic := a.config.TopicTemplate
	if topic == "" {
		topic = "factory/{line}/{source_id}/telemetry"
	}
	
	// Use sourceID from adapter if provided, otherwise use parameter
	actualSourceID := sourceID
	if actualSourceID == "" {
		actualSourceID = a.sourceID
	}
	if actualSourceID == "" {
		actualSourceID = "unknown"
	}
	
	line := a.config.Line
	if line == "" {
		line = "line-1"
	}
	
	topic = strings.ReplaceAll(topic, "{line}", line)
	topic = strings.ReplaceAll(topic, "{source_id}", actualSourceID)
	return topic
}

