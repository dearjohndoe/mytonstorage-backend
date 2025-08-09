#!/bin/bash

# Fix SSL Configuration Script
# This script manually adds SSL configuration to Nginx when certbot fails to modify it

set -e

DOMAIN=${1:-mytonstorage.org}
NGINX_CONFIG="/etc/nginx/sites-available/$DOMAIN"
BACKUP_CONFIG="/etc/nginx/sites-available/$DOMAIN.backup.$(date +%Y%m%d_%H%M%S)"

echo "🔧 Fixing SSL configuration for domain: $DOMAIN"
echo "=================================================="

# Check if we're running as root
if [ "$EUID" -ne 0 ]; then
    echo "❌ This script must be run as root (use sudo)"
    exit 1
fi

# Check if certificate exists
CERT_PATH="/etc/letsencrypt/live/$DOMAIN"
if [ ! -d "$CERT_PATH" ]; then
    echo "❌ SSL certificate not found at $CERT_PATH"
    echo "Run certbot first: certbot --nginx -d $DOMAIN"
    exit 1
fi

# Check if Nginx config exists
if [ ! -f "$NGINX_CONFIG" ]; then
    echo "❌ Nginx configuration not found: $NGINX_CONFIG"
    exit 1
fi

# Check if SSL is already configured
if grep -q "listen 443 ssl" "$NGINX_CONFIG"; then
    echo "✅ SSL already configured in Nginx"
    exit 0
fi

echo "📋 Backing up current Nginx configuration..."
cp "$NGINX_CONFIG" "$BACKUP_CONFIG"
echo "   Backup created: $BACKUP_CONFIG"

echo "🔧 Adding SSL configuration to Nginx..."

# Create the SSL server block
cat > "/tmp/ssl_server_block.conf" << EOF
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name $DOMAIN;
    root /var/www/$DOMAIN;
    index index.html;

    # SSL Configuration
    ssl_certificate $CERT_PATH/fullchain.pem;
    ssl_certificate_key $CERT_PATH/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # Gateway API proxy configuration
    location /api/v1/gateway/ {
        # Handle CORS preflight requests
        if (\$request_method = OPTIONS) {
            add_header 'Access-Control-Allow-Origin' '*' always;
            add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, DELETE, OPTIONS' always;
            add_header 'Access-Control-Allow-Headers' 'Content-Type, Authorization, X-Requested-With' always;
            add_header 'Access-Control-Max-Age' 1728000;
            add_header 'Content-Length' 0;
            add_header 'Content-Type' 'text/plain; charset=UTF-8';
            return 204;
        }

        # Proxy to gateway service
        proxy_pass http://localhost:9093;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        
        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
        send_timeout 30s;

        # CORS headers for actual requests
        add_header 'Access-Control-Allow-Origin' '*' always;
        add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, DELETE, OPTIONS' always;
        add_header 'Access-Control-Allow-Headers' 'Content-Type, Authorization, X-Requested-With' always;
    }

    # API proxy configuration
    location /api/ {
        # Handle CORS preflight requests
        if (\$request_method = OPTIONS) {
            add_header 'Access-Control-Allow-Origin' '*' always;
            add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, DELETE, OPTIONS' always;
            add_header 'Access-Control-Allow-Headers' 'Content-Type, Authorization, X-Requested-With' always;
            add_header 'Access-Control-Max-Age' 1728000;
            add_header 'Content-Length' 0;
            add_header 'Content-Type' 'text/plain; charset=UTF-8';
            return 204;
        }

        # Proxy to backend application
        proxy_pass http://localhost:9092;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        
        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
        send_timeout 30s;

        # CORS headers for actual requests
        add_header 'Access-Control-Allow-Origin' '*' always;
        add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, DELETE, OPTIONS' always;
        add_header 'Access-Control-Allow-Headers' 'Content-Type, Authorization, X-Requested-With' always;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://localhost:9092;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        
        # No caching for health checks
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        add_header Pragma "no-cache";
        add_header Expires "0";
    }

    location /metrics {        
        proxy_pass http://localhost:9092;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # Static files
    location / {
        try_files \$uri \$uri/ =404;
        
        # Cache static files
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }
    }

    # Deny access to hidden files
    location ~ /\. {
        deny all;
    }
}

EOF

# Modify the existing HTTP server block to redirect to HTTPS
echo "🔧 Modifying HTTP server block for HTTPS redirect..."

# Create new config with both HTTP (redirect) and HTTPS blocks
cat > "$NGINX_CONFIG" << EOF
# HTTP server block - redirect all traffic to HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN;
    
    # Redirect all HTTP traffic to HTTPS
    return 301 https://\$server_name\$request_uri;
}

EOF

# Append the SSL server block
cat "/tmp/ssl_server_block.conf" >> "$NGINX_CONFIG"

# Clean up temporary file
rm "/tmp/ssl_server_block.conf"

echo "🧪 Testing Nginx configuration..."
if nginx -t; then
    echo "✅ Nginx configuration is valid"
    
    echo "🔄 Reloading Nginx..."
    systemctl reload nginx
    
    echo "✅ SSL configuration applied successfully!"
    echo ""
    echo "🌐 Your site should now be available at:"
    echo "   https://$DOMAIN"
    echo "   https://$DOMAIN/api/"
    echo "   https://$DOMAIN/api/v1/gateway/"
    echo ""
    echo "🔒 SSL Status:"
    echo "   Certificate: $CERT_PATH/fullchain.pem"
    echo "   Private Key: $CERT_PATH/privkey.pem"
    echo "   Expires: $(openssl x509 -enddate -noout -in "$CERT_PATH/fullchain.pem" | cut -d= -f2)"
    
else
    echo "❌ Nginx configuration test failed!"
    echo "🔄 Restoring backup configuration..."
    cp "$BACKUP_CONFIG" "$NGINX_CONFIG"
    systemctl reload nginx
    echo "   Configuration restored from backup"
    exit 1
fi

echo "✅ Done!"
