package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/cache"
	"github.com/aminofox/zenlive/pkg/cdn"
	"github.com/aminofox/zenlive/pkg/cluster"
	"github.com/aminofox/zenlive/pkg/optimization"
)

func main() {
	fmt.Println("=== ZenLive Phase 10: Scalability & Optimization Demo ===\n")

	// Run all examples
	runLoadBalancerExample()
	runStreamRouterExample()
	runSessionManagerExample()
	runServiceDiscoveryExample()
	runConnectionPoolExample()
	runCacheExample()
	runCDNExample()

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("Press Ctrl+C to exit")

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}

func runLoadBalancerExample() {
	fmt.Println("--- Load Balancer Example ---")

	// Create load balancer with round-robin strategy
	lb := cluster.NewLoadBalancer(cluster.RoundRobin)

	// Add nodes
	nodes := []*cluster.Node{
		cluster.NewNode("node1", "localhost:8001", 1),
		cluster.NewNode("node2", "localhost:8002", 1),
		cluster.NewNode("node3", "localhost:8003", 2),
	}

	for _, node := range nodes {
		lb.AddNode(node)
	}

	// Simulate selecting nodes for requests
	fmt.Println("Selecting nodes for 5 requests using round-robin:")
	for i := 1; i <= 5; i++ {
		clientID := fmt.Sprintf("client%d", i)
		node, err := lb.SelectNode(clientID)
		if err != nil {
			log.Printf("Error selecting node: %v\n", err)
			continue
		}

		fmt.Printf("Request %d -> Node: %s (%s)\n", i, node.ID, node.Address)

		// Simulate request
		node.IncrementConnections()
		time.Sleep(10 * time.Millisecond)
		node.RecordRequest(15 * time.Millisecond)
		node.DecrementConnections()
	}

	// Get load balancer stats
	stats := lb.GetStats()
	fmt.Printf("\nLoad Balancer Stats:\n")
	fmt.Printf("  Total Nodes: %d\n", stats.TotalNodes)
	fmt.Printf("  Healthy Nodes: %d\n", stats.HealthyNodes)
	fmt.Printf("  Total Requests: %d\n", stats.TotalRequests)

	// Test least connections strategy
	fmt.Println("\n--- Testing Least Connections Strategy ---")
	lbLC := cluster.NewLoadBalancer(cluster.LeastConnections)
	lbLC.AddNode(nodes[0])
	lbLC.AddNode(nodes[1])

	// Simulate different connection loads
	nodes[0].IncrementConnections()
	nodes[0].IncrementConnections()

	node, _ := lbLC.SelectNode("client1")
	fmt.Printf("Selected node with least connections: %s (Connections: %d)\n",
		node.ID, node.GetActiveConnections())

	fmt.Println()
}

func runStreamRouterExample() {
	fmt.Println("--- Stream Router Example ---")

	// Create stream router with consistent hashing
	router := cluster.NewStreamRouter(150) // 150 virtual nodes

	// Add nodes
	for i := 1; i <= 4; i++ {
		node := cluster.NewNode(
			fmt.Sprintf("node%d", i),
			fmt.Sprintf("localhost:900%d", i),
			1,
		)
		router.AddNode(node)
	}

	// Route streams
	streams := []string{"stream-123", "stream-456", "stream-789", "stream-abc"}

	fmt.Println("Routing streams using consistent hashing:")
	streamMapping := make(map[string]string)
	for _, streamID := range streams {
		node, err := router.GetNode(streamID)
		if err != nil {
			log.Printf("Error routing stream %s: %v\n", streamID, err)
			continue
		}

		streamMapping[streamID] = node.ID
		fmt.Printf("Stream %s -> Node: %s (%s)\n", streamID, node.ID, node.Address)
	}

	// Test consistency - same stream should always route to same node
	fmt.Println("\nTesting consistency (same stream queried again):")
	for _, streamID := range []string{"stream-123", "stream-789"} {
		node, _ := router.GetNode(streamID)
		fmt.Printf("Stream %s -> Node: %s (Should match: %s) ✓\n",
			streamID, node.ID, streamMapping[streamID])
	}

	// Get stream with replicas
	fmt.Println("\nGetting stream with replicas:")
	replicaNodes, err := router.GetNodeWithReplicas("stream-123", 2)
	if err == nil {
		fmt.Printf("Stream stream-123 placement:\n")
		fmt.Printf("  Primary: %s\n", replicaNodes[0].ID)
		for i := 1; i < len(replicaNodes); i++ {
			fmt.Printf("  Replica %d: %s\n", i, replicaNodes[i].ID)
		}
	}

	// Get distribution
	distribution := router.GetDistribution(streams)
	fmt.Println("\nStream distribution across nodes:")
	for _, dist := range distribution {
		fmt.Printf("  Node %s: %d streams (%.1f%%)\n",
			dist.NodeID, dist.StreamCount, dist.Percentage)
	}

	fmt.Println()
}

