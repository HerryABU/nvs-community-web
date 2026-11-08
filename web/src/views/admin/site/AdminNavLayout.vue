<template>
  <div class="admin-page">
    <AdminHeader title="导航模块布局" subtitle="控制导航栏（前面）与「更多」下拉中的模块；仅超级管理员可配置" :icon="Menu" />
    <div class="admin-page-body admin-panel">
      <el-alert type="info" :closable="false" show-icon class="nl-warn">
        <template #title>配置说明</template>
        主导航（前面）适合高频入口；「更多」收纳低频模块。调整保存后<b>立即生效</b>，所有用户按新布局看到导航。
      </el-alert>

      <div v-loading="loading" class="nl-list">
        <div v-for="m in modules" :key="m.key" class="nl-row">
          <div class="nl-left">
            <el-icon class="nl-ico"><component :is="iconOf(m.icon)" /></el-icon>
            <span class="nl-label">{{ m.label }}</span>
          </div>
          <div class="nl-right">
            <el-select v-model="m.location" size="small" style="width: 120px">
              <el-option label="主导航（前面）" value="main" />
              <el-option label="更多" value="more" />
            </el-select>
            <el-switch v-model="m.visible" active-text="显示" inactive-text="隐藏" inline-prompt />
          </div>
        </div>
      </div>

      <div class="nl-actions">
        <el-button type="primary" :loading="saving" @click="save">保存布局（立即生效）</el-button>
        <el-button @click="resetDefault" :disabled="saving">恢复默认</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Menu, HomeFilled, Reading, ChatDotRound, UserFilled, Avatar, Document, MagicStick } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { opsApi, type NavModule } from '@/api/admin/ops';

const ICON_MAP: Record<string, any> = {
  HomeFilled, Reading, Menu, ChatDotRound, UserFilled, Avatar, Document, MagicStick,
};
function iconOf(name: string) {
  return ICON_MAP[name] || Menu;
}

// 默认布局（与后端 models.DefaultNavModules 一致）
function defaultModules(): NavModule[] {
  return [
    { key: 'home', label: '主页', icon: 'HomeFilled', path: '/', location: 'main', visible: true, sort: 0 },
    { key: 'novels', label: '小说', icon: 'Reading', path: '/novels', location: 'main', visible: true, sort: 1 },
    { key: 'category', label: '分类', icon: 'Menu', path: '', location: 'main', visible: true, sort: 2 },
    { key: 'forums', label: '论坛', icon: 'ChatDotRound', path: '/forums', location: 'main', visible: true, sort: 3 },
    { key: 'groups', label: '群组', icon: 'UserFilled', path: '/groups', location: 'main', visible: true, sort: 4 },
    { key: 'subculture', label: '亚文化创作区', icon: 'Avatar', path: '/subculture', location: 'more', visible: true, sort: 5 },
    { key: 'blogs', label: '博客', icon: 'Document', path: '/blogs', location: 'main', visible: true, sort: 6 },
    { key: 'ai', label: 'AI 工作台', icon: 'MagicStick', path: '/ai', location: 'main', visible: true, sort: 7 },
  ];
}

const loading = ref(false);
const saving = ref(false);
const modules = ref<NavModule[]>([]);

async function load() {
  loading.value = true;
  try {
    const res = await opsApi.adminNavLayout();
    if (res.data?.code === 0 && Array.isArray(res.data.data?.modules)) {
      modules.value = res.data.data.modules;
    } else {
      modules.value = defaultModules();
    }
  } catch {
    modules.value = defaultModules();
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    const res = await opsApi.saveNavLayout(modules.value);
    if (res.data?.code === 0) {
      ElMessage.success('已保存，立即生效');
      await load();
    } else {
      ElMessage.error(res.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

function resetDefault() {
  modules.value = defaultModules();
}

onMounted(load);
</script>

<style scoped>
.admin-page { --admin-content-max: 860px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.nl-warn { margin-bottom: 18px; }
.nl-list { display: flex; flex-direction: column; gap: 10px; }
.nl-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
  background: var(--bg-card, #fff);
}
.nl-left { display: flex; align-items: center; gap: 10px; }
.nl-ico { font-size: 1.1rem; color: var(--primary-color, #6366f1); }
.nl-label { font-size: 0.95rem; font-weight: 600; color: var(--text-color, #333); }
.nl-right { display: flex; align-items: center; gap: 16px; }
.nl-actions { display: flex; gap: 10px; margin-top: 20px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .nl-row { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .nl-label { color: #e2e8f0; }
</style>
