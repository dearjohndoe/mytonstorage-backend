#!/bin/bash

cd /opt/storage

env $(cat config.env | xargs) ./mtpo-backend >> /var/log/mytonstorage_backend.app/mytonstorage_backend.app.log 2>&1 &

sleep 5

if pgrep -f "./mtpo-backend" > /dev/null; then
    echo "✅ Backend application started successfully."
else
    echo "❌ Failed to start backend application."
    exit 1
fi
