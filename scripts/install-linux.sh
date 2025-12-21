#!/bin/bash
# TokMesh Server - Linux Installation Script
# This script installs TokMesh Server as a systemd service

set -euo pipefail

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/tokmesh-server"
DATA_DIR="/var/lib/tokmesh-server"
LOG_DIR="/var/log/tokmesh-server"
USER="tokmesh"
GROUP="tokmesh"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

detect_system() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VER=$VERSION_ID
        log_info "Detected OS: $OS $VER"
    else
        log_error "Cannot detect operating system"
        exit 1
    fi
}

check_systemd() {
    if ! command -v systemctl &> /dev/null; then
        log_error "systemd is required but not found"
        exit 1
    fi
}

create_user() {
    if id "$USER" &>/dev/null; then
        log_info "User $USER already exists"
    else
        log_info "Creating user $USER..."
        useradd --system --no-create-home \
            --home-dir "$DATA_DIR" \
            --shell /usr/sbin/nologin \
            --comment "TokMesh Server" \
            "$USER"
    fi
}

install_binaries() {
    log_info "Installing binaries..."

    # Check if binaries exist in current directory
    if [ ! -f "tokmesh-server" ]; then
        log_error "tokmesh-server binary not found in current directory"
        exit 1
    fi

    if [ ! -f "tokmesh-cli" ]; then
        log_error "tokmesh-cli binary not found in current directory"
        exit 1
    fi

    install -m 0755 tokmesh-server "$INSTALL_DIR/"
    install -m 0755 tokmesh-cli "$INSTALL_DIR/"

    log_info "Binaries installed to $INSTALL_DIR"
}

create_directories() {
    log_info "Creating directories..."

    # Config directory (root:tokmesh, 0750)
    install -d -o root -g "$GROUP" -m 0750 "$CONFIG_DIR"

    # Data directory (tokmesh:tokmesh, 0750)
    install -d -o "$USER" -g "$GROUP" -m 0750 "$DATA_DIR"
    install -d -o "$USER" -g "$GROUP" -m 0750 "$DATA_DIR/wal"
    install -d -o "$USER" -g "$GROUP" -m 0750 "$DATA_DIR/snapshots"

    # Log directory (tokmesh:tokmesh, 0750)
    install -d -o "$USER" -g "$GROUP" -m 0750 "$LOG_DIR"

    log_info "Directories created"
}

install_config() {
    log_info "Installing configuration..."

    # Check if config file exists
    if [ -f "$CONFIG_DIR/config.yaml" ]; then
        log_warn "Configuration file already exists at $CONFIG_DIR/config.yaml"
        log_warn "Creating backup at $CONFIG_DIR/config.yaml.backup"
        cp "$CONFIG_DIR/config.yaml" "$CONFIG_DIR/config.yaml.backup"
    fi

    # Copy config if provided
    if [ -f "config.yaml" ]; then
        install -o root -g "$GROUP" -m 0640 config.yaml "$CONFIG_DIR/"
        log_info "Configuration installed to $CONFIG_DIR/config.yaml"
    else
        log_warn "No config.yaml found in current directory"
        log_warn "Please manually create $CONFIG_DIR/config.yaml"
    fi
}

install_systemd_service() {
    log_info "Installing systemd service..."

    # Check if service file exists
    if [ ! -f "tokmesh-server.service" ]; then
        log_error "tokmesh-server.service not found in current directory"
        exit 1
    fi

    install -m 0644 tokmesh-server.service /etc/systemd/system/
    systemctl daemon-reload

    log_info "Systemd service installed"
}

enable_service() {
    log_info "Enabling service..."
    systemctl enable tokmesh-server
    log_info "Service enabled (will start on boot)"
}

print_summary() {
    echo ""
    log_info "Installation complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Review/edit configuration: $CONFIG_DIR/config.yaml"
    echo "  2. Start service: systemctl start tokmesh-server"
    echo "  3. Check status: systemctl status tokmesh-server"
    echo "  4. View logs: journalctl -u tokmesh-server -f"
    echo ""
    echo "Useful commands:"
    echo "  - Reload config: systemctl reload tokmesh-server"
    echo "  - Restart: systemctl restart tokmesh-server"
    echo "  - Stop: systemctl stop tokmesh-server"
    echo ""
}

# Main installation flow
main() {
    log_info "TokMesh Server Installation Script"
    echo ""

    check_root
    detect_system
    check_systemd
    create_user
    install_binaries
    create_directories
    install_config
    install_systemd_service
    enable_service
    print_summary
}

main "$@"
