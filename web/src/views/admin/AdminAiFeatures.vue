<template>
  <div class="page">
    <AdminHeader title="AI 功能配置" desc="平台级开关：能力开放范围、上下文长度、每日限额；旁边维护模型厂商字典">
      <template #actions>
        <el-button size="small" :loading="saving" type="primary" @click="save">保存</el-button>
      </template>
    </AdminHeader>

    <div class="cfg-grid">
      <!-- 能力开关 -->
      <el-card shadow="never" class="cfg-card">
        <SectionHeader title="能力开关" desc="关闭后对应入口在前台直接拒绝（不会悄悄降级）" />
        <div class="cfg-row">
          <div class="cfg-item">
            <div class="cfg-name">自备密钥（BYOK）</div>
            <div class="cfg-desc">允许用户用自己在钱包里填的上游密钥，不消耗平台代币</div>
          </div>
          <el-switch v-model="flags.allow_byok" />
        </div>
        <div class="cfg-row">
          <div class="cfg-item">
            <div class="cfg-name">混图（图生图）</div>
            <div class="cfg-desc">上传参考图再创作，依赖上游支持 images/edits 或 inlineData</div>
          </div>
          <el-switch v-model="flags.allow_i2i" />
        </div>
        <div class="cfg-row">
          <div class="cfg-item">
            <div class="cfg-name">换脸</div>
            <div class="cfg-desc">默认关闭（涉及肖像权风险），开启前请评估合规</div>
          </div>
          <el-switch v-model="flags.allow_faceswap" />
        </div>
        <div class="cfg-row">
          <div class="cfg-item">
            <div class="cfg-name">灵感广场公开</div>
            <div class="cfg-desc">关闭后作品流仅登录用户可见</div>
          </div>
          <el-switch v-model="flags.public_gallery" />
        </div>
      </el-card>

      <!-- 限额 -->
      <el-card shadow="never" class="cfg-card">
        <SectionHeader title="上下文与限额" desc="控制成本与滥用" />
        <div class="cfg-row">
          <div class="cfg-item">
            <div class="cfg-name">对话上下文条数</div>
            <div class="cfg-desc">每次请求携带最近 N 条历史（1–60，越大越贵）</div>
          </div>
          <el-input-number v-model="flags.context_msgs" :min="1" :max="60" size="small" controls-position="right" />
        </div>
        <div class="cfg-row">
          <div class="cfg-item">
            <div class="cfg-name">每人每日对话上限</div>
            <div class="cfg-desc">-1 不限 / 0 关闭对话 / N 次（按调用流水统计）</div>
          </div>
          <el-input-number v-model="flags.daily_chat_limit" :min="-1" :max="100000" size="small" controls-position="right" />
        </div>
      </el-card>
    </div>

    <!-- 厂商字典 -->
    <el-card shadow="never" class="cfg-card">
      <SectionHeader title="模型厂商" desc="前台按厂商筛选模型；改名会同步更新已归属的模型">
        <template #actions>
          <el-button size="small" @click="openVendor(null)">
            <el-icon><Plus /></el-icon> 新增厂商
          </el-button>
        </template>
      </SectionHeader>
      <el-table :data="vendors" size="small" v-loading="vendorLoading">
        <el-table-column prop="name" label="厂商" min-width="160" />
        <el-table-column prop="sort_order" label="排序" width="90" />
        <el-table-column label="启用" width="90">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" size="small" @change="toggleVendor(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="models" label="关联模型" width="100" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button text type="primary" size="small" @click="openVendor(row)">编辑</el-button>
            <el-button text type="danger" size="small" @click="removeVendor(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><EmptyState icon="🏷️" text="暂无厂商，点击右上角新增" /></template>
      </el-table>
    </el-card>

    <el-dialog v-model="vendorVisible" :title="editingVendor ? '编辑厂商' : '新增厂商'" width="420px">
      <el-form label-width="72px">
        <el-form-item label="名称" required>
          <el-input v-model="vendorForm.name" maxlength="32" placeholder="如 OpenAI / Anthropic / 字节跳动" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="vendorForm.sort_order" :min="0" :max="999" controls-position="right" />
          <span class="muted small">小者在前</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="vendorForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="vendorVisible = false">取消</el-button>
        <el-button type="primary" :loading="vendorSaving" @click="saveVendor">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { aiApi, type AiFeatureFlags, type AiVendor } from '@/api/ai';

const saving = ref(false);
const flags = reactive<AiFeatureFlags>({
  allow_byok: true,
  allow_i2i: true,
  allow_faceswap: false,
  context_msgs: 20,
  daily_chat_limit: -1,
  public_gallery: true,
});

async function loadFlags() {
  try {
    const res = await aiApi.getAiFeatures();
    if (res.data.code === 0) Object.assign(flags, res.data.data);
  } catch { /* ignore */ }
}

async function save() {
  saving.value = true;
  try {
    const res = await aiApi.setAiFeatures({ ...flags });
    if (res.data.code === 0) ElMessage.success('已保存（立即生效）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

// ---------- 厂商字典 ----------
const vendors = ref<AiVendor[]>([]);
const vendorLoading = ref(false);

async function loadVendors() {
  vendorLoading.value = true;
  try {
    const res = await aiApi.listAiVendors();
    if (res.data.code === 0) vendors.value = res.data.data.list || [];
  } catch { /* ignore */ }
  vendorLoading.value = false;
}

const vendorVisible = ref(false);
const vendorSaving = ref(false);
const editingVendor = ref<AiVendor | null>(null);
const vendorForm = reactive({ name: '', sort_order: 0, enabled: true });

function openVendor(row: AiVendor | null) {
  editingVendor.value = row;
  vendorForm.name = row?.name || '';
  vendorForm.sort_order = row?.sort_order ?? 0;
  vendorForm.enabled = row?.enabled ?? true;
  vendorVisible.value = true;
}

async function saveVendor() {
  if (!vendorForm.name.trim()) {
    ElMessage.warning('名称不能为空');
    return;
  }
  vendorSaving.value = true;
  try {
    const res = editingVendor.value
      ? await aiApi.updateAiVendor(editingVendor.value.id, {
          name: vendorForm.name.trim(),
          sort_order: vendorForm.sort_order,
          enabled: vendorForm.enabled,
        })
      : await aiApi.createAiVendor({ name: vendorForm.name.trim(), sort_order: vendorForm.sort_order });
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      vendorVisible.value = false;
      await loadVendors();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  vendorSaving.value = false;
}

async function toggleVendor(row: AiVendor) {
  try {
    await aiApi.updateAiVendor(row.id, { name: row.name, sort_order: row.sort_order, enabled: row.enabled });
  } catch (e: any) {
    row.enabled = !row.enabled;
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function removeVendor(row: AiVendor) {
  try {
    await ElMessageBox.confirm(`删除厂商「${row.name}」？` + (row.models ? `（仍有 ${row.models} 个模型引用，将无法删除）` : ''), '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiApi.deleteAiVendor(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await loadVendors();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(() => {
  loadFlags();
  loadVendors();
});
</script>

<style scoped>
.page { max-width: 1100px; }
.cfg-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 14px; margin: 14px 0; }
.cfg-card { border-radius: 12px; }
.cfg-row {
  display: flex; align-items: center; justify-content: space-between; gap: 16px;
  padding: 12px 0; border-bottom: 1px dashed var(--el-border-color-lighter, #eee);
}
.cfg-row:last-child { border-bottom: 0; }
.cfg-name { font-size: 0.86rem; font-weight: 600; color: var(--text-color, #333); }
.cfg-desc { font-size: 0.74rem; color: var(--text-light, #888); margin-top: 2px; line-height: 1.5; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; }
</style>
