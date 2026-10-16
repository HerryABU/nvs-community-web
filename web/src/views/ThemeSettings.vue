<template>
  <div class="settings-page">
    <div class="settings-header">
      <h2>我的主题</h2>
      <p class="sub">自定义你的专属网站外观（alist 式）。保存后立即在所有设备生效，覆盖站点默认主题。</p>
    </div>

    <el-alert type="info" :closable="false" show-icon class="tip">
      <template #title>安全说明</template>
      CSS 会经服务端白名单消毒（移除 <code>@import</code>、<code>expression()</code>、外部 <code>url()</code> 等危险内容）后再保存，可放心编写。
    </el-alert>

    <el-form label-width="100px" class="theme-form" v-loading="loading">
      <el-form-item label="个人主题 CSS">
        <el-input
          v-model="css"
          type="textarea"
          :rows="14"
          placeholder="例如：:root { --primary-color: #0ea5e9; }"
          :disabled="saving"
          class="css-editor"
        />
      </el-form-item>

      <el-form-item v-if="removed.length" label="被移除规则">
        <el-tag v-for="r in removed" :key="r" type="warning" class="rm-tag" size="small">{{ r }}</el-tag>
      </el-form-item>

      <el-form-item>
        <el-button type="primary" :loading="saving" @click="save">保存我的主题</el-button>
        <el-button :loading="saving" @click="resetMine">恢复默认（用站点主题）</el-button>
        <el-button @click="reload" :disabled="saving">重新加载</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { themeApi } from '@/api/theme';
import { useThemeStore } from '@/stores/theme';

const themeStore = useThemeStore();
const loading = ref(false);
const saving = ref(false);
const css = ref('');
const removed = ref<string[]>([]);

async function reload() {
  loading.value = true;
  try {
    const r = await themeApi.getMyTheme();
    css.value = r.data?.data?.css || '';
  } catch {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    const r = await themeApi.updateMyTheme(css.value);
    if (r.data?.code === 0) {
      removed.value = r.data.data?.removed || [];
      // 本地即时生效（store 重新拉取并注入）
      await themeStore.loadTheme();
      ElMessage.success(removed.value.length ? '已保存（部分规则因安全风险被移除）' : '已保存，立即生效');
    } else {
      ElMessage.error(r.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function resetMine() {
  saving.value = true;
  try {
    const r = await themeApi.updateMyTheme('');
    if (r.data?.code === 0) {
      css.value = '';
      removed.value = [];
      themeStore.resetMyTheme();
      await themeStore.loadTheme();
      ElMessage.success('已恢复为站点默认主题');
    } else {
      ElMessage.error(r.data?.message || '操作失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    saving.value = false;
  }
}

onMounted(reload);
</script>

<style scoped>
.settings-page { max-width: 820px; margin: 0 auto; padding: 24px 16px; }
.settings-header h2 { margin: 0 0 4px; }
.sub { color: var(--text-light); font-size: 0.9rem; margin: 0 0 16px; }
.tip { margin-bottom: 20px; }
.theme-form { margin-top: 8px; }
.css-editor :deep(textarea) { font-family: 'Cascadia Code', Consolas, monospace; font-size: 13px; }
.rm-tag { margin-right: 6px; }
</style>
