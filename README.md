# mytonstorage-backend

**[Русская версия](README.ru.md)**

Backend service for mytonstorage.org - a TON Storage web interface.

## Description

This backend service provides a complete API for managing file storage on TON Storage network:
- Handles file uploads and creates storage bags via TON Storage daemon
- Manages storage contracts lifecycle (initialization, top-up, withdrawal, provider updates)
- Provides TON Connect authentication for users
- Monitors storage contracts and notifies providers about new bags to download
- Exposes REST API endpoints for the frontend application
- Collects metrics via **Prometheus**

## Installation & Setup

To get started, you'll need a clean Debian 12 server with root user access.

1. **Download the server connection script**

Instead of password login, the security script requires using key-based authentication. This script should be run on your local machine, it doesn't require sudo, and will only forward keys for access.

```bash
wget https://raw.githubusercontent.com/dearjohndoe/mytonstorage-backend/refs/heads/master/scripts/init_server_connection.sh
```

2. **Forward keys and disable password access**

```bash
USERNAME=root PASSWORD=supersecretpassword HOST=123.45.67.89 bash init_server_connection.sh
```

In case of a man-in-the-middle error, you might need to remove known_hosts.

3. **Log into the remote machine and download the installation script**

```bash
ssh root@123.45.67.89 # If it asks for a password, the previous step failed.

wget https://raw.githubusercontent.com/dearjohndoe/mytonstorage-backend/refs/heads/master/scripts/setup_server.sh
```

4. **Run server setup and installation**

This will take a few minutes.

```bash
PG_USER=pguser PG_PASSWORD=pgpassword PG_DB=storagedb NEWFRONTENDUSER=janefrontside  NEWSUDOUSER=janedoe NEWUSER_PASSWORD=newpassword  INSTALL_SSL=false APP_USER=appuser API_PASSWORD=apipassword bash setup_server.sh
```

Upon completion, it will output useful information about server usage.

## Dev:
### VS Code Configuration
Create `.vscode/launch.json`:
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Package",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd",
            "buildFlags": "-tags=debug",    // to handle OPTIONS queries without nginx when dev
            "env": {...}
        }
    ]
}
```

## Project Structure

```
├── cmd/                   # Application entry point, configs, inits
├── pkg/                   # Application packages
│   ├── cache/             # Custom cache
│   ├── clients/           # TON blockchain and TON Storage clients
│   ├── httpServer/        # Fiber server handlers and routes
│   ├── models/            # DB and API data models
│   ├── repositories/      # Database layer (PostgreSQL)
│   ├── services/          # Business logic (auth, files, contracts, providers)
│   └── workers/           # Background workers
├── db/                    # Database schema
├── scripts/               # Setup and utility scripts
```

## API Endpoints

The server provides REST API endpoints for:
- User authentication via TON Connect
- File management (upload, delete, track unpaid bags, get minimal bags info)
- Storage contract operations (init, top-up, withdrawal, provider updates)
- Provider offers and rates

## Workers

The application runs several background workers:
- **Files Worker**: Removes unpaid and expired bags, triggers provider downloads, monitors download status
- **Cleaner Worker**: Maintains database hygiene and performs periodic cleanup tasks

## License

Apache-2.0



This project was created by order of a TON Foundation community member.
