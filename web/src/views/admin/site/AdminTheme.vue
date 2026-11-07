<template>
  <div class="admin-page">
    <AdminHeader title="自定义网站主题" subtitle="alist 式全局主题：CSS 经服务端白名单消毒后对所有访客生效" :icon="Brush" />
    <div class="admin-page-body admin-panel" v-loading="loading">
      <el-alert type="info" :closable="false" show-icon>
        <template #title>使用说明</template>
        在这里编写的 CSS 会注入到全站所有页面的 <code>&lt;style&gt;</code> 中，对所有访客生效（已登录用户的个人主题会覆盖它）。
        服务端会对 CSS 做白名单消毒（仅放行安全的属性/值，移除 <code>@import</code>、<code>expression()</code>、外部 <code>url()</code> 等危险内容），保存后即时全站生效。
      </el-alert>

      <!-- F 可视化主题（结构化控件）：零代码美化，与 alist 同款；生成的样式与下方原始 CSS 互补 -->
      <el-divider content-position="left">可视化主题（结构化控件）</el-divider>
      <el-alert type="success" :closable="false" show-icon class="tip">
        <template #title>零代码美化（对标 alist）</template>
        通过下方控件即可调整背景、毛玻璃、按钮圆角，以及<b>主题主色 / 强调色 / 表面色</b>。
        主色是全站唯一真源：改成任意颜色后，按钮渐变、Element Plus 组件、标签、分页、聚焦光晕与 AI 写作图表会<b>同步变色</b>，
        不会残留写死的紫色。生成的样式与「主题 CSS」文本互补——文本优先级更高，可覆盖控件效果；
        也可在文本里直接写 <code>:root { --primary-color: #e11d48; --accent-color: #0ea5e9; --surface-color: #fff7ed; }</code> 自定义任意配色。
        看板娘兼容任意外部脚本（如 alist/live2d）。
      </el-alert>
      <el-form label-width="120px" class="theme-form">
        <el-form-item label="配色预设">
          <div class="preset-row">
            <button
              v-for="p in colorPresets"
              :key="p.key"
              type="button"
              class="preset-chip"
              :title="`应用「${p.label}」配色到主色 / 强调色 / 表面色`"
              @click="applyPreset(p)"
            >
              <span class="preset-swatch" :style="{ background: p.primary }"><i :style="{ background: p.accent }" /></span>
              {{ p.label }}
            </button>
            <span class="mode-hint">点击即填充下方取色器（仍需点保存才会全站生效）</span>
          </div>
        </el-form-item>
        <el-form-item label="背景类型">
          <el-radio-group v-model="settings.background.type">
            <el-radio-button value="color">纯色</el-radio-button>
            <el-radio-button value="gradient">渐变</el-radio-button>
            <el-radio-button value="image">图片</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="bgLabel">
          <el-color-picker v-if="settings.background.type === 'color'" v-model="settings.background.value" />
          <el-input v-else v-model="settings.background.value" :placeholder="bgPlaceholder" style="max-width: 520px" />
        </el-form-item>

        <el-form-item label="毛玻璃">
          <el-switch v-model="settings.glass.enabled" />
          <span class="mode-hint">导航栏与玻璃面板的背景模糊（同步 CSS 变量 --glass-blur）</span>
        </el-form-item>
        <template v-if="settings.glass.enabled">
          <el-form-item label="模糊半径">
            <el-slider v-model="settings.glass.blur" :min="0" :max="40" :step="1" style="width: 220px" />
          </el-form-item>
          <el-form-item label="玻璃底色">
            <el-color-picker v-model="settings.glass.tint" />
            <el-slider v-model="settings.glass.opacity" :min="0" :max="1" :step="0.05" style="width: 160px; margin-left: 12px" />
            <span class="mode-hint">不透明度</span>
          </el-form-item>
        </template>

        <el-form-item label="按钮圆角">
          <el-slider v-model="settings.button.radius" :min="0" :max="28" :step="1" style="width: 220px" />
          <span class="mode-hint">px（同步 --radius-md / --radius-lg）</span>
        </el-form-item>
        <el-form-item label="主题主色">
          <el-color-picker v-model="settings.button.primary" />
          <span class="mode-hint">留空则沿用系统主色（同步 --primary-color）</span>
        </el-form-item>
        <el-form-item label="强调色">
          <el-color-picker v-model="settings.accent.color" />
          <span class="mode-hint">用于高亮、标签与 AI 写作可视化主节点/回环（同步 --accent-color / --aw-node / --aw-return）；留空沿用系统强调色</span>
        </el-form-item>
        <el-form-item label="表面色">
          <el-color-picker v-model="settings.surface.color" />
          <span class="mode-hint">卡片、面板等表面底色（同步 --surface-color）；留空沿用系统卡片色</span>
        </el-form-item>

        <el-form-item label="看板娘">
          <el-switch v-model="settings.mascot.enabled" />
          <span class="mode-hint">兼容 alist / live2d 等任意外部脚本</span>
        </el-form-item>
        <el-form-item v-if="settings.mascot.enabled" label="脚本地址">
          <el-input v-model="settings.mascot.script_url" placeholder="https://cdn.example.com/live2d-widget/sdk.js" style="max-width: 520px" />
          <span class="mode-hint">仅允许 http/https</span>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存可视化设置（全站生效）</el-button>
        </el-form-item>
      </el-form>

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

      <!-- F12 alist 式自定义 JS：管理员可写，对全站访客执行（与 alist 自定义 JS 一致） -->
      <el-divider content-position="left">站点自定义 JS（alist 式）</el-divider>
      <el-alert type="warning" :closable="false" show-icon class="tip">
        <template #title>高风险能力 · 仅管理员</template>
        此处 JS 会以 <code>&lt;script&gt;</code> 形式注入到<b>全站每一个访客的页面</b>并执行（与 alist 自定义 JS 一致）。
        仅管理员可编辑；后端会剥离 <code>&lt;/script&gt;</code> 等逃逸序列防止破坏页面，但脚本<b>逻辑本身不受限</b>——请仅写入你信任的代码。
        可通过 <code>window.NVS</code> / <code>window.alist</code> 读取站点状态、监听事件、调用安全 API。
      </el-alert>
      <el-form label-width="120px" class="theme-form">
        <el-form-item label="自定义 JS">
          <el-input
            v-model="js"
            type="textarea"
            :rows="12"
            placeholder="例如：(function(){ console.log('NVS theme loaded', window.NVS.siteName); })();"
            :disabled="saving"
            class="js-editor"
          />
        </el-form-item>
        <el-form-item label="JS 加载方式">
          <el-radio-group v-model="jsMode" :disabled="saving">
            <el-radio-button value="immediate">立即加载</el-radio-button>
            <el-radio-button value="idle">空闲时加载</el-radio-button>
            <el-radio-button value="delayed">延时加载</el-radio-button>
          </el-radio-group>
          <div class="mode-hint">
            <template v-if="jsMode === 'immediate'">
              随主题立即注入执行（与 alist 默认行为一致）。
            </template>
            <template v-else-if="jsMode === 'idle'">
              等首屏渲染完成（window load）后，在浏览器空闲时注入 —— 不与首屏抢主线程。
            </template>
            <template v-else>
              延迟指定秒数后注入 —— 适合埋点、统计、第三方挂件这类「不急」的脚本。
            </template>
          </div>
        </el-form-item>
        <el-form-item v-if="jsMode === 'delayed'" label="延迟秒数">
          <el-input-number v-model="jsDelay" :min="1" :max="600" :disabled="saving" />
          <span class="mode-hint">默认 3 秒；最大 600 秒。</span>
        </el-form-item>
        <el-form-item v-if="jsRemoved.length" label="被处理项">
          <el-tag v-for="r in jsRemoved" :key="r" type="warning" class="rm-tag" size="small">{{ r }}</el-tag>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存（全站生效）</el-button>
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
          <span class="preview-accent">强调色标签</span>
          <span class="preview-surface">表面色卡片</span>
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
import { themeApi, type ThemeJSMode, type ThemeSettings } from '@/api/home/theme';
import { useThemeStore } from '@/stores/theme';
import api from '@/api';

const themeStore = useThemeStore();

// 结构化主题设置（可视化控件绑定对象）
function defaultSettings(): ThemeSettings {
  return {
    background: { type: 'color', value: '' },
    glass: { enabled: true, blur: 20, opacity: 0.6, tint: '#ffffff' },
    button: { radius: 12, primary: '' },
    accent: { color: '' },
    surface: { color: '' },
    mascot: { enabled: false, script_url: '' },
  };
}
function parseSettings(raw?: string): ThemeSettings {
  const def = defaultSettings();
  if (!raw) return def;
  try {
    const p = JSON.parse(raw);
    return {
      background: { ...def.background, ...(p.background || {}) },
      glass: { ...def.glass, ...(p.glass || {}) },
      button: { ...def.button, ...(p.button || {}) },
      accent: { ...def.accent, ...(p.accent || {}) },
      surface: { ...def.surface, ...(p.surface || {}) },
      mascot: { ...def.mascot, ...(p.mascot || {}) },
    };
  } catch {
    return def;
  }
}
const settings = ref<ThemeSettings>(defaultSettings());

// 配色预设：一键把主色 / 强调色 / 表面色填进控件（仅本地，需保存生效）
const colorPresets = [
  { key: 'indigo', label: '星海蓝紫', primary: '#5b5fe3', accent: '#f59e0b', surface: '' },
  { key: 'rose', label: '玫红', primary: '#e11d48', accent: '#f43f5e', surface: '#fff1f2' },
  { key: 'cyan', label: '青碧', primary: '#0e7490', accent: '#06b6d4', surface: '#ecfeff' },
  { key: 'emerald', label: '翠绿', primary: '#059669', accent: '#10b981', surface: '#ecfdf5' },
  { key: 'violet', label: '紫罗兰', primary: '#7c3aed', accent: '#a78bfa', surface: '#f5f3ff' },
  { key: 'amber', label: '琥珀', primary: '#b45309', accent: '#f59e0b', surface: '#fffbeb' },
  { key: 'slate', label: '石墨', primary: '#334155', accent: '#64748b', surface: '#f8fafc' },
];
function applyPreset(p: (typeof colorPresets)[number]) {
  settings.value.button.primary = p.primary;
  settings.value.accent.color = p.accent;
  settings.value.surface.color = p.surface;
}

const bgLabel = computed(() =>
  settings.value.background.type === 'color' ? '背景颜色' : settings.value.background.type === 'gradient' ? '渐变串' : '图片 URL',
);
const bgPlaceholder = computed(() =>
  settings.value.background.type === 'gradient' ? 'linear-gradient(135deg,#6366f1,#ec4899)' : 'https://example.com/bg.jpg',
);
// 序列化为后端可接受的结构化 JSON（颜色/URL 清掉空值与 null）
function serializeSettings(): string {
  const s = settings.value;
  const clean = (v: any) => (v == null ? '' : String(v));
  return JSON.stringify({
    background: { type: s.background.type, value: clean(s.background.value) },
    glass: { enabled: !!s.glass.enabled, blur: Number(s.glass.blur) || 0, opacity: Number(s.glass.opacity) || 0, tint: clean(s.glass.tint) },
    button: { radius: Number(s.button.radius) || 0, primary: clean(s.button.primary) },
    accent: { color: clean(s.accent.color) },
    surface: { color: clean(s.surface.color) },
    mascot: { enabled: !!s.mascot.enabled, script_url: clean(s.mascot.script_url) },
  });
}

const loading = ref(false);
const saving = ref(false);
const css = ref('');
const js = ref('');
// JS 加载方式（alist「延迟加载」）：immediate 立即 / idle 空闲时 / delayed 延时
const jsMode = ref<ThemeJSMode>('immediate');
const jsDelay = ref(3);
const removed = ref<string[]>([]);
const jsRemoved = ref<string[]>([]);

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
    // ⚠️ 写入口在 admin 组（后端只给管理员开了 PUT），读接口才是公开的 /site-iframe。
    // 少了 /admin 前缀会打到 /api/site-iframe，方法不存在 → 保存静默失败。
    const r = await api.put('/admin/site-iframe', { url: iframeURL.value, enabled: iframeEnabled.value });
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

// 编辑器内即时预览（仅本地，不注入全局）：
// 可视化控件取值优先，其次才回退到主题 CSS 文本里的声明。
const previewStyle = computed(() => {
  const cssPrimary = css.value.match(/--primary-color\s*:\s*([^;]+);/);
  const bg = css.value.match(/body\s*\{[^}]*background\s*:\s*([^;]+);/);
  const s: Record<string, string> = {};
  const primary = settings.value.button.primary || (cssPrimary ? cssPrimary[1].trim() : '');
  if (primary) s['--preview-primary'] = primary;
  if (settings.value.accent.color) s['--preview-accent'] = settings.value.accent.color;
  if (settings.value.surface.color) s['--preview-surface'] = settings.value.surface.color;
  if (bg) s.background = bg[1].trim();
  return s;
});

async function reload() {
  loading.value = true;
  try {
    const r = await themeApi.getSiteTheme();
    css.value = r.data?.data?.css || '';
    js.value = r.data?.data?.js || '';
    jsMode.value = (r.data?.data?.js_mode as ThemeJSMode) || 'immediate';
    jsDelay.value = r.data?.data?.js_delay || 3;
    settings.value = parseSettings(r.data?.data?.settings);
  } catch {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    const r = await themeApi.updateSiteTheme(css.value, js.value, jsMode.value, jsDelay.value, serializeSettings());
    if (r.data?.code === 0) {
      removed.value = r.data.data?.removed || [];
      jsRemoved.value = (r.data.data?.removed || []).filter((x: string) => /script|注释/.test(x));
      ElMessage.success(removed.value.length ? '已保存（部分内容因安全风险被处理）' : '已保存，全站生效');
      // 即时刷新线上主题（无需整页刷新）
      themeStore.loadTheme();
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
    const r = await themeApi.updateSiteTheme('', '', 'immediate', 0, JSON.stringify(defaultSettings()));
    if (r.data?.code === 0) {
      css.value = '';
      js.value = '';
      jsMode.value = 'immediate';
      jsDelay.value = 3;
      settings.value = defaultSettings();
      removed.value = [];
      jsRemoved.value = [];
      ElMessage.success('已恢复默认');
      themeStore.loadTheme();
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
.mode-hint { font-size: 12px; color: var(--text-muted, #909399); margin-top: 4px; line-height: 1.6; }
.preset-row { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; width: 100%; }
.preset-chip {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 4px 10px 4px 5px; border-radius: var(--radius-full, 999px);
  border: 1px solid var(--border-color, #e5e7eb); background: var(--bg-card, #fff);
  color: var(--text-color); font-size: 12px; cursor: pointer;
  transition: border-color 0.2s, transform 0.2s, box-shadow 0.2s;
}
.preset-chip:hover { border-color: var(--primary-color); transform: translateY(-1px); box-shadow: var(--shadow-sm); }
.preset-swatch {
  width: 18px; height: 18px; border-radius: 50%; display: inline-flex;
  align-items: center; justify-content: center; flex-shrink: 0;
}
.preset-swatch i { width: 8px; height: 8px; border-radius: 50%; display: block; }
.css-editor :deep(textarea) { font-family: 'Cascadia Code', Consolas, monospace; font-size: 13px; }
.js-editor :deep(textarea) { font-family: 'Cascadia Code', Consolas, monospace; font-size: 13px; }
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
  color: #fff; padding: 6px 14px; border-radius: 8px;
  background: var(--preview-primary, var(--primary-color, #4f46e5));
}
.preview-accent {
  color: #fff; padding: 6px 14px; border-radius: 8px;
  background: var(--preview-accent, var(--accent-color, #f59e0b));
}
.preview-surface {
  padding: 6px 14px; border-radius: 8px; color: var(--text-color);
  border: 1px solid var(--border-color, #e5e7eb);
  background: var(--preview-surface, var(--bg-card, #fff));
}
.preview-bg { padding: 6px 14px; border-radius: 8px; }
</style>
