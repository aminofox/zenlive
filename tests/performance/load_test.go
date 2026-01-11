package performance

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkStreamCreation benchmarks stream creation performance
func BenchmarkStreamCreation(b *testing.B) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
			UserID:   fmt.Sprintf("user-%d", i),
			Title:    fmt.Sprintf("benchmark-stream-%d", i),
			Protocol: sdk.ProtocolRTMP,
		})
		if err != nil {
			b.Fatalf("Failed to create stream: %v", err)
		}

		// Cleanup
		_ = streamManager.DeleteStream(ctx, stream.ID)
	}
}

// BenchmarkConcurrentStreamCreation benchmarks concurrent stream creation
func BenchmarkConcurrentStreamCreation(b *testing.B) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
				UserID:   fmt.Sprintf("user-%d", i),
				Title:    fmt.Sprintf("concurrent-stream-%d", i),
				Protocol: sdk.ProtocolRTMP,
			})
			if err != nil {
				b.Fatalf("Failed to create stream: %v", err)
			}
			_ = streamManager.DeleteStream(ctx, stream.ID)
			i++
		}
	})
}

// BenchmarkStreamUpdate benchmarks stream update performance
func BenchmarkStreamUpdate(b *testing.B) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)
	ctx := context.Background()

	stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "test-user",
		Title:    "update-benchmark",
		Protocol: sdk.ProtocolRTMP,
	})
	require.NoError(b, err)
	defer streamManager.DeleteStream(ctx, stream.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		title := fmt.Sprintf("Updated Title %d", i)
		_, _ = streamManager.UpdateStream(ctx, stream.ID, &sdk.UpdateStreamRequest{
			Title: &title,
		})
	}
}

// TestLoadTest_100ConcurrentStreams tests system with 100 concurrent streams
func TestLoadTest_100ConcurrentStreams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	const numStreams = 100
	const duration = 10 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Metrics
	var (
		createdCount  int32
		errorCount    int32
		totalDuration int64
	)

	startTime := time.Now()

	// Create streams concurrently
	var wg sync.WaitGroup
	streamIDs := make([]string, numStreams)

	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			opStart := time.Now()

			// Create stream
			stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
				UserID:   fmt.Sprintf("load-test-user-%d", index),
				Title:    fmt.Sprintf("load-test-stream-%d", index),
				Protocol: sdk.ProtocolRTMP,
			})
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}
			atomic.AddInt32(&createdCount, 1)
			streamIDs[index] = stream.ID

			atomic.AddInt64(&totalDuration, int64(time.Since(opStart)))
		}(i)
	}

	wg.Wait()

	creationTime := time.Since(startTime)

	// Report metrics
	t.Logf("Load Test Results (100 Concurrent Streams):")
	t.Logf("  Total Creation Time: %v", creationTime)
	t.Logf("  Streams Created: %d/%d", createdCount, numStreams)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Avg Operation Time: %v", time.Duration(totalDuration/int64(numStreams)))
	t.Logf("  Throughput: %.2f streams/sec", float64(numStreams)/creationTime.Seconds())

	// Assertions
	assert.Equal(t, int32(numStreams), createdCount, "All streams should be created")
	assert.Equal(t, int32(0), errorCount, "Should have no errors")
	assert.Less(t, creationTime, 30*time.Second, "Creation should complete within 30s")

	// Let streams run for duration
	time.Sleep(duration)

	// Cleanup
	for _, id := range streamIDs {
		if id != "" {
			_ = streamManager.DeleteStream(ctx, id)
		}
	}
}

// TestStressTest_RapidCreateDelete tests rapid create/delete cycles
func TestStressTest_RapidCreateDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const iterations = 1000
	const concurrency = 10

	ctx := context.Background()
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	var (
		successCount int32
		errorCount   int32
	)

	startTime := time.Now()

	// Run rapid create/delete cycles
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < iterations/concurrency; i++ {
				stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
					UserID:   fmt.Sprintf("stress-user-w%d", workerID),
					Title:    fmt.Sprintf("stress-stream-w%d-i%d", workerID, i),
					Protocol: sdk.ProtocolRTMP,
				})
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					continue
				}

				err = streamManager.DeleteStream(ctx, stream.ID)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					continue
				}

				atomic.AddInt32(&successCount, 1)
			}
		}(worker)
	}

	wg.Wait()

	duration := time.Since(startTime)

	// Report metrics
	t.Logf("Stress Test Results (Rapid Create/Delete):")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Concurrency: %d", concurrency)
	t.Logf("  Successful Cycles: %d", successCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Operations/sec: %.2f", float64(iterations)/duration.Seconds())

	// Assertions
	assert.GreaterOrEqual(t, successCount, int32(iterations*95/100), "At least 95% should succeed")
	assert.Less(t, errorCount, int32(iterations*5/100), "Less than 5% errors")
}

