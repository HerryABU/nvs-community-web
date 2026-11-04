<template>
  <div class="admin-page">
    <AdminHeader
      title="AI 工作台菜单"
      subtitle="左栏自定义应用 · 站内面板或任意外部链接 · 由站长统一配置"
      :icon="Menu"
    />
    <div class="admin-page-body">
      <p class="hint-text">
        这里配置的是<b>用户看到的左栏菜单</b>，只有站长能改；用户不能自己添加应用。
        <br />
        · <b>站内面板</b>：填 <code>chat</code> / <code>models</code> / <code>image</code> /
        <code>video</code> / <code>gallery</code> / <code>agents</code> / <code>mind</code>，
        或站内路径如 <code>/forums</code>；
        · <b>外部链接</b>：填完整 <code>https://</code> 地址。
        <br />
        因为条目是你自己录入的，外部链接<b>不需要额外审核</b>；平台只拦
        <code>javascript:</code> / <code>data:</code> 这类会变成站内 XSS 的伪协议。
      </p>

      <div class="table-toolbar">
        <span class="muted small">共 {{ items.length }} 条（显示 {{ enabledCount }} 条）</span>
        <span>
          <el-button @click="addItem">添加菜单</el-button>
          <el-button type="primary" :loading="saving" @click="save">保存</el-button>
        </span>
      </div>

      <el-table :data="items" style="width:100%" row-key="_rid">
        <el-table-column label="顺序" width="90">
          <template #default="{ $index }">
            <el-button link size="small" :disabled="$index === 0" @click="move($index, -1)">↑</el-button>
            <el-button link size="small" :disabled="$index === items.length - 1" @click="move($index, 1)">↓</el-button>
          </template>
        </el-table-column>
        <el-table-column label="图标" width="80">
          <template #default="{ row }">
            <el-input v-model="row.emoji" size="small" maxlength="4" placeholder="🔗" />
          </template>
        </el-table-column>
        <el-table-column label="菜单名" min-width="130">
          <template #default="{ row }">
            <el-input v-model="row.title" size="small" maxlength="16" placeholder="如：写作圣经" />
          </template>
        </el-table-column>
        <el-table-column label="类型" width="120">
          <template #default="{ row }">
            <el-select v-model="row.kind" size="small">
              <el-option label="站内面板" value="panel" />
              <el-option label="外部链接" value="link" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="目标" min-width="220">
          <template #default="{ row }">
            <el-select
              v-if="row.kind === 'panel'"
              v-model="row.target"
              size="small"
              filterable
              allow-create
              default-first-option
              placeholder="选面板或直接输站内路径"
              style="width:100%"
            >
              <el-option v-for="p in PANEL_PRESETS" :key="p.value" :label="p.label" :value="p.value" />
            </el-select>
            <el-input v-else v-model="row.target" size="small" placeholder="https://example.com" />
          </template>
        </el-table-column>
        <el-table-column label="分组" width="130">
          <template #default="{ row }">
            <el-input v-model="row.group" size="small" maxlength="16" placeholder="自定义应用" />
          </template>
        </el-table-column>
        <el-table-column label="新标签" width="80">
          <template #default="{ row }">
            <el-switch v-model="row.new_tab" size="small" :disabled="row.kind !== 'link'" />
          </template>
        </el-table-column>
        <el-table-column label="显示" width="70">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" size="small" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ $index }">
            <el-button type="danger" link size="small" @click="remove($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <StateBlock
        v-if="!items.length && !loading"
        state="empty"
        title="还没有自定义菜单"
        description="点「添加菜单」可把常用面板或外部站点挂到 AI 工作台左栏。"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Menu } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { adminApi } from '@/api/admin';
import type { AiNavItem } from '@/api/ai/aiHub';

// 站内面板预设（allow-create 让站长也能直接输站内路径，如 /forums）
const PANEL_PRESETS = [
  { label: 'AI 对话', value: 'chat' },
  { label: '大模型广场', value: 'models' },
  { label: '专业绘画', value: 'image' },
  { label: '视频创作', value: 'video' },
  { label: '灵感广场', value: 'gallery' },
  { label: 'AI 智能体', value: 'agents' },
  { label: '思维导图', value: 'mind' },
  { label: '会员中心', value: 'membership' },
];

type Row = AiNavItem & { _rid: number };
const items = ref<Row[]>([]);
const loading = ref(false);
const saving = ref(false);
let rid = 1;

const enabledCount = computed(() => items.value.filter((i) => i.enabled).length);

function blank(): Row {
  return {
    _rid: rid++,
    key: '',
    title: '',
    emoji: '🔗',
    kind: 'panel',
    target: '',
    group: '自定义应用',
    new_tab: false,
    enabled: true,
  };
}

function addItem() {
  items.value.push(blank());
}

function remove(idx: number) {
  items.value.splice(idx, 1);
}

function move(idx: number, delta: number) {
  const to = idx + delta;
  if (to < 0 || to >= items.value.length) return;
  const [row] = items.value.splice(idx, 1);
  items.value.splice(to, 0, row);
}

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getAiNavItems();
    if (res.data.code === 0) {
      items.value = (res.data.data.items || []).map((it: AiNavItem) => ({ ...it, _rid: rid++ }));
    }
  } catch {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
}

async function save() {
  // 前端先挡一遍空名/空目标，省得看后端报错
  for (const it of items.value) {
    if (!it.title.trim()) {
      ElMessage.warning('有菜单没填名字');
      return;
    }
    if (!it.target.trim()) {
      ElMessage.warning(`菜单「${it.title}」没填跳转目标`);
      return;
    }
  }
  saving.value = true;
  try {
    const payload = items.value.map(({ _rid, ...rest }) => rest);
    const res = await adminApi.saveAiNavItems(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存，AI 工作台左栏立即生效');
      await load();
    } else {
      ElMessage.error(res.data.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.hint-text code {
  padding: 1px 5px; border-radius: 4px;
  background: var(--bg-page, #f1f5f9); font-size: 0.78rem;
}
</style>
