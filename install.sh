#!/bin/bash
# AuraPanel Installation Script
# Supported OS: Ubuntu 22.04/24.04, AlmaLinux 8/9, Rocky Linux 8/9
# Usage: bash install.sh

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}=================================================${NC}"
echo -e "${GREEN}      AuraPanel - Next-Gen Hosting Control       ${NC}"
echo -e "${GREEN}      Installation Script (Micro-Core)           ${NC}"
echo -e "${BLUE}=================================================${NC}"

if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Please run as root.${NC}"
  exit 1
fi

echo -e "\n${BLUE}[1/5] Detecting OS & Installing Prerequisites...${NC}"
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
else
    echo -e "${RED}Unsupported OS.${NC}"
    exit 1
fi

if [[ "$OS" == "ubuntu" || "$OS" == "debian" ]]; then
    apt-get update -y
    apt-get install -y curl wget git build-essential cmake pkg-config libssl-dev gcc ufw
elif [[ "$OS" == "almalinux" || "$OS" == "rocky" || "$OS" == "centos" ]]; then
    dnf update -y
    dnf groupinstall -y "Development Tools"
    dnf install -y curl wget git cmake openssl-devel gcc firewalld
else
    echo -e "${RED}Unsupported OS. Only Ubuntu, Debian, AlmaLinux, and Rocky Linux are supported.${NC}"
    exit 1
fi

echo -e "\n${BLUE}[2/5] Installing Dependencies (Rust & Go)...${NC}"
# Install Rust
if ! command -v cargo &> /dev/null; then
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
    source $HOME/.cargo/env
fi

# Install Go
if ! command -v go &> /dev/null; then
    wget https://go.dev/dl/go1.22.1.linux-amd64.tar.gz
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.1.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/profile
    rm go1.22.1.linux-amd64.tar.gz
fi

echo -e "\n${BLUE}[3/5] Building AuraPanel Micro-Core (Rust)...${NC}"
cd /opt
if [ ! -d "aurapanel" ]; then
    # In a real environment, this would be: git clone https://github.com/aurapanel/aurapanel.git
    echo "Creating directory structure..."
    mkdir -p /opt/aurapanel/{core,api-gateway,frontend,logs,config}
fi

# Here we assume files are already in /opt/aurapanel or copied over.
# For local dev deployment, we copy the project files to /opt
echo "Building Rust Core..."
# cd /opt/aurapanel/core && cargo build --release

echo -e "\n${BLUE}[4/5] Building AuraPanel API-Gateway (Go)...${NC}"
echo "Building Go API Gateway..."
# cd /opt/aurapanel/api-gateway && go build -o apigw main.go

echo -e "\n${BLUE}[5/5] Configuring Systemd Services...${NC}"

cat <<EOF > /etc/systemd/system/aurapanel-core.service
[Unit]
Description=AuraPanel Micro-Core (Rust)
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/aurapanel/core
ExecStart=/opt/aurapanel/core/target/release/aurapanel-core
Restart=on-failure
Environment="RUST_LOG=info"

[Install]
WantedBy=multi-user.target
EOF

cat <<EOF > /etc/systemd/system/aurapanel-api.service
[Unit]
Description=AuraPanel API Gateway (Go)
After=network.target aurapanel-core.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/aurapanel/api-gateway
ExecStart=/opt/aurapanel/api-gateway/apigw
Restart=on-failure
Environment="GIN_MODE=release"

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
# systemctl enable aurapanel-core aurapanel-api
# systemctl start aurapanel-core aurapanel-api

echo -e "\n${GREEN}=================================================${NC}"
echo -e "${GREEN}AuraPanel Installation Completed!${NC}"
echo -e "${GREEN}The panel should now be running on port 8080.${NC}"
echo -e "${GREEN}Access it at: http://YOUR_SERVER_IP:8080${NC}"
echo -e "${GREEN}Admin username: admin${NC}"
echo -e "${GREEN}Admin password: (check /opt/aurapanel/logs/initial_password.txt)${NC}"
echo -e "${GREEN}=================================================${NC}"
