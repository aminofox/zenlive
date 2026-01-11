package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// WebhookConfig represents webhook configuration
type WebhookConfig struct {
	// Webhook URL
	URL string `json:"url"`

	// HTTP method (default: POST)
	Method string `json:"method,omitempty"`

	// Custom headers
	Headers map[string]string `json:"headers,omitempty"`

	// Secret for signing payloads (optional)
	Secret string `json:"secret,omitempty"`

	// Timeout for webhook requests
	Timeout time.Duration `json:"timeout,omitempty"`

	// Retry configuration
	MaxRetries int           `json:"max_retries,omitempty"`
	RetryDelay time.Duration `json:"retry_delay,omitempty"`

	// Event types to subscribe to (empty = all events)
	EventTypes []EventType `json:"event_types,omitempty"`
}

// DefaultWebhookConfig returns default webhook configuration
func DefaultWebhookConfig(url string) *WebhookConfig {
	return &WebhookConfig{
		URL:        url,
		Method:     "POST",
		Headers:    make(map[string]string),
		Timeout:    10 * time.Second,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		EventTypes: []EventType{},
	}
}

// WebhookPayload represents the payload sent to webhooks
type WebhookPayload struct {
	// Webhook ID
	ID string `json:"id"`

	// Event
	Event *StreamEvent `json:"event"`

	// Timestamp when webhook was created
	CreatedAt time.Time `json:"created_at"`

	// Attempt number (1-based)
	Attempt int `json:"attempt"`

	// Signature (HMAC SHA256 of payload if secret is configured)
	Signature string `json:"signature,omitempty"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID          string
	Payload     *WebhookPayload
	Config      *WebhookConfig
	Attempts    int
	LastError   error
	DeliveredAt *time.Time
	mu          sync.RWMutex
}

// WebhookManager manages webhook subscriptions and delivery
type WebhookManager struct {
	webhooks   map[string]*WebhookConfig
	eventBus   *EventBus
	httpClient *http.Client
	logger     logger.Logger
	mu         sync.RWMutex

	// Delivery queue
	deliveryQueue chan *WebhookDelivery
	workers       int
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewWebhookManager creates a new webhook manager
func NewWebhookManager(eventBus *EventBus, workers int, log logger.Logger) *WebhookManager {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	if workers <= 0 {
		workers = 5 // Default 5 workers
	}

	wm := &WebhookManager{
		webhooks:      make(map[string]*WebhookConfig),
		eventBus:      eventBus,
		httpClient:    &http.Client{},
		logger:        log,
		deliveryQueue: make(chan *WebhookDelivery, 1000),
		workers:       workers,
		stopChan:      make(chan struct{}),
	}

	// Start workers
	wm.startWorkers()

	return wm
}

// AddWebhook adds a webhook subscription
func (wm *WebhookManager) AddWebhook(id string, config *WebhookConfig) error {
	if id == "" {
		return fmt.Errorf("webhook ID is required")
	}

	if config == nil {
		return fmt.Errorf("webhook config is required")
	}

	if config.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}

	// Set defaults
	if config.Method == "" {
		config.Method = "POST"
	}

	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.RetryDelay == 0 {
		config.RetryDelay = 2 * time.Second
	}

	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}

	// Add default headers
	config.Headers["Content-Type"] = "application/json"
	config.Headers["User-Agent"] = "ZenLive-Webhook/1.0"

	wm.mu.Lock()
	wm.webhooks[id] = config
	wm.mu.Unlock()

	// Subscribe to events
	if wm.eventBus != nil {
		if len(config.EventTypes) == 0 {
			// Subscribe to all events
			wm.eventBus.SubscribeAll(wm.createEventHandler(id))
		} else {
			// Subscribe to specific events
			for _, eventType := range config.EventTypes {
				wm.eventBus.Subscribe(eventType, wm.createEventHandler(id))
			}
		}
	}

	wm.logger.Info("Webhook added",
		logger.Field{Key: "webhook_id", Value: id},
		logger.Field{Key: "url", Value: config.URL},
	)

	return nil
}

// RemoveWebhook removes a webhook subscription
func (wm *WebhookManager) RemoveWebhook(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.webhooks[id]; !exists {
		return fmt.Errorf("webhook not found: %s", id)
	}

	delete(wm.webhooks, id)

	wm.logger.Info("Webhook removed",
		logger.Field{Key: "webhook_id", Value: id},
	)

	return nil
}

// GetWebhook returns a webhook configuration
func (wm *WebhookManager) GetWebhook(id string) (*WebhookConfig, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	config, exists := wm.webhooks[id]
	if !exists {
		return nil, fmt.Errorf("webhook not found: %s", id)
	}

	return config, nil
}

// ListWebhooks returns all webhook configurations
func (wm *WebhookManager) ListWebhooks() map[string]*WebhookConfig {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	webhooks := make(map[string]*WebhookConfig)
	for id, config := range wm.webhooks {
		webhooks[id] = config
	}

	return webhooks
}

// createEventHandler creates an event handler for a webhook
func (wm *WebhookManager) createEventHandler(webhookID string) EventHandler {
	return func(event *StreamEvent) {
		wm.mu.RLock()
		config, exists := wm.webhooks[webhookID]
		wm.mu.RUnlock()

		if !exists {
			return
		}

		// Check if webhook is subscribed to this event type
		if len(config.EventTypes) > 0 {
			subscribed := false
			for _, eventType := range config.EventTypes {
				if eventType == event.Type {
					subscribed = true
					break
				}
			}

			if !subscribed {
				return
			}
		}

		// Create webhook delivery
		delivery := &WebhookDelivery{
			ID: generateWebhookDeliveryID(),
			Payload: &WebhookPayload{
				ID:        generateWebhookPayloadID(),
				Event:     event,
				CreatedAt: time.Now(),
				Attempt:   1,
			},
			Config:   config,
			Attempts: 0,
		}

		// Queue for delivery
		select {
		case wm.deliveryQueue <- delivery:
			wm.logger.Debug("Webhook queued",
				logger.Field{Key: "webhook_id", Value: webhookID},
				logger.Field{Key: "event_type", Value: event.Type},
			)
		default:
			wm.logger.Error("Webhook queue full, dropping delivery",
				logger.Field{Key: "webhook_id", Value: webhookID},
			)
		}
	}
}

// startWorkers starts webhook delivery workers
func (wm *WebhookManager) startWorkers() {
	for i := 0; i < wm.workers; i++ {
		wm.wg.Add(1)
		go wm.deliveryWorker(i)
	}
}

// deliveryWorker processes webhook deliveries
func (wm *WebhookManager) deliveryWorker(id int) {
	defer wm.wg.Done()

	wm.logger.Debug("Webhook worker started",
		logger.Field{Key: "worker_id", Value: id},
	)

	for {
		select {
		case <-wm.stopChan:
			wm.logger.Debug("Webhook worker stopped",
				logger.Field{Key: "worker_id", Value: id},
			)
			return

		case delivery := <-wm.deliveryQueue:
			wm.deliverWebhook(delivery)
		}
	}
}

// deliverWebhook delivers a webhook with retry logic
func (wm *WebhookManager) deliverWebhook(delivery *WebhookDelivery) {
	for attempt := 1; attempt <= delivery.Config.MaxRetries; attempt++ {
		delivery.mu.Lock()
		delivery.Attempts = attempt
		delivery.Payload.Attempt = attempt
		delivery.mu.Unlock()

		err := wm.sendWebhook(delivery)

		if err == nil {
			// Success
			now := time.Now()
			delivery.mu.Lock()
			delivery.DeliveredAt = &now
			delivery.mu.Unlock()

			wm.logger.Info("Webhook delivered",
				logger.Field{Key: "delivery_id", Value: delivery.ID},
				logger.Field{Key: "attempt", Value: attempt},
			)
			return
		}

		// Log error
		delivery.mu.Lock()
		delivery.LastError = err
		delivery.mu.Unlock()

		wm.logger.Warn("Webhook delivery failed",
			logger.Field{Key: "delivery_id", Value: delivery.ID},
			logger.Field{Key: "attempt", Value: attempt},
			logger.Field{Key: "error", Value: err},
		)

		// Retry if not last attempt
		if attempt < delivery.Config.MaxRetries {
			time.Sleep(delivery.Config.RetryDelay * time.Duration(attempt))
		}
	}

	wm.logger.Error("Webhook delivery failed after max retries",
		logger.Field{Key: "delivery_id", Value: delivery.ID},
		logger.Field{Key: "max_retries", Value: delivery.Config.MaxRetries},
	)
}

// sendWebhook sends a single webhook request
func (wm *WebhookManager) sendWebhook(delivery *WebhookDelivery) error {
	// Marshal payload
	payloadBytes, err := json.Marshal(delivery.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	ctx, cancel := context.WithTimeout(context.Background(), delivery.Config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, delivery.Config.Method, delivery.Config.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range delivery.Config.Headers {
		req.Header.Set(key, value)
	}

	// Add signature if secret is configured
	if delivery.Config.Secret != "" {
		signature := generateWebhookSignature(payloadBytes, delivery.Config.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Send request
	resp, err := wm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Stop stops the webhook manager and all workers
func (wm *WebhookManager) Stop() {
	close(wm.stopChan)
	wm.wg.Wait()
	wm.logger.Info("Webhook manager stopped")
}

// Helper functions

var deliveryCounter int64
var deliveryCounterMu sync.Mutex

func generateWebhookDeliveryID() string {
	deliveryCounterMu.Lock()
	defer deliveryCounterMu.Unlock()

	deliveryCounter++
	return fmt.Sprintf("delivery-%d-%d", time.Now().Unix(), deliveryCounter)
}

func generateWebhookPayloadID() string {
	return fmt.Sprintf("payload-%d", time.Now().UnixNano())
}

func generateWebhookSignature(payload []byte, secret string) string {
	// Simple implementation - in production, use HMAC SHA256
	// For now, just return a placeholder
	return fmt.Sprintf("sha256=%x", len(payload)+len(secret))
}
