<template>
  <div class="mykeys-page">
    <div class="mykeys-container">
      <div class="mykeys-header">
        <div class="mykeys-title">
          <el-icon :size="26"><Key /></el-icon>
          <span>我的 API Key</span>
        </div>
        <div class="mykeys-sub">
          <span>第三方客户端 / 阅读器 / 爬虫合规访问本站 API 的凭证</span>
          <router-link to="/api-spider" class="spider-link">
            <el-icon><Connection /></el-icon> 🕷️ API Key 爬虫指南
          </router-link>
        </div>
      </div>

      <el-card class="settings-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <el-icon><Key /></el-icon>
            <span>API Key 管理</span>
          </div>
        </template>
        <el-alert
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 14px"
          :title="`当前身份：${roleLabel} · 本角色 Key 配额上限 ${roleQuota.req} 次 / ${roleQuota.window} 秒`"
        >
          <template #default>
            <div class="hint" style="margin: 4px 0 0">
              创建 API Key 后，调用本站 API 时携带 <code>X-API-Key</code> 请求头即可获得远高于匿名访问的配额。
              不带 Key 的请求会被压到很低的匿名配额，被识别为爬虫则更低，命中已知名头爬虫名单会直接 403 ——
              批量抓取/自动化脚本想提速，请在请求中带上你的 Key。可自设不超过角色上限的更小配额，随时停用/吊销。
              详细接入示例见站内页面 <router-link to="/api-spider">「🕷️ API Key 爬虫指南」</router-link>。
            </div>
          </template>
        </el-alert>

        <!-- 创建 -->
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
            <el-col :xs="24" :sm="11">
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
        <p v-if="!myKeys.length && !loadingKeys" class="hint" style="text-align: center; margin-top: 8px">尚未创建 API Key</p>
      </el-card>
    </div>

    <!-- 完整 Key 一次性展示 -->
    <el-dialog v-model="showNewKey" title="API Key 创建成功" width="540px">
      <el-alert type="warning" :closable="false" show-icon title="完整 Key 仅此一次展示，请立即复制保存" style="margin-bottom: 12px" />
      <div class="newkey-box">
        <code class="newkey-code">{{ newKeyValue }}</code>
        <el-button type="primary" @click="copyKey">复制</el-button>
      </div>
      <p class="hint">
        请求时携带 <code>X-API-Key: {{ newKeyPrefix }}…</code>（完整 Key）。服务端仅存储哈希，遗失后只能重新生成。
      </p>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { apiKeyApi } from '@/api/apikey';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Key, Connection } from '@element-plus/icons-vue';

// ====== API Key 管理（所有登录用户均可获取 / 管理自己的 Key） ======
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

onMounted(() => {
  loadMyKeys();
});
</script>

<style scoped>
.mykeys-page {
  min-height: 100vh;
  padding: 24px 16px 48px;
  background: var(--bg-page, #f5f7fa);
}
.mykeys-container {
  max-width: 960px;
  margin: 0 auto;
}
.mykeys-header {
  margin-bottom: 18px;
}
.mykeys-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text-color);
}
.mykeys-sub {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-top: 6px;
  font-size: 0.9rem;
  color: var(--text-muted, #909399);
}
.spider-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--primary-color, #6366f1);
  text-decoration: none;
  font-weight: 500;
}
.spider-link:hover {
  text-decoration: underline;
}
.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}
.settings-card {
  border-radius: 14px;
}
.newkey-box {
  display: flex;
  gap: 10px;
  align-items: center;
}
.newkey-code {
  flex: 1;
  word-break: break-all;
  background: var(--bg-soft, #f0f2f5);
  padding: 8px 10px;
  border-radius: 8px;
  font-family: monospace;
}
.hint {
  color: var(--text-muted, #909399);
  font-size: 0.86rem;
  line-height: 1.6;
}
.hint code,
.settings-card :deep(code) {
  background: rgba(99, 102, 241, 0.1);
  padding: 1px 5px;
  border-radius: 4px;
  font-family: monospace;
}
</style>
