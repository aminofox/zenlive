package cluster

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancingStrategy represents the load balancing algorithm
type LoadBalancingStrategy string

const (
	// RoundRobin distributes requests evenly across all nodes
	RoundRobin LoadBalancingStrategy = "round_robin"
	// LeastConnections routes to the node with fewest active connections
	LeastConnections LoadBalancingStrategy = "least_connections"
	// WeightedRoundRobin distributes based on node weights
	WeightedRoundRobin LoadBalancingStrategy = "weighted_round_robin"
	// IPHash uses client IP for consistent routing
	IPHash LoadBalancingStrategy = "ip_hash"
)

// Node represents a backend server node
type Node struct {
	ID              string    // Unique node identifier
	Address         string    // Node address (host:port)
	Weight          int       // Weight for weighted algorithms
	MaxConnections  int       // Maximum allowed connections
	Healthy         bool      // Health status
	LastHealthCheck time.Time // Last health check timestamp

	// Metrics
	ActiveConnections int32         // Current active connections
	TotalRequests     int64         // Total requests handled
	FailedRequests    int64         // Total failed requests
	AverageLatency    time.Duration // Average response latency

	mu sync.RWMutex
}

// NewNode creates a new backend node
func NewNode(id, address string, weight int) *Node {
	return &Node{
		ID:              id,
		Address:         address,
		Weight:          weight,
		MaxConnections:  1000, // Default max connections
		Healthy:         true,
		LastHealthCheck: time.Now(),
	}
}

// IncrementConnections increments the active connection count
func (n *Node) IncrementConnections() {
	atomic.AddInt32(&n.ActiveConnections, 1)
}

// DecrementConnections decrements the active connection count
func (n *Node) DecrementConnections() {
	atomic.AddInt32(&n.ActiveConnections, -1)
}

// GetActiveConnections returns the current active connection count
func (n *Node) GetActiveConnections() int32 {
	return atomic.LoadInt32(&n.ActiveConnections)
}

// RecordRequest records a successful request
func (n *Node) RecordRequest(latency time.Duration) {
	atomic.AddInt64(&n.TotalRequests, 1)

	n.mu.Lock()
	defer n.mu.Unlock()

	// Update average latency using exponential moving average
	if n.AverageLatency == 0 {
		n.AverageLatency = latency
	} else {
		n.AverageLatency = time.Duration(0.9*float64(n.AverageLatency) + 0.1*float64(latency))
	}
}

// RecordFailure records a failed request
func (n *Node) RecordFailure() {
	atomic.AddInt64(&n.FailedRequests, 1)
}

// SetHealthy sets the health status of the node
func (n *Node) SetHealthy(healthy bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Healthy = healthy
	n.LastHealthCheck = time.Now()
}

// IsHealthy returns whether the node is healthy
func (n *Node) IsHealthy() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.Healthy
}

// IsOverloaded checks if the node is overloaded
func (n *Node) IsOverloaded() bool {
	return n.GetActiveConnections() >= int32(n.MaxConnections)
}

// LoadBalancer manages load balancing across multiple nodes
type LoadBalancer struct {
	nodes    []*Node
	strategy LoadBalancingStrategy

	// Round robin counter
	rrIndex uint32

	mu sync.RWMutex
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(strategy LoadBalancingStrategy) *LoadBalancer {
	return &LoadBalancer{
		nodes:    make([]*Node, 0),
		strategy: strategy,
	}
}

// AddNode adds a node to the load balancer
func (lb *LoadBalancer) AddNode(node *Node) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.nodes = append(lb.nodes, node)
}

// RemoveNode removes a node from the load balancer
func (lb *LoadBalancer) RemoveNode(nodeID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for i, node := range lb.nodes {
		if node.ID == nodeID {
			lb.nodes = append(lb.nodes[:i], lb.nodes[i+1:]...)
			return
		}
	}
}

// GetNode retrieves a node by ID
func (lb *LoadBalancer) GetNode(nodeID string) (*Node, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	for _, node := range lb.nodes {
		if node.ID == nodeID {
			return node, nil
		}
	}

	return nil, errors.New("node not found")
}

// GetAllNodes returns all nodes
func (lb *LoadBalancer) GetAllNodes() []*Node {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	nodes := make([]*Node, len(lb.nodes))
	copy(nodes, lb.nodes)
	return nodes
}

// GetHealthyNodes returns all healthy nodes
func (lb *LoadBalancer) GetHealthyNodes() []*Node {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	healthy := make([]*Node, 0)
	for _, node := range lb.nodes {
		if node.IsHealthy() && !node.IsOverloaded() {
			healthy = append(healthy, node)
		}
	}

	return healthy
}

// SelectNode selects a node based on the configured strategy
func (lb *LoadBalancer) SelectNode(clientID string) (*Node, error) {
	switch lb.strategy {
	case RoundRobin:
		return lb.selectRoundRobin()
	case LeastConnections:
		return lb.selectLeastConnections()
	case WeightedRoundRobin:
		return lb.selectWeightedRoundRobin()
	case IPHash:
		return lb.selectIPHash(clientID)
	default:
		return lb.selectRoundRobin()
	}
}

