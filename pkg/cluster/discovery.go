package cluster

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ServiceStatus represents the status of a service
type ServiceStatus string

const (
	// ServiceStatusHealthy indicates the service is healthy
	ServiceStatusHealthy ServiceStatus = "healthy"
	// ServiceStatusDegraded indicates the service is degraded
	ServiceStatusDegraded ServiceStatus = "degraded"
	// ServiceStatusUnhealthy indicates the service is unhealthy
	ServiceStatusUnhealthy ServiceStatus = "unhealthy"
)

// ServiceInfo represents information about a service
type ServiceInfo struct {
	ID              string                 // Unique service identifier
	Name            string                 // Service name
	Address         string                 // Service address (host:port)
	NodeID          string                 // Node ID where service is running
	Status          ServiceStatus          // Current status
	Version         string                 // Service version
	StartTime       time.Time              // Service start time
	LastHealthCheck time.Time              // Last health check timestamp
	Metadata        map[string]interface{} // Additional metadata
	Tags            []string               // Service tags
}

// ServiceDiscovery manages service registration and discovery
type ServiceDiscovery interface {
	// Register registers a service
	Register(ctx context.Context, service *ServiceInfo) error

	// Deregister removes a service registration
	Deregister(ctx context.Context, serviceID string) error

	// GetService retrieves a service by ID
	GetService(ctx context.Context, serviceID string) (*ServiceInfo, error)

	// GetServices returns all services
	GetServices(ctx context.Context) ([]*ServiceInfo, error)

	// GetServicesByName returns all services with a specific name
	GetServicesByName(ctx context.Context, name string) ([]*ServiceInfo, error)

	// GetHealthyServices returns all healthy services
	GetHealthyServices(ctx context.Context) ([]*ServiceInfo, error)

	// UpdateServiceStatus updates the status of a service
	UpdateServiceStatus(ctx context.Context, serviceID string, status ServiceStatus) error

	// Watch watches for service changes
	Watch(ctx context.Context) (<-chan ServiceEvent, error)
}

// ServiceEvent represents a service change event
type ServiceEvent struct {
	Type    ServiceEventType // Event type
	Service *ServiceInfo     // Service information
}

// ServiceEventType represents the type of service event
type ServiceEventType string

const (
	// ServiceEventRegistered indicates a service was registered
	ServiceEventRegistered ServiceEventType = "registered"
	// ServiceEventDeregistered indicates a service was deregistered
	ServiceEventDeregistered ServiceEventType = "deregistered"
	// ServiceEventUpdated indicates a service was updated
	ServiceEventUpdated ServiceEventType = "updated"
	// ServiceEventHealthChanged indicates service health changed
	ServiceEventHealthChanged ServiceEventType = "health_changed"
)

// InMemoryServiceDiscovery implements ServiceDiscovery using in-memory storage
type InMemoryServiceDiscovery struct {
	services      map[string]*ServiceInfo
	nameIndex     map[string][]string // name -> serviceIDs
	nodeIndex     map[string][]string // nodeID -> serviceIDs
	watchers      []chan ServiceEvent
	healthChecker *ServiceHealthChecker
	mu            sync.RWMutex
}

// NewInMemoryServiceDiscovery creates a new in-memory service discovery
func NewInMemoryServiceDiscovery() *InMemoryServiceDiscovery {
	sd := &InMemoryServiceDiscovery{
		services:  make(map[string]*ServiceInfo),
		nameIndex: make(map[string][]string),
		nodeIndex: make(map[string][]string),
		watchers:  make([]chan ServiceEvent, 0),
	}

	// Create health checker
	sd.healthChecker = NewServiceHealthChecker(sd, 30*time.Second, func(service *ServiceInfo) ServiceStatus {
		// Default health check - just check if service is still registered
		if time.Since(service.LastHealthCheck) > 2*time.Minute {
			return ServiceStatusUnhealthy
		}
		return ServiceStatusHealthy
	})

	return sd
}

