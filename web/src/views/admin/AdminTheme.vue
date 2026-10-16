<template>
  <div class="admin-page">
    <AdminHeader title="自定义网站主题" subtitle="alist 式全局主题：CSS 经服务端白名单消毒后对所有访客生效" :icon="Brush" />
    <div class="admin-page-body admin-panel" v-loading="loading">
      <el-alert type="info" :closable="false" show-icon>
        <template #title>使用说明</template>
        在这里编写的 CSS 会注入到全站所有页面的 <code>&lt;style&gt;</code> 中，对所有访客生效（已登录用户的个人主题会覆盖它）。
        服务端会对 CSS 做白名单消毒（仅放行安全的属性/值，移除 <code>@import</code>、<code>expression()</code>、外部 <code>url()</code> 等危险内容），保存后即时全站生效。
      </el-alert>

      <el-form label-width="120px" class="theme-form">
        <el-form-item label="主题 CSS">
          <el-input
            v-model="css"
            type="textarea"
            :rows="16"
            placeholder="例如：:root { --primary-color: #e11d48; } body { background: #fafafa; }"
            :disabled="saving"
            class="css-editor"
          />
        </el-form-item>

        <el-form-item v-if="removed.length" label="被移除规则">
          <el-tag v-for="r in removed" :key="r" type="warning" class="rm-tag" size="small">{{ r }}</el-tag>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存（全站生效）</el-button>
          <el-button :loading="saving" @click="resetDefault">恢复默认（清空）</el-button>
          <el-button @click="reload" :disabled="saving">重新加载</el-button>
        </el-form-item>
      </el-form>

      <div class="theme-preview">
        <div class="preview-title">实时预览（仅本编辑器内预览，不影响线上）</div>
        <div class="preview-box" :style="previewStyle">
          <span class="preview-primary">主色按钮</span>
          <span class="preview-bg">背景区块</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Brush } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { themeApi } from '@/api/theme';

const loading = ref(false);
const saving = ref(false);
const css = ref('');
const removed = ref<string[]>([]);

// 编辑器内即时预览（仅本地，不注入全局）
const previewStyle = computed(() => {
  // 取 :root / body 这类顶层规则外的简单声明做轻量预览
  const m = css.value.match(/--primary-color\s*:\s*([^;]+);/);
  const bg = css.value.match(/body\s*\{[^}]*background\s*:\s*([^;]+);/);
  const s: Record<string, string> = {};
  if (m) s['--preview-primary'] = m[1].trim();
  if (bg) s.background = bg[1].trim();
  return s;
});

async function reload() {
  loading.value = true;
  try {
    const r = await themeApi.getSiteTheme();
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
    const r = await themeApi.updateSiteTheme(css.value);
    if (r.data?.code === 0) {
      removed.value = r.data.data?.removed || [];
      ElMessage.success(removed.value.length ? '已保存（部分规则因安全风险被移除）' : '已保存，全站生效');
    } else {
      ElMessage.error(r.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function resetDefault() {
  saving.value = true;
  try {
    const r = await themeApi.updateSiteTheme('');
    if (r.data?.code === 0) {
      css.value = '';
      removed.value = [];
      ElMessage.success('已恢复默认');
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
.theme-form { max-width: 880px; margin-top: 16px; }
.css-editor :deep(textarea) { font-family: 'Cascadia Code', Consolas, monospace; font-size: 13px; }
.rm-tag { margin-right: 6px; }
.theme-preview { margin-top: 24px; max-width: 880px; }
.preview-title { font-size: 0.85rem; color: var(--text-light); margin-bottom: 8px; }
.preview-box {
  display: flex; gap: 12px; align-items: center;
  padding: 20px; border-radius: 10px; border: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-color, #fff);
}
.preview-primary {
  color: #fff; padding: 4px 12px; border-radius: 6px;
  background: var(--preview-primary, var(--primary-color, #4f46e5));
}
.preview-bg { padding: 4px 12px; border-radius: 6px; }
</style>
