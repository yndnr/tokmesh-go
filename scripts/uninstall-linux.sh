#!/bin/bash
# TokMesh Server - Linux Uninstallation Script
# This script removes TokMesh Server from the system

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

stop_service() {
    log_info "Stopping tokmesh-server service..."
    if systemctl is-active --quiet tokmesh-server; then
        systemctl stop tokmesh-server
        log_info "Service stopped"
    else
        log_info "Service is not running"
    fi
}

disable_service() {
    log_info "Disabling tokmesh-server service..."
    if systemctl is-enabled --quiet tokmesh-server; then
        systemctl disable tokmesh-server
        log_info "Service disabled"
    else
        log_info "Service is not enabled"
    fi
}

remove_systemd_service() {
    log_info "Removing systemd service..."
    rm -f /etc/systemd/system/tokmesh-server.service
    systemctl daemon-reload
    systemctl reset-failed tokmesh-server 2>/dev/null || true
    log_info "Systemd service removed"
}

remove_binaries() {
    log_info "Removing binaries..."
    rm -f "$INSTALL_DIR/tokmesh-server"
    rm -f "$INSTALL_DIR/tokmesh-cli"
    log_info "Binaries removed"
}

remove_user() {
    if id "$USER" &>/dev/null; then
        log_info "Removing user $USER..."
        userdel "$USER" 2>/dev/null || log_warn "Failed to remove user $USER"
    fi
}

print_data_warning() {
    echo ""
    log_warn "Data and configuration files are preserved:"
    echo "  - Config: $CONFIG_DIR"
    echo "  - Data:   $DATA_DIR"
    echo "  - Logs:   $LOG_DIR"
    echo ""
    echo "To remove all data:"
    echo "  rm -rf $CONFIG_DIR $DATA_DIR $LOG_DIR"
    echo ""
}

confirm_removal() {
    echo ""
    log_warn "This will uninstall TokMesh Server from your system."
    echo ""
    read -p "Continue? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Uninstallation cancelled"
        exit 0
    fi
}

# Main uninstallation flow
main() {
    log_info "TokMesh Server Uninstallation Script"
    echo ""

    check_root
    confirm_removal
    stop_service
    disable_service
    remove_systemd_service
    remove_binaries
    remove_user

    echo ""
    log_info "Uninstallation complete!"
    print_data_warning
}

main "$@"
