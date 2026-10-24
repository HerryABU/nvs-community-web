# 启用仓库内 Git hooks（Windows PowerShell 版，一次性执行）
# 用法：powershell -ExecutionPolicy Bypass -File .githooks\install.ps1
$ErrorActionPreference = "Stop"

$root = git rev-parse --show-toplevel
Set-Location $root

git config core.hooksPath .githooks
if ($LASTEXITCODE -ne 0) { throw "设置 core.hooksPath 失败" }

Write-Host "已启用 .githooks" -ForegroundColor Green
Write-Host "  core.hooksPath = $(git config --get core.hooksPath)"
Write-Host "  hooks: pre-commit / commit-msg / pre-push"
Write-Host ""
Write-Host "绕过：SKIP_HOOKS=1 git commit|push   只跳单测：SKIP_TESTS=1 git push"
Write-Host "关闭：git config --unset core.hooksPath"
