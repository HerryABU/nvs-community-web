#!/usr/bin/env bash
# 启用仓库内 Git hooks（一次性执行）。
# 用法：bash .githooks/install.sh   或   make hooks
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"

git config core.hooksPath .githooks
chmod +x .githooks/pre-commit .githooks/commit-msg .githooks/pre-push 2>/dev/null || true

echo "已启用 .githooks"
echo "  core.hooksPath = $(git config --get core.hooksPath)"
echo "  hooks: pre-commit（暂存内容体检） / commit-msg（约定式提交） / pre-push（build+vet+test）"
echo
echo "绕过：SKIP_HOOKS=1 git commit|push   只跳单测：SKIP_TESTS=1 git push"
echo "关闭：git config --unset core.hooksPath"
