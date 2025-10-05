#!/bin/bash

# This script installs and configures Nginx for the mytonstorage application
# It sets up SSL certificates, configures the server, and enables the service
# Usage: DOMAIN=<domain> ./setup_nginx.sh

set -e

DOMAIN="${DOMAIN:-mytonstorage.org}"
INSTALL_SSL="${INSTALL_SSL:-true}"
# Use IP address if no domain is provided or if DOMAIN is an IP
if [[ "$DOMAIN" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    SITE_NAME="ip-${DOMAIN//./-}"
    SERVER_NAME="$DOMAIN"
else
    SITE_NAME="$DOMAIN"
    SERVER_NAME="$DOMAIN"
fi

NGINX_CONFIG="/etc/nginx/sites-available/$SITE_NAME"
NGINX_ENABLED="/etc/nginx/sites-enabled/$SITE_NAME"
WEB_ROOT="/var/www/$SITE_NAME"

echo "Installing Nginx..."
apt-get update
apt-get install -y nginx

echo "Creating Nginx configuration..."
mkdir -p "$WEB_ROOT"

cat > "$NGINX_CONFIG" << 'EOF'
server {
	server_name mytonstorage.org;
	root /var/www/mytonstorage.org;
	index index.html;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    client_max_body_size 100M;
    #proxy_max_temp_file_size 0;
    #proxy_buffering off;
    client_body_buffer_size 1M;

    # Gateway API proxy configuration
    location /api/v1/gateway/ {
        # Handle CORS preflight requests
        if ($request_method = OPTIONS) {
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
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
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

    # Gateway API proxy configuration
    location /api/v1/bans {
        # Handle CORS preflight requests
        if ($request_method = OPTIONS) {
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
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

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

    # Gateway API proxy configuration
    location /api/v1/reports {
        # Handle CORS preflight requests
        if ($request_method = OPTIONS) {
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
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

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
        if ($request_method = OPTIONS) {
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
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Extended timeouts for large file uploads (10 minutes)
        proxy_connect_timeout 600s;
        proxy_send_timeout 600s;
        proxy_read_timeout 600s;
        send_timeout 600s;
        
        # Large file upload settings
        client_max_body_size 4G;
        proxy_request_buffering off;
        proxy_buffering off;

        # CORS headers for actual requests
        add_header 'Access-Control-Allow-Origin' '*' always;
        add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, DELETE, OPTIONS' always;
        add_header 'Access-Control-Allow-Headers' 'Content-Type, Authorization, X-Requested-With' always;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://localhost:9092;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # No caching for health checks
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        add_header Pragma "no-cache";
        add_header Expires "0";
    }

    location /metrics {        
        proxy_pass http://localhost:9092;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /gateway/metrics {
        proxy_pass http://localhost:9093/api/v1/metrics;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Static files
    location / {
        try_files $uri $uri/ =404;
        
        # Cache static files
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }

        # Deny access to hidden files (only for static files)
        location ~ /\. {
            deny all;
        }
    }
}

EOF

rm -f /etc/nginx/sites-enabled/default

echo "Enabling the site..."
ln -sf "$NGINX_CONFIG" "$NGINX_ENABLED"

echo "Testing Nginx configuration..."
nginx -t

echo "Configuration test passed!"

chown -R www-data:www-data "$WEB_ROOT"
chmod -R 755 "$WEB_ROOT"

echo "Starting Nginx..."
systemctl enable nginx
systemctl restart nginx

install_ssl() {
    echo "Installing SSL certificate with Let's Encrypt..."
    apt-get install -y certbot python3-certbot-nginx
    
    # Generate SSL certificate
    certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos --email admin@"$DOMAIN" --redirect
    
    # Set up automatic renewal
    (crontab -l 2>/dev/null; echo "0 12 * * * /usr/bin/certbot renew --quiet") | crontab -
    
    systemctl restart nginx
}

# Check if SSL should be installed (only for domains, not IP addresses)
if [ "$INSTALL_SSL" = "true" ] && [[ ! "$DOMAIN" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    install_ssl
else
    if [[ "$DOMAIN" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "Skipping SSL installation for IP address. SSL certificates require a domain name."
    else
        echo "Skipping SSL installation. Set INSTALL_SSL=true to install SSL certificate."
    fi
fi

echo "✅ Nginx configuration completed successfully!"
echo "Site is available at: http://$DOMAIN"
echo "API endpoint: http://$DOMAIN/api/"
echo "Gateway API endpoint: http://$DOMAIN/api/v1/gateway/"
echo "Health check: http://$DOMAIN/health"
echo "Metrics: http://$DOMAIN/metrics"

if [ "$INSTALL_SSL" = "true" ] && [[ ! "$DOMAIN" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "SSL certificate installed. Site is also available at: https://$DOMAIN"
fi

echo "Done!"
