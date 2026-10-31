<template>
  <div class="settings-page">
    <SectionHeader title="我的主题" subtitle="自定义你的专属网站外观（alist 式）。保存后立即在所有设备生效，覆盖站点默认主题。" :icon="'🎨'" gradient="purple" />

    <el-alert type="info" :closable="false" show-icon class="tip">
      <template #title>安全说明</template>
      CSS 会经服务端白名单消毒（移除 <code>@import</code>、<code>expression()</code>、外部 <code>url()</code> 等危险内容）后再保存，可放心编写。
    </el-alert>

    <el-tabs v-model="activeTab" class="theme-tabs">
      <!-- 个人主题：仅自己登录后可见 -->
      <el-tab-pane label="个人主题" name="mine">
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
      </el-tab-pane>

      <!-- 个人自定义 JS：仅本人登录态执行（自 XSS，无横向危害），alist 式 -->
      <el-tab-pane label="个人 JS" name="js">
        <el-alert type="info" :closable="false" show-icon class="tip">
          <template #title>说明</template>
          此处的 JS 仅在你<b>自己登录后</b>的页面执行（不影响其他访客）。可用来做个性化交互特效，
          通过 <code>window.NVS</code> / <code>window.alist</code> 读取站点状态、监听事件。后端会剥离
          <code>&lt;/script&gt;</code> 等逃逸序列。
        </el-alert>
        <el-form label-width="100px" class="theme-form" v-loading="jsLoading">
          <el-form-item label="个人 JS">
            <el-input
              v-model="js"
              type="textarea"
              :rows="12"
              placeholder="例如：(function(){ document.body.classList.add('my-fx'); })();"
              :disabled="jsSaving"
              class="js-editor"
            />
          </el-form-item>

          <el-form-item v-if="jsRemoved.length" label="被处理项">
            <el-tag v-for="r in jsRemoved" :key="r" type="warning" class="rm-tag" size="small">{{ r }}</el-tag>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="jsSaving" @click="saveJs">保存我的 JS</el-button>
            <el-button :loading="jsSaving" @click="resetJs">清除</el-button>
            <el-button @click="reloadJs" :disabled="jsSaving">重新加载</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 作者主题：访客在你的作者页/作品页可见（G4） -->
      <el-tab-pane v-if="authStore.isAuthor" label="作者主题（访客可见）" name="author">
        <el-alert type="info" :closable="false" show-icon class="tip">
          <template #title>说明</template>
          此处的主题会在<b>你的作者主页与作品页</b>对访客生效（区别于上方仅你自己登录可见的「个人主题」）。同样经服务端消毒，仅支持安全 CSS。
        </el-alert>
        <el-form label-width="100px" class="theme-form" v-loading="authorLoading">
          <el-form-item label="作者主题 CSS">
            <el-input
              v-model="authorCss"
              type="textarea"
              :rows="14"
              placeholder="例如：.author-hero { background: linear-gradient(135deg,#0ea5e9,#6366f1); }"
              :disabled="authorSaving"
              class="css-editor"
            />
          </el-form-item>

          <el-form-item v-if="authorRemoved.length" label="被移除规则">
            <el-tag v-for="r in authorRemoved" :key="r" type="warning" class="rm-tag" size="small">{{ r }}</el-tag>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="authorSaving" @click="saveAuthor">保存作者主题</el-button>
            <el-button :loading="authorSaving" @click="resetAuthor">清除（用站点默认）</el-button>
            <el-button @click="reloadAuthor" :disabled="authorSaving">重新加载</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { themeApi } from '@/api/theme';
import api from '@/api';
import { useThemeStore } from '@/stores/theme';
import { useAuthStore } from '@/stores/auth';

const themeStore = useThemeStore();
const authStore = useAuthStore();
const activeTab = ref<'mine' | 'author'>('mine');

// ===== 个人主题 =====
const loading = ref(false);
const saving = ref(false);
const css = ref('');
const removed = ref<string[]>([]);

