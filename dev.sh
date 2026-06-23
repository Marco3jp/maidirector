#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

BACKEND_PORT=20135
FRONTEND_PORT=5173

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

ok()   { echo -e "  ${GREEN}✓${RESET} $1"; }
skip() { echo -e "  ${CYAN}→${RESET} $1 ${YELLOW}(スキップ)${RESET}"; }
warn() { echo -e "  ${YELLOW}!${RESET} $1"; }
fail() { echo -e "  ${RED}✕${RESET} $1"; }
header() { echo -e "\n${BOLD}[$1]${RESET}"; }

MISSING_COMMANDS=()

check_command() {
  local cmd="$1"
  local install_hint="$2"
  if command -v "$cmd" &>/dev/null; then
    ok "$cmd $(command -v "$cmd")"
  else
    fail "$cmd が見つかりません"
    warn "  インストール: $install_hint"
    MISSING_COMMANDS+=("$cmd")
  fi
}

echo -e "${BOLD}maidirector dev environment${RESET}"
echo "=============================="

header "1. コマンド確認"
check_command "go"     "brew install go"
check_command "node"   "brew install node"
check_command "npm"    "brew install node"
check_command "gh"     "brew install gh  →  gh auth login"

if [ ${#MISSING_COMMANDS[@]} -gt 0 ]; then
  echo ""
  fail "必須コマンドが不足しています: ${MISSING_COMMANDS[*]}"
  echo -e "  上記をインストールしてから再実行してください。"
  exit 1
fi

header "2. GitHub CLI"
if gh auth status &>/dev/null 2>&1; then
  GH_USER=$(gh api user --jq .login 2>/dev/null || echo "unknown")
  skip "gh 認証済み (user: $GH_USER)"
else
  warn "gh が未認証です。PR連携を使うには: gh auth login"
  warn "PR連携なしでも maidirector は動作します"
fi

header "3. npm 依存関係"
LOCK_FILE="$SCRIPT_DIR/web/package-lock.json"
HASH_FILE="$SCRIPT_DIR/web/node_modules/.package-lock-hash"
CURRENT_HASH=$(shasum "$LOCK_FILE" | awk '{print $1}')

if [ -d "$SCRIPT_DIR/web/node_modules" ] && [ -f "$HASH_FILE" ] && [ "$(cat "$HASH_FILE")" = "$CURRENT_HASH" ]; then
  skip "node_modules は最新 (package-lock.json 変更なし)"
else
  echo -e "  npm ci 実行中..."
  (cd "$SCRIPT_DIR/web" && npm ci)
  echo "$CURRENT_HASH" > "$HASH_FILE"
  ok "npm ci 完了"
fi

header "4. maidirector 起動"

BACKEND_PID=$(lsof -ti:"$BACKEND_PORT" 2>/dev/null || true)
if [ -n "$BACKEND_PID" ]; then
  skip "バックエンド はすでに起動中 (port ${BACKEND_PORT}, PID: $BACKEND_PID)"
else
  echo -e "  バックエンド起動中..."
  cd "$SCRIPT_DIR" && go run ./cmd/maidirector serve &>"$SCRIPT_DIR/.dev-backend.log" &
  sleep 3
  BACKEND_PID=$(lsof -ti:"$BACKEND_PORT" 2>/dev/null || true)
  if curl -sf "http://localhost:${BACKEND_PORT}/api/stories" &>/dev/null; then
    ok "バックエンド起動完了 (port ${BACKEND_PORT}, PID: ${BACKEND_PID:-?})"
  else
    warn "バックエンドの起動を待機中..."
    sleep 3
    BACKEND_PID=$(lsof -ti:"$BACKEND_PORT" 2>/dev/null || true)
    if curl -sf "http://localhost:${BACKEND_PORT}/api/stories" &>/dev/null; then
      ok "バックエンド起動完了 (port ${BACKEND_PORT}, PID: ${BACKEND_PID:-?})"
    else
      fail "バックエンドの起動に失敗しました"
      warn "ログ: $SCRIPT_DIR/.dev-backend.log"
      warn "手動で make dev-backend を実行してください"
    fi
  fi
fi

FRONTEND_PID=$(lsof -ti:"$FRONTEND_PORT" 2>/dev/null || true)
if [ -n "$FRONTEND_PID" ]; then
  skip "フロントエンド はすでに起動中 (port ${FRONTEND_PORT}, PID: $FRONTEND_PID)"
else
  echo -e "  フロントエンド dev server 起動中..."
  cd "$SCRIPT_DIR/web" && npx vite --host --port "$FRONTEND_PORT" &>"$SCRIPT_DIR/.dev-frontend.log" &
  sleep 2
  FRONTEND_PID=$(lsof -ti:"$FRONTEND_PORT" 2>/dev/null || true)
  ok "フロントエンド起動完了 (port ${FRONTEND_PORT}, PID: ${FRONTEND_PID:-?})"
fi

echo ""
echo -e "${BOLD}==============================${RESET}"
echo -e "${GREEN}${BOLD}起動完了!${RESET}"
echo ""
echo -e "  ダッシュボード:  ${CYAN}http://localhost:${FRONTEND_PORT}${RESET} (dev) / ${CYAN}http://localhost:${BACKEND_PORT}${RESET} (embed)"
echo -e "  API:             ${CYAN}http://localhost:${BACKEND_PORT}/api${RESET}"
echo ""
echo -e "  停止するには:"
echo -e "    ${YELLOW}lsof -ti:${BACKEND_PORT} | xargs kill   # バックエンド${RESET}"
echo -e "    ${YELLOW}lsof -ti:${FRONTEND_PORT} | xargs kill   # フロントエンド${RESET}"
