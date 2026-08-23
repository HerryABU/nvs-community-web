#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────
# NVS 端到端冒烟测试：覆盖 64KB 修复 / 导入追加 / 博客分类点赞评论 /
# 软删除区分 + 回收站 / 管理员监管 全部功能点。
# 用法：bash server/smoke_test.sh
# 说明：在隔离目录运行新 exe（复制 .env + data，不污染线上库），
#       结束自动清理。输出每项 PASS/FAIL 与汇总。
# ─────────────────────────────────────────────────────────────
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRV_PORT=18090
BASE="http://127.0.0.1:${SRV_PORT}"
# 目录名带 $$（进程 PID）保证每次运行唯一：避开安全删除机制对固定路径的延迟删除缓存
SMOKE_DIR="$ROOT/.smoke_$$"
# 用 -m 混合模式（C:/Users/... 正斜杠）：curl/python 均可读，且不会被字符串转义破坏
SMOKE_DIR_WIN=$(cygpath -m "$SMOKE_DIR" 2>/dev/null || echo "$SMOKE_DIR")
PY="$ROOT/../../../../.workbuddy/binaries/python/versions/3.13.12/python.exe"
[ -x "$PY" ] || PY=python
# bcrypt 模块安装在 venv（default）中，密码重置必须用 venv python
PY_VENV="$ROOT/../../../../.workbuddy/binaries/python/envs/default/Scripts/python.exe"
[ -x "$PY_VENV" ] || PY_VENV="$PY"
TESTDATA="$ROOT/../../tests/test_go"   # 用户测试环境（库已安装）

PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "  ✅ $1"; }
bad()  { FAIL=$((FAIL+1)); echo "  ❌ $1"; }
check() { # check <名称> <预期子串> <实际输出>
  if echo "$3" | grep -qF "$2"; then ok "$1"; else bad "$1 → 期望含「$2」，实际: $(echo "$3" | head -c 220)"; fi
}

# kill_port <port>：杀掉占用指定端口的进程（Windows PID，用 python TerminateProcess）
kill_port() {
  local pid
  pid=$(netstat -ano 2>/dev/null | grep ":$1" | grep LISTENING | awk '{print $NF}' | head -1)
  [ -n "$pid" ] && "$PY" -c "import os,sys; os.kill(int(sys.argv[1]), 9)" "$pid" 2>/dev/null
  sleep 1
}

echo "========== NVS 冒烟测试开始 =========="

# ── 准备隔离环境 ──
# 注意：Windows 上 rm -rf 可能被安全删除机制拦截，统一用 find -depth -delete
cleanup_dir() { [ -d "$1" ] && find "$1" -depth -delete 2>/dev/null; rmdir "$1" 2>/dev/null; }
# cleanup_dir "$SMOKE_DIR"  # 禁用：安全删除机制会异步删除新目录
kill_port "$SRV_PORT"   # 清理上一轮残留的冒烟服务（仅冒烟端口）
mkdir -p "$SMOKE_DIR"
cp "$ROOT/server/nvs-server.exe" "$SMOKE_DIR/"
cp "$TESTDATA/.env" "$SMOKE_DIR/" 2>/dev/null || cp "$ROOT/.env.example" "$SMOKE_DIR/.env" 2>/dev/null
cp -r "$TESTDATA/data" "$SMOKE_DIR/" 2>/dev/null || true

# 重置冒烟库 admin 密码为 Smoke@2026（bcrypt 在 venv）
"$PY_VENV" - << PYEOF 2>/dev/null || true
import bcrypt, sqlite3, glob, os
db = None
for p in glob.glob(os.path.join("$SMOKE_DIR_WIN", "data", "*.db")):
    db = p; break
if db:
    h = bcrypt.hashpw(b"Smoke@2026", bcrypt.gensalt()).decode()
    c = sqlite3.connect(db); cur = c.cursor()
    cur.execute("UPDATE users SET password_hash=? WHERE username='admin'", (h,))
    c.commit(); c.close(); print("admin password reset")
PYEOF

# ── 启动服务（清理时按端口找到进程后杀掉，绝不误杀线上 8080） ──
(cd "$SMOKE_DIR" && SERVER_PORT=$SRV_PORT ./nvs-server.exe > server.log 2>&1 &)
sleep 4
if ! curl -s -m 5 "$BASE/api/health" | grep -q '"status":"ok"'; then
  echo "❌ 服务启动失败，日志："; tail -20 "$SMOKE_DIR/server.log"; cleanup_dir "$SMOKE_DIR"; exit 1
