<template>
  <div class="page-container oc-page" v-loading="loading">
    <div class="oc-head">
      <h1 class="oc-title">OC 模式</h1>
      <el-tag v-if="setting?.enabled" size="small" type="success">{{ setting.mode === 'ai_only' ? '纯 AI 群' : '人机混合群' }}</el-tag>
      <el-tag v-else size="small" type="info">未启用</el-tag>
      <div class="oc-head-actions">
        <el-button size="small" @click="$router.push(`/group/${groupId}`)">返回群组</el-button>
      </div>
    </div>
    <p class="oc-sub">
      角色卡 + 世界书 + AI 角色互聊；产物可一键发布为话本小说。玩法层，群组治理照旧适用。
    </p>

    <el-tabs v-model="tab" class="oc-tabs">
      <el-tab-pane label="玩法设置" name="setting">
        <el-alert v-if="!canManage" type="info" :closable="false" title="仅群主/管理员可修改设置；你可以创建角色卡与参与对话流。" />
        <template v-else>
          <el-form label-width="120px" class="oc-form">
            <el-form-item label="启用 OC 模式"><el-switch v-model="form.enabled" /></el-form-item>
            <el-form-item label="群组类型">
              <el-radio-group v-model="form.mode">
                <el-radio value="mixed">混合（人 + AI 角色）</el-radio>
                <el-radio value="ai_only">纯 AI（用户当导演）</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="审核等级">
              <el-radio-group v-model="form.audit_level">
                <el-radio value="strict">严格（命中即停流待复核）</el-radio>
                <el-radio value="loose">宽松（只审公开与被举报）</el-radio>
                <el-radio value="redteam">红队（隔离环境，只记录不拦截）</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="审核时机">
              <el-radio-group v-model="form.audit_timing">
                <el-radio value="pre">先审后发</el-radio>
                <el-radio value="post">先发后审</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="审核窗口">
              <el-input-number v-model="form.window_size" :min="2" :max="50" />
              <span class="oc-tip" style="margin-left:8px">条发言一起审（多条一起审，避免逐条审的成本与上下文割裂）</span>
            </el-form-item>
            <el-form-item label="群日预算">
              <el-input-number v-model="form.daily_budget" :min="0" :step="1000" />
              <span class="oc-tip" style="margin-left:8px">当日已用 {{ dailyUsed }}，超额自动暂停对话流</span>
            </el-form-item>
            <el-form-item label="单流预算">
              <el-input-number v-model="form.flow_budget" :min="0" :step="500" />
              <span class="oc-tip" style="margin-left:8px">0 = 不限</span>
            </el-form-item>
            <el-form-item label="最大轮次">
              <el-input-number v-model="form.max_turns" :min="1" :max="500" />
              <span class="oc-tip" style="margin-left:8px">防无限循环</span>
            </el-form-item>
            <el-form-item v-if="form.audit_level === 'redteam'" label="红队授权">
              <el-switch v-model="form.redteam_enabled" />
              <el-input v-model="form.redteam_note" placeholder="测试范围 / 责任人（必填，留痕）" style="margin-left:12px;width:340px" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saving" @click="saveSetting">保存设置</el-button>
            </el-form-item>
          </el-form>
          <el-alert type="warning" :closable="false" class="oc-warn"
            title="红队模式必须隔离：不进社区内容流、不参与搜索/推荐、不对外分享；产出仅入安全审计库。" />
        </template>
      </el-tab-pane>

      <el-tab-pane label="角色卡" name="characters">
        <CharacterManager :group-id="groupId" :can-bind="canManage" @bound="load" />
      </el-tab-pane>

      <el-tab-pane label="世界书" name="books">
        <WorldBookManager :group-id="groupId" />
      </el-tab-pane>

      <el-tab-pane label="AI 角色" name="agents">
        <div class="oc-agents">
          <el-table :data="agents" size="small">
            <el-table-column label="角色" min-width="140">
              <template #default="{ row }">{{ row.character_name || `角色#${row.character_id}` }}</template>
            </el-table-column>
            <el-table-column label="模型" width="160">
              <template #default="{ row }">{{ row.model_name || '池子默认' }}</template>
            </el-table-column>
            <el-table-column label="发言策略" min-width="160">
              <template #default="{ row }">{{ row.strategy || '—' }}</template>
            </el-table-column>
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-switch :model-value="row.enabled" :disabled="!canManage" @change="(v:boolean) => toggleAgent(row, v)" />
              </template>
            </el-table-column>
            <el-table-column v-if="canManage" label="操作" width="100">
              <template #default="{ row }">
                <el-popconfirm title="从本群移除该 AI 角色？" @confirm="removeAgent(row)">
                  <template #reference><el-button size="small" text type="danger">移除</el-button></template>
                </el-popconfirm>
              </template>
            </el-table-column>
            <template #empty><el-empty description="还没有群内 AI 角色（在「角色卡」里点『加入本群 AI』）" :image-size="60" /></template>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane label="对话流" name="flow">
        <FlowPanel :group-id="groupId" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage } from 'element-plus';
import { ocApi, type OcSetting, type OcAgent } from '@/api/oc';
import CharacterManager from '@/components/oc/CharacterManager.vue';
import WorldBookManager from '@/components/oc/WorldBookManager.vue';
import FlowPanel from '@/components/oc/FlowPanel.vue';

const route = useRoute();
const groupId = computed(() => Number(route.params.id) || 0);

const loading = ref(false);
const saving = ref(false);
const tab = ref('setting');
const setting = ref<OcSetting | null>(null);
const agents = ref<OcAgent[]>([]);
const canManage = ref(false);
const dailyUsed = ref(0);

const form = reactive<Partial<OcSetting>>({
  enabled: false, mode: 'mixed', audit_level: 'strict', audit_timing: 'post',
  window_size: 5, daily_budget: 20000, flow_budget: 0, max_turns: 40,
  redteam_enabled: false, redteam_note: '',
});

async function load() {
  if (!groupId.value) return;
  loading.value = true;
  try {
    const res = await ocApi.getSetting(groupId.value);
    const d = res.data.data;
    setting.value = d?.setting || null;
    agents.value = d?.agents || [];
    canManage.value = !!d?.can_manage;
    dailyUsed.value = d?.daily_used || 0;
    if (d?.setting) Object.assign(form, d.setting);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载 OC 配置失败');
  } finally {
    loading.value = false;
  }
}

async function saveSetting() {
  saving.value = true;
  try {
    await ocApi.saveSetting(groupId.value, form);
    ElMessage.success('已保存');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function toggleAgent(row: OcAgent, enabled: boolean) {
  try {
    await ocApi.updateAgent(groupId.value, row.id, { enabled });
    row.enabled = enabled;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function removeAgent(row: OcAgent) {
  try {
    await ocApi.removeAgent(groupId.value, row.id);
    ElMessage.success('已移除');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '移除失败');
  }
}

onMounted(load);
</script>

<style scoped>
.oc-page { max-width: 1120px; margin: 0 auto; padding: 20px 16px 48px; }
.oc-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.oc-title { font-size: 1.5rem; margin: 0; }
.oc-head-actions { margin-left: auto; }
.oc-sub { color: var(--text-muted); font-size: 0.85rem; margin: 6px 0 16px; }
.oc-form { max-width: 760px; }
.oc-tip { font-size: 0.75rem; color: var(--text-muted); }
.oc-warn { margin-top: 12px; max-width: 760px; }
.oc-agents { max-width: 900px; }
</style>
