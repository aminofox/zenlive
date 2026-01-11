# ZenLive Documentation

## 📚 Documentation Index

### Getting Started
- **[Getting Started Guide](getting-started.md)** - Quick start guide for new users
- **[Configuration Guide](configuration.md)** - Complete configuration reference

### Architecture & Design
- **[Architecture Overview](architecture.md)** - System architecture, SDK philosophy, and design principles
  - Core components and protocols
  - SDK design philosophy (real-time vs persistence)
  - Configuration summary and best practices
  - Architecture analysis and recommendations

### Development Guides
- **[Testing Guide](testing.md)** - How to test your streaming application
- **[Migration Guide](migration.md)** - Migrate from other solutions or upgrade ZenLive
  - Migrating from Wowza, Ant Media, etc.
  - Version upgrade guides
  - Recent simplification changes (v2.0)

### Tutorials
- **[Tutorial 1: First Streaming Server](tutorials/01-first-streaming-server.md)** - Build your first RTMP+HLS server
- **[Tutorial 2: Recording Streams](tutorials/02-recording-streams.md)** - Add recording capabilities
- **[Tutorial 3: WebRTC Streaming](tutorials/03-webrtc-streaming.md)** - Low-latency streaming with WebRTC

### Advanced Topics
- **[Troubleshooting Guide](troubleshooting.md)** - Common issues and solutions
- **[Publishing to pkg.go.dev](publishing-to-pkg-go-dev.md)** - How to publish packages

---

## 🎯 Quick Links by Use Case

### "I want to build a livestream platform"
1. Read [Getting Started](getting-started.md)
2. Follow [Tutorial 1](tutorials/01-first-streaming-server.md)
3. Review [Architecture](architecture.md) to understand SDK philosophy
4. Check [Configuration](configuration.md) for production setup

### "I need to add chat to my streams"
1. Check SDK Philosophy section in [Architecture](architecture.md)
2. Note: SDK only delivers real-time messages, you persist to YOUR database
3. See chat examples in `/examples/chat/`

### "I'm migrating from another platform"
1. Read [Migration Guide](migration.md)
2. Check feature comparison tables
3. Review [Configuration](configuration.md) for equivalent settings

### "I have issues with my streams"
1. Check [Troubleshooting Guide](troubleshooting.md)
2. Verify configuration in [Configuration Guide](configuration.md)
3. Review logs and error messages

---

## 📖 Documentation Philosophy

ZenLive documentation follows these principles:

1. **Progressive Disclosure**: Start simple, add complexity as needed
2. **Example-Driven**: Every feature has working code examples
3. **Clear Separation**: SDK responsibilities vs. user responsibilities
4. **Production-Ready**: Focus on real-world deployment scenarios

---

## 🔗 External Resources

- **GitHub Repository**: [github.com/aminofox/zenlive](https://github.com/aminofox/zenlive)
- **API Documentation**: Auto-generated at [pkg.go.dev](https://pkg.go.dev/github.com/aminofox/zenlive)
- **Examples**: See `/examples/` directory in the repository

---

## 📝 Contributing to Documentation

Found an issue or want to improve the docs?

1. Documentation source is in `/docs/` directory
2. Use clear, concise language
3. Include code examples where applicable
4. Test all code examples before submitting

---

**Last Updated**: January 11, 2026
