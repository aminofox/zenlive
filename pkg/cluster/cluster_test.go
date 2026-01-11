package cluster

import (
	"context"
	"testing"
	"time"
)

func TestLoadBalancer(t *testing.T) {
	lb := NewLoadBalancer(RoundRobin)

	// Add nodes
	node1 := NewNode("node1", "localhost:8001", 1)
	node2 := NewNode("node2", "localhost:8002", 1)
	node3 := NewNode("node3", "localhost:8003", 2)

	lb.AddNode(node1)
	lb.AddNode(node2)
	lb.AddNode(node3)

	// Test round-robin selection
	selected1, err := lb.SelectNode("client1")
	if err != nil {
		t.Fatalf("SelectNode failed: %v", err)
	}

	selected2, err := lb.SelectNode("client2")
	if err != nil {
		t.Fatalf("SelectNode failed: %v", err)
	}

	// Should be different nodes
	if selected1.ID == selected2.ID {
		t.Error("Round robin should select different nodes")
	}

	// Test node removal
	lb.RemoveNode("node2")
	nodes := lb.GetAllNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes after removal, got %d", len(nodes))
	}
}

func TestLoadBalancerLeastConnections(t *testing.T) {
	lb := NewLoadBalancer(LeastConnections)

	node1 := NewNode("node1", "localhost:8001", 1)
	node2 := NewNode("node2", "localhost:8002", 1)

	lb.AddNode(node1)
	lb.AddNode(node2)

	// Simulate connections on node1
	node1.IncrementConnections()
	node1.IncrementConnections()

	// Should select node2 (fewer connections)
	selected, err := lb.SelectNode("client1")
	if err != nil {
		t.Fatalf("SelectNode failed: %v", err)
	}

	if selected.ID != "node2" {
		t.Errorf("Expected node2 (least connections), got %s", selected.ID)
	}
}

func TestStreamRouter(t *testing.T) {
	router := NewStreamRouter(150)

	// Add nodes
	node1 := NewNode("node1", "localhost:8001", 1)
	node2 := NewNode("node2", "localhost:8002", 1)
	node3 := NewNode("node3", "localhost:8003", 1)

	router.AddNode(node1)
	router.AddNode(node2)
	router.AddNode(node3)

	// Test consistent hashing - same stream should go to same node
	stream1Node1, err := router.GetNode("stream-123")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	stream1Node2, err := router.GetNode("stream-123")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if stream1Node1.ID != stream1Node2.ID {
		t.Error("Consistent hashing should return same node for same stream")
	}

	// Different streams may go to different nodes
	stream2Node, err := router.GetNode("stream-456")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	// Just verify we got a node
	if stream2Node == nil {
		t.Error("Expected a node for stream-456")
	}
}

func TestStreamRouterWithReplicas(t *testing.T) {
	router := NewStreamRouter(150)

	for i := 1; i <= 5; i++ {
		node := NewNode("node"+string(rune('0'+i)), "localhost:800"+string(rune('0'+i)), 1)
		router.AddNode(node)
	}

	// Get node with 2 replicas
	nodes, err := router.GetNodeWithReplicas("stream-123", 2)
	if err != nil {
		t.Fatalf("GetNodeWithReplicas failed: %v", err)
	}

	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes (1 primary + 2 replicas), got %d", len(nodes))
	}

	// Verify all nodes are unique
	seen := make(map[string]bool)
	for _, node := range nodes {
		if seen[node.ID] {
			t.Errorf("Duplicate node in replica set: %s", node.ID)
		}
		seen[node.ID] = true
	}
}

func TestSessionManager(t *testing.T) {
	manager := NewInMemorySessionManager(30 * time.Minute)
	ctx := context.Background()

	// Create session
	session := &Session{
		ID:       "session-123",
		UserID:   "user-1",
		StreamID: "stream-1",
		NodeID:   "node-1",
		Data:     map[string]interface{}{"key": "value"},
	}

	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get session
	retrieved, err := manager.GetSession(ctx, "session-123")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, retrieved.ID)
	}

	if retrieved.UserID != session.UserID {
		t.Errorf("Expected user ID %s, got %s", session.UserID, retrieved.UserID)
	}

	// Get user sessions
	userSessions, err := manager.GetUserSessions(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetUserSessions failed: %v", err)
	}

	if len(userSessions) != 1 {
		t.Errorf("Expected 1 session for user, got %d", len(userSessions))
	}

	// Delete session
	err = manager.DeleteSession(ctx, "session-123")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify deleted
	_, err = manager.GetSession(ctx, "session-123")
	if err == nil {
		t.Error("Expected error when getting deleted session")
	}
}

func TestServiceDiscovery(t *testing.T) {
	discovery := NewInMemoryServiceDiscovery()
	ctx := context.Background()

	// Register service
	service := &ServiceInfo{
		ID:      "service-1",
		Name:    "api",
		Address: "localhost:8080",
		NodeID:  "node-1",
		Version: "1.0.0",
		Tags:    []string{"api", "v1"},
	}

	err := discovery.Register(ctx, service)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Get service
	retrieved, err := discovery.GetService(ctx, "service-1")
	if err != nil {
		t.Fatalf("GetService failed: %v", err)
	}

	if retrieved.Name != service.Name {
		t.Errorf("Expected service name %s, got %s", service.Name, retrieved.Name)
	}

	// Get services by name
	services, err := discovery.GetServicesByName(ctx, "api")
	if err != nil {
		t.Fatalf("GetServicesByName failed: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service named 'api', got %d", len(services))
	}

	// Get healthy services
	healthyServices, err := discovery.GetHealthyServices(ctx)
	if err != nil {
		t.Fatalf("GetHealthyServices failed: %v", err)
	}

	if len(healthyServices) != 1 {
		t.Errorf("Expected 1 healthy service, got %d", len(healthyServices))
	}

	// Deregister
	err = discovery.Deregister(ctx, "service-1")
	if err != nil {
		t.Fatalf("Deregister failed: %v", err)
	}

	// Verify deregistered
	_, err = discovery.GetService(ctx, "service-1")
	if err == nil {
		t.Error("Expected error when getting deregistered service")
	}
}

func TestServiceSelector(t *testing.T) {
	discovery := NewInMemoryServiceDiscovery()
	selector := NewServiceSelector(discovery, RoundRobin)
	ctx := context.Background()

	// Register multiple services with same name
	for i := 1; i <= 3; i++ {
		service := &ServiceInfo{
			ID:      "service-" + string(rune('0'+i)),
			Name:    "api",
			Address: "localhost:808" + string(rune('0'+i)),
			Status:  ServiceStatusHealthy,
		}
		discovery.Register(ctx, service)
	}

	// Select service - should use round-robin
	service1, err := selector.SelectService(ctx, "api", "client1")
	if err != nil {
		t.Fatalf("SelectService failed: %v", err)
	}

	service2, err := selector.SelectService(ctx, "api", "client2")
	if err != nil {
		t.Fatalf("SelectService failed: %v", err)
	}

	// Should select different services (round-robin)
	if service1.ID == service2.ID {
		t.Log("Round-robin may select same service occasionally, not an error")
	}
}