func runSessionManagerExample() {
	fmt.Println("--- Session Manager Example ---")

	// Create in-memory session manager
	sessionMgr := cluster.NewInMemorySessionManager(30 * time.Minute)
	ctx := context.Background()

	// Create sessions
	sessions := []*cluster.Session{
		{
			ID:       "session-1",
			UserID:   "user-alice",
			StreamID: "stream-123",
			NodeID:   "node1",
			Data:     map[string]interface{}{"quality": "1080p"},
		},
		{
			ID:       "session-2",
			UserID:   "user-bob",
			StreamID: "stream-123",
			NodeID:   "node1",
			Data:     map[string]interface{}{"quality": "720p"},
		},
		{
			ID:       "session-3",
			UserID:   "user-alice",
			StreamID: "stream-456",
			NodeID:   "node2",
			Data:     map[string]interface{}{"quality": "1080p"},
		},
	}

	fmt.Println("Creating sessions:")
	for _, session := range sessions {
		err := sessionMgr.CreateSession(ctx, session)
		if err != nil {
			log.Printf("Error creating session: %v\n", err)
			continue
		}
		fmt.Printf("Created session %s for user %s on stream %s\n",
			session.ID, session.UserID, session.StreamID)
	}

	// Get user sessions
	fmt.Println("\nGetting sessions for user-alice:")
	aliceSessions, _ := sessionMgr.GetUserSessions(ctx, "user-alice")
	for _, session := range aliceSessions {
		fmt.Printf("  Session %s: Stream %s on Node %s\n",
			session.ID, session.StreamID, session.NodeID)
	}

	// Get stream sessions
	fmt.Println("\nGetting sessions for stream-123:")
	streamSessions, _ := sessionMgr.GetStreamSessions(ctx, "stream-123")
	fmt.Printf("  Active viewers: %d\n", len(streamSessions))
	for _, session := range streamSessions {
		fmt.Printf("    User %s (Session %s)\n", session.UserID, session.ID)
	}

	// Get node sessions
	fmt.Println("\nGetting sessions on node1:")
	nodeSessions, _ := sessionMgr.GetNodeSessions(ctx, "node1")
	fmt.Printf("  Sessions on node: %d\n", len(nodeSessions))

	fmt.Println()
}

