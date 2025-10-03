# LinHT Web Manager

A lightweight web-based management interface for LinHT, featuring an integrated terminal, file manager and container management.

## What It Does

LinHT Web Manager provides a simple web UI for managing containerized applications on LinHT. It includes:

- **Web Shell**: Browser-based terminal access
- **File Manager**: Upload, download, and manage files through the web interface
- **Docker/Podman Management**: List, create, start, stop, and delete containers and images

## Building

Use the [`Makefile`](Makefile:1) for building:

```bash
make build          # Build for current platform
make build-arm64    # Build for ARM64
make build-all      # Build for all platforms
make clean          # Clean build artifacts
make help           # Show all available targets
```

## License

This project is licensed under the GNU General Public License v3.0 - see the LICENSE file for details.