FROM alpine:3.19
 
# 使用 MySQL 8.0 原生客户端，兼容 Railway MySQL 8.0 的 caching_sha2_password 认证
RUN apk add --no-cache curl gzip \
    && apk add --no-cache --repository=https://dl-cdn.alpinelinux.org/alpine/edge/community mysql80-client
 
WORKDIR /app
 
COPY backup.sh /app/backup.sh
COPY entrypoint.sh /app/entrypoint.sh
 
RUN chmod +x /app/backup.sh /app/entrypoint.sh
 
ENTRYPOINT ["/app/entrypoint.sh"]