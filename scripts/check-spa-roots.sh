#!/usr/bin/env bash
# 校验：前端每条静态路由的首段，都必须在 server/main.go 的 knownSPARoots 白名单里。
#
# 为什么需要这个检查：
#   服务端用严格白名单做 SPA 回退（防代理前缀穿越）。新增页面若忘记登记，
#   症状是「点链接能进、直接访问或刷新却 404」——本地开发点进去看不出来，
#   只有用户刷新或分享链接时才暴露。本轮就是这样漏掉了 /membership、/collab、/live。
#
# 用法：bash scripts/check-spa-roots.sh   （CI 与 make check 都会跑）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ROUTER="$ROOT/web/src/router/index.ts"
MAIN="$ROOT/server/main.go"

for f in "$ROUTER" "$MAIN"; do
  [ -f "$f" ] || { echo "[spa-roots] 缺少文件：$f"; exit 2; }
done

front=$(grep -oE "path: '/[A-Za-z0-9_-]+" "$ROUTER" | sed "s|path: '/||" | sort -u)
allowed=$(sed -n '/var knownSPARoots = map\[string\]bool{/,/^}/p' "$MAIN" \
  | grep -oE '"[A-Za-z0-9_-]+":' | tr -d '":' | sort -u)

missing=$(comm -23 <(printf '%s\n' "$front") <(printf '%s\n' "$allowed") || true)

if [ -n "$missing" ]; then
  echo "[spa-roots] 以下前端路由首段未登记到 knownSPARoots，直接访问/刷新会 404："
  printf '  - /%s\n' $missing
  echo
  echo "修复：在 server/main.go 的 knownSPARoots 里补上对应键。"
  exit 1
fi

echo "[spa-roots] OK：前端 $(printf '%s\n' "$front" | wc -l) 个路由首段均已登记"
