#!/usr/bin/env bash
# 版本一致性校验 / 一键同步 —— 不依赖 make（这台机器没有 make，`make version` 跑不起来）
#
# 四处必须一致：
#   1. server/version/version.go   var AppVersion = "x.y.z"
#   2. web/package.json            顶层 "version"
#   3. web/package-lock.json       root 与 packages[""] 两个 version（⚠️ 依赖自己的 version 绝不能动）
#   4. README.md                   顶部 version 徽章
#
# 用法：
#   bash scripts/check-version.sh                  # 校验（默认，CI/本地通用）
#   bash scripts/check-version.sh --tag v4.2.0     # 额外校验 tag 与代码版本一致（发布流水线用）
#   bash scripts/check-version.sh --set 4.2.1      # 一次性把四处同步成 4.2.1，然后校验
#   bash scripts/check-version.sh --help
#
# 退出码：0 一致 / 1 不一致或同步失败 / 2 用法错误
#
# ⚠️ 为什么要有这个脚本：package-lock.json 里还有 6 处 "version" 是**依赖自己的版本号**，
#    用 `s.replace(/"version": "4.1.0"/g)` 这类全文替换会把依赖树一起改坏。
#    这里只按结构改「前两处」（root 与 packages[""]），并在改写前断言头部结构与预期一致。

set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT" || { echo "✗ 进不去项目根目录：$ROOT" >&2; exit 2; }

SET_VER=""
TAG_REF=""

usage() {
  cat <<'USAGE'
用法：bash scripts/check-version.sh [选项]

  （无参数）            校验四处版本号是否一致
  --tag <ref>          额外校验 tag 与代码版本一致（v4.2.0 与 4.2.0 都认）
  --set <x.y.z>        把四处版本号一次性同步成 x.y.z，然后校验
  -h, --help           显示本帮助

四处位置：server/version/version.go / web/package.json /
         web/package-lock.json（root + packages[""]）/ README.md 徽章
USAGE
}

die() { echo "✗ $*" >&2; exit "${2:-1}"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --set)      [ $# -ge 2 ] || die "--set 需要一个版本号" 2; SET_VER="$2"; shift 2 ;;
    --set=*)    SET_VER="${1#*=}"; shift ;;
    --tag)      [ $# -ge 2 ] || die "--tag 需要一个 tag 名" 2; TAG_REF="$2"; shift 2 ;;
    --tag=*)    TAG_REF="${1#*=}"; shift ;;
    -h|--help)  usage; exit 0 ;;
    *)          echo "未知参数：$1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ -n "$SET_VER" ] && ! printf '%s' "$SET_VER" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  die "--set 的版本号必须形如 4.2.1（收到：$SET_VER）" 2
fi

command -v node >/dev/null 2>&1 || die "需要 node（web/package.json 与 lock 的版本要用它读写）"

# ── 读取 ────────────────────────────────────────────────────────────────
read_code_version() {
  grep -E '^var AppVersion' server/version/version.go 2>/dev/null \
    | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || true
}
read_pkg_version() {
  node -p 'require("./web/package.json").version' 2>/dev/null || true
}
# 一次输出两个值：root 与 packages[""] 的 version
read_lock_versions() {
  node -e 'const l=require("./web/package-lock.json");const r=l.packages&&l.packages[""];console.log((l.version||"-")+" "+(r&&r.version||"-"))' 2>/dev/null || true
}
read_badge_version() {
  grep -oE 'badge/version-[0-9]+\.[0-9]+\.[0-9]+' README.md 2>/dev/null \
    | head -1 | sed 's|badge/version-||' || true
}

# ── 同步改写（--set） ────────────────────────────────────────────────────
if [ -n "$SET_VER" ]; then
  node - "$SET_VER" <<'NODE' || exit 1
const fs = require('fs');
const ver = process.argv[2];
const bad = (m) => { console.error('✗ ' + m); process.exit(1); };

// 1) version.go：只认行首的 `var AppVersion`（注释里的版本号不能碰）
{
  const p = 'server/version/version.go';
  const s = fs.readFileSync(p, 'utf8');
  const re = /^var AppVersion = "\d+\.\d+\.\d+"/m;
  if (!re.test(s)) bad('version.go 里找不到 `var AppVersion = "x.y.z"`');
  fs.writeFileSync(p, s.replace(re, 'var AppVersion = "' + ver + '"'));
}

