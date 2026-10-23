<template>
  <div class="admin-page">
    <AdminHeader title="AI 模型池" subtitle="OpenAI 兼容上游配置：密钥加密存储、按量计费，编辑区 AI 助手的数据来源" :icon="MagicStick" />
    <div class="admin-page-body">
      <p class="hint-text">
        AI 为<b>默认关闭</b>的增值能力：开启后用户可在 AI 工作台使用对话 / 绘画 / 视频，作者还能在章节编辑器调用「续写 / 润色 / 扩写 / 摘要」。
        <b>对话</b>按 tokens 计费（每 1K 代币价，0 = 免费），<b>绘画 / 视频</b>按次计费（每次调用代币价）；真实用量以上游返回为准，价格快照随代币流水落库可对账。
        上游密钥经 AES-256-GCM 加密落库，<b>只写不读</b>（编辑时留空表示不更换）。
        每种类型至少配置一个启用模型，用户端才会显示对应入口。
      </p>

      <!-- ============ 总开关 ============ -->
      <div class="admin-panel">
        <div class="enable-row">
          <span>启用 AI 功能</span>
          <el-switch v-model="config.enabled" @change="saveConfig" />
        </div>
        <p class="muted small" style="margin:0">关闭后用户端 AI 入口立即隐藏，进行中的调用不受影响。</p>
      </div>

      <!-- ============ 模型列表 ============ -->
      <div class="admin-panel" style="margin-top:16px">
        <SectionHeader title="模型列表" subtitle="可配置多个上游，按优先级取用" :icon="'🤖'" gradient="purple" />
        <div class="table-toolbar">
          <span class="muted small">共 {{ models.length }} 个模型（启用 {{ enabledCount }} 个）</span>
          <el-button type="primary" :icon="Plus" @click="openCreate">添加模型</el-button>
        </div>
        <el-table :data="models" v-loading="loading" style="width:100%">
          <el-table-column prop="name" label="展示名" min-width="110" show-overflow-tooltip />
          <el-table-column label="类型" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="kindTag(row.kind)">{{ kindLabel(row.kind) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="model_id" label="上游模型 ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="base_url" label="上游地址" min-width="180" show-overflow-tooltip />
          <el-table-column label="密钥" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.has_api_key ? 'success' : 'info'">
                {{ row.has_api_key ? '已配置' : '未配置' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="价格" min-width="150">
            <template #default="{ row }">
              <span v-if="row.kind === 'chat'" class="mono">{{ row.price_in || 0 }} / {{ row.price_out || 0 }}</span>
              <span v-else class="mono">{{ row.price_call || 0 }}</span>
              <span class="muted small">{{ row.kind === 'chat' ? '每 1K tokens 入/出' : '每次调用' }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="max_tokens" label="默认输出上限" width="120" />
          <el-table-column label="启用" width="80">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" size="small" @change="toggleEnabled(row)" />
            </template>
          </el-table-column>
          <el-table-column prop="priority" label="优先级" width="80" />
          <el-table-column label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button size="small" text type="danger" @click="removeModel(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><EmptyState icon="🤖" text="暂无模型，点击右上角添加" /></template>
        </el-table>
      </div>
    </div>

    <!-- ============ 创建 / 编辑对话框 ============ -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑模型' : '添加模型'" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="类型" required>
          <el-radio-group v-model="form.kind">
            <el-radio-button value="chat">对话</el-radio-button>
            <el-radio-button value="image">绘画</el-radio-button>
            <el-radio-button value="video">视频</el-radio-button>
          </el-radio-group>
          <div class="kind-hint">{{ kindHint(form.kind) }}</div>
        </el-form-item>
        <el-form-item label="展示名" required>
          <el-input v-model="form.name" placeholder="作者可见的名称，如：默认模型" maxlength="64" />
        </el-form-item>
        <el-form-item label="上游模型 ID" required>
          <el-input v-model="form.model_id" placeholder="如 gpt-4o-mini / deepseek-chat" maxlength="128" />
        </el-form-item>
        <el-form-item label="上游地址" required>
          <el-input v-model="form.base_url" placeholder="如 https://api.openai.com/v1（不含 /chat/completions）" />
        </el-form-item>
        <el-form-item label="API Key">
          <el-input
            v-model="form.api_key"
            type="password"
            show-password
            :placeholder="editing ? '留空表示不更换（当前已加密存储）' : '上游密钥，加密后存储'"
          />
        </el-form-item>
        <template v-if="form.kind === 'chat'">
          <el-form-item label="输入价格">
            <el-input-number v-model="form.price_in" :min="0" :max="1000000" :step="1" controls-position="right" />
            <span class="muted small">每 1K 输入 tokens 代币数，0 = 免费</span>
          </el-form-item>
          <el-form-item label="输出价格">
            <el-input-number v-model="form.price_out" :min="0" :max="1000000" :step="1" controls-position="right" />
            <span class="muted small">每 1K 输出 tokens 代币数，0 = 免费</span>
          </el-form-item>
          <el-form-item label="默认输出上限">
            <el-input-number v-model="form.max_tokens" :min="256" :max="8192" :step="256" controls-position="right" />
            <span class="muted small">tokens，单次调用上限 8192</span>
          </el-form-item>
          <el-form-item label="温度">
            <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" controls-position="right" />
          </el-form-item>
        </template>
        <el-form-item v-else label="单次价格">
          <el-input-number v-model="form.price_call" :min="0" :max="1000000" :step="1" controls-position="right" />
          <span class="muted small">每次生成收取代币数，0 = 免费（{{ form.kind === 'image' ? '绘画' : '视频' }}按次计费）</span>
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="form.priority" :min="0" :max="999" :step="1" controls-position="right" />
          <span class="muted small">大者优先（备用池预留）</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.note" placeholder="选填" maxlength="256" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveModel">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { MagicStick, Plus } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { aiApi, type AdminAiModel } from '@/api/ai';

// ---------- 总开关 ----------
const config = reactive<{ enabled: boolean }>({ enabled: false });

async function loadConfig() {
  try {
    const res = await aiApi.getConfig();
    if (res.data.code === 0) config.enabled = res.data.data.enabled;
  } catch { /* ignore */ }
}

async function saveConfig(v: boolean) {
  try {
    const res = await aiApi.setConfig({ enabled: v });
    if (res.data.code === 0) ElMessage.success(v ? 'AI 已开启' : 'AI 已关闭');
    else config.enabled = !v; // 失败回滚开关
  } catch {
    config.enabled = !v;
    ElMessage.error('保存失败');
  }
}

// ---------- 模型列表 ----------
const models = ref<AdminAiModel[]>([]);
const loading = ref(false);
const enabledCount = computed(() => models.value.filter((m) => m.enabled).length);

async function loadModels() {
  loading.value = true;
  try {
    const res = await aiApi.listModels();
    if (res.data.code === 0) models.value = res.data.data.list || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function toggleEnabled(row: AdminAiModel) {
  try {
    const res = await aiApi.updateModel(row.id, { enabled: row.enabled } as any);
    if (res.data.code !== 0) row.enabled = !row.enabled;
  } catch {
    row.enabled = !row.enabled;
    ElMessage.error('操作失败');
  }
}

// ---------- 创建 / 编辑 ----------
const dialogVisible = ref(false);
const saving = ref(false);
const editing = ref<AdminAiModel | null>(null);

function blankForm() {
  return {
    name: '', kind: 'chat', model_id: '', base_url: '', api_key: '',
    price_in: 0, price_out: 0, price_call: 0, max_tokens: 2048,
    temperature: 0.7, priority: 0, enabled: true, note: '',
  };
}
const form = reactive(blankForm());

function kindLabel(kind: string) {
  const map: Record<string, string> = { chat: '对话', image: '绘画', video: '视频' };
  return map[kind] || kind;
}
function kindTag(kind: string) {
  const map: Record<string, string> = { chat: 'primary', image: 'success', video: 'warning' };
  return (map[kind] as any) || 'info';
}
function kindHint(kind: string) {
  const map: Record<string, string> = {
    chat: '按 tokens 计费：流式对话 / 编辑区 AI 助手（/chat/completions）',
    image: '按次计费：同步出图（/images/generations），成功才扣款',
    video: '按次计费：异步任务（/videos 提交 + 查询），提交即扣款、失败退款',
  };
  return map[kind] || '';
}

function openCreate() {
  editing.value = null;
  Object.assign(form, blankForm());
  dialogVisible.value = true;
}

function openEdit(row: AdminAiModel) {
  editing.value = row;
  Object.assign(form, {
    name: row.name, kind: row.kind || 'chat', model_id: row.model_id, base_url: row.base_url, api_key: '',
    price_in: row.price_in, price_out: row.price_out, price_call: row.price_call || 0,
    max_tokens: row.max_tokens,
    temperature: row.temperature, priority: row.priority, enabled: row.enabled, note: row.note,
  });
  dialogVisible.value = true;
}

async function saveModel() {
  if (!form.name.trim() || !form.model_id.trim() || !form.base_url.trim()) {
    ElMessage.warning('展示名、上游模型 ID、上游地址均为必填');
    return;
  }
  saving.value = true;
  try {
    const payload: any = {
      name: form.name.trim(),
      kind: form.kind,
      model_id: form.model_id.trim(),
      base_url: form.base_url.trim(),
      price_in: form.price_in,
      price_out: form.price_out,
      price_call: form.price_call,
      max_tokens: form.max_tokens,
      temperature: form.temperature,
      priority: form.priority,
      enabled: form.enabled,
      note: form.note.trim(),
    };
    if (form.api_key.trim()) payload.api_key = form.api_key.trim();
    const res = editing.value
      ? await aiApi.updateModel(editing.value.id, payload)
      : await aiApi.createModel(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      dialogVisible.value = false;
      await loadModels();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

async function removeModel(row: AdminAiModel) {
  try {
    await ElMessageBox.confirm(`确定删除模型「${row.name}」？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiApi.deleteModel(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await loadModels();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(() => { loadConfig(); loadModels(); });
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; line-height: 1.6; }
.enable-row { display: flex; align-items: center; justify-content: space-between; padding: 8px 4px; font-weight: 600; }
.table-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; margin-left: 6px; }
.mono { font-family: Consolas, 'Courier New', monospace; }
.kind-hint { font-size: 0.75rem; color: var(--text-light, #999); margin-top: 2px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
