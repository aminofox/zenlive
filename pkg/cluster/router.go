package cluster

import (
	"errors"
	"hash/crc32"
	"sort"
	"sync"
)

// StreamRouter routes streams to nodes using consistent hashing
type StreamRouter struct {
	nodes        map[string]*Node  // Map of node ID to node
	ring         []uint32          // Sorted hash ring
	ringMap      map[uint32]string // Hash to node ID mapping
	virtualNodes int               // Number of virtual nodes per physical node
	mu           sync.RWMutex
}

// NewStreamRouter creates a new stream router
func NewStreamRouter(virtualNodes int) *StreamRouter {
	if virtualNodes <= 0 {
		virtualNodes = 150 // Default number of virtual nodes
	}

	return &StreamRouter{
		nodes:        make(map[string]*Node),
		ring:         make([]uint32, 0),
		ringMap:      make(map[uint32]string),
		virtualNodes: virtualNodes,
	}
}

// AddNode adds a node to the consistent hash ring
func (sr *StreamRouter) AddNode(node *Node) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.nodes[node.ID] = node

	// Add virtual nodes to the ring
	for i := 0; i < sr.virtualNodes; i++ {
		virtualKey := sr.getVirtualNodeKey(node.ID, i)
		hash := sr.hash(virtualKey)

		sr.ring = append(sr.ring, hash)
		sr.ringMap[hash] = node.ID
	}

	// Sort the ring
	sort.Slice(sr.ring, func(i, j int) bool {
		return sr.ring[i] < sr.ring[j]
	})
}

// RemoveNode removes a node from the consistent hash ring
func (sr *StreamRouter) RemoveNode(nodeID string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	delete(sr.nodes, nodeID)

	// Remove virtual nodes from the ring
	newRing := make([]uint32, 0)
	for _, hash := range sr.ring {
		if sr.ringMap[hash] != nodeID {
			newRing = append(newRing, hash)
		} else {
			delete(sr.ringMap, hash)
		}
	}

	sr.ring = newRing
}

// GetNode returns the node responsible for a given stream
func (sr *StreamRouter) GetNode(streamID string) (*Node, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	if len(sr.ring) == 0 {
		return nil, errors.New("no nodes available")
	}

	hash := sr.hash(streamID)

	// Binary search to find the first node with hash >= stream hash
	idx := sort.Search(len(sr.ring), func(i int) bool {
		return sr.ring[i] >= hash
	})

	// Wrap around if we've gone past the end
	if idx >= len(sr.ring) {
		idx = 0
	}

	nodeID := sr.ringMap[sr.ring[idx]]
	node, exists := sr.nodes[nodeID]
	if !exists {
		return nil, errors.New("node not found in mapping")
	}

	return node, nil
}

// GetNodeWithReplicas returns the node and replica nodes for a stream
func (sr *StreamRouter) GetNodeWithReplicas(streamID string, replicaCount int) ([]*Node, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	if len(sr.nodes) == 0 {
		return nil, errors.New("no nodes available")
	}

	if replicaCount >= len(sr.nodes) {
		replicaCount = len(sr.nodes) - 1
	}

	hash := sr.hash(streamID)

	// Find the primary node
	idx := sort.Search(len(sr.ring), func(i int) bool {
		return sr.ring[i] >= hash
	})

	if idx >= len(sr.ring) {
		idx = 0
	}

	// Collect unique nodes
	selectedNodes := make([]*Node, 0, replicaCount+1)
	seenNodes := make(map[string]bool)

	for i := 0; i < len(sr.ring) && len(selectedNodes) <= replicaCount; i++ {
		currentIdx := (idx + i) % len(sr.ring)
		nodeID := sr.ringMap[sr.ring[currentIdx]]

		if !seenNodes[nodeID] {
			if node, exists := sr.nodes[nodeID]; exists {
				selectedNodes = append(selectedNodes, node)
				seenNodes[nodeID] = true
			}
		}
	}

	if len(selectedNodes) == 0 {
		return nil, errors.New("no nodes found")
	}

	return selectedNodes, nil
}

// GetAllNodes returns all nodes in the router
func (sr *StreamRouter) GetAllNodes() []*Node {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	nodes := make([]*Node, 0, len(sr.nodes))
	for _, node := range sr.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// hash computes a hash for a key
func (sr *StreamRouter) hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

// getVirtualNodeKey generates a key for a virtual node
func (sr *StreamRouter) getVirtualNodeKey(nodeID string, index int) string {
	return nodeID + "#" + string(rune(index))
}

// RebalanceStreams returns a map of streams that need to be moved
func (sr *StreamRouter) RebalanceStreams(currentMapping map[string]string) map[string]string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	rebalance := make(map[string]string)

	for streamID, currentNodeID := range currentMapping {
		node, err := sr.GetNode(streamID)
		if err != nil {
			continue
		}

		// If the stream should be on a different node, add to rebalance map
		if node.ID != currentNodeID {
			rebalance[streamID] = node.ID
		}
	}

	return rebalance
}

// StreamDistribution represents the distribution of streams across nodes
type StreamDistribution struct {
	NodeID      string  // Node identifier
	StreamCount int     // Number of streams on this node
	Percentage  float64 // Percentage of total streams
}

// GetDistribution returns the distribution of streams across nodes
func (sr *StreamRouter) GetDistribution(streamIDs []string) []StreamDistribution {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	distribution := make(map[string]int)

	// Count streams per node
	for _, streamID := range streamIDs {
		node, err := sr.GetNode(streamID)
		if err != nil {
			continue
		}
		distribution[node.ID]++
	}

	// Convert to distribution stats
	total := len(streamIDs)
	result := make([]StreamDistribution, 0, len(distribution))

	for nodeID, count := range distribution {
		percentage := 0.0
		if total > 0 {
			percentage = float64(count) / float64(total) * 100.0
		}

		result = append(result, StreamDistribution{
			NodeID:      nodeID,
			StreamCount: count,
			Percentage:  percentage,
		})
	}

	// Sort by stream count descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].StreamCount > result[j].StreamCount
	})

	return result
}

// RouterStats represents statistics for the router
type RouterStats struct {
	TotalNodes   int     // Total number of nodes
	VirtualNodes int     // Total virtual nodes in the ring
	RingSize     int     // Size of the hash ring
	AverageLoad  float64 // Average streams per node
	LoadVariance float64 // Variance in load distribution
}

// GetStats returns router statistics
func (sr *StreamRouter) GetStats() RouterStats {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	stats := RouterStats{
		TotalNodes:   len(sr.nodes),
		VirtualNodes: sr.virtualNodes,
		RingSize:     len(sr.ring),
	}

	return stats
}