// 2) package.json：只改顶层 version
{
  const p = 'web/package.json';
  const s = fs.readFileSync(p, 'utf8');
  const re = /("version"\s*:\s*")\d+\.\d+\.\d+(")/;
  if (!re.test(s)) bad('package.json 里找不到顶层 version');
  fs.writeFileSync(p, s.replace(re, '$1' + ver + '$2'));
}

// 3) package-lock.json：只改前两处（root 与 packages[""]）。
//    改写前先断言头部确实只有这两个 version，结构变了就拒绝，免得改坏依赖树。
{
  const p = 'web/package-lock.json';
  const s = fs.readFileSync(p, 'utf8');
  const head = s.slice(0, 800);
  const headHits = (head.match(/"version"\s*:\s*"\d+\.\d+\.\d+"/g) || []).length;
  if (headHits !== 2) {
    bad('package-lock.json 头部结构与预期不符（期望恰好 2 处 version，实际 ' + headHits + ' 处），已拒绝改写');
  }
  let seen = 0;
  const out = s.replace(/("version"\s*:\s*")\d+\.\d+\.\d+(")/g,
    (m, a, b) => (seen++ < 2 ? a + ver + b : m));
  fs.writeFileSync(p, out);
}

// 4) README.md：只改顶部徽章；「版本更新记录」里的历史标题（### v4.2.0 …）不动
{
  const p = 'README.md';
  const s = fs.readFileSync(p, 'utf8');
  const re = /(badge\/version-)\d+\.\d+\.\d+(-)/;
  if (!re.test(s)) bad('README.md 里找不到 version 徽章（badge/version-x.y.z-）');
  fs.writeFileSync(p, s.replace(re, '$1' + ver + '$2'));
}

console.log('已同步四处版本号 → ' + ver);
NODE
fi

# ── 校验 ────────────────────────────────────────────────────────────────
CODE_VER=$(read_code_version)
[ -n "$CODE_VER" ] || die "读不到 server/version/version.go 里的 AppVersion"

PKG_VER=$(read_pkg_version)
read -r LOCK_ROOT LOCK_PKG <<<"$(read_lock_versions)"
BADGE_VER=$(read_badge_version)

[ -n "$PKG_VER" ]  || die "读不到 web/package.json 的 version"
[ -n "$LOCK_ROOT" ] || die "读不到 web/package-lock.json 的 version"
[ -n "$BADGE_VER" ] || die "读不到 README.md 的 version 徽章"

FAIL=0
row() { # row <标签> <实际值>
  if [ "$2" = "$CODE_VER" ]; then
    printf '  ✓ %-34s %s\n' "$1" "$2"
  else
    printf '  ✗ %-34s %s  （应为 %s）\n' "$1" "${2:-<空>}" "$CODE_VER"
    FAIL=1
  fi
}

echo "版本基准（server/version/version.go）：$CODE_VER"
echo
row "server/version/version.go"     "$CODE_VER"
row "web/package.json"              "$PKG_VER"
row "web/package-lock.json root"    "$LOCK_ROOT"
row "web/package-lock.json pkg[\"\"]" "$LOCK_PKG"
row "README.md 徽章"                "$BADGE_VER"

if [ -n "$TAG_REF" ]; then
  TAG_VER="${TAG_REF#v}"   # v4.2.0 与 4.2.0 都归一成 4.2.0
  if [ "$TAG_VER" = "$CODE_VER" ]; then
    printf '  ✓ %-34s %s\n' "tag" "$TAG_REF"
  else
    printf '  ✗ %-34s %s  （应为 v%s）\n' "tag" "$TAG_REF" "$CODE_VER"
    FAIL=1
  fi
fi

echo
if [ "$FAIL" -ne 0 ]; then
  echo "✗ 版本号不一致。一条命令同步：bash scripts/check-version.sh --set $CODE_VER"
  exit 1
fi

echo "✓ 版本号一致（$CODE_VER）"