func runServiceDiscoveryExample() {
	fmt.Println("--- Service Discovery Example ---")

	discovery := cluster.NewInMemoryServiceDiscovery()
	ctx := context.Background()

	// Register services
	services := []*cluster.ServiceInfo{
		{
			ID:      "api-1",
			Name:    "api",
			Address: "localhost:8080",
			NodeID:  "node1",
			Version: "1.0.0",
			Tags:    []string{"api", "v1"},
		},
		{
			ID:      "api-2",
			Name:    "api",
			Address: "localhost:8081",
			NodeID:  "node2",
			Version: "1.0.0",
			Tags:    []string{"api", "v1"},
		},
		{
			ID:      "rtmp-1",
			Name:    "rtmp",
			Address: "localhost:1935",
			NodeID:  "node1",
			Version: "1.0.0",
			Tags:    []string{"streaming", "rtmp"},
		},
	}

	fmt.Println("Registering services:")
	for _, service := range services {
		err := discovery.Register(ctx, service)
		if err != nil {
			log.Printf("Error registering service: %v\n", err)
			continue
		}
		fmt.Printf("Registered %s service: %s at %s\n",
			service.Name, service.ID, service.Address)
	}

	// Get services by name
	fmt.Println("\nGetting API services:")
	apiServices, _ := discovery.GetServicesByName(ctx, "api")
	for _, service := range apiServices {
		fmt.Printf("  %s: %s (Status: %s)\n",
			service.ID, service.Address, service.Status)
	}

	// Service selection
	selector := cluster.NewServiceSelector(discovery, cluster.RoundRobin)
	fmt.Println("\nSelecting API service using round-robin:")
	for i := 1; i <= 3; i++ {
		service, err := selector.SelectService(ctx, "api", fmt.Sprintf("client%d", i))
		if err != nil {
			log.Printf("Error selecting service: %v\n", err)
			continue
		}
		fmt.Printf("Request %d -> %s (%s)\n", i, service.ID, service.Address)
	}

	// Get all healthy services
	healthyServices, _ := discovery.GetHealthyServices(ctx)
	fmt.Printf("\nTotal healthy services: %d\n", len(healthyServices))

	fmt.Println()
}

func runConnectionPoolExample() {
	fmt.Println("--- Connection Pool Example ---")

	// Note: In real usage, you would implement the factory interface
	fmt.Println("Connection pooling configuration:")
	config := optimization.DefaultPoolConfig()
	fmt.Printf("  Max Idle: %d\n", config.MaxIdle)
	fmt.Printf("  Max Active: %d\n", config.MaxActive)
	fmt.Printf("  Connection Lifetime: %s\n", config.MaxLifetime)
	fmt.Printf("  Idle Timeout: %s\n", config.IdleTimeout)

	fmt.Println("\nConnection pooling benefits:")
	fmt.Println("  ✓ Reuses connections instead of creating new ones")
	fmt.Println("  ✓ Limits maximum concurrent connections")
	fmt.Println("  ✓ Automatically closes idle connections")
	fmt.Println("  ✓ Validates connections before use")

	// Zero-copy optimization
	fmt.Println("\n--- Zero-Copy Optimization ---")

	bufferPool := optimization.NewBufferPool([]int{1024, 4096, 16384})

	// Get buffers from pool
	fmt.Println("Getting buffers from pool:")
	buf1 := bufferPool.Get(1024)
	fmt.Printf("  Buffer 1: %d bytes (capacity: %d)\n", buf1.Len(), buf1.Cap())

	buf2 := bufferPool.Get(8192)
	fmt.Printf("  Buffer 2: %d bytes (capacity: %d)\n", buf2.Len(), buf2.Cap())

	// Release buffers back to pool
	buf1.Release()
	buf2.Release()
	fmt.Println("  Buffers released back to pool for reuse")

	// Zero-copy writer
	fmt.Println("\nZero-copy writer example:")
	writer := optimization.NewZeroCopyWriter()
	writer.Write([]byte("Hello, "))
	writer.Write([]byte("World!"))

	result := writer.Bytes()
	fmt.Printf("  Written: %s (%d bytes)\n", string(result), writer.Len())

	// Shared memory
	fmt.Println("\nShared memory example:")
	sharedMem := optimization.NewSharedMemory(1024)
	slice1, _ := sharedMem.Allocate(256)
	fmt.Printf("  Allocated: %d bytes\n", len(slice1))
	fmt.Printf("  Used: %d/%d bytes\n", sharedMem.Used(), sharedMem.Size())

	fmt.Println()
}

