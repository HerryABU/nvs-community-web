<template>
  <div class="novels-section">
    <h2 class="section-title-bar">
      <span class="title-bar-text">🔑 API Key 管理</span>
      <span class="title-bar-count">{{ myKeys.length }} 个</span>
    </h2>
    <div class="apikey-panel">
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 14px"
        :title="`当前身份：${roleLabel} · 本角色 Key 配额上限 ${roleQuota.req} 次 / ${roleQuota.window} 秒`">
        <template #default>
          <div class="apikey-hint">
            创建 API Key 后，调用本站 API 时携带 <code>X-API-Key</code> 请求头即可获得远高于匿名访问的配额。
            不带 Key 的请求会被压到很低的匿名配额，被识别为爬虫则更低。批量抓取 / 自动化脚本想提速，请带上你的 Key。
            详细接入示例见站内页面 <router-link to="/api-spider">「🕷️ API Key 爬虫指南」</router-link>。
          </div>
        </template>
      </el-alert>

      <el-form label-width="90px" label-position="top" style="margin-bottom: 16px">
        <el-row :gutter="12">
          <el-col :xs="24" :sm="8">
            <el-form-item label="备注名">
              <el-input v-model="createForm.name" placeholder="如：我的阅读器" size="small" />
            </el-form-item>
          </el-col>
          <el-col :xs="12" :sm="5">
            <el-form-item label="配额（次/分）">
              <el-input-number v-model="createForm.quota" :min="1" :max="roleQuota.req" size="small" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :xs="12" :sm="11">
            <el-form-item label="过期时间（可选）">
              <el-date-picker
                v-model="createForm.expiresAt"
                type="date"
                placeholder="永久有效"
                size="small"
                style="width: 100%"
                value-format="YYYY-MM-DD"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="IP 白名单（可选）">
          <el-input v-model="createForm.allowedIps" placeholder="如 1.2.3.4, 10.0.0.0/8（留空 = 不限 IP）" size="small" />
        </el-form-item>
        <el-button type="primary" size="small" @click="createKey" :loading="creatingKey">创建 API Key</el-button>
      </el-form>

      <el-divider style="margin: 8px 0 14px" />
      <el-table :data="myKeys" size="small" v-loading="loadingKeys" style="width: 100%">
        <el-table-column prop="name" label="备注" min-width="100">
          <template #default="{ row }">{{ row.name || '—' }}</template>
        </el-table-column>
        <el-table-column label="Key 前缀" width="160">
          <template #default="{ row }"><code>{{ row.key_prefix }}</code></template>
        </el-table-column>
        <el-table-column label="配额" width="100">
          <template #default="{ row }">{{ row.quota_per_min }} 次/分</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">{{ row.is_active ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="过期" width="100">
          <template #default="{ row }">{{ row.expires_at ? row.expires_at.slice(0, 10) : '永久' }}</template>
        </el-table-column>
        <el-table-column label="最近使用" width="150">
          <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
        </el-table-column>
        <el-table-column label="调用次数" width="90">
          <template #default="{ row }">{{ row.total_calls ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="toggleKey(row)">{{ row.is_active ? '停用' : '启用' }}</el-button>
            <el-button size="small" text type="warning" @click="regenerateKey(row)">重生成</el-button>
            <el-button size="small" text type="danger" @click="deleteKey(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <p v-if="!myKeys.length && !loadingKeys" class="apikey-empty">尚未创建 API Key</p>
    </div>

    <!-- 完整 Key 一次性展示 -->
    <el-dialog v-model="showNewKey" title="API Key 创建成功" width="540px">
      <el-alert type="warning" :closable="false" show-icon title="完整 Key 仅此一次展示，请立即复制保存" style="margin-bottom: 12px" />
      <div class="newkey-box">
        <code class="newkey-code">{{ newKeyValue }}</code>
        <el-button type="primary" @click="copyKey">复制</el-button>
      </div>
      <p class="apikey-hint">
        请求时携带 <code>X-API-Key: {{ newKeyPrefix }}…</code>（完整 Key）。服务端仅存储哈希，遗失后只能重新生成。
      </p>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { authorDataVersion } from './authorRefresh';
import { ref, reactive, onMounted, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiKeyApi } from '@/api/apikey';

// API Key 视图：作者工作台内直接创建 / 停用 / 重生成 / 删除 Key。
const myKeys = ref<any[]>([]);
const loadingKeys = ref(false);
const roleLabel = ref('读者');
const roleQuota = reactive({ req: 60, window: 60 });
const createForm = reactive({ name: '', quota: 0, allowedIps: '', expiresAt: '' });
const creatingKey = ref(false);
const showNewKey = ref(false);
const newKeyValue = ref('');
const newKeyPrefix = ref('');

