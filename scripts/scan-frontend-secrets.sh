#!/usr/bin/env bash
# 扫描前端构建产物中的疑似密钥（N-01/验收 5）：命中即非零退出。
# 用法：bash scripts/scan-frontend-secrets.sh [dist 目录]
set -uo pipefail

DIST="${1:-server/dist}"
if [ ! -d "$DIST" ]; then
  echo "目录不存在：$DIST" >&2
  exit 2
fi

# 常见密钥模式（宁可误报，不可漏报）
PATTERNS=(
  'sk-[A-Za-z0-9]{20,}'                 # OpenAI 风格
  'sk-ant-[A-Za-z0-9_-]{20,}'           # Anthropic
  'AIza[0-9A-Za-z_-]{30,}'              # Google API Key
  'AKIA[0-9A-Z]{16}'                    # AWS Access Key
  'ghp_[A-Za-z0-9]{30,}'                # GitHub PAT
  'xox[baprs]-[A-Za-z0-9-]{10,}'        # Slack
  '-----BEGIN (RSA |EC |)PRIVATE KEY-----'  # 私钥
  'mj-api-secret'                       # 明确的上游密钥头名
  'Bearer [A-Za-z0-9._-]{24,}'           # 硬编码 Bearer
)

HITS=0
for p in "${PATTERNS[@]}"; do
  # 只看文本类产物
  out=$(grep -rIlE --binary-files=without-match "$p" "$DIST" 2>/dev/null || true)
  if [ -n "$out" ]; then
    echo "⚠️  命中模式 $p ："
    echo "$out" | sed 's/^/    /'
    HITS=$((HITS+1))
  fi
done

if [ "$HITS" -gt 0 ]; then
  echo "❌ 共 $HITS 类疑似密钥命中，请检查构建产物（不要把上游密钥打进前端）" >&2
  exit 1
fi
echo "✅ 未发现疑似密钥（扫描 $DIST）"
