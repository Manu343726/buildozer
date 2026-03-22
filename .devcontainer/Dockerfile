# Start from Debian base image with Go pre-installed
FROM mcr.microsoft.com/devcontainers/go:1-bullseye

# Install C/C++ development tools, CMake, and GNU Make
RUN apt-get update && apt-get install -y \
    build-essential \
    gcc \
    g++ \
    gdb \
    cmake \
    make \
    ninja-build \
    clang \
    clang-format \
    clang-tidy \
    clangd \
    git \
    wget \
    curl \
    pkg-config \
    zsh \
    && rm -rf /var/lib/apt/lists/*

# Install Docker CLI (for Docker-in-Docker or connecting to Docker daemon)
RUN curl -fsSL https://get.docker.com -o get-docker.sh && chmod +x get-docker.sh && ./get-docker.sh && rm get-docker.sh

# Configure vscode user with docker group access
RUN groupadd -f docker || true && \
    usermod -aG docker vscode && \
    chsh -s /bin/zsh vscode

# Install Oh My Zsh for vscode user (if not already installed)
RUN su - vscode -c 'if [ ! -d ~/.oh-my-zsh ]; then sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended; fi'

# Configure zsh with nice plugins and theme for vscode user
RUN su - vscode -c "sed -i 's/ZSH_THEME=\"robbyrussell\"/ZSH_THEME=\"agnoster\"/' ~/.zshrc" && \
    su - vscode -c "sed -i 's/plugins=(git)/plugins=(git golang docker docker-compose kubectl command-not-found)/' ~/.zshrc"

# Enable VS Code shell integration for zsh
RUN su - vscode -c 'echo "# VS Code shell integration" >> ~/.zshrc' && \
    su - vscode -c 'echo "[[ \"\$TERM_PROGRAM\" == \"vscode\" ]] && . \"\$(code --locate-shell-integration-path zsh)\"" >> ~/.zshrc'

USER vscode

# Install Go-based tools for protobuf code generation
RUN go install github.com/bufbuild/buf/cmd/buf@v1.35.1 && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