// selectRoundRobin selects a node using round-robin algorithm
func (lb *LoadBalancer) selectRoundRobin() (*Node, error) {
	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) == 0 {
		return nil, errors.New("no healthy nodes available")
	}

	// Atomically increment and get the index
	index := atomic.AddUint32(&lb.rrIndex, 1)
	selectedNode := healthyNodes[index%uint32(len(healthyNodes))]

	return selectedNode, nil
}

// selectLeastConnections selects the node with the least active connections
func (lb *LoadBalancer) selectLeastConnections() (*Node, error) {
	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) == 0 {
		return nil, errors.New("no healthy nodes available")
	}

	var selectedNode *Node
	minConnections := int32(-1)

	for _, node := range healthyNodes {
		connections := node.GetActiveConnections()
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selectedNode = node
		}
	}

	return selectedNode, nil
}

// selectWeightedRoundRobin selects a node using weighted round-robin
func (lb *LoadBalancer) selectWeightedRoundRobin() (*Node, error) {
	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) == 0 {
		return nil, errors.New("no healthy nodes available")
	}

	// Calculate total weight
	totalWeight := 0
	for _, node := range healthyNodes {
		totalWeight += node.Weight
	}

	if totalWeight == 0 {
		// Fall back to round-robin if no weights set
		return lb.selectRoundRobin()
	}

	// Get current index and wrap around total weight
	index := atomic.AddUint32(&lb.rrIndex, 1)
	targetWeight := int(index % uint32(totalWeight))

	// Find the node that corresponds to this weight
	currentWeight := 0
	for _, node := range healthyNodes {
		currentWeight += node.Weight
		if targetWeight < currentWeight {
			return node, nil
		}
	}

	// Fallback (shouldn't reach here)
	return healthyNodes[0], nil
}

// selectIPHash selects a node based on client IP hash
func (lb *LoadBalancer) selectIPHash(clientID string) (*Node, error) {
	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) == 0 {
		return nil, errors.New("no healthy nodes available")
	}

	// Simple hash function
	hash := uint32(0)
	for _, char := range clientID {
		hash = hash*31 + uint32(char)
	}

	index := hash % uint32(len(healthyNodes))
	return healthyNodes[index], nil
}

// HealthChecker checks node health periodically
type HealthChecker struct {
	loadBalancer  *LoadBalancer
	checkInterval time.Duration
	checkFunc     func(node *Node) bool
	stopChan      chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(lb *LoadBalancer, checkInterval time.Duration, checkFunc func(node *Node) bool) *HealthChecker {
	return &HealthChecker{
		loadBalancer:  lb,
		checkInterval: checkInterval,
		checkFunc:     checkFunc,
	}
}

// Start starts the health checker
func (hc *HealthChecker) Start() {
	if hc.stopChan != nil {
		return // Already running
	}

	hc.stopChan = make(chan struct{})
	go hc.run()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	if hc.stopChan != nil {
		close(hc.stopChan)
		hc.stopChan = nil
	}
}

// run performs periodic health checks
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkAllNodes()
		case <-hc.stopChan:
			return
		}
	}
}

// checkAllNodes checks the health of all nodes
func (hc *HealthChecker) checkAllNodes() {
	nodes := hc.loadBalancer.GetAllNodes()

	for _, node := range nodes {
		healthy := hc.checkFunc(node)
		node.SetHealthy(healthy)
	}
}

// LoadBalancerStats represents statistics for the load balancer
type LoadBalancerStats struct {
	TotalNodes     int           // Total number of nodes
	HealthyNodes   int           // Number of healthy nodes
	TotalRequests  int64         // Total requests across all nodes
	FailedRequests int64         // Total failed requests
	AverageLatency time.Duration // Average latency across all nodes
	NodesStats     []NodeStats   // Per-node statistics
}

// NodeStats represents statistics for a single node
type NodeStats struct {
	NodeID            string        // Node identifier
	Address           string        // Node address
	Healthy           bool          // Health status
	ActiveConnections int32         // Active connections
	TotalRequests     int64         // Total requests
	FailedRequests    int64         // Failed requests
	AverageLatency    time.Duration // Average latency
	Weight            int           // Node weight
}

// GetStats returns load balancer statistics
func (lb *LoadBalancer) GetStats() LoadBalancerStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	stats := LoadBalancerStats{
		TotalNodes: len(lb.nodes),
		NodesStats: make([]NodeStats, 0, len(lb.nodes)),
	}

	totalLatency := time.Duration(0)
	healthyCount := 0

	for _, node := range lb.nodes {
		node.mu.RLock()

		nodeStats := NodeStats{
			NodeID:            node.ID,
			Address:           node.Address,
			Healthy:           node.Healthy,
			ActiveConnections: node.GetActiveConnections(),
			TotalRequests:     atomic.LoadInt64(&node.TotalRequests),
			FailedRequests:    atomic.LoadInt64(&node.FailedRequests),
			AverageLatency:    node.AverageLatency,
			Weight:            node.Weight,
		}

		stats.TotalRequests += nodeStats.TotalRequests
		stats.FailedRequests += nodeStats.FailedRequests
		totalLatency += node.AverageLatency

		if node.Healthy {
			healthyCount++
		}

		node.mu.RUnlock()

		stats.NodesStats = append(stats.NodesStats, nodeStats)
	}

	stats.HealthyNodes = healthyCount

	if len(lb.nodes) > 0 {
		stats.AverageLatency = totalLatency / time.Duration(len(lb.nodes))
	}

	return stats
}
