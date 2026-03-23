#!/bin/bash
set -e

# 用环境变量替换时间
echo "${CRON_SCHEDULE:-0 2 * * *} root /app/backup.sh >> /proc/1/fd/1 2>&1" > /etc/cron.d/backup
chmod 0644 /etc/cron.d/backup

exec cron -f