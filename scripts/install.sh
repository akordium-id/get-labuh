#!/bin/bash
set -euo pipefail

# Labuh Installation Script for Ubuntu/Debian

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LABUH_USER="labuh"
LABUH_GROUP="labuh"
LABUH_DATA_DIR="/var/lib/labuh"
LABUH_CONFIG_DIR="/etc/labuh"
LABUH_SERVICE_FILE="/etc/systemd/system/labuh.service"
LABUH_BIN="/usr/local/bin/labuh"

log() {
    echo "[LABUH] $1"
}

fail() {
    log "FAILED: $1"
    exit 1
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        fail "This script must be run as root (use sudo)"
    fi
}

install_dependencies() {
    log "Installing dependencies..."
    apt-get update
    apt-get install -y --no-install-recommends \
        curl \
        wget \
        git \
        ca-certificates \
        sqlite3 \
        build-essential \
        systemd
}

install_go() {
    if command -v go &> /dev/null; then
        log "Go already installed: $(go version)"
        return
    fi

    log "Installing Go 1.26..."
    GO_VERSION="1.26.0"
    ARCH=$(dpkg --print-architecture)
    
    case $ARCH in
        amd64) GO_ARCH="amd64" ;;
        arm64) GO_ARCH="arm64" ;;
        armhf) GO_ARCH="armv6l" ;;
        *) fail "Unsupported architecture: $ARCH" ;;
    esac

    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz

    export PATH=$PATH:/usr/local/go/bin
    log "Go installed: $(go version)"
}

install_docker() {
    if command -v docker &> /dev/null; then
        log "Docker already installed"
        return
    fi

    log "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
}

create_user() {
    if ! id "$LABUH_USER" &>/dev/null; then
        log "Creating labuh user..."
        useradd --system --shell /usr/sbin/nologin --home-dir "$LABUH_DATA_DIR" "$LABUH_USER"
    fi
}

setup_directories() {
    log "Creating directories..."
    mkdir -p "$LABUH_DATA_DIR"/{backups,plugins}
    mkdir -p "$LABUH_CONFIG_DIR"
    chown -R "$LABUH_USER:$LABUH_GROUP" "$LABUH_DATA_DIR"
}

build_labuh() {
    log "Building Labuh..."
    cd "$PROJECT_ROOT"
    
    if ! command -v templ &> /dev/null; then
        log "Installing templ..."
        go install github.com/a-h/templ/cmd/templ@latest
    fi
    
    if ! command -v tailwindcss &> /dev/null; then
        log "Installing tailwindcss..."
        npm install -g tailwindcss @tailwindcss/cli
    fi
    
    templ generate
    tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.css --minify || true
    go build -o "$LABUH_BIN" ./cmd/labuh
    chmod +x "$LABUH_BIN"
    chown "$LABUH_USER:$LABUH_GROUP" "$LABUH_BIN"
}

create_systemd_service() {
    log "Creating systemd service..."
    
    cat > "$LABUH_SERVICE_FILE" <<EOF
[Unit]
Description=Labuh Self-hosted PaaS
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=$LABUH_USER
Group=$LABUH_GROUP
WorkingDirectory=$LABUH_DATA_DIR
ExecStart=$LABUH_BIN
Restart=always
RestartSec=5
Environment="PORT=3000"
Environment="DATABASE_URL=$LABUH_DATA_DIR/labuh.db"
EnvironmentFile=-$LABUH_CONFIG_DIR/labuh.env

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    log "Systemd service created at $LABUH_SERVICE_FILE"
}

setup_firewall() {
    log "Configuring firewall..."
    if command -v ufw &> /dev/null; then
        ufw allow 3000/tcp || true
    fi
    if command -v firewall-cmd &> /dev/null; then
        firewall-cmd --add-port=3000/tcp --permanent || true
        firewall-cmd --reload || true
    fi
}

print_summary() {
    log "========================================"
    log "Labuh installation complete!"
    log "========================================"
    log ""
    log "Next steps:"
    log "1. Edit $LABUH_CONFIG_DIR/labuh.env and set required environment variables:"
    log "   - SESSION_SECRET"
    log "   - LABUH_MASTER_KEY"
    log ""
    log "2. Start Labuh:"
    log "   systemctl enable --now labuh"
    log ""
    log "3. Access Labuh:"
    log "   http://$(hostname -I | awk '{print $1}'):3000"
    log ""
    log "4. View logs:"
    log "   journalctl -u labuh -f"
    log ""
    log "Configuration directory: $LABUH_CONFIG_DIR"
    log "Data directory: $LABUH_DATA_DIR"
    log "Binary: $LABUH_BIN"
}

main() {
    check_root
    install_dependencies
    install_go
    install_docker
    create_user
    setup_directories
    build_labuh
    create_systemd_service
    setup_firewall
    print_summary
}

main
