# Publishing ZenLive SDK to pkg.go.dev

This guide explains how to publish the ZenLive SDK to pkg.go.dev (Go's official package documentation site) so that other developers can easily discover and use your SDK.

---

## Prerequisites

✅ Your SDK is already properly configured:
- Module path: `github.com/aminofox/zenlive`
- Go version: 1.24.0
- All code follows Go conventions
- Documentation comments are in place

---

## Step 1: Ensure Your Code is on GitHub

pkg.go.dev automatically indexes Go modules from public version control systems (primarily GitHub).

**Check your repository**:
```bash
cd /Users/seang/Downloads/dev/zen-live

# Check remote URL
git remote -v
```

**Expected output**:
```
origin  https://github.com/aminofox/zenlive.git (fetch)
origin  https://github.com/aminofox/zenlive.git (push)
```

**If not set up yet**:
```bash
# Initialize git (if not already done)
git init

# Add all files
git add .

# Commit
git commit -m "Release v1.0.0: Production-ready livestreaming SDK"

# Create GitHub repository at https://github.com/aminofox/zenlive
# Then add remote
git remote add origin https://github.com/aminofox/zenlive.git

# Push to GitHub
git push -u origin main
```

---

## Step 2: Tag Your Release with Semantic Versioning

pkg.go.dev uses Git tags to identify module versions. You MUST tag your release.

**Create v1.0.0 tag**:
```bash
# Create annotated tag (recommended)
git tag -a v1.0.0 -m "Release v1.0.0: Production-ready livestreaming SDK

Features:
- Multi-protocol streaming (RTMP, HLS, WebRTC)
- Real-time chat system
- Recording & storage (MP4/FLV, S3)
- Authentication & security (JWT, RBAC)
- Analytics & monitoring (Prometheus)
- Interactive features (polls, gifts, reactions)
- Scalability (clustering, load balancing)
- Comprehensive documentation & examples

See RELEASE_NOTES.md for full details."

# Verify tag was created
git tag -l

# Push tag to GitHub
git push origin v1.0.0
```

**Tag naming conventions**:
- **v1.0.0**: First stable release (what you should use)
- **v1.0.1**: Patch release (bug fixes)
- **v1.1.0**: Minor release (new features, backward compatible)
- **v2.0.0**: Major release (breaking changes)

**Important**: Always prefix with `v` (e.g., `v1.0.0`, NOT `1.0.0`)

---

## Step 3: Trigger pkg.go.dev Indexing

pkg.go.dev will automatically discover your module when:
1. Someone requests it via `go get`
2. You manually request indexing

### Option A: Automatic (Recommended)

Just wait 15-30 minutes after pushing your tag. pkg.go.dev crawlers will discover it.

### Option B: Manual Request

Visit this URL in your browser:
```
https://pkg.go.dev/github.com/aminofox/zenlive@v1.0.0
```

If the module isn't indexed yet, you'll see a button **"Request Indexing"**. Click it.

### Option C: Trigger via Go Proxy

Run this command to fetch through the Go proxy (which triggers indexing):
```bash
GOPROXY=https://proxy.golang.org GO111MODULE=on \
  go get github.com/aminofox/zenlive@v1.0.0
```

---

## Step 4: Verify Your Module is Published

### Check pkg.go.dev

Visit:
```
https://pkg.go.dev/github.com/aminofox/zenlive
```

You should see:
- ✅ Module documentation
- ✅ Package list (pkg/auth, pkg/streaming, etc.)
- ✅ README.md content
- ✅ Function signatures and documentation
- ✅ Examples
- ✅ Version badge (v1.0.0)

### Check Go Proxy

Verify module is in Go proxy cache:
```bash
curl https://proxy.golang.org/github.com/aminofox/zenlive/@v/list
```

Expected output:
```
v1.0.0
```

### Test Installation

Try installing your module:
```bash
# In a temporary directory
mkdir /tmp/test-zenlive
cd /tmp/test-zenlive

go mod init test
go get github.com/aminofox/zenlive@v1.0.0

# Should download successfully
```

---

## Step 5: Improve Documentation Quality

pkg.go.dev automatically extracts documentation from your code comments. Follow these best practices:

### Package-level Documentation

**Current**: Already have good package docs in zenlive.go ✅

**Best practice**:
```go
// Package zenlive provides a production-ready Go SDK for building 
// livestreaming platforms with multi-protocol support (RTMP, HLS, WebRTC).
//
// Quick Start
//
// Create a new ZenLive SDK instance:
//
//     cfg := &zenlive.Config{
//         RTMPPort: 1935,
//         HLSPort:  8080,
//     }
//     sdk, err := zenlive.New(cfg)
//     if err != nil {
//         log.Fatal(err)
//     }
//     sdk.Start()
//     defer sdk.Stop()
//
// For more examples, see https://github.com/aminofox/zenlive/tree/main/examples
package zenlive
```

### Function Documentation

**Good example** (already in your code):
```go
// New creates a new ZenLive SDK instance with the provided configuration.
// It initializes all components including streaming servers, chat, storage,
// and analytics.
//
// Returns an error if configuration is invalid or initialization fails.
func New(cfg *Config) (*SDK, error) {
    // ...
}
```

### Example Functions

pkg.go.dev recognizes `Example` functions. **You already have examples/** directory, but can also add:

**File**: `zenlive_example_test.go`
```go
package zenlive_test

import (
    "log"
    "github.com/aminofox/zenlive"
)

func Example() {
    cfg := &zenlive.Config{
        RTMPPort: 1935,
        HLSPort:  8080,
    }
    
    sdk, err := zenlive.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    
    sdk.Start()
    defer sdk.Stop()
    
    // Output: (examples show on pkg.go.dev)
}

func ExampleNew() {
    cfg := &zenlive.Config{
        RTMPPort: 1935,
    }
    
    sdk, _ := zenlive.New(cfg)
    sdk.Start()
}
```

---

## Step 6: Add Badges to README

Make your README more attractive with badges:

```markdown
# ZenLive

[![Go Reference](https://pkg.go.dev/badge/github.com/aminofox/zenlive.svg)](https://pkg.go.dev/github.com/aminofox/zenlive)
[![Go Report Card](https://goreportcard.com/badge/github.com/aminofox/zenlive)](https://goreportcard.com/report/github.com/aminofox/zenlive)
[![GitHub release](https://img.shields.io/github/v/release/aminofox/zenlive)](https://github.com/aminofox/zenlive/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
```

**Your README.md already has these!** ✅

---

## Step 7: Update After Each Release

When you release new versions:

```bash
# Make your changes
git add .
git commit -m "Add feature X"

# Create new tag
git tag -a v1.1.0 -m "Release v1.1.0: Add feature X"

# Push
git push origin main
git push origin v1.1.0
```

pkg.go.dev will automatically index the new version.

---

## Common Issues and Solutions

### Issue 1: Module Not Found

**Symptom**: `go get` fails with "module not found"

**Solutions**:
1. Wait 15-30 minutes after pushing tag
2. Check tag name starts with `v` (e.g., `v1.0.0`)
3. Verify repository is public
4. Manually request indexing on pkg.go.dev

### Issue 2: Documentation Not Showing

**Symptom**: pkg.go.dev shows module but no docs

**Solutions**:
1. Ensure package comments start with `// Package packagename`
2. Run `go doc` locally to verify:
   ```bash
   go doc github.com/aminofox/zenlive
   ```
3. Re-request indexing

### Issue 3: Examples Not Showing

**Symptom**: Examples don't appear on pkg.go.dev

**Solutions**:
1. Rename functions to `Example`, `ExampleNew`, etc.
2. Put in `*_test.go` files
3. Include `// Output:` comment

### Issue 4: Old Version Showing

**Symptom**: pkg.go.dev shows old version as "latest"

**Solutions**:
1. Ensure new tag follows semantic versioning
2. Clear Go module cache: `go clean -modcache`
3. Wait for cache to update (can take 30 minutes)

---

## Verification Checklist

Before announcing your release, verify:

- [ ] ✅ Code is pushed to GitHub
- [ ] ✅ Tagged with `v1.0.0`
- [ ] ✅ Tag is pushed to GitHub
- [ ] ✅ Module appears on pkg.go.dev
- [ ] ✅ Documentation is readable
- [ ] ✅ README.md shows on pkg.go.dev overview
- [ ] ✅ All packages are documented
- [ ] ✅ Examples work
- [ ] ✅ `go get github.com/aminofox/zenlive@v1.0.0` succeeds
- [ ] ✅ Badges in README work

---

## Complete Command Sequence

Here's the complete sequence to publish v1.0.0:

```bash
# 1. Ensure you're in the project directory
cd /Users/seang/Downloads/dev/zen-live

# 2. Check git status
git status

# 3. Add and commit all changes
git add .
git commit -m "Release v1.0.0: Production-ready livestreaming SDK

- Multi-protocol streaming (RTMP, HLS, WebRTC)
- Real-time chat system
- Recording & storage
- Authentication & security
- Analytics & monitoring
- Interactive features
- Scalability features
- Complete documentation

See RELEASE_NOTES.md for full details."

# 4. Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0

Production-ready livestreaming SDK with multi-protocol support.

Features:
- Multi-protocol streaming (RTMP, HLS, WebRTC)
- Real-time chat system
- Recording & storage (MP4/FLV, S3)
- Authentication & security (JWT, RBAC)
- Analytics & monitoring (Prometheus)
- Interactive features (polls, gifts, reactions)
- Scalability (clustering, load balancing)

See RELEASE_NOTES.md for complete changelog."

# 5. Push to GitHub (if not already)
git push -u origin main

# 6. Push tag
git push origin v1.0.0

# 7. Wait 15-30 minutes, then visit:
#    https://pkg.go.dev/github.com/aminofox/zenlive@v1.0.0

# 8. Or manually trigger:
open "https://pkg.go.dev/github.com/aminofox/zenlive@v1.0.0"

# 9. Verify installation works
cd /tmp
mkdir test-zenlive && cd test-zenlive
go mod init test
go get github.com/aminofox/zenlive@v1.0.0
# Should succeed
```

---

## After Publishing

### Announce Your Release

1. **Hacker News**: Submit to https://news.ycombinator.com/submit
2. **Reddit**: Post to r/golang
3. **Twitter/X**: Tweet with #golang hashtag
4. **Dev.to**: Write a blog post
5. **GitHub Discussions**: Announce in Discussions tab

### Monitor

- GitHub stars ⭐
- pkg.go.dev page views
- Issues and questions
- Download counts (via pkg.go.dev)

### Maintain

- Respond to issues promptly
- Review pull requests
- Release bug fixes (v1.0.1, v1.0.2)
- Add features in minor releases (v1.1.0, v1.2.0)
- Update documentation

---

## Resources

- **pkg.go.dev**: https://pkg.go.dev
- **Go Modules Reference**: https://go.dev/ref/mod
- **Versioning Guide**: https://go.dev/doc/modules/version-numbers
- **Documentation Guidelines**: https://go.dev/blog/godoc
- **Publishing Modules**: https://go.dev/doc/modules/publishing

---

## Quick Reference

**Module path**: `github.com/aminofox/zenlive`

**Installation**:
```bash
go get github.com/aminofox/zenlive@v1.0.0
```

**Import**:
```go
import "github.com/aminofox/zenlive"
```

**Documentation**:
- pkg.go.dev: https://pkg.go.dev/github.com/aminofox/zenlive
- GitHub: https://github.com/aminofox/zenlive

**Support**:
- Issues: https://github.com/aminofox/zenlive/issues
- Discussions: https://github.com/aminofox/zenlive/discussions

---

**Status**: Ready to publish ✅  
**Next step**: Run the command sequence above to release v1.0.0!
