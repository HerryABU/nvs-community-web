<template>
  <div class="admin-page">
    <AdminHeader title="违禁词库" subtitle="标记 / 删除 / 替换 · 关键词 + 正则双模式 · 可配置适用范围" :icon="Warning" />
    <div class="admin-page-body admin-panel">
      <el-tabs v-model="activeTab">
        <!-- ═══════════ Tab 1: 规则管理 ═══════════ -->
        <el-tab-pane label="规则管理" name="rules">
          <el-alert
            type="info"
            :closable="false"
            show-icon
            style="margin-bottom: 16px"
            title="违禁词规则库"
            description="规则在章节、评论、论坛、博客等用户内容保存前动态应用（Go RE2 正则语法），修改后立即生效。动作「标记」= 命中内容正常保存但进入下方待审列表，由管理员处理；「删除」= 删除匹配内容；「替换」= 替换为指定文本。匹配方式「关键词」= 字面匹配自动转义，填词即用；「正则」= 支持 (?i) 忽略大小写、(?m) 多行、\p{Han} Unicode 属性等全方位正则。"
          />

          <div class="admin-toolbar">
            <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon> 新增违禁词</el-button>
            <el-button size="small" @click="openTest()"><el-icon><Search /></el-icon> 测试规则</el-button>
            <el-button size="small" @click="loadWords" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
            <el-button size="small" type="warning" plain @click="switchToFlags"><el-icon><Warning /></el-icon> 待审列表（{{ pendingCount }}）</el-button>
          </div>

          <el-table :data="words" v-loading="loading" style="width: 100%">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="name" label="规则名称" min-width="130" />
            <el-table-column label="匹配方式" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="row.match_type === 'regex' ? 'info' : 'success'">{{ row.match_type === 'regex' ? '正则' : '关键词' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="违禁词 / 正则" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                <code class="rule-pattern">{{ row.pattern }}</code>
              </template>
            </el-table-column>
            <el-table-column label="动作" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="actionTag(row.action)">{{ actionLabel(row.action) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="替换文本" width="110">
              <template #default="{ row }">
                <span v-if="row.action === 'replace' && row.replacement" class="rule-replace">{{ row.replacement }}</span>
                <span v-else class="muted">—</span>
              </template>
            </el-table-column>
            <el-table-column label="适用范围" width="170">
              <template #default="{ row }">
                <div v-if="row.scope === 'all' || !row.scope"><el-tag size="small" type="primary" effect="plain">全部内容</el-tag></div>
                <div v-else style="display: flex; flex-wrap: wrap; gap: 4px">
                  <el-tag v-for="s in (row.scope || '').split(',')" :key="s" size="small" effect="plain">{{ scopeLabel(s) }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="80">
              <template #default="{ row }">
                <el-switch :model-value="row.enabled" @change="(v: any) => toggleEnabled(row, v)" />
              </template>
            </el-table-column>
            <el-table-column prop="description" label="说明" min-width="140" show-overflow-tooltip />
            <el-table-column label="操作" width="220" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="openEdit(row)">编辑</el-button>
                <el-button size="small" @click="openTest(row)">测试</el-button>
                <el-button size="small" type="danger" @click="removeWord(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty><EmptyState icon="📭" text="暂无违禁词" description="点击「新增违禁词」添加" /></template>
          </el-table>
        </el-tab-pane>

        <!-- ═══════════ Tab 2: 标记待审 ═══════════ -->
        <el-tab-pane :label="`标记待审${pendingCount > 0 ? '（' + pendingCount + '）' : ''}`" name="flags">
          <el-alert
            type="warning"
            :closable="false"
            show-icon
            style="margin-bottom: 16px"
            title="标记待审"
            description="以下内容命中了「标记」动作的违禁词规则，内容已正常发布但被标记。请逐条审核：确认违规可「确认并删除」（软删除 + 留证），误报可「放行」。"
          />

          <div class="admin-toolbar">
            <el-radio-group v-model="flagStatus" size="small" @change="loadFlags">
              <el-radio-button value="">全部</el-radio-button>
              <el-radio-button value="pending">待审</el-radio-button>
              <el-radio-button value="confirmed">已确认违规</el-radio-button>
              <el-radio-button value="dismissed">已放行</el-radio-button>
            </el-radio-group>
            <el-button size="small" @click="loadFlags" :loading="flagLoading"><el-icon><Refresh /></el-icon> 刷新</el-button>
          </div>

          <el-table :data="flags" v-loading="flagLoading" style="width: 100%">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column label="内容类型" width="90">
              <template #default="{ row }">
                <el-tag size="small" effect="plain">{{ contentTypeLabel(row.content_type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="内容ID" width="70">
              <template #default="{ row }">
                <span class="mono">{{ row.content_id }}</span>
              </template>
            </el-table-column>
            <el-table-column label="作者ID" width="70">
              <template #default="{ row }">
                <span class="mono">{{ row.user_id }}</span>
              </template>
            </el-table-column>
            <el-table-column label="标题 / 片段" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">
                <div v-if="row.content_title" class="flag-title">{{ row.content_title }}</div>
                <div class="flag-snippet">{{ row.snippet || '—' }}</div>
              </template>
            </el-table-column>
            <el-table-column label="命中违禁词" min-width="150">
              <template #default="{ row }">
                <div style="display: flex; flex-wrap: wrap; gap: 4px">
                  <el-tag v-for="(m, i) in (row.matched_list || [])" :key="i" size="small" type="danger">{{ m }}</el-tag>
                  <span v-if="!(row.matched_list || []).length" class="muted">—</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="flagStatusTag(row.status)">{{ flagStatusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="时间" width="165">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="220" fixed="right">
              <template #default="{ row }">
                <template v-if="row.status === 'pending'">
                  <el-button size="small" type="danger" @click="handleFlag(row, 'confirmed', true)">确认并删除</el-button>
                  <el-button size="small" type="primary" plain @click="handleFlag(row, 'confirmed', false)">确认违规</el-button>
                  <el-button size="small" @click="handleFlag(row, 'dismissed')">放行</el-button>
                </template>
                <span v-else class="muted">{{ row.note || '已处理' }}</span>
              </template>
            </el-table-column>
            <template #empty><EmptyState icon="🔍" text="暂无标记记录" /></template>
          </el-table>

          <div style="margin-top: 12px; display: flex; justify-content: flex-end">
            <el-pagination
              layout="prev, pager, next, total"
              :total="flagTotal"
              :page-size="flagPageSize"
              :current-page="flagPage"
              @current-change="flagPage = $event; loadFlags()"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 新增/编辑规则弹窗 -->
    <el-dialog v-model="showDialog" :title="editingId ? '编辑违禁词' : '新增违禁词'" width="600px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="规则名称" required>
          <el-input v-model="form.name" placeholder="如：涉赌广告词" maxlength="128" />
        </el-form-item>
        <el-form-item label="匹配方式">
          <el-radio-group v-model="form.match_type">
            <el-radio-button value="literal">关键词（字面匹配）</el-radio-button>
            <el-radio-button value="regex">正则表达式</el-radio-button>
          </el-radio-group>
          <div class="hint-text">「关键词」= 填词即用，自动转义正则元字符；「正则」= Go RE2 语法，支持 (?i)、(?m)、\p{Han} 等。</div>
        </el-form-item>
        <el-form-item :label="form.match_type === 'regex' ? '正则表达式' : '违禁词'" required>
          <el-input v-model="form.pattern" type="textarea" :rows="3" :placeholder="form.match_type === 'regex' ? '如：(?i)casino|赌博' : '如：违禁词' " />
        </el-form-item>
        <el-form-item label="动作">
          <el-radio-group v-model="form.action">
            <el-radio-button value="mark">标记（待审）</el-radio-button>
            <el-radio-button value="delete">删除</el-radio-button>
            <el-radio-button value="replace">替换</el-radio-button>
          </el-radio-group>
          <div class="hint-text">「标记」= 内容正常发布但进入待审列表，由管理员处理；「删除」= 删除匹配内容；「替换」= 替换为下方文本。</div>
        </el-form-item>
        <el-form-item v-if="form.action === 'replace'" label="替换文本" required>
          <el-input v-model="form.replacement" placeholder="替换为指定文本（如：***）" maxlength="256" />
        </el-form-item>
        <el-form-item label="适用范围">
          <el-checkbox-group v-model="form.scopeList">
            <el-checkbox value="chapter">章节</el-checkbox>
            <el-checkbox value="comment">评论</el-checkbox>
            <el-checkbox value="forum">论坛</el-checkbox>
            <el-checkbox value="blog">博客</el-checkbox>
          </el-checkbox-group>
          <div class="hint-text">不选或全选 = 全部内容生效。</div>
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
        <el-button type="primary" :loading="submitting" @click="submitWord">
          {{ editingId ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 测试规则弹窗 -->
    <el-dialog v-model="showTest" title="测试违禁词规则" width="640px">
      <el-form label-width="90px">
        <el-form-item label="匹配方式">
          <el-radio-group v-model="testForm.match_type">
            <el-radio-button value="literal">关键词</el-radio-button>
            <el-radio-button value="regex">正则</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="testForm.match_type === 'regex' ? '正则表达式' : '违禁词'" required>
          <el-input v-model="testForm.pattern" type="textarea" :rows="2" placeholder="输入要测试的关键词或正则" />
        </el-form-item>
        <el-form-item label="动作">
          <el-radio-group v-model="testForm.action">
            <el-radio-button value="mark">标记</el-radio-button>
            <el-radio-button value="delete">删除</el-radio-button>
            <el-radio-button value="replace">替换</el-radio-button>
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
import { Plus, Search, Refresh, Warning } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const activeTab = ref('rules');

// ═══════════ 规则管理 ═══════════
const words = ref<any[]>([]);
const loading = ref(false);
const showDialog = ref(false);
const editingId = ref<number | null>(null);
const submitting = ref(false);
const form = reactive({ name: '', match_type: 'literal', pattern: '', action: 'mark', replacement: '', scopeList: [] as string[], enabled: true, description: '' });

// ═══════════ 标记待审 ═══════════
const flags = ref<any[]>([]);
const flagLoading = ref(false);
const flagStatus = ref('pending');
const flagPage = ref(1);
const flagPageSize = 20;
const flagTotal = ref(0);
const pendingCount = ref(0);

// ═══════════ 测试 ═══════════
const showTest = ref(false);
const testing = ref(false);
const testForm = reactive({ match_type: 'literal', pattern: '', action: 'mark', replacement: '', text: '' });
const testResult = ref<any>(null);

async function loadWords() {
  loading.value = true;
  try {
    const res = await adminApi.getBadWords();
    words.value = res.data.data || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function loadFlags() {
  flagLoading.value = true;
  try {
    const res = await adminApi.getBadWordFlags({ status: flagStatus.value, page: flagPage.value, page_size: flagPageSize });
    const d = res.data.data || {};
    flags.value = d.items || [];
    flagTotal.value = d.total || 0;
    pendingCount.value = d.pending_count ?? 0;
  } catch { /* ignore */ }
  flagLoading.value = false;
}

async function loadPendingCount() {
  try {
    const res = await adminApi.getBadWordFlags({ status: 'pending', page: 1, page_size: 1 });
    pendingCount.value = (res.data.data || {}).total || 0;
  } catch { /* ignore */ }
}

function switchToFlags() {
  activeTab.value = 'flags';
  flagStatus.value = 'pending';
  loadFlags();
}

function scopeToForm(row: any) {
  if (!row.scope || row.scope === 'all') return ['chapter', 'comment', 'forum', 'blog'];
  return (row.scope || '').split(',').filter(Boolean);
}
function formToScope(list: string[]) {
  if (!list.length || list.length === 4) return 'all';
  return list.join(',');
}

function openAdd() {
  editingId.value = null;
  form.name = '';
  form.match_type = 'literal';
  form.pattern = '';
  form.action = 'mark';
  form.replacement = '';
  form.scopeList = [];
  form.enabled = true;
  form.description = '';
  showDialog.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  form.name = row.name || '';
  form.match_type = row.match_type || 'literal';
  form.pattern = row.pattern || '';
  form.action = row.action || 'mark';
  form.replacement = row.replacement || '';
  form.scopeList = scopeToForm(row);
  form.enabled = !!row.enabled;
  form.description = row.description || '';
  showDialog.value = true;
}

async function submitWord() {
  if (!form.name.trim()) { ElMessage.warning('请填写规则名称'); return; }
  if (!form.pattern.trim()) { ElMessage.warning('请填写违禁词或正则表达式'); return; }
  if (form.action === 'replace' && !form.replacement.trim()) { ElMessage.warning('替换动作必须填写替换文本'); return; }
  submitting.value = true;
  const payload = {
    name: form.name,
    match_type: form.match_type,
    pattern: form.pattern,
    action: form.action,
    replacement: form.replacement,
    scope: formToScope(form.scopeList),
    enabled: form.enabled,
    description: form.description,
  };
  try {
    if (editingId.value) {
      await adminApi.updateBadWord(editingId.value, payload);
      ElMessage.success('规则已更新');
    } else {
      await adminApi.createBadWord(payload);
      ElMessage.success('规则已创建，已即时生效');
    }
    showDialog.value = false;
    await loadWords();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  submitting.value = false;
}

async function toggleEnabled(row: any, v: boolean) {
  try {
    await adminApi.updateBadWord(row.id, { enabled: v });
    row.enabled = v;
    ElMessage.success(v ? '已启用' : '已禁用');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
    await loadWords();
  }
}

async function removeWord(row: any) {
  try {
    await ElMessageBox.confirm(`确定要删除违禁词「${row.name}」吗？`, '确认删除', { type: 'warning' });
    await adminApi.deleteBadWord(row.id);
    ElMessage.success('已删除');
    await loadWords();
  } catch { /* cancelled */ }
}

function openTest(row?: any) {
  testForm.match_type = row?.match_type || 'literal';
  testForm.pattern = row?.pattern || '';
  testForm.action = row?.action || 'mark';
  testForm.replacement = row?.replacement || '';
  testForm.text = '';
  testResult.value = null;
  showTest.value = true;
}

async function runTest() {
  if (!testForm.pattern.trim()) { ElMessage.warning('请输入违禁词或正则表达式'); return; }
  if (!testForm.text) { ElMessage.warning('请输入测试文本'); return; }
  testing.value = true;
  try {
    const res = await adminApi.testBadWord({ ...testForm });
    testResult.value = res.data.data;
  } catch (e: any) {
    testResult.value = null;
    ElMessage.error(e?.response?.data?.message || '测试失败');
  }
  testing.value = false;
}

async function handleFlag(row: any, status: string, deleteContent = false) {
  const label = status === 'confirmed' ? (deleteContent ? '确认违规并删除该内容？删除将软删除并留证，可追溯。' : '确认该内容违规？') : '确认该内容为误报并放行？';
  try {
    await ElMessageBox.confirm(label, status === 'dismissed' ? '放行' : '确认违规', { type: 'warning' });
    await adminApi.handleBadWordFlag(row.id, { status, delete_content: deleteContent });
    ElMessage.success(deleteContent ? '已确认违规并删除内容' : '已处理');
    await loadFlags();
    await loadPendingCount();
  } catch { /* cancelled */ }
}

// ═══════════ 展示辅助 ═══════════
function actionTag(a?: string) {
  const map: Record<string, string> = { mark: 'warning', replace: 'warning', delete: 'danger' };
  return (map[a || 'mark'] as any) || 'info';
}
function actionLabel(a?: string) {
  const map: Record<string, string> = { mark: '标记', replace: '替换', delete: '删除' };
  return map[a || 'mark'] || a || '标记';
}
function scopeLabel(s: string) {
  const map: Record<string, string> = { chapter: '章节', comment: '评论', forum: '论坛', blog: '博客' };
  return map[s] || s;
}
function contentTypeLabel(t: string) {
  const map: Record<string, string> = { comment: '评论', chapter: '章节', thread: '帖子', post: '回复', blog: '博客' };
  return map[t] || t;
}
function flagStatusTag(s: string) {
  const map: Record<string, string> = { pending: 'warning', confirmed: 'danger', dismissed: 'success' };
  return (map[s] as any) || 'info';
}
function flagStatusLabel(s: string) {
  const map: Record<string, string> = { pending: '待审', confirmed: '已确认违规', dismissed: '已放行' };
  return map[s] || s;
}
function formatTime(t?: string) {
  if (!t) return '—';
  return t.replace('T', ' ').slice(0, 16);
}

onMounted(() => {
  loadWords();
  loadPendingCount();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1200px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1200px; }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); line-height: 1.6; }
.muted { color: var(--text-light, #999); }
.mono { font-family: 'Consolas', 'Monaco', monospace; }
.rule-pattern { font-family: 'Consolas', 'Monaco', monospace; font-size: 0.85rem; color: var(--el-color-danger); background: var(--el-color-danger-light-9); padding: 2px 6px; border-radius: 3px; }
.rule-replace { color: var(--el-color-primary); font-family: 'Consolas', 'Monaco', monospace; font-size: 0.85rem; }
.flag-title { font-weight: 600; color: var(--text-color, #333); margin-bottom: 2px; }
.flag-snippet { font-size: 0.82rem; color: var(--text-light, #666); line-height: 1.5; max-height: 48px; overflow: hidden; }
.test-result { padding: 0 12px; }
.test-line { display: flex; align-items: flex-start; gap: 8px; margin-bottom: 10px; }
.test-label { font-weight: 600; color: var(--text-color, #333); flex-shrink: 0; }
.test-matches { display: flex; flex-wrap: wrap; gap: 6px; }
.test-output { flex: 1; margin: 0; padding: 10px 12px; background: var(--bg-soft); border: 1px solid var(--border-color); border-radius: 6px; font-size: 0.85rem; white-space: pre-wrap; word-break: break-all; font-family: 'Consolas', 'Monaco', monospace; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
[data-theme="dark"] .test-label { color: #e2e8f0; }
[data-theme="dark"] .flag-title { color: #e2e8f0; }
[data-theme="dark"] .test-output { background: #1e293b; border-color: #334155; color: #cbd5e1; }
[data-theme="dark"] .rule-pattern { background: #2d1b22; color: #f472b6; }
</style>
