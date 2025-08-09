#!/bin/bash

# SSL Debug Script
# This script helps diagnose SSL certificate issues

DOMAIN=${1:-mytonstorage.org}

echo "🔍 Debugging SSL setup for domain: $DOMAIN"
echo "=================================================="

# Check if domain resolves
echo "1. DNS Resolution:"
if dig +short "$DOMAIN" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' > /dev/null; then
    echo "   ✅ Domain resolves to: $(dig +short "$DOMAIN")"
else
    echo "   ❌ Domain does not resolve or no A record found"
fi

# Check Nginx status
echo -e "\n2. Nginx Status:"
if systemctl is-active nginx > /dev/null 2>&1; then
    echo "   ✅ Nginx is running"
else
    echo "   ❌ Nginx is not running"
fi

# Check Nginx configuration
echo -e "\n3. Nginx Configuration Test:"
if nginx -t > /dev/null 2>&1; then
    echo "   ✅ Nginx configuration is valid"
else
    echo "   ❌ Nginx configuration has errors:"
    nginx -t 2>&1 | sed 's/^/      /'
fi

# Check certificate files
echo -e "\n4. Certificate Files:"
CERT_PATH="/etc/letsencrypt/live/$DOMAIN"
if [ -d "$CERT_PATH" ]; then
    echo "   ✅ Certificate directory exists: $CERT_PATH"
    if [ -f "$CERT_PATH/fullchain.pem" ]; then
        echo "   ✅ fullchain.pem exists"
        echo "      Certificate expires: $(openssl x509 -enddate -noout -in "$CERT_PATH/fullchain.pem" | cut -d= -f2)"
    else
        echo "   ❌ fullchain.pem missing"
    fi
    if [ -f "$CERT_PATH/privkey.pem" ]; then
        echo "   ✅ privkey.pem exists"
    else
        echo "   ❌ privkey.pem missing"
    fi
else
    echo "   ❌ Certificate directory does not exist: $CERT_PATH"
fi

# Check Nginx sites
echo -e "\n5. Nginx Sites:"
SITE_CONFIG="/etc/nginx/sites-available/$DOMAIN"
SITE_ENABLED="/etc/nginx/sites-enabled/$DOMAIN"

if [ -f "$SITE_CONFIG" ]; then
    echo "   ✅ Site configuration exists: $SITE_CONFIG"
else
    echo "   ❌ Site configuration missing: $SITE_CONFIG"
fi

if [ -L "$SITE_ENABLED" ]; then
    echo "   ✅ Site is enabled: $SITE_ENABLED"
else
    echo "   ❌ Site is not enabled: $SITE_ENABLED"
fi

# Check if SSL is configured in Nginx
echo -e "\n6. SSL Configuration in Nginx:"
if [ -f "$SITE_CONFIG" ]; then
    if grep -q "listen 443 ssl" "$SITE_CONFIG"; then
        echo "   ✅ SSL listener (443) found in configuration"
    else
        echo "   ❌ SSL listener (443) NOT found in configuration"
        echo "   ℹ️  This suggests certbot didn't modify the Nginx config"
    fi
    
    if grep -q "ssl_certificate" "$SITE_CONFIG"; then
        echo "   ✅ SSL certificate directives found"
    else
        echo "   ❌ SSL certificate directives NOT found"
    fi
fi

# Check ports
echo -e "\n7. Port Status:"
if netstat -tlnp | grep :80 > /dev/null; then
    echo "   ✅ Port 80 is listening"
else
    echo "   ❌ Port 80 is not listening"
fi

if netstat -tlnp | grep :443 > /dev/null; then
    echo "   ✅ Port 443 is listening"
else
    echo "   ❌ Port 443 is not listening"
fi

# Test HTTP connection
echo -e "\n8. Connection Tests:"
if curl -s -o /dev/null -w "%{http_code}" "http://$DOMAIN" | grep -q "200\|301\|302"; then
    echo "   ✅ HTTP connection works"
else
    echo "   ❌ HTTP connection failed"
fi

# Test HTTPS connection
if curl -s -k -o /dev/null -w "%{http_code}" "https://$DOMAIN" | grep -q "200\|301\|302"; then
    echo "   ✅ HTTPS connection works (ignoring certificate validation)"
else
    echo "   ❌ HTTPS connection failed"
fi

# Check recent logs
echo -e "\n9. Recent Nginx Error Logs:"
if [ -f /var/log/nginx/error.log ]; then
    echo "   Last 5 errors from Nginx:"
    tail -5 /var/log/nginx/error.log | sed 's/^/      /' || echo "      No recent errors"
else
    echo "   ❌ Nginx error log not found"
fi

echo -e "\n10. Certbot Certificates:"
certbot certificates 2>/dev/null | grep -A 5 "$DOMAIN" || echo "   ❌ No certificates found for $DOMAIN"

echo -e "\n🔧 Suggested Actions:"
if ! netstat -tlnp | grep :443 > /dev/null; then
    echo "   • Port 443 not listening - SSL might not be configured"
    echo "   • Try running: certbot --nginx -d $DOMAIN"
fi

if [ -f "$SITE_CONFIG" ] && ! grep -q "listen 443 ssl" "$SITE_CONFIG"; then
    echo "   • Nginx SSL configuration missing"
    echo "   • Certbot may have failed to modify Nginx config"
    echo "   • Check: journalctl -u certbot"
fi

echo -e "\n📝 Manual SSL Check Commands:"
echo "   curl -I https://$DOMAIN"
echo "   openssl s_client -connect $DOMAIN:443 -servername $DOMAIN"
echo "   journalctl -u nginx -f"
echo "   journalctl -u certbot"

echo -e "\n✅ Debug complete!"
