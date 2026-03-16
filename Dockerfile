FROM alpine:3.19

# 安装 mysqldump、curl、gzip、crond
RUN apk add --no-cache mysql-client curl gzip

WORKDIR /app

COPY backup.sh /app/backup.sh
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/backup.sh /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]