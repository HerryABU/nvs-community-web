<template>
  <div class="admin-page">
    <AdminHeader title="AI 智能体" subtitle="对话预设（人设）：用户点击卡片即带系统提示词开会话" :icon="Avatar" />
    <div class="admin-page-body">
      <p class="hint-text">
        智能体是「系统提示词预设」：用户在 AI 工作台点选后，新会话将以其人设与开场白开始。
        内置种子可修改/禁用；删除不可恢复（进行中的会话不受影响，沿用会话快照）。
      </p>

      <div class="table-toolbar">
        <span class="muted small">共 {{ agents.length }} 个智能体（启用 {{ enabledCount }} 个）</span>
        <el-button type="primary" :icon="Plus" @click="openCreate">添加智能体</el-button>
      </div>

      <el-table :data="agents" v-loading="loading" style="width:100%">
        <el-table-column label="图标" width="70">
          <template #default="{ row }"><span class="agent-emoji">{{ row.emoji || '🤖' }}</span></template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="110" show-overflow-tooltip />
        <el-table-column prop="description" label="简介" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.description || '—' }}</template>
        </el-table-column>
        <el-table-column label="系统提示词" min-width="220" show-overflow-tooltip>
          <template #default="{ row }"><span class="prompt-cell">{{ row.system_prompt }}</span></template>
        </el-table-column>
        <el-table-column label="来源" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.builtin ? 'info' : 'success'">{{ row.builtin ? '内置' : '自定义' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" size="small" @change="toggleEnabled(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="sort_order" label="排序" width="70" />
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="removeAgent(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><EmptyState icon="🎭" text="暂无智能体" /></template>
      </el-table>
    </div>

    <!-- ============ 创建 / 编辑对话框 ============ -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑智能体' : '添加智能体'" width="560px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：写作教练" maxlength="64" />
        </el-form-item>
        <el-form-item label="图标">
          <el-input v-model="form.emoji" placeholder="一个 emoji，如 ✍️" maxlength="8" style="width:120px" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.description" placeholder="一句话说明这个智能体做什么" maxlength="256" />
        </el-form-item>
        <el-form-item label="系统提示词" required>
          <el-input
            v-model="form.system_prompt"
            type="textarea"
            :rows="6"
            resize="none"
            placeholder="定义智能体的人设与行为准则，将作为每次对话的 system 消息"
          />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" placeholder="如：写作 / 编程 / 绘画 / 翻译（用于市场分组）" maxlength="32" />
        </el-form-item>
        <el-form-item label="开场白">
          <el-input v-model="form.greeting" placeholder="用户进入会话时看到的第一句话" maxlength="256" />
        </el-form-item>
        <el-form-item label="上游类型">
          <el-radio-group v-model="form.upstream">
            <el-radio-button value="">平台模型池</el-radio-button>
            <el-radio-button value="coze">扣子 Coze</el-radio-button>
          </el-radio-group>
          <div class="muted small">选「扣子」后由 Coze 官方智能体接管（系统提示词失效）</div>
        </el-form-item>
        <template v-if="form.upstream === 'coze'">
          <el-form-item label="Bot ID" required>
            <el-input v-model="form.bot_id" placeholder="Coze 智能体 ID（bot_id）" maxlength="64" />
          </el-form-item>
          <el-form-item label="API 域名">
            <el-select v-model="form.coze_base" clearable placeholder="默认 https://api.coze.cn" style="width:100%">
              <el-option label="国内 https://api.coze.cn" value="https://api.coze.cn" />
              <el-option label="国际 https://api.coze.com" value="https://api.coze.com" />
            </el-select>
          </el-form-item>
          <el-form-item label="访问令牌">
            <el-input
              v-model="form.coze_token"
              type="password"
              show-password
              :placeholder="editing ? '留空表示不更换（已加密存储）' : 'Coze 个人访问令牌 PAT'"
            />
            <div class="muted small">仅写不读：保存后前端无法再取回明文</div>
          </el-form-item>
        </template>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" :max="999" :step="1" controls-position="right" />
          <span class="muted small">小者在前</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveAgent">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Avatar, Plus } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { aiHubApi } from '@/api/aiHub';

const agents = ref<any[]>([]);
const loading = ref(false);
const enabledCount = computed(() => agents.value.filter((a) => a.enabled).length);

async function loadAgents() {
  loading.value = true;
  try {
    const res = await aiHubApi.adminAgents();
    if (res.data.code === 0) agents.value = res.data.data.list || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function toggleEnabled(row: any) {
  try {
    const res = await aiHubApi.adminUpdateAgent(row.id, { enabled: row.enabled });
    if (res.data.code !== 0) row.enabled = !row.enabled;
  } catch {
    row.enabled = !row.enabled;
    ElMessage.error('操作失败');
  }
}

// ---------- 创建 / 编辑 ----------
const dialogVisible = ref(false);
const saving = ref(false);
const editing = ref<any | null>(null);

function blankForm() {
  return {
    name: '', emoji: '', description: '', system_prompt: '', greeting: '',
    sort_order: 0, enabled: true,
    category: '', upstream: '', bot_id: '', coze_base: 'https://api.coze.cn', coze_token: '',
  };
}
const form = reactive(blankForm());

function openCreate() {
  editing.value = null;
  Object.assign(form, blankForm());
  dialogVisible.value = true;
}

function openEdit(row: any) {
  editing.value = row;
  Object.assign(form, {
    name: row.name, emoji: row.emoji, description: row.description,
    system_prompt: row.system_prompt, greeting: row.greeting,
    sort_order: row.sort_order, enabled: row.enabled,
    category: row.category || '', upstream: row.upstream || '',
    bot_id: row.bot_id || '', coze_base: row.coze_base || 'https://api.coze.cn', coze_token: '',
  });
  dialogVisible.value = true;
}

async function saveAgent() {
  if (!form.name.trim()) {
    ElMessage.warning('名称不能为空');
    return;
  }
  if (!form.system_prompt.trim()) {
    ElMessage.warning('系统提示词不能为空');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      name: form.name.trim(),
      emoji: form.emoji.trim(),
      description: form.description.trim(),
      system_prompt: form.system_prompt.trim(),
      greeting: form.greeting.trim(),
      sort_order: form.sort_order,
      enabled: form.enabled,
      category: form.category.trim(),
      upstream: form.upstream,
      bot_id: form.bot_id.trim(),
      coze_base: form.coze_base,
      ...(form.coze_token.trim() ? { coze_token: form.coze_token.trim() } : {}),
    };
    const res = editing.value
      ? await aiHubApi.adminUpdateAgent(editing.value.id, payload)
      : await aiHubApi.adminCreateAgent(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      dialogVisible.value = false;
      await loadAgents();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

async function removeAgent(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除智能体「${row.name}」？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiHubApi.adminDeleteAgent(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await loadAgents();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(loadAgents);
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; line-height: 1.6; }
.table-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.agent-emoji { font-size: 1.3rem; }
.prompt-cell {
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical;
  overflow: hidden; font-size: 0.8rem; color: var(--text-light, #666);
}
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; margin-left: 6px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