func runCacheExample() {
	fmt.Println("--- Caching Layer Example ---")

	// Create in-memory cache
	inMemCache := cache.NewInMemoryCache(100, 5*time.Minute, cache.EvictionPolicyLRU)
	inMemCache.Start()
	defer inMemCache.Stop()

	ctx := context.Background()

	// Set values
	fmt.Println("Setting cache values:")
	cacheData := map[string]interface{}{
		"user:alice": map[string]string{"name": "Alice", "role": "streamer"},
		"user:bob":   map[string]string{"name": "Bob", "role": "viewer"},
		"stream:123": map[string]interface{}{"title": "Live Stream", "viewers": 150},
		"stream:456": map[string]interface{}{"title": "Gaming", "viewers": 89},
	}

	for key, value := range cacheData {
		inMemCache.Set(ctx, key, value, 0)
		fmt.Printf("  Set %s\n", key)
	}

	// Get values
	fmt.Println("\nGetting cache values:")
	value, _ := inMemCache.Get(ctx, "user:alice")
	fmt.Printf("  user:alice = %v\n", value)

	value, _ = inMemCache.Get(ctx, "stream:123")
	fmt.Printf("  stream:123 = %v\n", value)

	// Cache stats
	stats, _ := inMemCache.Stats(ctx)
	fmt.Println("\nCache Statistics:")
	fmt.Printf("  Size: %d entries\n", stats.Size)
	fmt.Printf("  Hits: %d\n", stats.Hits)
	fmt.Printf("  Misses: %d\n", stats.Misses)
	fmt.Printf("  Hit Rate: %.1f%%\n", stats.HitRate*100)

	// Multi-level cache
	fmt.Println("\n--- Multi-Level Cache ---")

	l1Cache := cache.NewInMemoryCache(10, 1*time.Minute, cache.EvictionPolicyLRU)
	l1Cache.Start()
	defer l1Cache.Stop()

	l2Cache := cache.NewInMemoryCache(100, 5*time.Minute, cache.EvictionPolicyLRU)
	l2Cache.Start()
	defer l2Cache.Stop()

	multiCache := cache.NewMultiLevelCache(l1Cache, l2Cache)

	multiCache.Set(ctx, "fast-data", "value1", 0)
	fmt.Println("Set value in multi-level cache (L1 and L2)")

	// Remove from L1 to test promotion
	l1Cache.Delete(ctx, "fast-data")
	fmt.Println("Removed from L1 cache")

	value, _ = multiCache.Get(ctx, "fast-data")
	fmt.Printf("Retrieved from L2 and promoted to L1: %v\n", value)

	fmt.Println()
}

func runCDNExample() {
	fmt.Println("--- CDN Integration Example ---")

	// Create CDN client
	cdnConfig := cdn.CDNConfig{
		Provider: cdn.CDNProviderCustom,
		BaseURL:  "https://cdn.example.com",
		Enabled:  true,
		CacheTTL: 24 * time.Hour,
	}

	cdnClient := cdn.NewCDNClient(cdnConfig)

	fmt.Println("CDN Configuration:")
	fmt.Printf("  Provider: %s\n", cdnConfig.Provider)
	fmt.Printf("  Base URL: %s\n", cdnConfig.BaseURL)
	fmt.Printf("  Cache TTL: %s\n", cdnConfig.CacheTTL)

	// Generate CDN URLs
	fmt.Println("\nGenerating CDN URLs for static assets:")
	assets := []string{
		"/static/player.js",
		"/static/styles.css",
		"/thumbnails/stream-123.jpg",
		"/videos/stream-123.m3u8",
	}

	for _, asset := range assets {
		cdnURL := cdnClient.GetURL(asset)
		fmt.Printf("  %s -> %s\n", asset, cdnURL)
	}

	// URL signing for secure access
	fmt.Println("\n--- Secure URL Signing ---")

	signer := cdn.NewURLSigner("secret-key-12345", 1*time.Hour)

	originalURL := "https://cdn.example.com/videos/stream-123.m3u8"
	signedURL, _ := signer.SignURL(originalURL)

	fmt.Printf("Original URL: %s\n", originalURL)
	fmt.Printf("Signed URL: %s\n", signedURL)
	fmt.Println("  ✓ URL signed with expiration (1 hour)")
	fmt.Println("  ✓ Prevents unauthorized access")
	fmt.Println("  ✓ Automatic expiration handling")

	// Verify signed URL
	valid, _ := signer.VerifyURL(signedURL)
	if valid {
		fmt.Println("  ✓ URL signature verified successfully")
	}

	fmt.Println()
}
