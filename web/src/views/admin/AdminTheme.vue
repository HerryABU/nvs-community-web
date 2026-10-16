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

      <!-- G3 全站 opt-in 嵌入 iframe：allowlist 单一地址 + 启用开关（沙箱渲染） -->
      <el-divider content-position="left">全站嵌入区（页脚上方通栏）</el-divider>
      <el-alert type="warning" :closable="false" show-icon class="tip">
        <template #title>安全说明</template>
        仅允许 http/https 链接（自动拒绝 javascript:/data:/file: 等危险协议）。启用后该地址将以带 sandbox 的
        <code>&lt;iframe&gt;</code> 形式渲染在站点页脚上方通栏，仅嵌入外部页面，不执行本站点任何脚本上下文，<b>不会</b>加载任意用户 JS。
      </el-alert>
      <el-form label-width="120px" class="iframe-form" v-loading="iframeLoading">
        <el-form-item label="启用嵌入区">
          <el-switch v-model="iframeEnabled" :disabled="iframeSaving" @change="saveIframe" />
        </el-form-item>
        <el-form-item label="嵌入地址">
          <el-input
            v-model="iframeURL"
            placeholder="https://example.com/widget"
            :disabled="iframeSaving"
            @keyup.enter="saveIframe"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="iframeSaving" @click="saveIframe">保存嵌入配置</el-button>
          <el-button @click="loadIframe" :disabled="iframeSaving">重新加载</el-button>
          <el-button v-if="iframeEnabled && iframeURL" link type="primary" @click="previewIframe = !previewIframe">
            {{ previewIframe ? '隐藏预览' : '预览' }}
          </el-button>
        </el-form-item>
        <el-form-item v-if="previewIframe && iframeEnabled && iframeURL">
          <iframe :src="iframeURL" sandbox="allow-scripts allow-same-origin" class="iframe-preview" title="嵌入预览" />
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
import api from '@/api';

const loading = ref(false);
const saving = ref(false);
const css = ref('');
const removed = ref<string[]>([]);

// G3 全站嵌入 iframe 配置（管理员 allowlist）
const iframeLoading = ref(false);
const iframeSaving = ref(false);
const iframeEnabled = ref(false);
const iframeURL = ref('');
const previewIframe = ref(false);

async function loadIframe() {
  iframeLoading.value = true;
  try {
    const r = await api.get('/site-iframe');
    iframeEnabled.value = !!r.data?.data?.enabled;
    iframeURL.value = r.data?.data?.url || '';
  } catch {
    ElMessage.error('嵌入配置加载失败');
  } finally {
    iframeLoading.value = false;
  }
}

async function saveIframe() {
  iframeSaving.value = true;
  try {
    const r = await api.put('/site-iframe', { url: iframeURL.value, enabled: iframeEnabled.value });
    if (r.data?.code === 0) {
      iframeEnabled.value = !!r.data.data?.enabled;
      iframeURL.value = r.data.data?.url || '';
      ElMessage.success('已保存嵌入配置');
    } else {
      ElMessage.error(r.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    iframeSaving.value = false;
  }
}

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

onMounted(() => {
  reload();
  loadIframe();
});
</script>

<style scoped>
.theme-form { max-width: 880px; margin-top: 16px; }
.css-editor :deep(textarea) { font-family: 'Cascadia Code', Consolas, monospace; font-size: 13px; }
.rm-tag { margin-right: 6px; }
.iframe-form { max-width: 880px; margin-top: 16px; }
.iframe-preview { width: 100%; height: 320px; border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; }
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
