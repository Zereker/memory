# Memory System Makefile

.PHONY: help build run test clean status init clear reset preview infra-up infra-down

# Go build settings
BINARY := bin/memory
CONFIG := configs/config.toml
GO_FLAGS := -v

# Python CLI
CLI := python3 scripts/cli.py

# Index name (可通过 make init INDEX=memories_test 指定)
INDEX ?= memories

# Default target
help:
	@echo "Memory System Commands"
	@echo ""
	@echo "Build & Run:"
	@echo "  make build        Build the memory server"
	@echo "  make run          Run the memory server"
	@echo ""
	@echo "Infrastructure:"
	@echo "  make infra-up     Start OpenSearch and Neo4j (Docker)"
	@echo "  make infra-down   Stop infrastructure"
	@echo "  make status       Show service status"
	@echo "  make init         Initialize indexes (INDEX=memories)"
	@echo "  make clear        Clear index data (INDEX=memories)"
	@echo "  make reset        Clear + init (INDEX=memories)"
	@echo ""
	@echo "Testing:"
	@echo "  make test         Run Go tests"
	@echo "  make test-quick   Quick smoke test"
	@echo "  make test-store   Store test data"
	@echo "  make test-retrieve  Retrieve test"
	@echo "  make test-full    Full test (store + retrieve)"
	@echo "  make preview      Reset + full test"
	@echo ""
	@echo "Other:"
	@echo "  make clean        Clean build artifacts"
	@echo "  make tidy         Run go mod tidy"

# ============================================================
# Build & Run
# ============================================================

build:
	@echo "Building..."
	go build $(GO_FLAGS) -o $(BINARY) ./cmd/memory

run: build
	./$(BINARY) -config $(CONFIG)

# ============================================================
# Infrastructure
# ============================================================

infra-up:
	@echo "Starting OpenSearch..."
	docker run -d --name opensearch \
		-p 9200:9200 -p 9600:9600 \
		-e "discovery.type=single-node" \
		-e "DISABLE_SECURITY_PLUGIN=true" \
		opensearchproject/opensearch:2.11.0 || true
	@echo "Starting Neo4j..."
	docker run -d --name neo4j \
		-p 7474:7474 -p 7687:7687 \
		-e NEO4J_AUTH=neo4j/YOUR_NEO4J_PASSWORD \
		neo4j:5.15.0 || true
	@echo "Waiting for services to start (10s)..."
	@sleep 10
	@$(MAKE) status

infra-down:
	@echo "Stopping infrastructure..."
	docker stop opensearch neo4j 2>/dev/null || true
	docker rm opensearch neo4j 2>/dev/null || true
	@echo "Done."

status:
	@$(CLI) status

init:
	@$(CLI) init --index $(INDEX)

clear:
	@$(CLI) clear --index $(INDEX)

reset:
	@$(CLI) reset --index $(INDEX)

# ============================================================
# Testing
# ============================================================

test:
	go test -v ./...

test-quick:
	@$(CLI) test quick

test-store:
	@$(CLI) test store

test-retrieve:
	@$(CLI) test retrieve

test-full:
	@$(CLI) test full

preview:
	@$(CLI) preview full

# ============================================================
# Utilities
# ============================================================

clean:
	rm -rf bin/
	rm -rf data/test_results_*.md

tidy:
	go mod tidy
