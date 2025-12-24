# Contributing to Memory

Thank you for your interest in contributing to Memory! This document provides guidelines and steps for contributing.

## Getting Started

### Prerequisites

- Go 1.24+
- Docker and Docker Compose
- OpenSearch 2.x
- Neo4j 5.x

### Development Setup

1. Clone the repository:
```bash
git clone https://github.com/Zereker/memory.git
cd memory
```

2. Copy the example configuration:
```bash
cp configs/config.toml.example configs/config.toml
```

3. Edit `configs/config.toml` with your credentials.

4. Start the dependencies:
```bash
make init
```

5. Run the application:
```bash
make run
```

## How to Contribute

### Reporting Issues

- Use GitHub Issues to report bugs or suggest features
- Check existing issues before creating a new one
- Provide detailed reproduction steps for bugs

### Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Make your changes
4. Run tests: `make test`
5. Commit with clear messages
6. Push and create a Pull Request

### Commit Message Guidelines

Use clear, descriptive commit messages:
- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for test additions/changes

Example: `feat: add support for custom embedding models`

### Code Style

- Follow Go best practices and conventions
- Run `go fmt` before committing
- Ensure all tests pass
- Add tests for new functionality

## Project Structure

```
memory/
├── cmd/           # Application entry points
├── configs/       # Configuration files
├── docs/          # Documentation
├── internal/      # Internal packages
│   ├── action/    # Business logic actions
│   ├── api/       # HTTP/MCP server
│   ├── entity/    # Domain entities
│   └── storage/   # Storage implementations
└── scripts/       # Utility scripts
```

## Questions?

Feel free to open an issue for any questions about contributing.
