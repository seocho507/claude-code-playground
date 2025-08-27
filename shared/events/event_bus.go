package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event represents a domain event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
}

// Handler represents an event handler function
type Handler func(ctx context.Context, event Event) error

// EventBus provides pub/sub event system for microservices
type EventBus struct {
	client     *redis.Client
	namespace  string
	handlers   map[string][]Handler
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewEventBus creates a new event bus
func NewEventBus(client *redis.Client, serviceName string) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &EventBus{
		client:    client,
		namespace: fmt.Sprintf("events:%s", serviceName),
		handlers:  make(map[string][]Handler),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// channelKey generates a namespaced channel key
func (eb *EventBus) channelKey(eventType string) string {
	return fmt.Sprintf("%s:%s", eb.namespace, eventType)
}

// globalChannelKey generates a global channel key for cross-service events
func (eb *EventBus) globalChannelKey(eventType string) string {
	return fmt.Sprintf("events:global:%s", eventType)
}

// Publish publishes an event to a specific event type channel
func (eb *EventBus) Publish(ctx context.Context, event Event) error {
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Version == "" {
		event.Version = "1.0"
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to both service-specific and global channels
	channels := []string{
		eb.channelKey(event.Type),
		eb.globalChannelKey(event.Type),
	}

	for _, channel := range channels {
		if err := eb.client.Publish(ctx, channel, data).Err(); err != nil {
			return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
		}
	}

	log.Printf("📨 Published event: %s (type: %s, source: %s)", event.ID, event.Type, event.Source)
	return nil
}

// Subscribe subscribes to events of specific types
func (eb *EventBus) Subscribe(eventTypes ...string) error {
	channels := make([]string, 0, len(eventTypes)*2)
	
	// Subscribe to both local and global channels
	for _, eventType := range eventTypes {
		channels = append(channels, 
			eb.channelKey(eventType),
			eb.globalChannelKey(eventType),
		)
	}

	pubsub := eb.client.Subscribe(eb.ctx, channels...)
	
	eb.wg.Add(1)
	go eb.handleMessages(pubsub)
	
	log.Printf("🎧 Subscribed to event types: %v", eventTypes)
	return nil
}

// SubscribePattern subscribes to events using pattern matching
func (eb *EventBus) SubscribePattern(patterns ...string) error {
	namespacedPatterns := make([]string, 0, len(patterns)*2)
	
	for _, pattern := range patterns {
		namespacedPatterns = append(namespacedPatterns,
			fmt.Sprintf("%s:%s", eb.namespace, pattern),
			fmt.Sprintf("events:global:%s", pattern),
		)
	}

	pubsub := eb.client.PSubscribe(eb.ctx, namespacedPatterns...)
	
	eb.wg.Add(1)
	go eb.handleMessages(pubsub)
	
	log.Printf("🎧 Subscribed to event patterns: %v", patterns)
	return nil
}

// handleMessages processes incoming messages
func (eb *EventBus) handleMessages(pubsub *redis.PubSub) {
	defer eb.wg.Done()
	
	ch := pubsub.Channel()
	
	for {
		select {
		case <-eb.ctx.Done():
			pubsub.Close()
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("❌ Failed to unmarshal event: %v", err)
				continue
			}
			
			eb.handleEvent(event)
		}
	}
}

// handleEvent processes a single event
func (eb *EventBus) handleEvent(event Event) {
	eb.mu.RLock()
	handlers := eb.handlers[event.Type]
	eb.mu.RUnlock()
	
	if len(handlers) == 0 {
		return
	}
	
	log.Printf("📬 Received event: %s (type: %s, source: %s)", event.ID, event.Type, event.Source)
	
	for _, handler := range handlers {
		go func(h Handler) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			if err := h(ctx, event); err != nil {
				log.Printf("❌ Event handler error for %s: %v", event.ID, err)
			}
		}(handler)
	}
}

// RegisterHandler registers an event handler for specific event types
func (eb *EventBus) RegisterHandler(eventType string, handler Handler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	log.Printf("🔧 Registered handler for event type: %s", eventType)
}

// Close closes the event bus
func (eb *EventBus) Close() error {
	eb.cancel()
	eb.wg.Wait()
	return nil
}

// Predefined Event Types for microservices communication
const (
	// User Events
	UserCreated      = "user.created"
	UserUpdated      = "user.updated"
	UserDeleted      = "user.deleted"
	UserLoggedIn     = "user.logged_in"
	UserLoggedOut    = "user.logged_out"
	UserPasswordChanged = "user.password_changed"
	
	// Auth Events
	TokenIssued      = "auth.token_issued"
	TokenRevoked     = "auth.token_revoked"
	TokenRefreshed   = "auth.token_refreshed"
	SessionCreated   = "auth.session_created"
	SessionExpired   = "auth.session_expired"
	
	// System Events
	ServiceStarted   = "system.service_started"
	ServiceStopped   = "system.service_stopped"
	HealthCheck      = "system.health_check"
	
	// Cache Events
	CacheInvalidated = "cache.invalidated"
	CacheWarmed      = "cache.warmed"
)

// Helper functions for creating common events

// NewUserEvent creates a user-related event
func NewUserEvent(eventType, source string, userID string, userData interface{}) Event {
	return Event{
		Type:   eventType,
		Source: source,
		Data:   userData,
		Metadata: map[string]interface{}{
			"user_id": userID,
		},
	}
}

// NewAuthEvent creates an auth-related event
func NewAuthEvent(eventType, source string, userID, sessionID string, authData interface{}) Event {
	return Event{
		Type:   eventType,
		Source: source,
		Data:   authData,
		Metadata: map[string]interface{}{
			"user_id":    userID,
			"session_id": sessionID,
		},
	}
}

// NewSystemEvent creates a system-related event
func NewSystemEvent(eventType, source string, systemData interface{}) Event {
	return Event{
		Type:   eventType,
		Source: source,
		Data:   systemData,
	}
}

// Event Router for complex event processing
type EventRouter struct {
	bus     *EventBus
	routes  map[string][]RouteHandler
	mu      sync.RWMutex
}

// RouteHandler represents a route-specific handler
type RouteHandler struct {
	Condition func(Event) bool
	Handler   Handler
}

// NewEventRouter creates a new event router
func NewEventRouter(bus *EventBus) *EventRouter {
	return &EventRouter{
		bus:    bus,
		routes: make(map[string][]RouteHandler),
	}
}

// AddRoute adds a conditional route for event processing
func (er *EventRouter) AddRoute(eventType string, condition func(Event) bool, handler Handler) {
	er.mu.Lock()
	defer er.mu.Unlock()
	
	er.routes[eventType] = append(er.routes[eventType], RouteHandler{
		Condition: condition,
		Handler:   handler,
	})
	
	// Register with event bus if first route for this type
	if len(er.routes[eventType]) == 1 {
		er.bus.RegisterHandler(eventType, er.routeEvent)
	}
}

// routeEvent routes events based on conditions
func (er *EventRouter) routeEvent(ctx context.Context, event Event) error {
	er.mu.RLock()
	routes := er.routes[event.Type]
	er.mu.RUnlock()
	
	for _, route := range routes {
		if route.Condition == nil || route.Condition(event) {
			if err := route.Handler(ctx, event); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// Convenience methods for common routing conditions

// RouteBySource routes events by source service
func (er *EventRouter) RouteBySource(eventType, source string, handler Handler) {
	er.AddRoute(eventType, func(e Event) bool {
		return e.Source == source
	}, handler)
}

// RouteByMetadata routes events by metadata values
func (er *EventRouter) RouteByMetadata(eventType, key string, value interface{}, handler Handler) {
	er.AddRoute(eventType, func(e Event) bool {
		if e.Metadata == nil {
			return false
		}
		return e.Metadata[key] == value
	}, handler)
}

// RouteByDataField routes events by data field values (using reflection)
func (er *EventRouter) RouteByDataField(eventType, field string, value interface{}, handler Handler) {
	er.AddRoute(eventType, func(e Event) bool {
		if e.Data == nil {
			return false
		}
		
		v := reflect.ValueOf(e.Data)
		if v.Kind() == reflect.Map {
			mapValue := v.Interface().(map[string]interface{})
			return mapValue[field] == value
		}
		
		// Handle struct fields
		if v.Kind() == reflect.Struct {
			fieldValue := v.FieldByName(field)
			if fieldValue.IsValid() {
				return fieldValue.Interface() == value
			}
		}
		
		return false
	}, handler)
}