#!/bin/sh
set -e

# Railway dynamically assigns PORT; fall back to 8080 for local dev
APP_PORT="${PORT:-8080}"

# MySQL — Railway plugin injects MYSQL_URL or individual vars
MYSQL_ADDR="${MYSQL_HOST:-localhost}:${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-}"
MYSQL_DBNAME="${MYSQL_DATABASE:-ppanel}"
MYSQL_MAX_IDLE="${MYSQL_MAX_IDLE:-5}"
MYSQL_MAX_OPEN="${MYSQL_MAX_OPEN:-20}"

# Redis — Railway plugin injects REDIS_URL or individual vars
REDIS_ADDR="${REDIS_HOST:-localhost}:${REDIS_PORT:-6379}"
REDIS_PASS="${REDIS_PASSWORD:-}"
REDIS_DB="${REDIS_DB:-0}"

# JWT — generate a random secret if not provided
JWT_SECRET="${JWT_SECRET:-$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 32)}"
JWT_EXPIRE="${JWT_EXPIRE:-604800}"

# Admin account
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-adminPassword}"

# Write ppanel.yaml from environment variables
cat > /app/etc/ppanel.yaml <<EOF
Host: 0.0.0.0
Port: ${APP_PORT}
TLS:
    Enable: false
    CertFile: ""
    KeyFile: ""
Debug: false
JwtAuth:
    AccessSecret: ${JWT_SECRET}
    AccessExpire: ${JWT_EXPIRE}
Logger:
    ServiceName: PPanel
    Mode: console
    Encoding: json
    TimeFormat: "2006-01-02 15:04:05.000"
    Level: info
    MaxContentLength: 0
    Compress: false
    Stat: false
    KeepDays: 7
    StackCooldownMillis: 100
    MaxBackups: 0
    MaxSize: 0
    Rotation: daily
MySQL:
    Addr: ${MYSQL_ADDR}
    Username: ${MYSQL_USER}
    Password: ${MYSQL_PASSWORD}
    Dbname: ${MYSQL_DBNAME}
    Config: charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai
    MaxIdleConns: ${MYSQL_MAX_IDLE}
    MaxOpenConns: ${MYSQL_MAX_OPEN}
    SlowThreshold: 1000
Redis:
    Host: ${REDIS_ADDR}
    Pass: ${REDIS_PASS}
    DB: ${REDIS_DB}
Administrator:
    Email: '${ADMIN_EMAIL}'
    Password: '${ADMIN_PASSWORD}'
EOF

echo "==> ppanel.yaml generated"
echo "==> Starting ppanel on port ${APP_PORT}..."

exec /app/ppanel run --config etc/ppanel.yaml