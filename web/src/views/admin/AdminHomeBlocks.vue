<template>
  <div class="admin-page">
    <AdminHeader title="自定义主页" subtitle="编排首页区块（头图/公告/模块/博客/版块/最新帖/自定义HTML/链接），支持排序与一键布局" :icon="Grid" />
    <div class="admin-page-body admin-panel" v-loading="loading">
      <!-- 操作栏 -->
      <div class="admin-toolbar">
        <div class="admin-toolbar-left">
          <el-button type="primary" @click="openCreate">新增区块</el-button>
          <el-dropdown @command="applyPreset" trigger="click">
            <el-button>套用布局预设<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-for="p in presets" :key="p.key" :command="p.key">
                  <b>{{ p.name }}</b> — {{ p.desc }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div class="admin-toolbar-right">
          <el-tag type="info">拖动或点箭头调整顺序，保存后即时生效</el-tag>
          <el-button @click="reload" :disabled="loading">刷新</el-button>
        </div>
      </div>

      <!-- 区块列表 -->
      <el-table :data="blocks" border size="small" class="block-table" row-key="id">
        <template #empty>
          <StateBlock
            :state="loading ? 'loading' : 'empty'"
            skeleton="table"
            :rows="4"
            compact
            title="暂无数据"
            description="换个筛选条件试试，或等有数据后再来看。"
          />
        </template>
        <el-table-column label="顺序" width="70" align="center">
          <template #empty>
            <StateBlock
              :state="loading ? 'loading' : 'empty'"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row, $index }">
            <div class="order-ctrl">
              <el-button link :disabled="$index === 0" @click="move($index, -1)"><el-icon><Top /></el-icon></el-button>
              <el-button link :disabled="$index === blocks.length - 1" @click="move($index, 1)"><el-icon><Bottom /></el-icon></el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="120">
          <template #empty>
            <StateBlock
              :state="loading ? 'loading' : 'empty'"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row }">{{ typeName(row.type) }}</template>
        </el-table-column>
        <el-table-column label="标题" min-width="160" prop="title" show-overflow-tooltip />
        <el-table-column label="启用" width="90" align="center">
          <template #empty>
            <StateBlock
              :state="loading ? 'loading' : 'empty'"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row }">
            <el-switch v-model="row.enabled" :disabled="saving" @change="(v: any) => toggleEnabled(row, v)" />
          </template>
        </el-table-column>
        <el-table-column label="排序值" width="90" align="center" prop="sort_order" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #empty>
            <StateBlock
              :state="loading ? 'loading' : 'empty'"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑区块' : '新增区块'" width="640px">
      <el-form label-width="100px" v-loading="saving">
        <el-form-item label="区块类型">
          <el-select v-model="form.type" :disabled="!!editing" placeholder="选择类型">
            <el-option v-for="t in HOME_BLOCK_TYPES" :key="t.type" :label="`${t.name}（${t.type}）`" :value="t.type" />
          </el-select>
          <div class="form-hint">{{ typeDesc(form.type) }}</div>
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="128" show-word-limit placeholder="区块标题（可空，部分类型有默认）" />
        </el-form-item>
        <el-form-item label="是否启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="排序值">
          <el-input-number v-model="form.sort_order" :min="0" :max="9999" />
          <span class="form-hint">数值越小越靠前</span>
        </el-form-item>
        <el-form-item label="配置 (JSON)">
          <el-input v-model="form.config" type="textarea" :rows="6"
            :placeholder="form.type === 'custom' ? 'HTML 片段（服务端会消毒，移除危险内容）' : 'JSON 配置，如 limit:8 或 view:grid'" />
          <div class="form-hint">自定义 HTML 类型直接填 HTML；其余类型填 JSON。保存时校验格式。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Grid, ArrowDown, Top, Bottom } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminHomeBlockApi, HOME_BLOCK_TYPES, type HomeBlock, type HomeBlockPreset } from '@/api/home';
import StateBlock from '@/components/common/StateBlock.vue';

const loading = ref(false);
const saving = ref(false);
const blocks = ref<HomeBlock[]>([]);
const presets = ref<HomeBlockPreset[]>([]);

const dialogVisible = ref(false);
const editing = ref<HomeBlock | null>(null);
const form = reactive<{ type: string; title: string; config: string; enabled: boolean; sort_order: number }>({
  type: 'blogs',
  title: '',
  config: '',
  enabled: true,
  sort_order: 0,
});

