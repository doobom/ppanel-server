#!/bin/sh
set -e

# ── 必要环境变量检查 ──────────────────────────────────────────
for var in MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE TELEGRAM_BOT_TOKEN TELEGRAM_CHAT_ID; do
  eval val=\$$var
  if [ -z "$val" ]; then
    echo "[ERROR] 环境变量 $var 未设置"
    exit 1
  fi
done

# ── 文件名 ────────────────────────────────────────────────────
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
FILENAME="${MYSQL_DATABASE}_${TIMESTAMP}.sql.gz"
FILEPATH="/tmp/${FILENAME}"

echo "[INFO] 开始备份数据库: ${MYSQL_DATABASE} -> ${FILENAME}"

# ── 执行备份 ──────────────────────────────────────────────────
mysqldump \
  -h "${MYSQL_HOST}" \
  -P "${MYSQL_PORT}" \
  -u "${MYSQL_USER}" \
  -p"${MYSQL_PASSWORD}" \
  --single-transaction \
  --routines \
  --triggers \
  "${MYSQL_DATABASE}" | gzip > "${FILEPATH}"

FILE_SIZE=$(du -sh "${FILEPATH}" | cut -f1)
echo "[INFO] 备份完成，文件大小: ${FILE_SIZE}"

# ── 发送到 Telegram ───────────────────────────────────────────
echo "[INFO] 正在发送到 Telegram..."

CAPTION="$(printf '🗄 <b>MySQL 备份</b>\n📦 数据库: <code>%s</code>\n📅 时间: <code>%s</code>\n💾 大小: <code>%s</code>' "${MYSQL_DATABASE}" "${TIMESTAMP}" "${FILE_SIZE}")"

RESPONSE=$(curl -s -w "\n%{http_code}" \
  -F "chat_id=${TELEGRAM_CHAT_ID}" \
  -F "document=@${FILEPATH};filename=${FILENAME}" \
  -F "caption=${CAPTION}" \
  -F "parse_mode=HTML" \
  "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendDocument")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "200" ]; then
  echo "[INFO] 发送成功 ✅"
else
  echo "[ERROR] 发送失败，HTTP ${HTTP_CODE}"
  echo "[ERROR] 响应: ${BODY}"
  rm -f "${FILEPATH}"
  exit 1
fi

# ── 清理临时文件 ──────────────────────────────────────────────
rm -f "${FILEPATH}"
echo "[INFO] 临时文件已清理，备份任务完成 🎉"