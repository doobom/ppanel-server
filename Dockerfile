FROM mysql:8.0-debian
 
RUN apt-get update && apt-get install -y curl gzip cron && rm -rf /var/lib/apt/lists/*
 
WORKDIR /app
 
COPY backup.sh /app/backup.sh
COPY entrypoint.sh /app/entrypoint.sh
 
RUN chmod +x /app/backup.sh /app/entrypoint.sh
 
ENTRYPOINT ["/app/entrypoint.sh"]