// Register registers a service
func (sd *InMemoryServiceDiscovery) Register(ctx context.Context, service *ServiceInfo) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	if service.ID == "" {
		return errors.New("service ID cannot be empty")
	}

	service.StartTime = time.Now()
	service.LastHealthCheck = time.Now()
	if service.Status == "" {
		service.Status = ServiceStatusHealthy
	}

	sd.services[service.ID] = service

	// Update name index
	if service.Name != "" {
		sd.nameIndex[service.Name] = append(sd.nameIndex[service.Name], service.ID)
	}

	// Update node index
	if service.NodeID != "" {
		sd.nodeIndex[service.NodeID] = append(sd.nodeIndex[service.NodeID], service.ID)
	}

	// Notify watchers
	sd.notifyWatchers(ServiceEvent{
		Type:    ServiceEventRegistered,
		Service: service,
	})

	return nil
}

// Deregister removes a service registration
func (sd *InMemoryServiceDiscovery) Deregister(ctx context.Context, serviceID string) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	service, exists := sd.services[serviceID]
	if !exists {
		return errors.New("service not found")
	}

	delete(sd.services, serviceID)

	// Remove from name index
	if service.Name != "" {
		sd.removeFromIndex(sd.nameIndex, service.Name, serviceID)
	}

	// Remove from node index
	if service.NodeID != "" {
		sd.removeFromIndex(sd.nodeIndex, service.NodeID, serviceID)
	}

	// Notify watchers
	sd.notifyWatchers(ServiceEvent{
		Type:    ServiceEventDeregistered,
		Service: service,
	})

	return nil
}

// GetService retrieves a service by ID
func (sd *InMemoryServiceDiscovery) GetService(ctx context.Context, serviceID string) (*ServiceInfo, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	service, exists := sd.services[serviceID]
	if !exists {
		return nil, errors.New("service not found")
	}

	return service, nil
}

// GetServices returns all services
func (sd *InMemoryServiceDiscovery) GetServices(ctx context.Context) ([]*ServiceInfo, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	services := make([]*ServiceInfo, 0, len(sd.services))
	for _, service := range sd.services {
		services = append(services, service)
	}

	return services, nil
}

// GetServicesByName returns all services with a specific name
func (sd *InMemoryServiceDiscovery) GetServicesByName(ctx context.Context, name string) ([]*ServiceInfo, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	serviceIDs := sd.nameIndex[name]
	services := make([]*ServiceInfo, 0, len(serviceIDs))

	for _, serviceID := range serviceIDs {
		if service, exists := sd.services[serviceID]; exists {
			services = append(services, service)
		}
	}

	return services, nil
}

// GetHealthyServices returns all healthy services
func (sd *InMemoryServiceDiscovery) GetHealthyServices(ctx context.Context) ([]*ServiceInfo, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	services := make([]*ServiceInfo, 0)
	for _, service := range sd.services {
		if service.Status == ServiceStatusHealthy {
			services = append(services, service)
		}
	}

	return services, nil
}

// UpdateServiceStatus updates the status of a service
func (sd *InMemoryServiceDiscovery) UpdateServiceStatus(ctx context.Context, serviceID string, status ServiceStatus) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	service, exists := sd.services[serviceID]
	if !exists {
		return errors.New("service not found")
	}

	oldStatus := service.Status
	service.Status = status
	service.LastHealthCheck = time.Now()

	// Notify watchers if status changed
	if oldStatus != status {
		sd.notifyWatchers(ServiceEvent{
			Type:    ServiceEventHealthChanged,
			Service: service,
		})
	}

	return nil
}

// Watch watches for service changes
func (sd *InMemoryServiceDiscovery) Watch(ctx context.Context) (<-chan ServiceEvent, error) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	eventChan := make(chan ServiceEvent, 100)
	sd.watchers = append(sd.watchers, eventChan)

	return eventChan, nil
}

// StartHealthChecking starts periodic health checking
func (sd *InMemoryServiceDiscovery) StartHealthChecking() {
	sd.healthChecker.Start()
}

// StopHealthChecking stops health checking
func (sd *InMemoryServiceDiscovery) StopHealthChecking() {
	sd.healthChecker.Stop()
}

