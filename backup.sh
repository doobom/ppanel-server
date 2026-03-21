#!/bin/sh
set -euo pipefail

# ── 环境变量 ────────────────────────────────────────────────────────────────
: "${MYSQL_HOST:=mysql}"               # Railway 服务名，通常是 mysql 或 db
: "${MYSQL_PORT:=3306}"
: "${MYSQL_USER:=root}"
: "${MYSQL_PASSWORD?}"                 # 必须
: "${MYSQL_DATABASES?}"                # 逗号分隔，如 db1,db2
: "${TELEGRAM_BOT_TOKEN?}"
: "${TELEGRAM_CHAT_ID?}"
: "${KEEP_LOCAL_BACKUPS:=3}"           # 容器内保留最近几份（0=不保留）
: "${PRE_BACKUP_SQL:=}"                # 可选：备份前执行的 SQL（如 "FLUSH LOGS;")
: "${POST_BACKUP_SQL:=}"               # 可选：备份后执行的 SQL

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_DIR="/backups"
LAST_STATUS="/app/last_backup_status"

# 确保备份目录存在
mkdir -p "$BACKUP_DIR"

# ── 函数：执行 SQL ────────────────────────────────────────────────────────
run_sql() {
  local sql="$1"
  if [ -z "$sql" ]; then return; fi
  echo "[INFO] 执行自定义 SQL: $sql"
  mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "$sql" || {
    echo "[WARN] 自定义 SQL 执行失败，但继续备份"
  }
}

# ── 备份前检查连接 ───────────────────────────────────────────────────────
echo "[INFO] 检查 MySQL 连接..."
mysqladmin --host="$MYSQL_HOST" --port="$MYSQL_PORT" --user="$MYSQL_USER" --password="$MYSQL_PASSWORD" ping || {
  send_telegram "❌ MySQL 连接失败，无法备份" "连接 ping 失败"
  echo "failed" > "$LAST_STATUS"
  exit 1
}

# ── 备份前自定义 SQL ─────────────────────────────────────────────────────
run_sql "$PRE_BACKUP_SQL"

# ── 备份每个数据库 ───────────────────────────────────────────────────────
SUCCESS=true
ERROR_MSG=""

for DB in $(echo "$MYSQL_DATABASES" | tr ',' ' '); do
  DB=$(echo "$DB" | xargs)  # 去空格
  [ -z "$DB" ] && continue

  FILENAME="${DB}_${TIMESTAMP}.sql.gz"
  FILEPATH=$(mktemp -p "$BACKUP_DIR" "${FILENAME}.XXXXXX") || {
    echo "[ERROR] 创建临时文件失败"
    SUCCESS=false
    ERROR_MSG="$ERROR_MSG\n临时文件创建失败"
    continue
  }

  trap 'rm -f "$FILEPATH"' EXIT

  echo "[INFO] 备份数据库 $DB → $FILENAME"

  mysqldump \
    -h "$MYSQL_HOST" \
    -P "$MYSQL_PORT" \
    -u "$MYSQL_USER" \
    -p"$MYSQL_PASSWORD" \
    --single-transaction \
    --routines \
    --triggers \
    --events \
    --max-allowed-packet=128M \
    "$DB" | gzip > "$FILEPATH" || {
      echo "[ERROR] $DB 备份失败"
      SUCCESS=false
      ERROR_MSG="$ERROR_MSG\n$db 备份失败"
      rm -f "$FILEPATH"
      continue
  }

  FILE_SIZE=$(du -sh "$FILEPATH" | cut -f1)
  echo "[INFO] $DB 备份完成，大小: $FILE_SIZE"

  # 发送 Telegram
  send_telegram_file "$FILEPATH" "$DB" "$TIMESTAMP" "$FILE_SIZE"

  # 保留策略：删除旧文件（只保留当前成功的 + 旧的 N-1 份）
  if [ "$KEEP_LOCAL_BACKUPS" -gt 0 ]; then
    find "$BACKUP_DIR" -name "*.sql.gz" -type f | sort -r | tail -n +$((KEEP_LOCAL_BACKUPS + 1)) | xargs -r rm -f
  else
    rm -f "$FILEPATH"
  fi

  trap - EXIT
done

# ── 备份后自定义 SQL ─────────────────────────────────────────────────────
run_sql "$POST_BACKUP_SQL"

# ── 更新状态文件给 HEALTHCHECK 用 ────────────────────────────────────────
if $SUCCESS; then
  echo "success ${TIMESTAMP}" > "$LAST_STATUS"
else
  echo "failed ${TIMESTAMP} ${ERROR_MSG}" > "$LAST_STATUS"
fi

send_telegram() {
  local text="$1"
  local extra="${2:-}"
  curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
    -d chat_id="${TELEGRAM_CHAT_ID}" \
    -d text="${text}${extra}" \
    -d parse_mode="HTML" > /dev/null || true
}

send_telegram_file() {
  local filepath="$1"
  local db="$2"
  local ts="$3"
  local size="$4"
  local status="✅ 成功"
  local caption

  if [ ! -f "$filepath" ]; then
    status="❌ 失败（文件丢失）"
  fi

  caption="$(printf '🗄 <b>MySQL 备份 (%s)</b>\n📦 数据库: <code>%s</code>\n📅 时间: <code>%s</code>\n💾 大小: <code>%s</code>' "$status" "$db" "$ts" "$size")"

  RESPONSE=$(curl -s -w "\n%{http_code}" \
    -F "chat_id=${TELEGRAM_CHAT_ID}" \
    -F "document=@${filepath};filename=${db}_${ts}.sql.gz" \
    -F "caption=${caption}" \
    -F "parse_mode=HTML" \
    "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendDocument")

  HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

  if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] Telegram 发送失败 HTTP $HTTP_CODE"
  fi
}