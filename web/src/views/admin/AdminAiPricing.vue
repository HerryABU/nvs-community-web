<template>
  <div class="admin-page ai-pricing-page">
    <AdminHeader
      title="模型收费"
      subtitle="用户侧看到的单价从这里来：分类默认价 + 单模型覆盖价"
      :icon="Coin"
    />
    <div class="admin-page-body">
      <el-alert type="info" :closable="false" class="intro">
        这里只管「向用户收多少」。<b>上游渠道与卡密池在「AI 模型池 → 按渠道」</b>，
        两件事不要混：渠道决定成本，这里决定售价。
        用户在前台 <code>/ai/pricing</code> 看到的就是本页配置。
      </el-alert>

      <!-- 分类默认价 -->
      <div class="admin-panel">
        <SectionHeader
          title="分类默认价"
          subtitle="模型未单独定价时套用这个；0 表示免费"
          :icon="'💰'"
          gradient="amber"
        />
        <el-form label-width="140px" class="price-form">
          <el-form-item label="对话 · 输入价">
            <el-input-number v-model="cat.chat.price_in" :min="0" :max="1000000" />
            <span class="unit">代币 / 1K tokens</span>
          </el-form-item>
          <el-form-item label="对话 · 输出价">
            <el-input-number v-model="cat.chat.price_out" :min="0" :max="1000000" />
            <span class="unit">代币 / 1K tokens</span>
          </el-form-item>
          <el-form-item label="绘画 · 每次">
            <el-input-number v-model="cat.image.price_call" :min="0" :max="1000000" />
            <span class="unit">代币 / 次</span>
          </el-form-item>
          <el-form-item label="视频 · 每次">
            <el-input-number v-model="cat.video.price_call" :min="0" :max="1000000" />
            <span class="unit">代币 / 次</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingCat" @click="saveCategories">保存分类价</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 单模型覆盖价 -->
      <div class="admin-panel" style="margin-top:16px">
        <SectionHeader
          title="单模型价格"
          subtitle="按模型覆盖分类默认价；改完点行尾保存"
          :icon="'🏷️'"
          gradient="blue"
        />
        <el-table :data="models" size="small" stripe v-loading="loading">
          <template #empty>
            <StateBlock state="empty" title="还没有模型" description="先到「AI 模型池」添加或从渠道拉取。" />
          </template>
          <el-table-column prop="name" label="模型" min-width="160" show-overflow-tooltip />
          <el-table-column label="类型" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="kindTag(row.kind)">{{ kindLabel(row.kind) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="输入价 / 1K" width="150">
            <template #default="{ row }">
              <el-input-number v-if="row.kind === 'chat'" v-model="row.price_in" :min="0" size="small" :controls="false" style="width:110px" />
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="输出价 / 1K" width="150">
            <template #default="{ row }">
              <el-input-number v-if="row.kind === 'chat'" v-model="row.price_out" :min="0" size="small" :controls="false" style="width:110px" />
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="每次价" width="140">
            <template #default="{ row }">
              <el-input-number v-if="row.kind !== 'chat'" v-model="row.price_call" :min="0" size="small" :controls="false" style="width:110px" />
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button link type="primary" size="small" :loading="row._saving" @click="saveModel(row)">保存</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Coin } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiApi, type AdminAiModel } from '@/api/ai';

const loading = ref(false);
const savingCat = ref(false);
const models = ref<any[]>([]);
const cat = reactive({
  chat: { price_in: 0, price_out: 0 },
  image: { price_call: 0 },
  video: { price_call: 0 },
});

function kindLabel(k: string) {
  return k === 'chat' ? '对话' : k === 'image' ? '绘画' : k === 'video' ? '视频' : k;
}
function kindTag(k: string) {
  return k === 'chat' ? 'primary' : k === 'image' ? 'success' : 'warning';
}

async function loadAll() {
  loading.value = true;
  try {
    const [pm, cp] = await Promise.all([aiApi.models(), aiApi.categoryPrices()]);
    const list: AdminAiModel[] = (pm.data?.data?.models || pm.data?.data?.list || []) as any;
    models.value = (list || []).map((m: any) => ({ ...m, _saving: false }));

    const prices: any[] = (cp.data?.data?.prices || cp.data?.data?.list || []) as any;
    for (const p of prices || []) {
      if (p.kind === 'chat') {
        cat.chat.price_in = p.price_in ?? 0;
        cat.chat.price_out = p.price_out ?? 0;
      } else if (p.kind === 'image') {
        cat.image.price_call = p.price_call ?? 0;
      } else if (p.kind === 'video') {
        cat.video.price_call = p.price_call ?? 0;
      }
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function saveCategories() {
  savingCat.value = true;
  try {
    await aiApi.saveCategoryPrices({
      chat: { price_in: cat.chat.price_in, price_out: cat.chat.price_out },
      image: { price_call: cat.image.price_call },
      video: { price_call: cat.video.price_call },
    });
    ElMessage.success('分类价已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingCat.value = false;
  }
}

async function saveModel(row: any) {
  row._saving = true;
  try {
    // 复用模型编辑接口：只提交价格三件套，其余字段后端按「部分更新」处理
    await aiApi.updateModel(row.id, {
      price_in: row.price_in, price_out: row.price_out, price_call: row.price_call,
    } as any);
    ElMessage.success(`已保存「${row.name}」`);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    row._saving = false;
  }
}

onMounted(loadAll);
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.intro { margin-bottom: 14px; line-height: 1.7; }
.price-form :deep(.el-form-item) { margin-bottom: 12px; }
.unit { margin-left: 8px; font-size: 0.78rem; color: var(--text-light, #94a3b8); }
.muted { color: var(--text-light, #bbb); }
</style>
