# Development Container Configuration

This directory contains the development container configuration for buildozer.

## Features

The devcontainer includes:

- **Go** - Full Go toolchain for development
- **C/C++** - Complete build tools (gcc, g++, clang, gdb)
- **CMake** - Modern build system
- **GNU Make** - Traditional build automation
- **Docker** - Docker CLI with Docker-in-Docker support
- **Zsh** - Modern shell with Oh My Zsh framework
- **VS Code Extensions** - Go, C++, CMake, Git Lens, and more

## Setup

### Prerequisites
- Docker Desktop installed
- VS Code with "Dev Containers" extension

### Getting Started

1. Open the workspace in VS Code
2. Click the green icon in the bottom left corner
3. Select "Reopen in Container"
4. Wait for the container to build and start (first time may take 5-10 minutes)

VS Code will automatically:
- Build the Docker image based on the Dockerfile
- Create and start a container
- Mount the workspace
- Install recommended extensions
- Run post-create commands (Go modules download, linting tools)
- Configure zsh with Oh My Zsh and useful plugins

### Shell Integration

VS Code shell integration is automatically enabled. This provides:
- Command execution tracking
- Automatic directory detection
- Better error handling in integrated terminal

### Available Tools

Once in the container, you'll have access to:

```bash
# Go
go version
go mod download
go build ./...

# C/C++
gcc --version
g++ --version
clang --version
cmake --version

# Build tools
make --version
ninja -v

# Docker (Docker-in-Docker)
docker ps
docker build .

# Zsh with plugins for:
# - git
# - golang
# - docker
# - docker-compose
# - kubectl
```

### Shell Customization

The zsh configuration includes:
- **Theme**: agnoster (with Git status indicators)
- **Plugins**: git, golang, docker, docker-compose, kubectl
- **VS Code Integration**: Shell integration enabled

You can further customize `~/.zshrc` inside the container.

### Environment Variables

The container has access to:
- `GOPATH`: Go workspace
- `PATH`: Updated for Go binaries
- Standard build environment variables

### Build Examples

```bash
# Go project
go build ./...
go test ./...

# C project with CMake
mkdir -p build
cd build
cmake ..
make

# Go + C integration
CGO_ENABLED=1 go build -v ./...
```

### Docker in Docker

The container is configured to access the Docker daemon from the host machine through socket mounting. This allows you to:

```bash
docker ps
docker build .
docker run ...
```

### Extending the Configuration

To add more tools or modify the environment:

1. Edit `Dockerfile` to add packages or tools
2. Edit `devcontainer.json` to:
   - Add VS Code extensions
   - Configure additional settings
   - Add port forwarding
   - Mount additional volumes

After modifying, rebuild the container:
- VS Code will prompt you to rebuild, or
- Right-click the bottom-left Dev Container indicator → "Rebuild Container"

## Troubleshooting

### Container won't build
- Check Docker is running
- Delete the image and rebuild: `docker rm <container_name>`

### Go tools not available
- Wait for `postCreateCommand` to complete (check terminal output)
- May need to close and reopen VS Code

### Docker commands not working
- Ensure Docker Desktop is running on the host
- Check socket permissions: `ls -la /var/run/docker.sock`

### Oh My Zsh not showing themes properly
- Ensure a Nerd Font or Powerline font is selected in VS Code
- Settings → Terminal › Integrated: Font Family

### C/C++ IntelliSense not working
- Wait for extension to configure (may take 30s after opening)
- Run C++ extension setup command if needed