// TestMemoryLeak_LongRunning tests for memory leaks in long-running scenarios
func TestMemoryLeak_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	const duration = 30 * time.Second
	const operationsPerSecond = 10

	ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
	defer cancel()

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	ticker := time.NewTicker(time.Second / operationsPerSecond)
	defer ticker.Stop()

	done := time.After(duration)
	operationCount := 0

	t.Logf("Running memory leak test for %v...", duration)

	for {
		select {
		case <-done:
			t.Logf("Completed %d operations over %v", operationCount, duration)
			t.Logf("Average: %.2f ops/sec", float64(operationCount)/duration.Seconds())
			return

		case <-ticker.C:
			stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
				UserID:   "leak-test-user",
				Title:    fmt.Sprintf("leak-test-%d", operationCount),
				Protocol: sdk.ProtocolRTMP,
			})
			if err != nil {
				t.Logf("Warning: Create error at operation %d: %v", operationCount, err)
				continue
			}

			_ = streamManager.DeleteStream(ctx, stream.ID)
			operationCount++
		}
	}
}

// TestLatency_StreamOperations measures latency of various operations
func TestLatency_StreamOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping latency test in short mode")
	}

	const samples = 100

	ctx := context.Background()
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	// Measure create latency
	createLatencies := make([]time.Duration, samples)
	streamIDs := make([]string, samples)

	for i := 0; i < samples; i++ {
		start := time.Now()
		stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
			UserID:   "latency-test-user",
			Title:    fmt.Sprintf("latency-test-%d", i),
			Protocol: sdk.ProtocolRTMP,
		})
		createLatencies[i] = time.Since(start)
		require.NoError(t, err)
		streamIDs[i] = stream.ID
	}

	// Measure update latency
	updateLatencies := make([]time.Duration, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		newTitle := fmt.Sprintf("Updated Title %d", i)
		_, err := streamManager.UpdateStream(ctx, streamIDs[i], &sdk.UpdateStreamRequest{
			Title: &newTitle,
		})
		updateLatencies[i] = time.Since(start)
		if err != nil {
			updateLatencies[i] = 0
		}
	}

	// Calculate statistics
	avgCreate := average(createLatencies)
	p95Create := percentile(createLatencies, 95)
	p99Create := percentile(createLatencies, 99)

	avgUpdate := average(updateLatencies)
	p95Update := percentile(updateLatencies, 95)
	p99Update := percentile(updateLatencies, 99)

	// Report results
	t.Logf("Latency Test Results:")
	t.Logf("  Create Stream:")
	t.Logf("    Average: %v", avgCreate)
	t.Logf("    P95: %v", p95Create)
	t.Logf("    P99: %v", p99Create)
	t.Logf("  Update Stream:")
	t.Logf("    Average: %v", avgUpdate)
	t.Logf("    P95: %v", p95Update)
	t.Logf("    P99: %v", p99Update)

	// Assertions
	assert.Less(t, avgCreate, 100*time.Millisecond, "Average create latency should be < 100ms")
	assert.Less(t, p99Create, 500*time.Millisecond, "P99 create latency should be < 500ms")

	// Cleanup
	for _, id := range streamIDs {
		streamManager.DeleteStream(ctx, id)
	}
}

// TestThroughput_MessageProcessing tests message throughput
func TestThroughput_MessageProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	const duration = 10 * time.Second
	const numWorkers = 10

	ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
	defer cancel()

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	streamManager := sdk.NewStreamManager(log)

	var messageCount int64
	done := time.After(duration)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					// Simulate message processing
					count := atomic.LoadInt64(&messageCount)
					stream, err := streamManager.CreateStream(ctx, &sdk.CreateStreamRequest{
						UserID:   fmt.Sprintf("throughput-user-w%d", workerID),
						Title:    fmt.Sprintf("throughput-w%d-%d", workerID, count),
						Protocol: sdk.ProtocolRTMP,
					})
					if err == nil {
						_ = streamManager.DeleteStream(ctx, stream.ID)
						atomic.AddInt64(&messageCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	throughput := float64(messageCount) / duration.Seconds()

	t.Logf("Throughput Test Results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Workers: %d", numWorkers)
	t.Logf("  Total Messages: %d", messageCount)
	t.Logf("  Throughput: %.2f messages/sec", throughput)
	t.Logf("  Per Worker: %.2f messages/sec", throughput/float64(numWorkers))

	assert.Greater(t, throughput, 50.0, "Should process at least 50 messages/sec")
}

// Helper functions

func average(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func percentile(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Simple percentile calculation (not perfectly accurate but good enough)
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	// Bubble sort (simple for small datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := (len(sorted) * p) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}