// ===== 个人自定义 JS（仅本人登录态执行） =====
const jsLoading = ref(false);
const jsSaving = ref(false);
const js = ref('');
const jsRemoved = ref<string[]>([]);

async function reloadJs() {
  jsLoading.value = true;
  try {
    const r = await themeApi.getMyTheme();
    js.value = r.data?.data?.js || '';
  } catch {
    ElMessage.error('加载失败');
  } finally {
    jsLoading.value = false;
  }
}

async function saveJs() {
  jsSaving.value = true;
  try {
    const r = await themeApi.updateMyTheme(css.value, js.value);
    if (r.data?.code === 0) {
      jsRemoved.value = (r.data.data?.removed || []).filter((x: string) => /script|注释/.test(x));
      await themeStore.loadTheme();
      ElMessage.success(jsRemoved.value.length ? '已保存（部分内容因安全风险被处理）' : '已保存，立即生效');
    } else {
      ElMessage.error(r.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    jsSaving.value = false;
  }
}

async function resetJs() {
  jsSaving.value = true;
  try {
    const r = await themeApi.updateMyTheme(css.value, '');
    if (r.data?.code === 0) {
      js.value = '';
      jsRemoved.value = [];
      await themeStore.loadTheme();
      ElMessage.success('已清除个人 JS');
    } else {
      ElMessage.error(r.data?.message || '操作失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    jsSaving.value = false;
  }
}

async function reload() {
  loading.value = true;
  try {
    const r = await themeApi.getMyTheme();
    css.value = r.data?.data?.css || '';
    js.value = r.data?.data?.js || '';
  } catch {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    const r = await themeApi.updateMyTheme(css.value, js.value);
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
    const r = await themeApi.updateMyTheme('', '');
    if (r.data?.code === 0) {
      css.value = '';
      js.value = '';
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

// ===== 作者主题（访客在作者页可见，G4） =====
const authorLoading = ref(false);
const authorSaving = ref(false);
const authorCss = ref('');
const authorRemoved = ref<string[]>([]);

async function reloadAuthor() {
  const id = authStore.user?.id;
  if (!id) return;
  authorLoading.value = true;
  try {
    const r = await api.get(`/author/${id}/theme`);
    authorCss.value = r.data?.data?.css || '';
  } catch {
    ElMessage.error('作者主题加载失败');
  } finally {
    authorLoading.value = false;
  }
}

async function saveAuthor() {
  const id = authStore.user?.id;
  if (!id) return;
  authorSaving.value = true;
  try {
    const r = await api.put(`/author/${id}/theme`, { css: authorCss.value });
    if (r.data?.code === 0) {
      authorRemoved.value = r.data.data?.removed || [];
      ElMessage.success(authorRemoved.value.length ? '已保存（部分规则因安全风险被移除）' : '已保存，访客将在你的作者页看到');
    } else {
      ElMessage.error(r.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    authorSaving.value = false;
  }
}

async function resetAuthor() {
  const id = authStore.user?.id;
  if (!id) return;
  authorSaving.value = true;
  try {
    const r = await api.put(`/author/${id}/theme`, { css: '' });
    if (r.data?.code === 0) {
      authorCss.value = '';
      authorRemoved.value = [];
      ElMessage.success('已清除作者主题');
    } else {
      ElMessage.error(r.data?.message || '操作失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    authorSaving.value = false;
  }
}

onMounted(() => {
  reload();
  if (authStore.isAuthor) reloadAuthor();
});
</script>

<style scoped>
.settings-page { max-width: 820px; margin: 0 auto; padding: 24px 16px; }
.tip { margin-bottom: 20px; }
.theme-form { margin-top: 8px; }
.theme-tabs { margin-top: 8px; }
.css-editor :deep(textarea) { font-family: 'Cascadia Code', Consolas, monospace; font-size: 13px; }
.js-editor :deep(textarea) { font-family: 'Cascadia Code', Consolas, monospace; font-size: 13px; }
.rm-tag { margin-right: 6px; }
</style>
