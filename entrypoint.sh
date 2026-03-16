#!/bin/sh

# 备份执行时间，默认每天 02:00（UTC），可通过环境变量 CRON_SCHEDULE 覆盖
CRON_SCHEDULE="${CRON_SCHEDULE:-0 2 * * *}"

echo "[INFO] 备份服务启动，cron: ${CRON_SCHEDULE}"

# 写入 crontab
echo "${CRON_SCHEDULE} /app/backup.sh >> /proc/1/fd/1 2>&1" | crontab -

# 启动 crond（前台运行，保持容器存活）
crond -f -l 2