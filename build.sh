#!/bin/bash

set -euo pipefail

# Build configuration
APP_NAME="memory"
BUILD_DIR="bin"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Clean build artifacts
clean() {
    log_info "Cleaning build artifacts..."
    rm -rf "$BUILD_DIR"
    log_info "Clean completed."
}

# Build the application
build() {
    log_info "Building $APP_NAME (version: $VERSION)..."

    mkdir -p "$BUILD_DIR"

    go build -ldflags="$LDFLAGS" -o "$BUILD_DIR/$APP_NAME" "./cmd/$APP_NAME"

    log_info "Build completed: $BUILD_DIR/$APP_NAME"
}

# Run tests
test() {
    log_info "Running tests..."
    go test -v -race -cover ./...
    log_info "Tests completed."
}

# Build for multiple platforms
build_all() {
    log_info "Building for multiple platforms..."

    mkdir -p "$BUILD_DIR"

    platforms=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64")

    for platform in "${platforms[@]}"; do
        GOOS="${platform%/*}"
        GOARCH="${platform#*/}"
        output="$BUILD_DIR/${APP_NAME}-${GOOS}-${GOARCH}"

        log_info "Building for $GOOS/$GOARCH..."
        GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="$LDFLAGS" -o "$output" "./cmd/$APP_NAME"
    done

    log_info "Multi-platform build completed."
}

# Show usage
usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  build     Build the application (default)"
    echo "  clean     Clean build artifacts"
    echo "  test      Run tests"
    echo "  all       Build for all platforms"
    echo "  help      Show this help message"
}

# Main
case "${1:-build}" in
    build)
        build
        ;;
    clean)
        clean
        ;;
    test)
        test
        ;;
    all)
        build_all
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        log_error "Unknown command: $1"
        usage
        exit 1
        ;;
esac
