<template>
  <div class="admin-page">
    <AdminHeader title="XSS 规则库" subtitle="维护 XSS 防御正则规则（动态更新）" :icon="Key" />
    <div class="admin-page-body admin-panel">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
        title="动态 XSS 防御规则"
        description="规则会在评论、帖子、章节、博客等所有用户内容保存前动态应用（Go RE2 正则语法）。默认「转义」：匹配到的 HTML 标签转义为纯文本完整显示（不删除、不残缺）；也可选「替换」或「删除」。修改后立即生效，无需重启或改代码。"
      />

      <div style="margin-bottom: 12px; display: flex; gap: 8px">
        <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon> 新增规则</el-button>
        <el-button size="small" @click="openTest()"><el-icon><Search /></el-icon> 测试正则</el-button>
        <el-button size="small" @click="loadRules" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
      </div>

      <el-table :data="rules" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="规则名称" min-width="140" />
        <el-table-column label="正则表达式" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="rule-pattern">{{ row.pattern }}</code>
          </template>
        </el-table-column>
        <el-table-column label="动作" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="actionTag(row.action)">{{ actionLabel(row.action) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="替换文本" width="120">
          <template #default="{ row }">
            <span v-if="row.action === 'replace' && row.replacement" class="rule-replace">{{ row.replacement }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="(v: any) => toggleEnabled(row, v)" />
          </template>
        </el-table-column>
        <el-table-column prop="description" label="说明" min-width="160" show-overflow-tooltip />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" @click="openTest(row.pattern, row.action, row.replacement)">测试</el-button>
            <el-button size="small" type="danger" @click="removeRule(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="暂无规则，点击「新增规则」添加" /></template>
      </el-table>
    </div>

    <!-- 新增/编辑规则弹窗 -->
    <el-dialog v-model="showDialog" :title="editingId ? '编辑规则' : '新增规则'" width="560px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="规则名称" required>
          <el-input v-model="form.name" placeholder="如：拦截会话过期钓鱼弹层" maxlength="128" />
        </el-form-item>
        <el-form-item label="正则表达式" required>
          <el-input v-model="form.pattern" type="textarea" :rows="3" placeholder="Go RE2 正则，如：会话已过期" />
          <div class="hint-text">正则语法为 Go RE2（不支持反向引用、lookaround）。</div>
        </el-form-item>
        <el-form-item label="动作">
          <el-radio-group v-model="form.action">
            <el-radio-button value="escape">转义（完整显示）</el-radio-button>
            <el-radio-button value="replace">替换</el-radio-button>
            <el-radio-button value="delete">删除</el-radio-button>
          </el-radio-group>
          <div class="hint-text">「转义」= 匹配到的 HTML 标签转义为纯文本完整显示（默认）；「替换」= 替换为下方文本；「删除」= 删除匹配内容。</div>
        </el-form-item>
        <el-form-item v-if="form.action === 'replace'" label="替换文本">
          <el-input v-model="form.replacement" placeholder="替换为指定文本" maxlength="256" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选：规则用途说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitRule">
          {{ editingId ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 测试正则弹窗 -->
    <el-dialog v-model="showTest" title="测试正则规则" width="620px">
      <el-form label-width="90px">
        <el-form-item label="正则表达式" required>
          <el-input v-model="testForm.pattern" type="textarea" :rows="2" placeholder="输入要测试的正则" />
        </el-form-item>
        <el-form-item label="动作">
          <el-radio-group v-model="testForm.action">
            <el-radio-button value="escape">转义</el-radio-button>
            <el-radio-button value="replace">替换</el-radio-button>
            <el-radio-button value="delete">删除</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="testForm.action === 'replace'" label="替换文本">
          <el-input v-model="testForm.replacement" placeholder="替换为指定文本" maxlength="256" />
        </el-form-item>
        <el-form-item label="测试文本" required>
          <el-input v-model="testForm.text" type="textarea" :rows="4" placeholder="粘贴要测试的内容" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showTest = false">关闭</el-button>
        <el-button type="primary" :loading="testing" @click="runTest">测试</el-button>
      </template>

      <el-divider v-if="testResult" content-position="left">测试结果</el-divider>
      <div v-if="testResult" class="test-result">
        <div class="test-line"><span class="test-label">匹配数量：</span><el-tag :type="testResult.match_count > 0 ? 'danger' : 'success'">{{ testResult.match_count }}</el-tag></div>
        <div v-if="testResult.matches && testResult.matches.length" class="test-line">
          <span class="test-label">匹配项：</span>
          <div class="test-matches">
            <el-tag v-for="(m, i) in testResult.matches" :key="i" type="danger" size="small" class="test-match">{{ m }}</el-tag>
          </div>
        </div>
        <div class="test-line">
          <span class="test-label">处理结果：</span>
          <pre class="test-output">{{ testResult.result }}</pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Plus, Search, Refresh, Key } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const rules = ref<any[]>([]);
const loading = ref(false);
const showDialog = ref(false);
const editingId = ref<number | null>(null);
const submitting = ref(false);
const form = reactive({ name: '', pattern: '', action: 'escape', replacement: '', enabled: true, description: '' });

const showTest = ref(false);
const testing = ref(false);
const testForm = reactive({ pattern: '', action: 'escape', replacement: '', text: '' });
const testResult = ref<any>(null);

async function loadRules() {
  loading.value = true;
  try {
    const res = await adminApi.getXssRules();
    rules.value = res.data.data || [];
  } catch { /* ignore */ }
  loading.value = false;
}

function openAdd() {
  editingId.value = null;
  form.name = '';
  form.pattern = '';
  form.action = 'escape';
  form.replacement = '';
  form.enabled = true;
  form.description = '';
  showDialog.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  form.name = row.name || '';
  form.pattern = row.pattern || '';
  form.action = row.action || 'escape';
  form.replacement = row.replacement || '';
  form.enabled = !!row.enabled;
  form.description = row.description || '';
  showDialog.value = true;
}

async function submitRule() {
  if (!form.name.trim()) { ElMessage.warning('请填写规则名称'); return; }
  if (!form.pattern.trim()) { ElMessage.warning('请填写正则表达式'); return; }
  submitting.value = true;
  try {
    if (editingId.value) {
      await adminApi.updateXssRule(editingId.value, { ...form });
      ElMessage.success('规则已更新');
    } else {
      await adminApi.createXssRule({ ...form });
      ElMessage.success('规则已创建，已即时生效');
    }
    showDialog.value = false;
    await loadRules();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  submitting.value = false;
}

async function toggleEnabled(row: any, v: boolean) {
  try {
    await adminApi.updateXssRule(row.id, { enabled: v });
    row.enabled = v;
    ElMessage.success(v ? '已启用' : '已禁用');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
    await loadRules();
  }
}

async function removeRule(row: any) {
  try {
    await ElMessageBox.confirm(`确定要删除规则「${row.name}」吗？`, '确认删除', { type: 'warning' });
    await adminApi.deleteXssRule(row.id);
    ElMessage.success('已删除');
    await loadRules();
  } catch { /* cancelled */ }
}

function openTest(pattern?: string, action?: string, replacement?: string) {
  testForm.pattern = pattern || '';
  testForm.action = action || 'escape';
  testForm.replacement = replacement || '';
  testForm.text = '';
  testResult.value = null;
  showTest.value = true;
}

function actionTag(a?: string) {
  const map: Record<string, string> = { escape: 'success', replace: 'warning', delete: 'danger' };
  return (map[a || 'escape'] as any) || 'info';
}
function actionLabel(a?: string) {
  const map: Record<string, string> = { escape: '转义', replace: '替换', delete: '删除' };
  return map[a || 'escape'] || a || '转义';
}

async function runTest() {
  if (!testForm.pattern.trim()) { ElMessage.warning('请输入正则表达式'); return; }
  if (!testForm.text) { ElMessage.warning('请输入测试文本'); return; }
  testing.value = true;
  try {
    const res = await adminApi.testXssRule({ ...testForm });
    testResult.value = res.data.data;
  } catch (e: any) {
    testResult.value = null;
    ElMessage.error(e?.response?.data?.message || '测试失败');
  }
  testing.value = false;
}

onMounted(loadRules);
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); line-height: 1.6; }
.muted { color: var(--text-light, #999); }
.rule-pattern { font-family: 'Consolas', 'Monaco', monospace; font-size: 0.85rem; color: var(--el-color-danger); background: var(--el-color-danger-light-9); padding: 2px 6px; border-radius: 3px; }
.rule-replace { color: var(--el-color-primary); font-family: 'Consolas', 'Monaco', monospace; font-size: 0.85rem; }
.test-result { padding: 0 12px; }
.test-line { display: flex; align-items: flex-start; gap: 8px; margin-bottom: 10px; }
.test-label { font-weight: 600; color: var(--text-color, #333); flex-shrink: 0; }
.test-matches { display: flex; flex-wrap: wrap; gap: 6px; }
.test-output { flex: 1; margin: 0; padding: 10px 12px; background: var(--bg-soft); border: 1px solid var(--border-color); border-radius: 6px; font-size: 0.85rem; white-space: pre-wrap; word-break: break-all; font-family: 'Consolas', 'Monaco', monospace; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
[data-theme="dark"] .test-label { color: #e2e8f0; }
[data-theme="dark"] .test-output { background: #1e293b; border-color: #334155; color: #cbd5e1; }
[data-theme="dark"] .rule-pattern { background: #2d1b22; color: #f472b6; }
</style>