function typeName(t: string) {
  return HOME_BLOCK_TYPES.find((x) => x.type === t)?.name || t;
}
function typeDesc(t: string) {
  return HOME_BLOCK_TYPES.find((x) => x.type === t)?.desc || '';
}

async function reload() {
  loading.value = true;
  try {
    const [listR, presetR] = await Promise.all([adminHomeBlockApi.list(), adminHomeBlockApi.presets()]);
    blocks.value = (listR.data?.data?.items || []).slice().sort((a, b) => (a.sort_order || 0) - (b.sort_order || 0));
    presets.value = presetR.data?.data?.items || [];
  } catch {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = null;
  form.type = 'blogs';
  form.title = '';
  form.config = '';
  form.enabled = true;
  form.sort_order = blocks.value.length;
  dialogVisible.value = true;
}

function openEdit(row: HomeBlock) {
  editing.value = row;
  form.type = row.type;
  form.title = row.title || '';
  form.config = row.config || '';
  form.enabled = !!row.enabled;
  form.sort_order = row.sort_order || 0;
  dialogVisible.value = true;
}

async function submit() {
  saving.value = true;
  try {
    let r;
    if (editing.value) {
      r = await adminHomeBlockApi.update(editing.value.id, {
        title: form.title, config: form.config, enabled: form.enabled, sort_order: form.sort_order,
      });
    } else {
      r = await adminHomeBlockApi.create({
        type: form.type, title: form.title, config: form.config, enabled: form.enabled, sort_order: form.sort_order,
      });
    }
    if (r.data?.code === 0) {
      ElMessage.success('已保存');
      dialogVisible.value = false;
      await reload();
    } else {
      ElMessage.error(r.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function toggleEnabled(row: HomeBlock, val: boolean) {
  saving.value = true;
  try {
    const r = await adminHomeBlockApi.update(row.id, { enabled: val });
    if (r.data?.code !== 0) {
      row.enabled = !val; // 回滚
      ElMessage.error(r.data?.message || '更新失败');
    }
  } catch {
    row.enabled = !val;
    ElMessage.error('更新失败');
  } finally {
    saving.value = false;
  }
}

async function remove(row: HomeBlock) {
  try {
    await ElMessageBox.confirm(`确定删除区块「${row.title || typeName(row.type)}」？`, '删除确认', { type: 'warning' });
  } catch {
    return;
  }
  saving.value = true;
  try {
    const r = await adminHomeBlockApi.remove(row.id);
    if (r.data?.code === 0) {
      ElMessage.success('已删除');
      await reload();
    } else {
      ElMessage.error(r.data?.message || '删除失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  } finally {
    saving.value = false;
  }
}

async function move(index: number, dir: number) {
  const next = index + dir;
  if (next < 0 || next >= blocks.value.length) return;
  const arr = blocks.value.slice();
  [arr[index], arr[next]] = [arr[next], arr[index]];
  // 重新分配排序值（保证稳定）
  arr.forEach((b, i) => (b.sort_order = i * 10));
  blocks.value = arr;
  saving.value = true;
  try {
    const r = await adminHomeBlockApi.reorder(arr.map((b) => b.id));
    if (r.data?.code !== 0) {
      ElMessage.error(r.data?.message || '排序失败');
      await reload();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '排序失败');
    await reload();
  } finally {
    saving.value = false;
  }
}

async function applyPreset(key: string) {
  saving.value = true;
  try {
    const r = await adminHomeBlockApi.applyPreset(key);
    if (r.data?.code === 0) {
      ElMessage.success('已套用布局预设');
      await reload();
    } else {
      ElMessage.error(r.data?.message || '套用失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '套用失败');
  } finally {
    saving.value = false;
  }
}

onMounted(reload);
</script>

<style scoped>
.admin-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-wrap: wrap; gap: 8px; }
.admin-toolbar-left, .admin-toolbar-right { display: flex; align-items: center; gap: 8px; }
.block-table { margin-top: 4px; }
.order-ctrl { display: flex; flex-direction: column; align-items: center; }
.order-ctrl .el-button { margin: 0; padding: 2px; }
.form-hint { color: var(--text-light, #999); font-size: 0.78rem; margin-top: 4px; }
</style>
