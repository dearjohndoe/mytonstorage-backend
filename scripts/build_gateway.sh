#!/bin/bash

# This script builds the gateway application for the mytonstorage.
# Also generates the .env file with necessary configurations.

# Add Go to PATH
export PATH=$PATH:/usr/local/go/bin

cd "$WORK_DIR/mytonstorage-gateway/"

go build -buildvcs=false -o mtpo-gateway ./cmd || exit 1

SYSTEM_PRIVATE_KEY=$(openssl rand -hex 32)

cat <<EOL > config.env
SYSTEM_PORT=9093
TON_CONFIG_URL=https://ton-blockchain.github.io/global.config.json
SYSTEM_ACCESS_TOKENS=
SYSTEM_PRIVATE_KEY=${SYSTEM_PRIVATE_KEY}
DB_HOST=${HOST:-localhost}
DB_PORT=5432
DB_USER=${PG_USER}
DB_PASSWORD=${PG_PASSWORD}
DB_NAME=${PG_DB}
SYSTEM_LOG_LEVEL=0
SYSTEM_STORE_HISTORY_DAYS=90
TON_STORAGE_BASE_URL=http://127.0.0.1:13474
BAGS_DIR_FOR_STORAGE=/var/storage
TON_STORAGE_LOGIN=${API_USER}
TON_STORAGE_PASSWORD=${API_PASSWORD}
EOL

mkdir -p /opt/storage/gateway/
mv mtpo-gateway /opt/storage/gateway/
mv config.env /opt/storage/gateway/
cp -r ./templates /opt/storage/

echo "Gateway application built and configuration file created successfully."

