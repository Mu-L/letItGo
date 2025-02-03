#!/bin/bash

# Variables
APP_DIR=$(pwd)
BUILD_DIR="$APP_DIR/build"
BIN_DIR="/usr/local/bin/letItGo"
SERVICE_DIR="/etc/systemd/system"
USER="ubuntu"

# Function to create the systemd service
create_systemd_service() {
    local service_name=$1
    cat <<EOF | sudo tee "$SERVICE_DIR/$service_name.service"
[Unit]
Description=$service_name Service
After=network.target

[Service]
ExecStart=$BIN_DIR/$service_name
Restart=always
RestartSec=3
StartLimitInterval=500
StartLimitBurst=5
User=$USER
WorkingDirectory=$BIN_DIR
StandardOutput=syslog
StandardError=syslog

[Install]
WantedBy=multi-user.target
EOF
}

# Move binaries to /usr/local/bin
echo "Moving binaries to /usr/local/bin..."
sudo cp "$BUILD_DIR/api" "$BIN_DIR/"
sudo cp "$BUILD_DIR/producer" "$BIN_DIR/"
sudo cp "$BUILD_DIR/consumer" "$BIN_DIR/"

# Create systemd service files
echo "Creating systemd service files..."
create_systemd_service "api"
create_systemd_service "producer"
create_systemd_service "consumer"

# Reload systemd, enable and start services
echo "Reloading systemd and starting services..."
sudo systemctl daemon-reload
sudo systemctl enable api.service producer.service consumer.service
sudo systemctl start api.service producer.service consumer.service

# Check status
echo "Checking service statuses..."
sudo systemctl status api.service
sudo systemctl status producer.service
sudo systemctl status consumer.service

echo "Deployment complete!"
