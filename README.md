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
