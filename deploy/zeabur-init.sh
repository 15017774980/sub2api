#!/usr/bin/env bash
# 一键生成 Zeabur 部署所需的 .env 文件（含强随机密钥）
# 用法： bash deploy/zeabur-init.sh
# 可选： ADMIN_EMAIL=you@example.com bash deploy/zeabur-init.sh

set -euo pipefail

OUT="$(cd "$(dirname "$0")" && pwd)/zeabur.env.local"

# ---- 颜色 ----
if [[ -t 1 ]]; then
  RED=$'\033[31m'; YEL=$'\033[33m'; GRN=$'\033[32m'; BLD=$'\033[1m'; RST=$'\033[0m'
else
  RED=""; YEL=""; GRN=""; BLD=""; RST=""
fi

# ---- 依赖检查 ----
if ! command -v openssl >/dev/null 2>&1; then
  echo "${RED}错误：未找到 openssl，请先安装。${RST}" >&2
  echo "  macOS:   brew install openssl" >&2
  echo "  Ubuntu:  sudo apt-get install openssl" >&2
  echo "  Windows: 推荐使用 Git Bash（自带 openssl）" >&2
  exit 1
fi

# ---- 已存在文件确认 ----
if [[ -f "$OUT" ]]; then
  echo "${YEL}已存在 $OUT${RST}"
  echo "${YEL}覆盖将作废原有密钥（旧 JWT 立刻失效，旧 TOTP 解不开历史 2FA）${RST}"
  read -rp "确认覆盖？输入 yes 继续：" ans
  if [[ "$ans" != "yes" ]]; then
    echo "已取消。"
    exit 0
  fi
fi

# ---- 询问管理员邮箱 ----
ADMIN_EMAIL="${ADMIN_EMAIL:-}"
if [[ -z "$ADMIN_EMAIL" ]]; then
  read -rp "管理员邮箱（回车使用默认 admin@sub2api.local）: " ADMIN_EMAIL
  ADMIN_EMAIL="${ADMIN_EMAIL:-admin@sub2api.local}"
fi

# ---- 生成密钥（OS 级 CSPRNG）----
TOTP_KEY="$(openssl rand -hex 32)"
JWT_KEY="$(openssl rand -hex 32)"
ADMIN_PWD="$(openssl rand -base64 24)"

# ---- 写入文件（仅本人可读写）----
umask 077
cat > "$OUT" <<EOF
# Sub2API · Zeabur 真实环境变量（本机生成，已被 .gitignore 排除）
# 直接整段粘贴到 Zeabur 控制台 → Sub2API 服务 → Variables → Raw Editor

SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_MODE=production

DATABASE_HOST=\${POSTGRES_HOST}
DATABASE_PORT=\${POSTGRES_PORT}
DATABASE_USER=\${POSTGRES_USERNAME}
DATABASE_PASSWORD=\${POSTGRES_PASSWORD}
DATABASE_DBNAME=\${POSTGRES_DATABASE}
DATABASE_SSLMODE=disable

REDIS_HOST=\${REDIS_HOST}
REDIS_PORT=\${REDIS_PORT}
REDIS_PASSWORD=\${REDIS_PASSWORD}
REDIS_DB=0

JWT_SECRET=${JWT_KEY}
TOTP_ENCRYPTION_KEY=${TOTP_KEY}

ADMIN_EMAIL=${ADMIN_EMAIL}
ADMIN_PASSWORD=${ADMIN_PWD}

AUTO_SETUP=true
RUN_MODE=standard
TZ=Asia/Shanghai
EOF
chmod 600 "$OUT" 2>/dev/null || true

# ---- 输出 ----
echo
echo "${GRN}已生成：$OUT${RST}"
echo
cat "$OUT"
echo
echo "${BLD}================ 下一步 ================${RST}"
echo "1. Zeabur 控制台 → Sub2API 服务 → Variables → Raw Editor"
echo "   把上面文件全文粘贴进去保存"
echo
echo "2. ${RED}${BLD}立即把以下两项备份到密码管理器（1Password / Bitwarden 等）${RST}"
echo "   - JWT_SECRET"
echo "   - TOTP_ENCRYPTION_KEY  ${RED}（丢失=所有用户 2FA 永久失效，不可恢复）${RST}"
echo
echo "3. 首次登录后在 Profile 页给管理员开启 2FA，并改一次密码"
echo "${BLD}=========================================${RST}"