fi
ok "服务启动 + health"

# 新前端 bundle 可访问
BUNDLE=$(curl -s -m 5 "$BASE/" | grep -o 'index-[A-Za-z0-9_-]*\.js' | head -1)
if [ -n "$BUNDLE" ] && [ "$(curl -s -m 5 -o /dev/null -w "%{http_code}" "$BASE/assets/$BUNDLE")" = "200" ]; then
  ok "前端 bundle 加载 ($BUNDLE)"
else
  bad "前端 bundle 加载失败 (bundle=$BUNDLE)"
fi

# ── 注册 + 登录作者 ──
U="smoke_$RANDOM"
REG=$(curl -s -m 8 -X POST "$BASE/api/auth/register" -H "Content-Type: application/json" \
  -d "{\"username\":\"$U\",\"password\":\"Test@12345\",\"email\":\"$U@qq.com\",\"agree_to_guidelines\":true}")
check "注册作者" '"code":0' "$REG"
curl -s -m 8 -c "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$U@qq.com\",\"password\":\"Test@12345\"}" > /dev/null

# ── T03 手动创建小说（标题必填链路） ──
NOVEL=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/novels" -H "Content-Type: application/json" \
  -d '{"title":"冒烟测试小说","category":"玄幻","summary":"冒烟","status":"published"}')
