#!/bin/bash

REPO_URL="https://github.com/xssnick/tonutils-storage.git"
REPO_NAME="tonutils-storage"
BRANCH="master"
HOST="localhost"
SERVICE_NAME="ton-storage"

get_own_ip() {
    curl -s ifconfig.me || hostname -I | awk '{print $1}'
}

set -e
USER=$(whoami)
UDP_PORT=47431
API_PORT=13474

STORAGE_PATH=${STORAGE_PATH:-/opt/ton-storage}
SRC_DIR="/opt/ton"
BIN_DIR="/usr/local/bin"
DB_DIR="$STORAGE_PATH/db"
STORAGE_CONFIG_PATH="$DB_DIR/config.json"
REPO_PATH="$SRC_DIR/$REPO_NAME"

echo "Installing TON Storage..."
echo "Storage path: $STORAGE_PATH"
echo "User: $USER"
echo "UDP Port: $UDP_PORT"
echo "API Port: $API_PORT"

echo "Creating directories..."
mkdir -p "$SRC_DIR"
mkdir -p "$STORAGE_PATH"
mkdir -p "$DB_DIR"
chown -R "$USER:$USER" "$SRC_DIR" "$STORAGE_PATH" "$DB_DIR"

echo "Cloning repository..."
cd "$SRC_DIR"
if [ -d "$REPO_NAME" ]; then
    cd "$REPO_NAME"
    git pull
else
    git clone "$REPO_URL"
    cd "$REPO_NAME"
fi

mkdir -p ../bin

echo "Compiling..."
go build -o "../bin/$REPO_NAME" cli/main.go

cp "../bin/$REPO_NAME" "$BIN_DIR/"

SYSTEMD_PATH="/etc/systemd/system/$SERVICE_NAME.service"
bash -c "cat > \"$SYSTEMD_PATH\" << EOF
[Unit]
Description=My TON Storage
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$STORAGE_PATH
ExecStart=$BIN_DIR/$REPO_NAME --daemon --db $DB_DIR --api localhost:$API_PORT --api-login $API_USER --api-password $API_PASSWORD
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF"

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"

echo "First launch to create config..."
systemctl start "$SERVICE_NAME"
sleep 10
systemctl stop "$SERVICE_NAME"

EXTERNAL_IP=$(get_own_ip)
echo "External IP: $EXTERNAL_IP"

if [ -f "$STORAGE_CONFIG_PATH" ]; then
    echo "Configuring storage..."
    
    jq --arg listen_addr "0.0.0.0:$UDP_PORT" \
       --arg external_ip "$EXTERNAL_IP" \
       '.ListenAddr = $listen_addr | .ExternalIP = $external_ip' \
       "$STORAGE_CONFIG_PATH" > /tmp/storage_config.json && \
    mv /tmp/storage_config.json "$STORAGE_CONFIG_PATH"
else
    echo "Error: config file not found at $STORAGE_CONFIG_PATH"
    exit 1
fi

echo "Running..."
systemctl start "$SERVICE_NAME"

echo "Installation completed successfully!"
echo "TON Storage is running on UDP port: $UDP_PORT"
echo "API is available at: $HOST:$API_PORT"
echo "Configuration is saved at: $STORAGE_CONFIG_PATH"
echo "Service status: systemctl status $SERVICE_NAME"
