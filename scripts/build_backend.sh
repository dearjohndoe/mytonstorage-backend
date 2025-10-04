#!/bin/bash

# This script builds the backend application for the mytonstorage.
# Also generates the .env file with necessary configurations.

# Add Go to PATH
export PATH=$PATH:/usr/local/go/bin

cd "$WORK_DIR/mytonstorage-backend/"

go build -buildvcs=false -o mtpo-backend ./cmd || exit 1

SYSTEM_PRIVATE_KEY=$(openssl rand -hex 32)

cat <<EOL > config.env
SYSTEM_PORT=9092
TON_CONFIG_URL=https://ton-blockchain.github.io/global.config.json
SYSTEM_ACCESS_TOKENS=
SYSTEM_ADMIN_AUTH_TOKENS=
SYSTEM_PRIVATE_KEY=${SYSTEM_PRIVATE_KEY}
BATCH_SIZE=100
DB_HOST=${HOST:-localhost}
DB_PORT=5432
DB_USER=${PG_USER}
DB_PASSWORD=${PG_PASSWORD}
DB_NAME=${PG_DB}
SYSTEM_LOG_LEVEL=0
TON_STORAGE_BASE_URL=http://127.0.0.1:13474
BAGS_DIR_FOR_STORAGE=/var/storage
TON_STORAGE_LOGIN=${API_USER}
TON_STORAGE_PASSWORD=${API_PASSWORD}
EOL

mv mtpo-backend /opt/storage/
mv config.env /opt/storage/

echo "Backend application built and configuration file created successfully."