// notifyWatchers notifies all watchers of an event
func (sd *InMemoryServiceDiscovery) notifyWatchers(event ServiceEvent) {
	for _, watcher := range sd.watchers {
		select {
		case watcher <- event:
		default:
			// Skip if channel is full
		}
	}
}

// removeFromIndex removes a service ID from an index
func (sd *InMemoryServiceDiscovery) removeFromIndex(index map[string][]string, key, serviceID string) {
	services := index[key]
	for i, id := range services {
		if id == serviceID {
			index[key] = append(services[:i], services[i+1:]...)
			break
		}
	}

	if len(index[key]) == 0 {
		delete(index, key)
	}
}

// ServiceHealthChecker performs health checks on services
type ServiceHealthChecker struct {
	discovery     *InMemoryServiceDiscovery
	checkInterval time.Duration
	checkFunc     func(service *ServiceInfo) ServiceStatus
	stopChan      chan struct{}
}

// NewServiceHealthChecker creates a new service health checker
func NewServiceHealthChecker(discovery *InMemoryServiceDiscovery, checkInterval time.Duration, checkFunc func(service *ServiceInfo) ServiceStatus) *ServiceHealthChecker {
	return &ServiceHealthChecker{
		discovery:     discovery,
		checkInterval: checkInterval,
		checkFunc:     checkFunc,
	}
}

// Start starts the health checker
func (shc *ServiceHealthChecker) Start() {
	if shc.stopChan != nil {
		return // Already running
	}

	shc.stopChan = make(chan struct{})
	go shc.run()
}

// Stop stops the health checker
func (shc *ServiceHealthChecker) Stop() {
	if shc.stopChan != nil {
		close(shc.stopChan)
		shc.stopChan = nil
	}
}

// run performs periodic health checks
func (shc *ServiceHealthChecker) run() {
	ticker := time.NewTicker(shc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			shc.checkAll()
		case <-shc.stopChan:
			return
		}
	}
}

// checkAll checks the health of all services
func (shc *ServiceHealthChecker) checkAll() {
	services, err := shc.discovery.GetServices(context.Background())
	if err != nil {
		return
	}

	for _, service := range services {
		newStatus := shc.checkFunc(service)
		if newStatus != service.Status {
			shc.discovery.UpdateServiceStatus(context.Background(), service.ID, newStatus)
		}
	}
}

// ServiceSelector selects a service based on a strategy
type ServiceSelector struct {
	discovery ServiceDiscovery
	strategy  LoadBalancingStrategy
	rrIndex   uint32
	mu        sync.Mutex
}

// NewServiceSelector creates a new service selector
func NewServiceSelector(discovery ServiceDiscovery, strategy LoadBalancingStrategy) *ServiceSelector {
	return &ServiceSelector{
		discovery: discovery,
		strategy:  strategy,
	}
}

// SelectService selects a service by name using the configured strategy
func (ss *ServiceSelector) SelectService(ctx context.Context, serviceName string, clientID string) (*ServiceInfo, error) {
	services, err := ss.discovery.GetServicesByName(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// Filter for healthy services
	healthyServices := make([]*ServiceInfo, 0)
	for _, service := range services {
		if service.Status == ServiceStatusHealthy {
			healthyServices = append(healthyServices, service)
		}
	}

	if len(healthyServices) == 0 {
		return nil, errors.New("no healthy services available")
	}

	switch ss.strategy {
	case RoundRobin:
		return ss.selectRoundRobin(healthyServices), nil
	case IPHash:
		return ss.selectIPHash(healthyServices, clientID), nil
	default:
		return ss.selectRoundRobin(healthyServices), nil
	}
}

// selectRoundRobin selects a service using round-robin
func (ss *ServiceSelector) selectRoundRobin(services []*ServiceInfo) *ServiceInfo {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	index := ss.rrIndex % uint32(len(services))
	ss.rrIndex++

	return services[index]
}

// selectIPHash selects a service based on client ID hash
func (ss *ServiceSelector) selectIPHash(services []*ServiceInfo, clientID string) *ServiceInfo {
	hash := uint32(0)
	for _, char := range clientID {
		hash = hash*31 + uint32(char)
	}

	index := hash % uint32(len(services))
	return services[index]
}
