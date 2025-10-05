#!/bin/bash

# Main server setup script that automates the entire server configuration process
# This script runs directly on the target server, downloads all necessary scripts
# from GitHub, installs PostgreSQL, configures Nginx, sets up log rotation,
# installs the backend application, secures the server, and initializes the database.
#
# Usage: Download and run with environment variables:
# wget https://raw.githubusercontent.com/dearjohndoe/mytonstorage-backend/master/scripts/setup_server.sh
# chmod +x setup_server.sh
# PG_USER=<pguser> PG_PASSWORD=<pgpassword> PG_DB=<database> \
# NEWFRONTENDUSER=<frontenduser> \
# NEWSUDOUSER=<newuser> NEWUSER_PASSWORD=<newpassword> \
# DOMAIN=<domain> INSTALL_SSL=<true|false> API_PASSWORD=<apipassword> \
# ./setup_server.sh

set -e

PG_VERSION="15"
GITHUB_REPO="dearjohndoe/mytonstorage-backend"
GITHUB_REPO_GATEWAY="dearjohndoe/mytonstorage-gateway"
GITHUB_BRANCH=${GITHUB_BRANCH:-master}
WORK_DIR=${WORK_DIR:-/tmp/storage}
API_USER=${API_USER:-uClient}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_required_vars() {
    local required_vars=(
        "PG_USER"
        "PG_PASSWORD"
        "PG_DB"
        "NEWSUDOUSER"
        "NEWFRONTENDUSER"
        "NEWUSER_PASSWORD"
        "API_PASSWORD"
    )
    
    local missing_vars=()
    
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var}" ]]; then
            missing_vars+=("$var")
        fi
    done
    
    if [[ ${#missing_vars[@]} -gt 0 ]]; then
        print_error "Missing required environment variables:"
        for var in "${missing_vars[@]}"; do
            echo "  - $var"
        done
        echo ""
        echo "Usage example:"
        echo "PG_USER=pguser PG_PASSWORD=secret PG_DB=storagedb \\"
        echo "NEWFRONTENDUSER=frontend \\"
        echo "NEWSUDOUSER=johndoe NEWUSER_PASSWORD=newsecurepassword \\"
        echo "INSTALL_SSL=false API_PASSWORD=apipassword \\"
        echo "./setup_server.sh"
        echo ""
        echo "Note: DOMAIN is optional. If not provided, will use server's hostname/IP."
        echo "      SSL certificates require a domain name."
        exit 1
    fi
}

setup_work_directory() {
    print_status "Setting up work directory..."

    if [ -d "mytonstorage-gateway" ]; then
        echo "Gateway repository exists, pulling latest changes..."
        cd mytonstorage-gateway || exit 1
        git pull
    else
        echo "Cloning repository..."
        git clone https://github.com/$GITHUB_REPO_GATEWAY.git
        cd mytonstorage-gateway || exit 1
    fi

    cd "$WORK_DIR"

    if [ -d "mytonstorage-backend" ]; then
        echo "Backend repository exists, pulling latest changes..."
        cd mytonstorage-backend || exit 1
        git pull
    else
        echo "Cloning repository..."
        git clone https://github.com/$GITHUB_REPO.git
        cd mytonstorage-backend || exit 1
        git checkout $GITHUB_BRANCH
    fi
    
    print_success "Work directory set up successfully."
}

execute_script() {
    local script_name=$1
    
    if [[ ! -f "$script_name" ]]; then
        print_error "Script not found: $script_name"
        exit 1
    fi
    
    local env_vars=""
    local vars_to_pass=(
        "PG_VERSION" "PG_USER" "PG_PASSWORD" "PG_DB"
        "NEWFRONTENDUSER" "WORK_DIR"
        "NEWSUDOUSER" "NEWUSER_PASSWORD" "DOMAIN" "INSTALL_SSL"
        "STORAGE_PATH" "API_PASSWORD" "API_USER"
    )
    
    for var in "${vars_to_pass[@]}"; do
        if [[ -n "${!var}" ]]; then
            export $var="${!var}"
        fi
    done

    if ! bash "$script_name"; then
        print_error "Script $script_name failed with exit code $?"
        exit 1
    fi
}

install_deps() {
    print_status "Installing required dependencies..."
    
    apt-get update
    apt-get upgrade -y
    apt-get install -y wget curl gnupg lsb-release git jq

    if ! command -v go &> /dev/null && [ ! -f /usr/local/go/bin/go ]; then
        print_status "Installing Go..."
        wget https://go.dev/dl/go1.25.1.linux-amd64.tar.gz
        tar -C /usr/local -xzf go1.25.1.linux-amd64.tar.gz
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
        rm go1.25.1.linux-amd64.tar.gz
    fi

    export PATH=$PATH:/usr/local/go/bin

    if ! command -v node &> /dev/null; then
        wget -qO- https://deb.nodesource.com/setup_20.x | bash -
        apt-get install -y nodejs
    fi
}

get_server_info() {
    HOST=$(hostname -I | awk '{print $1}')
    if [[ -z "$HOST" ]]; then
        HOST=$(hostname -f)
    fi
    
    print_status "Detected server information:"
    echo "Server IP/Hostname: $HOST"
}

main() {
    print_status "Starting server setup process..."
    
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root"
        echo "Please run: sudo $0"
        exit 1
    fi

    mkdir -p "$WORK_DIR"
    cd "$WORK_DIR" || exit 1

    check_required_vars

    install_deps
    
    get_server_info
    
    DOMAIN="${DOMAIN:-$HOST}"
    
    print_status "All required environment variables are set"
    echo "Server IP/Hostname: $HOST"
    echo "New sudo user: $NEWSUDOUSER"
    echo "New frontend user: $NEWFRONTENDUSER"
    echo "Storage path: $STORAGE_PATH"
    echo "PostgreSQL version: $PG_VERSION"
    echo "PostgreSQL database: $PG_DB"
    echo "Domain/IP: $DOMAIN"
    echo ""
    
    print_status "Step 1: Downloading scripts and configuration files..."
    setup_work_directory
    cd "$WORK_DIR/mytonstorage-backend/scripts" || exit 1
    
    print_status "Step 2: Setting up PostgreSQL..."
    execute_script "psql_setup.sh"
    
    print_status "Step 3: Disabling postgres user remote access..."
    execute_script "ib_disable_postgres_user.sh"
    
    print_status "Step 4: Initializing database..."
    execute_script "init_db.sh"
    
    print_status "Step 5: Setting up Nginx..."
    execute_script "setup_nginx.sh"
    
    print_status "Step 6: Installing ton storage..."
    execute_script "install_ton_storage.sh"
    
    print_status "Step 7: Securing the server..."
    export PASSWORD="$NEWUSER_PASSWORD"  # secure_server.sh expects PASSWORD env var
    execute_script "secure_server.sh"

    print_status "Step 8: Setting up log rotation..."
    execute_script "logs_rotation.sh"

    print_status "Step 9: Building backend application..."
    execute_script "build_backend.sh"

    print_status "Step 10: Building gateway application..."
    execute_script "build_gateway.sh"

    print_status "Step 11: Running the backend application..."
    su - "$NEWSUDOUSER" -c "cd $WORK_DIR/mytonstorage-backend/scripts && bash run.sh"

    print_status "Step 12: Running the gateway application..."
    su - "$NEWSUDOUSER" -c "cd $WORK_DIR/mytonstorage-gateway/scripts && bash run.sh"

    print_status "Step 13: Building and deploying frontend..."
    su - "$NEWFRONTENDUSER" -c "cd $WORK_DIR/mytonstorage-backend/scripts && HOST='$HOST' DOMAIN='$DOMAIN' INSTALL_SSL='$INSTALL_SSL' bash build_frontend.sh"

    print_success "Server setup completed successfully!"
    echo ""
    echo "Summary:"
    echo "✅ Dependencies installed (Go, Node.js)"
    echo "✅ Repository cloned from GitHub"
    echo "✅ PostgreSQL $PG_VERSION installed and configured"
    echo "✅ PostgreSQL user remote access disabled (postgres user)"
    echo "✅ Database '$PG_DB' initialized with schema"
    echo "✅ Nginx installed and configured"
    echo "✅ TON Storage daemon installed and running"
    echo "✅ Log rotation configured"
    echo "✅ Server secured (firewall, SSH hardening, user '$NEWSUDOUSER' created)"
    echo "✅ Backend application built and deployed"
    echo "✅ Backend service started"
    echo "✅ Frontend application built and deployed"
    echo "✅ Frontend user created: $NEWFRONTENDUSER"
    echo ""
    echo "You can now connect to your server using:"
    echo "ssh $NEWSUDOUSER@$HOST"
    echo ""
    echo "Switch to frontend user:"
    echo "sudo su $NEWFRONTENDUSER"
    echo ""
    echo "Web services:"
    if [[ "$INSTALL_SSL" == "true" ]]; then
        echo "Website: https://$DOMAIN"
        echo "API: https://$DOMAIN/api/"
        echo "Health check: https://$DOMAIN/health"
        echo "Metrics: https://$DOMAIN/metrics"
    else
        echo "Website: http://$DOMAIN"
        echo "API: http://$DOMAIN/api/"
        echo "Health check: http://$DOMAIN/health"
        echo "Metrics: http://$DOMAIN/metrics"
    fi
    echo ""
    echo "Backend application:"
    echo "Install directory: /opt/storage"
    echo "Configuration: /opt/storage/config.env"
    echo "Log file: /var/log/mytonstorage_backend.app/mytonstorage_backend.app.log"
    echo "Start service: cd /opt/storage && env \$(cat config.env | xargs) ./mtpo-backend >> /var/log/mytonstorage_backend.app/mytonstorage_backend.app.log 2>&1 &"
    echo "View logs: tail -f /var/log/mytonstorage_backend.app/mytonstorage_backend.app.log"
    echo ""
    echo "Gateway application:"
    echo "Install directory: /opt/storage"
    echo "Binary: /opt/storage/mtpo-gateway"
    echo "Log file: /var/log/mytonstorage_gateway.app/mytonstorage_gateway.app.log"
    echo "Start service: cd /opt/storage && ./mtpo-gateway >> /var/log/mytonstorage_gateway.app/mytonstorage_gateway.app.log 2>&1 &"
    echo "View logs: tail -f /var/log/mytonstorage_gateway.app/mytonstorage_gateway.app.log"
    echo ""
    echo "TON Storage daemon:"
    echo "Service name: ton-storage"
    echo "Storage path: ${STORAGE_PATH:-/opt/ton-storage}"
    echo "Database path: ${STORAGE_PATH:-/opt/ton-storage}/db"
    echo "Config file: ${STORAGE_PATH:-/opt/ton-storage}/db/config.json"
    echo "Binary location: /usr/local/bin/tonutils-storage"
    echo "UDP Port: 47431"
    echo "API URL: http://localhost:13474"
    echo "API Username: $NEWSUDOUSER"
    echo "API Password: $API_PASSWORD"
    echo "Service status: systemctl status ton-storage"
    echo "Service start: systemctl start ton-storage"
    echo "Service stop: systemctl stop ton-storage"
    echo "Service restart: systemctl restart ton-storage"
    echo "View logs: journalctl -u ton-storage -f"
    echo ""
    echo "Database connection details:"
    echo "Host: $HOST"
    echo "Port: 5432"
    echo "Database: $PG_DB"
    echo "User: $PG_USER"
    echo "Connect: psql -h $HOST -p 5432 -U $PG_USER -d $PG_DB"
    echo ""
    echo ""
    echo "SAVE THESE CREDENTIALS SECURELY!"
    echo ""
    echo ""
    echo "Cleanup: rm -rf $WORK_DIR"
}

main "$@"
