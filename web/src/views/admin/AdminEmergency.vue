<template>
  <div class="admin-page">
    <div class="admin-page-header">
      <el-button text @click="$router.push('/admin')"><el-icon><ArrowLeft /></el-icon> 返回后台</el-button>
      <h2>内容抢救台</h2>
    </div>

    <div class="admin-page-body">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
        title="隔离的应急窗口 · 强入口封锁"
        description="无需知道内部 ID——直接粘贴站内 URL 路径（如 /thread/123、/blog/5、/novel/3）即可定位并预览，确认后硬删除。所有操作需先通过二级密码解锁（5 分钟有效），删除走后端业务 API（含日志与统计回退），物理删除后不可恢复。"
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
            支持：/thread/:id /post/:id /comment/:id /blog/:id /novel/:id /forum/:id，也兼容 /author/:aid/... 与完整 URL
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
              sandbox="allow-same-origin allow-scripts"
              class="em-iframe"
              title="内容渲染预览"
            />
            <div class="em-iframe-tip">已用 sandbox 隔离（禁顶层跳转、禁表单），恶意脚本无法逃逸或影响后台。</div>
          </div>

          <!-- 硬删除 -->
          <div class="em-danger">
            <el-button type="danger" :loading="deleting" @click="doDelete">
              硬删除（物理删除，不可恢复）
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
import { ArrowLeft, Lock, Search, WarningFilled } from '@element-plus/icons-vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

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
      `确定要物理删除这条${typeLabel(preview.value.type)}（${preview.value.path || preview.value.id}）吗？此操作不可恢复！`,
      '危险操作确认',
      { type: 'error', confirmButtonText: '硬删除', cancelButtonText: '取消' },
    );
    deleting.value = true;
    await adminApi.deleteEmergencyContent(contentPath.value.trim(), token.value);
    ElMessage.success('已物理删除');
    preview.value = null;
    contentPath.value = '';
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return;
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
  deleting.value = false;
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
  const map: Record<string, string> = { comment: '评论', thread: '帖子', post: '回复', blog: '博客', novel: '作品', forum: '论坛板块' };
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
  const map: Record<string, string> = { unlock: 'success', unlock_fail: 'danger', preview: 'info', delete: 'danger', set_password: 'warning' };
  return (map[a] as any) || 'info';
}
function actionLabel(a: string) {
  const map: Record<string, string> = { unlock: '解锁', unlock_fail: '解锁失败', preview: '预览', delete: '删除', set_password: '改密码' };
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
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 900px; }
.em-card { background: #fff; border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; padding: 16px 20px; margin-bottom: 16px; }
.em-card-title { display: flex; align-items: center; gap: 8px; font-weight: 600; color: var(--text-color, #333); margin-bottom: 12px; }
.em-unlock, .em-fetch { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.em-path-hint { width: 100%; font-size: 0.78rem; color: var(--text-light, #999); }
.em-title { margin-bottom: 10px; font-weight: 600; color: var(--text-color, #333); }
.em-meta { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; margin-bottom: 12px; color: var(--text-color, #333); }
.em-meta code { background: #eef2ff; color: #4f46e5; padding: 1px 6px; border-radius: 4px; font-size: 0.85rem; }
.em-view-switch { margin-bottom: 12px; }
.em-plain { margin: 0; padding: 14px 16px; background: #f5f7fa; border: 1px solid #e5e7eb; border-radius: 6px; font-size: 0.9rem; white-space: pre-wrap; word-break: break-all; min-height: 120px; color: #333; }
.em-iframe-wrap { margin-bottom: 12px; }
.em-iframe { width: 100%; height: 320px; border: 1px solid #ccc; border-radius: 6px; background: #fff; }
.em-iframe-tip { font-size: 0.78rem; color: var(--text-light, #999); margin-top: 6px; }
.em-danger { margin-top: 16px; padding-top: 16px; border-top: 1px dashed var(--border-color, #e5e7eb); }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
[data-theme="dark"] .em-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .em-card-title, [data-theme="dark"] .em-meta { color: #e2e8f0; }
[data-theme="dark"] .em-plain { background: #0f172a; border-color: #334155; color: #cbd5e1; }
[data-theme="dark"] .em-iframe { background: #fff; }
</style>
