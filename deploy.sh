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
sudo mv "$BUILD_DIR/letItGo-api" "$BIN_DIR/"
sudo mv "$BUILD_DIR/letItGo-producer" "$BIN_DIR/"
sudo mv "$BUILD_DIR/letItGo-consumer" "$BIN_DIR/"

# Create systemd service files
echo "Creating systemd service files..."
create_systemd_service "letItGo-api"
create_systemd_service "letItGo-producer"
create_systemd_service "letItGo-consumer"

# Reload systemd, enable and start services
echo "Reloading systemd and starting services..."
sudo systemctl daemon-reload
sudo systemctl enable letItGo-api.service letItGo-producer.service letItGo-consumer.service
sudo systemctl start letItGo-api.service letItGo-producer.service letItGo-consumer.service

# Check status
echo "Checking service statuses..."
sudo systemctl status letItGo-api.service
sudo systemctl status letItGo-producer.service
sudo systemctl status letItGo-consumer.service

echo "Deployment complete!"
