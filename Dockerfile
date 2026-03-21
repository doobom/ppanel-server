FROM debian:bookworm-slim

# 安装必要工具（mariadb-client 提供 mysqldump / mysql / mysqladmin）
RUN apt-get update && apt-get install -y --no-install-recommends \
    mariadb-client \
    curl \
    gzip \
    cron \
    tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app /backups

# 设置时区（可选，根据你位置调整；Railway 默认 UTC，可不设）
ENV TZ=Etc/UTC
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

WORKDIR /app

# 复制脚本
COPY backup.sh /app/backup.sh
RUN chmod +x /app/backup.sh

# 使用 crontab 文件（更规范）
COPY crontab /etc/cron.d/backup
RUN chmod 0644 /etc/cron.d/backup

# 状态文件用于 HEALTHCHECK
RUN touch /app/last_backup_status && chmod 666 /app/last_backup_status

# 前台运行 cron，日志输出到 stdout（Railway 会收集）
CMD ["cron", "-f", "-L", "15"]

HEALTHCHECK --interval=5m --timeout=10s --start-period=1m --retries=3 \
  CMD sh -c '\
    pidof cron > /dev/null || exit 1; \
    grep -q "^success" /app/last_backup_status || { \
      LAST=$(cat /app/last_backup_status 2>/dev/null || echo "never"); \
      echo "Last backup: $LAST"; \
      [ "$LAST" = "never" ] && exit 0; exit 1; \
    }; \
    exit 0'