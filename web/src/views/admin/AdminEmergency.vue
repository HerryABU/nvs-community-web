<template>
  <div class="admin-page">
    <AdminHeader title="内容抢救台" subtitle="二级密码应急预览 / 软删除留证" :icon="WarningFilled" color="var(--gradient-accent)" />
    <div class="admin-page-body admin-panel">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
        title="隔离的应急窗口 · 强入口封锁"
        description="无需知道内部 ID——直接粘贴站内 URL 路径（如 /novel/3、/thread/123、/blog/5）即可定位并预览，确认后软删除（留证）。输入小说/博客 URL 会展示其全部评论，输入帖子 URL 会展示全部回复。所有操作需先通过二级密码解锁（5 分钟有效），删除会先将完整内容快照归档到 softdel_ 表再软删除，全程可追溯。"
      />

      <!-- 二级密码解锁 -->
      <div class="em-card">
        <div class="em-card-title">
          <el-icon><Lock /></el-icon> 二级密码解锁
          <el-tag v-if="unlocked" type="success" size="small">已解锁（{{ countdown }}s）</el-tag>
        </div>
        <div class="em-unlock">
          <el-input
            v-model="password"
            type="password"
            placeholder="输入二级密码"
            show-password
            style="width: 260px"
            @keyup.enter="doUnlock"
          />
          <el-button type="primary" :loading="unlocking" @click="doUnlock">解锁</el-button>
          <el-button text type="info" @click="showSetPassword = true">设置 / 修改密码</el-button>
        </div>
      </div>

      <!-- 内容预览 -->
      <div class="em-card">
        <div class="em-card-title"><el-icon><Search /></el-icon> 拉取内容预览</div>
        <div class="em-fetch">
          <el-input
            v-model="contentPath"
            placeholder="站内 URL 路径，如 /thread/123 或 /blog/5"
            style="width: 380px"
            clearable
            @keyup.enter="doPreview"
          />
          <el-button type="primary" :loading="previewing" @click="doPreview">拉取预览</el-button>
          <div class="em-path-hint">
            支持：/thread/:id /post/:id /comment/:id /blog/:id /novel/:id /chapter/:id /novel/:id/read/:num /forum/:id，也兼容 /author/:aid/... 与完整 URL
            · 输入小说 / 博客 / 帖子 URL 会展示其全部评论或回复，输入章节 URL 会展示章节正文
          </div>
        </div>

        <!-- 预览结果 -->
        <template v-if="preview">
          <el-divider content-position="left">预览结果</el-divider>
          <div class="em-meta">
            <span>类型：<el-tag size="small">{{ typeLabel(preview.type) }}</el-tag></span>
            <span>ID：{{ preview.id }}</span>
            <span v-if="preview.path">路径：<code>{{ preview.path }}</code></span>
            <span>作者：<strong>{{ preview.author || '未知' }}</strong></span>
            <span v-if="preview.created_at">时间：{{ preview.created_at }}</span>
          </div>

          <div v-if="preview.title" class="em-title">标题：{{ preview.title }}</div>

          <!-- 关联评论 / 回复（小说/博客/帖子等）——输入 URL 优先展示所有评论 -->
          <div v-if="preview.comments" class="em-comments">
            <div class="em-comments-head">
              全部评论 / 回复（共 {{ preview.comments.length }} 条）
              <span class="em-comments-tip">每条评论可独立切换「纯文本 / iframe 沙箱」隔离渲染</span>
            </div>
            <EmptyState v-if="preview.comments.length === 0" icon="💬" text="暂无评论" />
            <div v-for="c in preview.comments" :key="c.id" class="em-comment-item">
              <div class="em-comment-meta">
                <span class="em-comment-author">{{ c.author || '匿名' }}</span>
                <el-tag v-if="c.chapter" size="small" type="info">第{{ c.chapter }}章</el-tag>
                <el-tag size="small" type="info" effect="plain">{{ c.type === 'post' ? '回复' : '评论' }}</el-tag>
                <span class="em-comment-id">#{{ c.id }}</span>
                <span class="em-comment-time">{{ formatTime(c.created_at) }}</span>
                <el-button
                  size="small" type="danger" text
                  :loading="deletingId === c.id"
                  @click="deleteSubItem(c)"
                >删除</el-button>
              </div>
              <!-- 单条评论：纯文本 / iframe 沙箱 隔离渲染切换 -->
              <div class="em-comment-view-switch">
                <el-radio-group :model-value="commentMode(c.id)" size="small" @update:model-value="commentModes[c.id] = $event">
                  <el-radio-button value="plain">纯文本</el-radio-button>
                  <el-radio-button value="iframe">iframe 渲染</el-radio-button>
                </el-radio-group>
              </div>
              <pre v-if="commentMode(c.id) === 'plain'" class="em-comment-plain">{{ c.content_plain || c.content }}</pre>
              <iframe v-else :srcdoc="c.content || ''" sandbox="allow-scripts" class="em-comment-iframe" title="评论渲染预览" />
            </div>
          </div>

          <!-- 纯文本 / iframe 沙箱 切换 -->
          <div class="em-view-switch">
            <el-radio-group v-model="viewMode">
              <el-radio-button value="plain">纯文本（安全，推荐）</el-radio-button>
              <el-radio-button value="iframe">渲染预览（iframe 沙箱）</el-radio-button>
            </el-radio-group>
          </div>

          <!-- 纯文本 -->
          <pre v-if="viewMode === 'plain'" class="em-plain">{{ preview.content_plain }}</pre>

          <!-- iframe 沙箱 -->
          <div v-else class="em-iframe-wrap">
            <iframe
              :srcdoc="preview.content_raw || ''"
              sandbox="allow-scripts"
              class="em-iframe"
              title="内容渲染预览"
            />
            <div class="em-iframe-tip">已用 sandbox 隔离（禁顶层跳转、禁表单），恶意脚本无法逃逸或影响后台。</div>
          </div>

          <!-- 软删除（留证） -->
          <div class="em-danger">
            <el-button type="danger" :loading="deleting" @click="doDelete">
              软删除（归档留证，可追溯）
            </el-button>
          </div>
        </template>
      </div>

      <!-- 审计日志 -->
      <div class="em-card">
        <div class="em-card-title">
          <el-icon><WarningFilled /></el-icon> 抢救台审计日志
          <el-button text size="small" @click="loadAudit">刷新</el-button>
        </div>
        <el-table :data="auditLogs" size="small" max-height="320" style="width: 100%">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="operator" label="操作者" width="110" />
          <el-table-column prop="ip" label="IP" width="130" />
          <el-table-column label="动作" width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="actionTag(row.action)">{{ actionLabel(row.action) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="目标" width="140">
            <template #default="{ row }">
              <span v-if="row.target_type">{{ typeLabel(row.target_type) }} #{{ row.target_id }}</span>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column prop="detail" label="详情" min-width="180" show-overflow-tooltip />
          <el-table-column label="时间" width="160">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
          <template #empty>
            <EmptyState icon="🗂️" text="暂无审计日志" description="解锁并操作后，相关记录会显示在这里" />
          </template>
        </el-table>
      </div>
    </div>

    <!-- 设置二级密码弹窗 -->
    <el-dialog v-model="showSetPassword" title="设置 / 修改二级密码" width="420px">
      <el-form label-width="80px">
        <el-form-item v-if="hasOldPassword" label="旧密码">
          <el-input v-model="setForm.old_password" type="password" show-password placeholder="修改时需验证旧密码" />
        </el-form-item>
        <el-form-item label="新密码" required>
          <el-input v-model="setForm.new_password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showSetPassword = false">取消</el-button>
        <el-button type="primary" :loading="setting" @click="doSetPassword">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onBeforeUnmount, onMounted } from 'vue';
import { Lock, Search, WarningFilled } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';

const password = ref('');
const unlocked = ref(false);
const token = ref('');
const unlocking = ref(false);
const countdown = ref(0);
let timer: any = null;

const contentPath = ref('');
const previewing = ref(false);
const preview = ref<any>(null);
const viewMode = ref('plain');
const deleting = ref(false);
const deletingId = ref<number | null>(null);

// 每条评论独立的「纯文本 / iframe 沙箱」渲染模式（按评论 id 记录）
const commentModes = reactive<Record<number, string>>({});
function commentMode(id: number) {
  return commentModes[id] || 'plain';
}

const showSetPassword = ref(false);
const setting = ref(false);
const hasOldPassword = ref(false);
const setForm = reactive({ old_password: '', new_password: '' });

function startCountdown() {
  countdown.value = 300;
  if (timer) clearInterval(timer);
  timer = setInterval(() => {
    countdown.value -= 1;
    if (countdown.value <= 0) {
      clearInterval(timer);
      unlocked.value = false;
      token.value = '';
    }
  }, 1000);
}

async function doUnlock() {
  if (!password.value) { ElMessage.warning('请输入二级密码'); return; }
  unlocking.value = true;
  try {
    const res = await adminApi.unlockEmergency({ password: password.value });
    token.value = res.data.data.token;
    unlocked.value = true;
    password.value = '';
    startCountdown();
    ElMessage.success('已解锁');
  } catch (e: any) {
    if (e?.response?.data?.message?.includes('尚未设置')) {
      ElMessage.warning('尚未设置二级密码，请先点击「设置密码」');
      showSetPassword.value = true;
    } else {
      ElMessage.error(e?.response?.data?.message || '解锁失败');
    }
  }
  unlocking.value = false;
}

async function doPreview() {
  if (!unlocked.value) { ElMessage.warning('请先解锁二级密码'); return; }
  if (!contentPath.value.trim()) { ElMessage.warning('请输入站内 URL 路径'); return; }
  previewing.value = true;
  try {
    const res = await adminApi.previewEmergencyContent(contentPath.value.trim(), token.value);
    preview.value = res.data.data;
    viewMode.value = 'plain';
  } catch (e: any) {
    preview.value = null;
    if (e?.response?.data?.message?.includes('未验证')) { unlocked.value = false; token.value = ''; }
    ElMessage.error(e?.response?.data?.message || '拉取失败');
  }
  previewing.value = false;
}

async function doDelete() {
  if (!unlocked.value) { ElMessage.warning('请先解锁二级密码'); return; }
  if (!preview.value) { ElMessage.warning('请先拉取内容预览'); return; }
  try {
    await ElMessageBox.confirm(
      `确定要软删除这条${typeLabel(preview.value.type)}（${preview.value.path || preview.value.id}）吗？删除前会归档留证到 softdel 表，原始记录软删除（可追溯）。`,
      '危险操作确认',
      { type: 'error', confirmButtonText: '软删除并留证', cancelButtonText: '取消' },
    );
    deleting.value = true;
    await adminApi.deleteEmergencyContent(contentPath.value.trim(), token.value);
    ElMessage.success('已软删除并留证');
    preview.value = null;
    contentPath.value = '';
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return;
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
  deleting.value = false;
}

// 删除预览列表中的单条评论 / 回复（归档留证 + 软删除）
async function deleteSubItem(item: any) {
  if (!unlocked.value) { ElMessage.warning('请先解锁二级密码'); return; }
  if (!token.value) { ElMessage.warning('二级密码 token 已失效，请重新解锁'); return; }
  const kind = item.type === 'post' ? '回复' : '评论';
  try {
    await ElMessageBox.confirm(
      `确定要软删除这条${kind}（#${item.id}，作者 ${item.author || '未知'}）吗？删除前会归档留证到 softdel 表。`,
      '危险操作确认',
      { type: 'error', confirmButtonText: '软删除', cancelButtonText: '取消' },
    );
    deletingId.value = item.id;
    const path = `/${item.type === 'post' ? 'post' : 'comment'}/${item.id}`;
    await adminApi.deleteEmergencyContent(path, token.value);
    ElMessage.success('已软删除并留证');
    if (preview.value?.comments) {
      preview.value.comments = preview.value.comments.filter((x: any) => x.id !== item.id);
    }
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return;
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
  deletingId.value = null;
}

async function doSetPassword() {
  if (!setForm.new_password || setForm.new_password.length < 6) {
    ElMessage.warning('新密码至少 6 位');
    return;
  }
  setting.value = true;
  try {
    await adminApi.setEmergencyPassword({
      old_password: setForm.old_password || undefined,
      new_password: setForm.new_password,
    });
    ElMessage.success('二级密码已设置');
    showSetPassword.value = false;
    setForm.old_password = '';
    setForm.new_password = '';
    hasOldPassword.value = true;
    unlocked.value = false;
    token.value = '';
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '设置失败');
  }
  setting.value = false;
}

function typeLabel(t: string) {
  const map: Record<string, string> = { comment: '评论', thread: '帖子', post: '回复', blog: '博客', novel: '作品', chapter: '章节', forum: '论坛板块', user: '用户', report: '举报' };
  return map[t] || t;
}

const auditLogs = ref<any[]>([]);

async function loadAudit() {
  try {
    const res = await adminApi.getEmergencyAudit();
    auditLogs.value = res.data.data || [];
  } catch { /* ignore */ }
}

function actionTag(a: string) {
  const map: Record<string, string> = {
    unlock: 'success', unlock_fail: 'danger', preview: 'info', delete: 'danger', set_password: 'warning',
    content_delete: 'danger', content_hide: 'warning', content_restore: 'success',
    user_mute: 'warning', user_unmute: 'success', user_ban: 'danger', user_unban: 'success',
    report_handle: 'info',
  };
  return (map[a] as any) || 'info';
}
function actionLabel(a: string) {
  const map: Record<string, string> = {
    unlock: '解锁', unlock_fail: '解锁失败', preview: '预览', delete: '软删除', set_password: '改密码',
    content_delete: '删除内容', content_hide: '屏蔽内容', content_restore: '恢复内容',
    user_mute: '禁言', user_unmute: '解除禁言', user_ban: '封禁', user_unban: '解封',
    report_handle: '举报处理',
  };
  return map[a] || a;
}
function formatTime(s?: string) {
  if (!s) return '';
  return new Date(s).toLocaleString('zh-CN');
}

onMounted(() => { loadAudit(); });
onBeforeUnmount(() => { if (timer) clearInterval(timer); });
</script>

<style scoped>
/* 全量令牌化：颜色一律走 global.css 设计令牌（暗色自动翻转，不再手写 [data-theme] 覆盖）。
   唯一例外：iframe 容器保持 #fff——其内部渲染的是用户内容 HTML（按亮色编写），
   暗色下也应为亮底，与 cherry 编辑器预览行为一致。 */
.admin-page { --admin-content-max: 900px; min-height: 100vh; padding: 24px; background: var(--bg-page); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color); }
.admin-page-body { max-width: 900px; }
.em-card { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 10px; padding: 16px 20px; margin-bottom: 16px; }
.em-card-title { display: flex; align-items: center; gap: 8px; font-weight: 600; color: var(--text-color); margin-bottom: 12px; }
.em-unlock, .em-fetch { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.em-path-hint { width: 100%; font-size: 0.78rem; color: var(--text-muted); }
.em-title { margin-bottom: 10px; font-weight: 600; color: var(--text-color); }
.em-comments { margin-bottom: 12px; }
.em-comments-head { font-weight: 600; color: var(--text-color); margin-bottom: 8px; padding-bottom: 6px; border-bottom: 1px solid var(--border-color); }
.em-comment-item { padding: 8px 10px; margin-bottom: 8px; background: var(--bg-subtle); border: 1px solid var(--border-color); border-radius: 6px; }
.em-comment-meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 4px; }
.em-comment-author { font-weight: 600; color: var(--primary-color); }
.em-comment-id { color: var(--text-muted); font-size: 0.8rem; }
.em-comment-time { color: var(--text-muted); font-size: 0.8rem; margin-left: auto; }
.em-comment-content { color: var(--text-color); font-size: 0.88rem; white-space: pre-wrap; word-break: break-all; }
.em-comments-tip { font-weight: 400; font-size: 0.75rem; color: var(--text-muted); margin-left: 8px; }
.em-comment-view-switch { margin: 4px 0 6px; }
.em-comment-plain { margin: 0; padding: 8px 10px; background: var(--bg-subtle); border: 1px solid var(--border-color); border-radius: 6px; font-size: 0.86rem; white-space: pre-wrap; word-break: break-all; color: var(--text-color); }
.em-comment-iframe { width: 100%; height: 180px; border: 1px solid var(--border-color); border-radius: 6px; background: #fff; }
.em-meta { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; margin-bottom: 12px; color: var(--text-color); }
.em-meta code { background: var(--bg-soft); color: var(--primary-color); padding: 1px 6px; border-radius: 4px; font-size: 0.85rem; }
.em-view-switch { margin-bottom: 12px; }
.em-plain { margin: 0; padding: 14px 16px; background: var(--bg-soft); border: 1px solid var(--border-color); border-radius: 6px; font-size: 0.9rem; white-space: pre-wrap; word-break: break-all; min-height: 120px; color: var(--text-color); }
.em-iframe-wrap { margin-bottom: 12px; }
.em-iframe { width: 100%; height: 320px; border: 1px solid var(--border-color); border-radius: 6px; background: #fff; }
.em-iframe-tip { font-size: 0.78rem; color: var(--text-muted); margin-top: 6px; }
.em-danger { margin-top: 16px; padding-top: 16px; border-top: 1px dashed var(--border-color); }
</style>
