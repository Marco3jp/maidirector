#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
QDRANT_COMPOSE_DIR="$(cd "$SCRIPT_DIR/../agent-extended-memory" && pwd)"

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
check_command "docker" "brew install --cask docker"
check_command "ollama" "brew install ollama"
check_command "gh"     "brew install gh  →  gh auth login"

if [ ${#MISSING_COMMANDS[@]} -gt 0 ]; then
  echo ""
  fail "必須コマンドが不足しています: ${MISSING_COMMANDS[*]}"
  echo -e "  上記をインストールしてから再実行してください。"
  exit 1
fi

header "2. Docker (Qdrant)"
if ! docker info &>/dev/null; then
  fail "Docker デーモンが起動していません"
  warn "Docker Desktop を起動してから再実行してください"
  exit 1
fi

QDRANT_RUNNING=$(docker ps --filter "ancestor=qdrant/qdrant" --filter "status=running" -q 2>/dev/null)
if [ -n "$QDRANT_RUNNING" ]; then
  skip "Qdrant はすでに起動中 (localhost:6333)"
else
  QDRANT_STOPPED=$(docker ps -a --filter "ancestor=qdrant/qdrant" --filter "status=exited" -q 2>/dev/null)
  if [ -n "$QDRANT_STOPPED" ]; then
    echo -e "  Qdrant コンテナを再起動中..."
    docker compose -f "$QDRANT_COMPOSE_DIR/docker-compose.yml" up -d qdrant
  else
    echo -e "  Qdrant コンテナを新規起動中..."
    docker compose -f "$QDRANT_COMPOSE_DIR/docker-compose.yml" up -d qdrant
  fi
  sleep 2
  if curl -sf http://localhost:6333/healthz &>/dev/null; then
    ok "Qdrant 起動完了 (localhost:6333)"
  else
    fail "Qdrant の起動に失敗しました"
    exit 1
  fi
fi

header "3. Ollama"
if curl -sf http://localhost:11434/api/tags &>/dev/null; then
  skip "Ollama はすでに起動中 (localhost:11434)"
else
  echo -e "  Ollama を起動中..."
  ollama serve &>/dev/null &
  OLLAMA_PID=$!
  for i in {1..10}; do
    if curl -sf http://localhost:11434/api/tags &>/dev/null; then
      break
    fi
    sleep 1
  done
  if curl -sf http://localhost:11434/api/tags &>/dev/null; then
    ok "Ollama 起動完了 (PID: $OLLAMA_PID)"
  else
    fail "Ollama の起動に失敗しました"
    warn "手動で ollama serve を実行してください"
    exit 1
  fi
fi

INSTALLED_MODELS=$(curl -sf http://localhost:11434/api/tags | python3 -c "import sys,json; [print(m['name'].split(':')[0]+':'+m['name'].split(':')[1] if ':' in m['name'] else m['name']) for m in json.load(sys.stdin).get('models',[])]" 2>/dev/null)

for model in "qwen2.5:7b" "nomic-embed-text"; do
  model_base="${model%%:*}"
  if echo "$INSTALLED_MODELS" | grep -q "$model_base"; then
    skip "モデル $model はインストール済み"
  else
    echo -e "  モデル $model をダウンロード中 (初回のみ、時間がかかります)..."
    ollama pull "$model"
    ok "モデル $model ダウンロード完了"
  fi
done

header "4. GitHub CLI"
if gh auth status &>/dev/null 2>&1; then
  GH_USER=$(gh api user --jq .login 2>/dev/null || echo "unknown")
  skip "gh 認証済み (user: $GH_USER)"
else
  warn "gh が未認証です。PR連携を使うには: gh auth login"
  warn "PR連携なしでも maidirector は動作します"
fi

header "5. npm 依存関係"
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

header "6. maidirector 起動"

BACKEND_PID=$(lsof -ti:8080 2>/dev/null || true)
if [ -n "$BACKEND_PID" ]; then
  skip "バックエンド はすでに起動中 (port 8080, PID: $BACKEND_PID)"
else
  echo -e "  バックエンド起動中..."
  cd "$SCRIPT_DIR" && go run ./cmd/maidirector serve &>"$SCRIPT_DIR/.dev-backend.log" &
  sleep 3
  BACKEND_PID=$(lsof -ti:8080 2>/dev/null || true)
  if curl -sf http://localhost:8080/api/stories &>/dev/null; then
    ok "バックエンド起動完了 (port 8080, PID: ${BACKEND_PID:-?})"
  else
    warn "バックエンドの起動を待機中..."
    sleep 3
    BACKEND_PID=$(lsof -ti:8080 2>/dev/null || true)
    if curl -sf http://localhost:8080/api/stories &>/dev/null; then
      ok "バックエンド起動完了 (port 8080, PID: ${BACKEND_PID:-?})"
    else
      fail "バックエンドの起動に失敗しました"
      warn "ログ: $SCRIPT_DIR/.dev-backend.log"
      warn "手動で make dev-backend を実行してください"
    fi
  fi
fi

FRONTEND_PID=$(lsof -ti:5173 2>/dev/null || true)
if [ -n "$FRONTEND_PID" ]; then
  skip "フロントエンド はすでに起動中 (port 5173, PID: $FRONTEND_PID)"
else
  echo -e "  フロントエンド dev server 起動中..."
  cd "$SCRIPT_DIR/web" && npx vite --host &>"$SCRIPT_DIR/.dev-frontend.log" &
  sleep 2
  FRONTEND_PID=$(lsof -ti:5173 2>/dev/null || true)
  ok "フロントエンド起動完了 (port 5173, PID: ${FRONTEND_PID:-?})"
fi

echo ""
echo -e "${BOLD}==============================${RESET}"
echo -e "${GREEN}${BOLD}起動完了!${RESET}"
echo ""
echo -e "  ダッシュボード:  ${CYAN}http://localhost:5173${RESET} (dev) / ${CYAN}http://localhost:8080${RESET} (embed)"
echo -e "  API:             ${CYAN}http://localhost:8080/api${RESET}"
echo -e "  Qdrant:          ${CYAN}http://localhost:6333/dashboard${RESET}"
echo -e "  Ollama:          ${CYAN}http://localhost:11434${RESET}"
echo ""
echo -e "  停止するには:"
echo -e "    ${YELLOW}lsof -ti:8080 | xargs kill   # バックエンド${RESET}"
echo -e "    ${YELLOW}lsof -ti:5173 | xargs kill   # フロントエンド${RESET}"
echo -e "    ${YELLOW}# Qdrant/Ollama はそのまま残して OK${RESET}"
