# Mysql Backup

railway.com mysql database backup.

## manual

1. 在 Railway 项目里点 + New → GitHub Repo → 选 ppanel-backup
2. 在这个新服务的 Variables 面板填入以下环境变量：

```text
MYSQL_HOST        = ${{MySQL.MYSQLHOST}}
MYSQL_PORT        = ${{MySQL.MYSQLPORT}}
MYSQL_DATABASES    = ${{MySQL.MYSQLDATABASE}}
MYSQL_USER        = ${{MySQL.MYSQLUSER}}
MYSQL_PASSWORD    = ${{MySQL.MYSQLPASSWORD}}
TELEGRAM_BOT_TOKEN = 你的 bot token
TELEGRAM_CHAT_ID   = 你的 chat id
KEEP_LOCAL_BACKUPS  = 0  ← 是否保留本地备份文件，0 不保留，1 保留
PRE_BACKUP_SQL      =  # 备份前执行的 SQL 语句，按需调整，例如：
  # SET GLOBAL max_allowed_packet=1073741824;  # 设置 max_allowed_packet 为 1GB，避免大表备份失败
POST_BACKUP_SQL     = # 备份后执行的 SQL 语句，按需调整，例如：
  # OPTIMIZE TABLE your_table;  # 备份后优化表，释放空间


CRON_SCHEDULE      = 0 2 * * *   ← UTC 时间，对应北京时间早上10点，可按需调整
```

3. Deploy — 服务启动后会静默等待，到时间自动执行备份并发文件到你的 Telegram。

## troubleshooting

如果备份失败，首先检查日志，看看是哪个步骤出问题了。常见问题包括：

- 证书问题导致 curl 连接 Telegram API 失败
- 数据库连接问题导致 mysqldump 失败
- 文件权限问题导致备份文件无法创建或读取

针对证书问题，可以在容器里执行以下命令来验证：

```shell
# 确认证书文件存在且可读
ls -l /etc/ssl/certs/ca-certificates.crt
cat /etc/ssl/certs/ca-certificates.crt | head -n 20  # 应该看到 -----BEGIN CERTIFICATE-----

# 测试 curl
curl -v https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getMe

curl -v -w "\n%{http_code}" \
    -F "chat_id=${TELEGRAM_CHAT_ID}" \
    -F "document=@${filepath};filename=${db}_${ts}.sql.gz" \
    -F "caption=${caption}" \
    -F "parse_mode=HTML" \
    "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendDocument"
```

如果 mysqldump 失败，可以在容器里执行以下命令来验证：

```shell
# 测试数据库连接
mysql -h ${MYSQL_HOST} -P ${MYSQL_PORT} -u ${MYSQL_USER} -p${MYSQL_PASSWORD} -e "SHOW DATABASES;"
# 测试 mysqldump
mysqldump -h ${MYSQL_HOST} -P ${MYSQL_PORT} -u ${MYSQL_USER} -p${MYSQL_PASSWORD} --databases ${MYSQL_DATABASES} --single-transaction --quick --lock-tables=false > test_backup.sql
```

如果文件权限有问题，可以在容器里执行以下命令来验证：

```shell
# 测试当前目录权限
touch test_file.txt
ls -l test_file.txt
```

如果以上步骤都正常，但备份仍然失败，请检查以下几点：

- 确保环境变量正确设置，特别是数据库连接信息和 Telegram 相关信息
- 确保服务有足够的权限访问数据库和创建备份文件
- 确保服务能够访问 Telegram API，检查网络设置和防火墙规则

如果问题仍然无法解决，请将日志中的错误信息复制出来，搜索相关错误代码或消息，或者在社区论坛寻求帮助。