function formatTime(s?: string) {
  if (!s) return '—';
  return new Date(s).toLocaleString('zh-CN');
}

async function loadMyKeys() {
  loadingKeys.value = true;
  try {
    const res = await apiKeyApi.list();
    const d = res.data.data || {};
    myKeys.value = d.list || [];
    const rl: Record<string, string> = { admin: '站长', author: '作者', reader: '读者' };
    roleLabel.value = rl[d.role] || '读者';
    roleQuota.req = d.role_quota?.req || 60;
    roleQuota.window = d.role_quota?.window || 60;
  } catch { /* ignore */ }
  loadingKeys.value = false;
}

async function createKey() {
  if (createForm.quota < 1) createForm.quota = roleQuota.req;
  creatingKey.value = true;
  try {
    const res = await apiKeyApi.create({
      name: createForm.name,
      quota_per_min: createForm.quota,
      allowed_ips: createForm.allowedIps,
      expires_at: createForm.expiresAt || undefined,
    });
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      newKeyPrefix.value = res.data.data.key_prefix || '';
      showNewKey.value = true;
      ElMessage.success('API Key 已创建');
      createForm.name = '';
      createForm.allowedIps = '';
      createForm.expiresAt = '';
      await loadMyKeys();
    } else {
      ElMessage.error(res.data.message || '创建失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  }
  creatingKey.value = false;
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(newKeyValue.value);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择复制');
  }
}

async function toggleKey(row: any) {
  try {
    await apiKeyApi.update(row.id, { is_active: !row.is_active });
    ElMessage.success('已更新状态');
    await loadMyKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function regenerateKey(row: any) {
  try {
    await ElMessageBox.confirm('重新生成后旧 Key 立即失效，确定继续？', '重新生成', { type: 'warning' });
    const res = await apiKeyApi.regenerate(row.id);
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      newKeyPrefix.value = res.data.data.key_prefix || '';
      showNewKey.value = true;
      ElMessage.success('已重新生成');
      await loadMyKeys();
    }
  } catch { /* cancelled */ }
}

async function deleteKey(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除 API Key「${row.key_prefix}」吗？删除后立即失效。`, '删除 API Key', { type: 'warning' });
    await apiKeyApi.remove(row.id);
    ElMessage.success('已删除');
    await loadMyKeys();
  } catch { /* cancelled */ }
}

onMounted(loadMyKeys);

// 其它视图改动了数据（如回收站恢复作品）→ 重新拉取
watch(authorDataVersion, () => { loadMyKeys });
</script>

<style scoped>
@import './section.css';

/* API Key 管理面板 */
.apikey-panel {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 14px;
  padding: 20px;
}
[data-theme="light"] .apikey-panel {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}
.apikey-hint {
  margin: 4px 0 0;
  font-size: 0.85rem;
  line-height: 1.5;
  color: var(--text-light, #999);
}
.apikey-hint code {
  background: #eef2ff;
  color: #4f46e5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.82rem;
}
[data-theme="dark"] .apikey-hint code {
  background: rgba(99, 102, 241, 0.15);
  color: #a5b4fc;
}
.apikey-empty {
  text-align: center;
  margin-top: 8px;
  font-size: 0.85rem;
  color: var(--text-light, #999);
}
.apikey-panel :deep(code) {
  background: #eef2ff;
  color: #4f46e5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.82rem;
}
[data-theme="dark"] .apikey-panel :deep(code) {
  background: rgba(99, 102, 241, 0.15);
  color: #a5b4fc;
}
.apikey-panel .newkey-box { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.apikey-panel .newkey-code { flex: 1; background: #0f172a; color: #4ade80; padding: 10px 12px; border-radius: 8px; font-size: 0.82rem; word-break: break-all; }
[data-theme="light"] .apikey-panel .newkey-code { background: #f1f5f9; color: #15803d; }

/* 完整 Key 一次性展示（弹窗被 teleport 到 body，原样式挂在 .apikey-panel 下不生效，这里补一份） */
.newkey-box { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.newkey-code {
  flex: 1; background: #0f172a; color: #4ade80;
  padding: 10px 12px; border-radius: 8px; font-size: 0.82rem; word-break: break-all;
}
[data-theme="light"] .newkey-code { background: #f1f5f9; color: #15803d; }
</style>