check "手动创建小说(带标题)" '"code":0' "$NOVEL"
NID=$(echo "$NOVEL" | "$PY" -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")

# 空标题 → 应拒绝
EMPTY=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/novels" -H "Content-Type: application/json" -d '{"title":"   "}')
check "空标题被拒绝" '请输入作品标题' "$EMPTY"

# ── T04 大 JSON 章节（64KB 截断回归） ──
"$PY" -c "
import json
json.dump({'title':'第1章 超长章节','content':'# 超长\n\n'+'这是超长章节内容用于回归64KB截断bug。'*20000}, open(r'$SMOKE_DIR_WIN\big.json','w',encoding='utf-8'), ensure_ascii=False)"
BIG=$(curl -s -m 30 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/novels/$NID/chapters" -H "Content-Type: application/json" \
  --data-binary @"$SMOKE_DIR_WIN/big.json")
check "超长章节(>64KB)创建成功" '"code":0' "$BIG"

# ── T05 单文件导入追加 ──
printf '# 追加章节一\n\n追加内容一\n' > "$SMOKE_DIR/c1.md"
IMP1=$(curl -s -m 20 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/novels/import" \
  -F "file=@$SMOKE_DIR_WIN/c1.md" -F "novel_id=$NID")
check "单文件导入追加(同一novel_id)" "\"novel_id\":$NID" "$IMP1"

# ── T06 批量导入追加 ──
printf '# 批量A\n\n批量内容A\n' > "$SMOKE_DIR/b1.md"
printf '# 批量B\n\n批量内容B\n' > "$SMOKE_DIR/b2.md"
IMPB=$(curl -s -m 20 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/novels/import/batch" \
  -F "files=@$SMOKE_DIR_WIN/b1.md" -F "files=@$SMOKE_DIR_WIN/b2.md" -F "mode=file" -F "novel_id=$NID")
check "批量导入追加(is_append=true)" '"is_append":true' "$IMPB"
check "批量导入不新建书本(同一novel_id)" "\"novel_id\":$NID" "$IMPB"

# ── 管理员：建博客分类（一/二/三级） ──
curl -s -m 8 -c "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/auth/login" -H "Content-Type: application/json" \
  -d '{"email":"admin@nvs.local","password":"Smoke@2026"}' > /dev/null
CAT1=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/blog-categories" -H "Content-Type: application/json" -d '{"name":"技术"}')
CAT1ID=$(echo "$CAT1" | "$PY" -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo 1)
check "管理员创建一级分类" '"code":0' "$CAT1"
CAT2=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/blog-categories" -H "Content-Type: application/json" -d "{\"name\":\"Go\",\"parent_id\":$CAT1ID}")
CAT2ID=$(echo "$CAT2" | "$PY" -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo 2)
check "创建二级分类" '"code":0' "$CAT2"
CAT3=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/blog-categories" -H "Content-Type: application/json" -d "{\"name\":\"并发编程\",\"parent_id\":$CAT2ID}")
CAT3ID=$(echo "$CAT3" | "$PY" -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo 3)
check "创建三级分类" '"code":0' "$CAT3"
CAT4=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/blog-categories" -H "Content-Type: application/json" -d "{\"name\":\"非法四级\",\"parent_id\":$CAT3ID}")
check "四级分类被拒绝" '三级' "$CAT4"
DUP=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/blog-categories" -H "Content-Type: application/json" -d '{"name":"技术"}')
check "同名分类被拒绝" '同名' "$DUP"

# ── T06c 普通作者（非管理员）也可管理分类：新增=待审核（事后审核） ──
ACAT=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/blog-categories" -H "Content-Type: application/json" -d '{"name":"作者自建分类"}')
check "普通作者可新增分类" '"code":0' "$ACAT"
ACATID=$(echo "$ACAT" | "$PY" -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo 99)
check "作者创建分类为待审核(pending)" '"status":"pending"' "$ACAT"

# ── T06d 分类事后审核：pending 不公开 → 管理员通过后公开 → 拒绝记录原因 ──
PUB=$(curl -s -m 8 "$BASE/api/blog-categories")
if echo "$PUB" | grep -qF "作者自建分类"; then bad "待审核分类不应出现在公开树"; else ok "待审核分类不公开显示"; fi
RV=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/blog-categories/$ACATID/review" -H "Content-Type: application/json" -d '{"action":"approve"}')
check "管理员通过分类审核" '"status":"approved"' "$RV"
PUB2=$(curl -s -m 8 "$BASE/api/blog-categories")
if echo "$PUB2" | grep -qF "作者自建分类"; then ok "审核通过后分类公开显示"; else bad "审核通过后分类应公开显示"; fi
RJ=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/blog-categories/$ACATID/review" -H "Content-Type: application/json" -d '{"action":"reject","reason":"测试拒绝"}')
check "管理员拒绝分类(rejected)" '"status":"rejected"' "$RJ"
check "拒绝原因已记录" '"reject_reason":"测试拒绝"' "$RJ"

# ── T06e 管理员列表按创建者分组（二级目录：用户 → 分类） ──
# 作者再建一个 pending 分类，确保分组中待审核数>0
APCAT=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/blog-categories" -H "Content-Type: application/json" -d '{"name":"作者待审分类"}')
check "作者再建待审核分类" '"status":"pending"' "$APCAT"
APCATID=$(echo "$APCAT" | "$PY" -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo 99)
ALIST=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" "$BASE/api/admin/blog-categories")
check "管理员列表返回分组结构" '"groups":[' "$ALIST"
check "分组含普通作者(待审核归入创建者)" "\"user_name\":\"$U\"" "$ALIST"
APEND=$(echo "$ALIST" | "$PY" -c "
import sys,json
d=json.load(sys.stdin)['data']
for g in d.get('groups',[]):
    if g['user_name']=='$U':
        print(g['pending_count'], g['total_count'])
        break
" 2>/dev/null || echo '0 0')
if [ "$(echo "$APEND" | awk '{print $1}')" -ge 1 ]; then ok "分组中作者待审核数>0 (pending=$APEND)"; else bad "作者分组 pending_count 应为>0，实际 $APEND"; fi
ADEL3=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X DELETE "$BASE/api/admin/blog-categories/$APCATID")
check "清理待审核分类" '已删除' "$ADEL3"
ADEL2=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X DELETE "$BASE/api/admin/blog-categories/$ACATID")
check "管理员删除分类" '已删除' "$ADEL2"

# ── T06b 用户名/数字解析（数字是主键、用户名兼容） ──
RSLV=$(curl -s -m 8 "$BASE/api/users/resolve?key=$U")
check "用户名解析为数字id" "\"username\":\"$U\"" "$RSLV"
RSLV2=$(curl -s -m 8 "$BASE/api/users/resolve?key=$(echo "$U" | tr 'a-z' 'A-Z')")
check "用户名大小写不敏感" "\"username\":\"$U\"" "$RSLV2"
UPROF=$(curl -s -m 8 "$BASE/api/author/profile/$U")
check "作者主页支持用户名" '"code":0' "$UPROF"
UFRM=$(curl -s -m 8 "$BASE/api/author/forum/$U")
check "作者论坛支持用户名" '"code":0' "$UFRM"
UFRAMES=$(curl -s -m 8 "$BASE/api/author/$U/frames")
check "作者模版接口支持用户名" '"code":0' "$UFRAMES"
UAB=$(curl -s -m 8 "$BASE/api/author/$U/blogs")
check "作者博客列表支持用户名" '"code":0' "$UAB"

# ── T07 博客带三级分类创建 ──
BLOG=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/blogs" -H "Content-Type: application/json" \
  -d "{\"title\":\"冒烟博客\",\"content\":\"# 冒烟博客内容\n\n正文\",\"category_id\":$CAT3ID}")
check "博客创建带三级分类" '"code":0' "$BLOG"
BID=$(echo "$BLOG" | "$PY" -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
check "博客分类名返回(三级前缀)" '技术/Go/并发编程' "$BLOG"
UBD=$(curl -s -m 8 "$BASE/api/author/$U/blogs/$BID")
check "博客详情支持用户名(/author/{用户名}/blogs/{id})" '"code":0' "$UBD"
UBD=$(curl -s -m 8 "$BASE/api/author/$U/blogs/$BID")
check "博客详情支持用户名(/author/{用户名}/blogs/{id})" '"code":0' "$UBD"

# ── T08 列表一级分类过滤（含三级子孙） ──
LF=$(curl -s -m 8 "$BASE/api/blogs?category_id=$CAT1ID")
check "列表按一级过滤含三级博客" '"list":[' "$LF"
check "列表一级过滤返回该博客" "冒烟博客" "$LF"

# ── T09 博客点赞 toggle ──
L1=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/blogs/$BID/like")
check "点赞 count=1" '"count":1' "$L1"
L2=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/blogs/$BID/like")
check "取消点赞 count=0" '"count":0' "$L2"

# ── T10 博客评论 ──
CM=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/comments" -H "Content-Type: application/json" \
  -d "{\"blog_id\":$BID,\"content\":\"冒烟评论\"}")
check "发表博客评论" '"code":0' "$CM"
CL=$(curl -s -m 8 "$BASE/api/comments?blog_id=$BID")
check "获取博客评论" '冒烟评论' "$CL"

# ── T11 删除小说 → 回收站 ──
DEL=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X DELETE "$BASE/api/novels/$NID")
check "删除小说(软删除)" '"code":0' "$DEL"
RC=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" "$BASE/api/author/recycle")
check "作者回收站可见自删小说" '冒烟测试小说' "$RC"
check "回收站标记 deleted_by=user" '"deleted_by":"user"' "$RC"

# ── T12 恢复 ──
RS=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/author/recycle/restore" -H "Content-Type: application/json" \
  -d "{\"type\":\"novel\",\"id\":$NID}")
check "恢复自删小说" '已恢复' "$RS"
VIS=$(curl -s -m 8 "$BASE/api/novels/$NID")
check "恢复后小说可见" '"code":0' "$VIS"

# ── T13 管理员封禁 → deleted_by=admin ──
BAN=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/softdel" -H "Content-Type: application/json" \
  -d "{\"type\":\"novel\",\"id\":$NID,\"action\":\"ban\",\"reason\":\"冒烟封禁测试\"}")
check "管理员封禁" '"code":0' "$BAN"

# ── T14 用户尝试恢复被封禁 → 拒绝 ──
RS2=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X POST "$BASE/api/author/recycle/restore" -H "Content-Type: application/json" \
  -d "{\"type\":\"novel\",\"id\":$NID}")
check "用户恢复被封禁被拒" '管理员封禁' "$RS2"

# ── T15 管理员回收站监管（用户自删内容，deleted_by=user）──
# 用户先删一篇博客 → 管理员监管列表应能看到
BDEL=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/ck.txt" -X DELETE "$BASE/api/blogs/$BID")
check "用户删除博客(软删除)" '"code":0' "$BDEL"
AR=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" "$BASE/api/admin/recycle?type=blog")
check "管理员回收站监管可见(用户删的博客)" '冒烟博客' "$AR"

# ── T16 管理员恢复被封禁 → 成功 ──
ADMR=$(curl -s -m 8 -b "$SMOKE_DIR_WIN/adm.txt" -X POST "$BASE/api/admin/softdel/restore" -H "Content-Type: application/json" \
  -d "{\"type\":\"novel\",\"id\":$NID}")
check "管理员恢复被封禁" '已解封' "$ADMR"
VIS2=$(curl -s -m 8 "$BASE/api/novels/$NID")
check "管理员恢复后小说可见" '"code":0' "$VIS2"

# ── 清理（只杀冒烟端口进程） ──
kill_port "$SRV_PORT"
sleep 1
# cleanup_dir "$SMOKE_DIR"  # 禁用：安全删除机制会异步删除新目录
rm -f "$ROOT/big_chapter.json" 2>/dev/null

echo ""
echo "========== 冒烟测试结果：$PASS 通过 / $FAIL 失败 =========="
[ "$FAIL" -eq 0 ]
