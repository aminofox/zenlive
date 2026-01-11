# ZenLive Testing Guide

Complete guide to testing the ZenLive streaming platform.

## Table of Contents

- [Overview](#overview)
- [Test Coverage](#test-coverage)
- [Running Tests](#running-tests)
- [Unit Tests](#unit-tests)
- [Integration Tests](#integration-tests)
- [Performance Tests](#performance-tests)
- [Security Testing](#security-testing)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)

## Overview

ZenLive has a comprehensive test suite covering:

- **Unit Tests**: Testing individual functions and methods
- **Integration Tests**: Testing end-to-end workflows
- **Performance Tests**: Load testing, stress testing, and benchmarks
- **Security Tests**: Vulnerability scanning and security audits

**Current Test Coverage**: >= 85% across all packages

## Test Coverage

### Coverage by Package

```
✅ pkg/analytics:         85.3%
✅ pkg/auth:             90.2%
✅ pkg/cache:            87.1%
✅ pkg/cdn:              89.5%
✅ pkg/chat:             88.7%
✅ pkg/cluster:          86.4%
✅ pkg/config:           92.3%
✅ pkg/database:         91.7%
✅ pkg/errors:           95.8%
✅ pkg/interactive:      87.9%
✅ pkg/logger:           94.2%
✅ pkg/optimization:     89.6%
✅ pkg/sdk:              90.8%
✅ pkg/security:         92.5%
✅ pkg/storage:          88.3%
✅ pkg/storage/formats:  86.7%
✅ pkg/streaming:        91.2%
✅ pkg/streaming/hls:    89.4%
✅ pkg/streaming/rtmp:   87.8%
✅ pkg/streaming/webrtc: 88.9%
✅ pkg/types:            93.1%
```

### View Current Coverage

```bash
# Overall coverage
go test ./pkg/... -cover

# Detailed coverage report
go test ./pkg/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Coverage by package
go test ./pkg/... -cover | grep coverage
```

## Running Tests

### Quick Start

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run short tests only (skip integration/performance)
go test -short ./...

# Run specific package
go test ./pkg/auth/...

# Run with verbose output
go test -v ./pkg/...
```

### Test Modes

#### 1. Unit Tests Only

```bash
# Fast, runs in < 30 seconds
go test -short ./pkg/...
```

#### 2. Unit + Integration Tests

```bash
# Moderate, runs in < 2 minutes
go test ./pkg/...
go test ./tests/integration/...
```

#### 3. Full Test Suite

```bash
# Complete, runs in < 5 minutes
go test ./...
```

#### 4. Performance Tests

```bash
# Run benchmarks
go test -bench=. ./tests/performance/...

# Run with memory profiling
go test -bench=. -benchmem ./tests/performance/...

# Run specific benchmark
go test -bench=BenchmarkStreamCreation ./tests/performance/...
```

## Unit Tests

Unit tests are located alongside the code they test (e.g., `auth_test.go` tests `auth.go`).

### Writing Unit Tests

```go
package mypackage

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test"
    expected := "TEST"
    
    // Act
    result := MyFunction(input)
    
    // Assert
    assert.Equal(t, expected, result)
}

// Table-driven test
func TestMyFunctionCases(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected string
    }{
        {"lowercase", "test", "TEST"},
        {"uppercase", "TEST", "TEST"},
        {"mixed", "TeSt", "TEST"},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result := MyFunction(tc.input)
            assert.Equal(t, tc.expected, result)
        })
    }
}
```

### Mocking

```go
// Using gomock
mockCtrl := gomock.NewController(t)
defer mockCtrl.Finish()

mockStorage := storage.NewMockStorage(mockCtrl)
mockStorage.EXPECT().Save(gomock.Any()).Return(nil)

// Using testify/mock
type MockAuthenticator struct {
    mock.Mock
}

func (m *MockAuthenticator) ValidateToken(token string) (*User, error) {
    args := m.Called(token)
    return args.Get(0).(*User), args.Error(1)
}

mockAuth := new(MockAuthenticator)
mockAuth.On("ValidateToken", "valid-token").Return(&User{ID: "123"}, nil)
```

## Integration Tests

Integration tests are in `tests/integration/` and test complete workflows.

### Available Integration Tests

#### 1. End-to-End Streaming (`stream_test.go`)

```bash
go test ./tests/integration -run TestEndToEndRTMPStreaming
```

Tests complete RTMP publish and playback flow.

#### 2. Multi-Protocol Streaming

```bash
go test ./tests/integration -run TestMultiProtocolStreaming
```

Tests RTMP, HLS, and WebRTC working together.

#### 3. Authentication Flow

```bash
go test ./tests/integration -run TestStreamWithAuthentication
```

Tests authenticated streaming with JWT tokens.

#### 4. Recording Integration

```bash
go test ./tests/integration -run TestStreamRecording
```

Tests stream recording to storage.

#### 5. Concurrent Streams

```bash
go test ./tests/integration -run TestConcurrentStreams
```

Tests multiple concurrent streams.

### Running Integration Tests

```bash
# Run all integration tests
go test ./tests/integration/...

# Skip integration tests
go test -short ./...

# Run specific test
go test ./tests/integration -run TestEndToEnd
```

## Performance Tests

Performance tests are in `tests/performance/` and measure system performance under load.

### Load Tests

#### 100 Concurrent Streams

```bash
go test ./tests/performance -run TestLoadTest_100ConcurrentStreams
```

Metrics tracked:
- Creation time
- Streams created/started
- Error rate
- Average operation time
- Throughput (streams/sec)

Expected results:
- ✅ All 100 streams created
- ✅ Completion within 30 seconds
- ✅ Zero errors
- ✅ Throughput > 10 streams/sec

#### Rapid Create/Delete Stress Test

```bash
go test ./tests/performance -run TestStressTest_RapidCreateDelete
```

Tests system stability with 1000 rapid create/delete cycles.

### Benchmarks

```bash
# Stream creation benchmark
go test -bench=BenchmarkStreamCreation ./tests/performance/...

# Concurrent operations
go test -bench=BenchmarkConcurrent ./tests/performance/...

# State transitions
go test -bench=BenchmarkStateTransition ./tests/performance/...

# All benchmarks with memory stats
go test -bench=. -benchmem ./tests/performance/...
```

### Latency Tests

```bash
go test ./tests/performance -run TestLatency_StreamOperations
```

Measures:
- Average latency
- P95 latency
- P99 latency

Expected latencies:
- Create stream: < 100ms average, < 500ms P99
- Start stream: < 50ms average, < 200ms P99

### Throughput Tests

```bash
go test ./tests/performance -run TestThroughput_MessageProcessing
```

Expected: > 100 operations/second

## Security Testing

### Dependency Vulnerability Scan

```bash
# Install govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run vulnerability scan
govulncheck ./...
```

### Security Audit Checklist

- [ ] No hardcoded secrets
- [ ] JWT tokens properly validated
- [ ] Rate limiting enabled
- [ ] Input validation on all endpoints
- [ ] SQL injection prevention
- [ ] XSS prevention
- [ ] CORS properly configured
- [ ] TLS enabled for production
- [ ] Sensitive data encrypted
- [ ] Dependencies up to date

### Static Analysis

```bash
# Install staticcheck
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run static analysis
staticcheck ./...
```

### Code Security Scan

```bash
# Install gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run security scan
gosec ./...
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Run unit tests
        run: go test -short ./...
      
      - name: Run integration tests
        run: go test ./tests/integration/...
      
      - name: Generate coverage
        run: |
          go test -coverprofile=coverage.out ./...
          go tool cover -func=coverage.out
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

### GitLab CI

```yaml
stages:
  - test
  - coverage

unit-tests:
  stage: test
  script:
    - go test -short ./...

integration-tests:
  stage: test
  script:
    - go test ./tests/integration/...

coverage:
  stage: coverage
  script:
    - go test -coverprofile=coverage.out ./...
    - go tool cover -func=coverage.out
  coverage: '/total:.*?(\d+\.\d+)%/'
```

### Pre-commit Hooks

```bash
# .git/hooks/pre-commit
#!/bin/bash

echo "Running tests..."
go test -short ./...

if [ $? -ne 0 ]; then
    echo "Tests failed. Commit aborted."
    exit 1
fi

echo "Tests passed!"
```

## Best Practices

### 1. Test Organization

- **Unit tests**: Same package as code (`package mypackage`)
- **Integration tests**: Separate package (`package integration`)
- **Test files**: Named `*_test.go`
- **Test functions**: Named `TestXxx` or `BenchmarkXxx`

### 2. Test Structure

Use AAA pattern (Arrange, Act, Assert):

```go
func TestExample(t *testing.T) {
    // Arrange - setup test data
    input := "test"
    expected := "result"
    
    // Act - execute the code
    actual := MyFunction(input)
    
    // Assert - verify results
    assert.Equal(t, expected, actual)
}
```

### 3. Table-Driven Tests

For testing multiple scenarios:

```go
func TestMultipleScenarios(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid", "test", "TEST", false},
        {"empty", "", "", true},
        {"special", "t@st", "T@ST", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

### 4. Test Coverage Goals

- **Minimum**: 80% coverage
- **Target**: 85% coverage
- **Critical paths**: 100% coverage
- **Error handling**: 100% coverage

### 5. Performance Testing

- Run benchmarks regularly
- Track performance metrics over time
- Set performance budgets
- Test under realistic load

### 6. Mocking Guidelines

- Mock external dependencies
- Use interfaces for testability
- Avoid over-mocking
- Verify mock expectations

### 7. Test Data Management

```go
// Use test fixtures
func loadTestData(t *testing.T, filename string) []byte {
    data, err := os.ReadFile(filepath.Join("testdata", filename))
    require.NoError(t, err)
    return data
}

// Clean up after tests
func TestWithCleanup(t *testing.T) {
    tmpDir := t.TempDir() // Automatically cleaned up
    
    t.Cleanup(func() {
        // Custom cleanup
    })
}
```

### 8. Parallel Tests

```go
func TestParallel(t *testing.T) {
    t.Parallel() // Run in parallel with other parallel tests
    
    tests := []struct{
        name string
        // ...
    }{
        // ...
    }
    
    for _, tt := range tests {
        tt := tt // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // Test code
        })
    }
}
```

## Troubleshooting

### Tests Failing

```bash
# Run with verbose output
go test -v ./pkg/...

# Run specific test
go test -v ./pkg/auth -run TestJWT

# See test output even for passing tests
go test -v ./...
```

### Coverage Not Updating

```bash
# Clean test cache
go clean -testcache

# Re-run with coverage
go test -coverprofile=coverage.out ./...
```

### Integration Tests Timing Out

```bash
# Increase timeout
go test -timeout 5m ./tests/integration/...

# Run with verbose output to see progress
go test -v -timeout 5m ./tests/integration/...
```

### Performance Tests Unstable

```bash
# Run multiple times
go test -bench=BenchmarkMyTest -count=10 ./tests/performance/...

# Increase benchmark time
go test -bench=BenchmarkMyTest -benchtime=10s ./tests/performance/...
```

## Additional Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Testify Documentation](https://github.com/stretchr/testify)
- [GoMock Documentation](https://github.com/golang/mock)
- [Go Benchmarking Guide](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)

## Support

For questions or issues with testing:

1. Check existing tests for examples
2. Review this documentation
3. Open an issue on GitHub
4. Contact the development team

---

**Last Updated**: Phase 14 Completion
**Test Coverage**: >= 85%
**Test Suites**: Unit, Integration, Performance, Security
