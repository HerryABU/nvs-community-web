<template>
  <!-- AI 绘画面板：左侧 320 参数区 + 右侧纵向作品流（布局参照参考站 painting 页） -->
  <div class="ai-paint">
    <!-- ===== 左：参数（固定 320，独立滚动） ===== -->
    <aside class="pd-side">
      <div class="pd-side-scroll">
        <div class="pd-block">
          <div class="pd-block-title">模式</div>
          <div class="pd-modes">
            <button
              v-for="m in MODES"
              :key="m.key"
              type="button"
              :class="['pd-mode', { active: mode === m.key }]"
              @click="mode = m.key"
            >
              <b>{{ m.name }}</b>
              <span>{{ m.desc }}</span>
            </button>
          </div>
        </div>

        <div class="pd-block">
          <div class="pd-block-title">模型</div>
          <div v-if="!status.image_models.length" class="pd-empty">暂无绘画模型</div>
          <button
            v-for="m in status.image_models"
            :key="m.id"
            type="button"
            :class="['pd-model', { active: modelId === m.id }]"
            @click="modelId = m.id"
          >
            <span class="pd-model-main">
              <span class="pd-model-name">{{ m.name }}</span>
              <span v-if="apiKindTag(m)" class="pd-model-tag">{{ apiKindTag(m) }}</span>
            </span>
            <span class="pd-model-price">{{ m.price_call || 0 }} {{ status.token_name }}/张</span>
          </button>
        </div>

        <div class="pd-block">
          <div class="pd-block-title">
            提示词
            <button class="pd-link" type="button" @click="promptOpen = true">提示词库</button>
          </div>
          <el-input v-model="prompt" type="textarea" :rows="4" resize="none" :placeholder="promptPlaceholder" />
        </div>

        <div class="pd-block">
          <div class="pd-block-title">比例</div>
          <div class="pd-ratios">
            <button
              v-for="r in RATIOS"
              :key="r.value"
              type="button"
              :class="['pd-ratio', { active: ratio === r.value }]"
              @click="ratio = r.value"
            >
              <b>{{ r.value }}</b>
              <span>{{ r.label }}</span>
            </button>
          </div>
        </div>

        <!-- 参考图（混图 / 换脸） -->
        <div v-if="mode !== 't2i'" class="pd-block">
          <div class="pd-block-title">
            参考图 <span class="pd-block-sub">{{ refImages.length }}/{{ maxRefs }}</span>
          </div>
          <div class="pd-refs">
            <div v-for="(img, i) in refImages" :key="i" class="pd-ref">
              <img :src="img" alt="参考图" />
              <span class="pd-ref-del" @click="removeRef(i)">×</span>
              <span class="pd-ref-index">{{ i + 1 }}</span>
            </div>
            <label v-if="refImages.length < maxRefs" class="pd-ref-add">
              <input type="file" accept="image/png,image/jpeg,image/webp" hidden @change="onPickRef" />
              <span>＋</span>
            </label>
          </div>
          <p class="pd-hint">{{ modeHint }}</p>
        </div>

        <div class="pd-block">
          <div class="pd-block-title">风格</div>
          <el-input v-model="style" size="small" placeholder="可选，如 anime / 国风" />
        </div>

        <div class="pd-block">
          <div class="pd-block-title">张数</div>
          <el-radio-group v-model="n" size="small" :disabled="isMJ">
            <el-radio-button :value="1">1</el-radio-button>
            <el-radio-button :value="2">2</el-radio-button>
            <el-radio-button :value="4">4</el-radio-button>
          </el-radio-group>
          <p v-if="isMJ" class="pd-hint">Midjourney 协议单次 1 张</p>
        </div>
      </div>

      <!-- 底部固定：费用 + 生成 -->
      <div class="pd-side-foot">
        <div class="pd-cost">
          <span class="pd-cost-model">{{ currentModel?.name || '未选模型' }}</span>
          <span class="pd-cost-balance">余额 {{ status.balance }} {{ status.token_name }}</span>
        </div>
        <el-button
          type="primary"
          size="large"
          class="pd-generate"
          :loading="generating"
          :disabled="!prompt.trim()"
          @click="generate"
        >
          生成 · {{ (currentModel?.price_call || 0) * n }} {{ status.token_name }}
        </el-button>
      </div>
    </aside>

    <!-- ===== 右：纵向作品流 ===== -->
    <section class="pd-stream">
      <header class="pd-stream-head">
        <div>
          <h2 class="pd-stream-title">作品流</h2>
          <p class="pd-stream-sub">
            <template v-if="history.length">共 {{ history.length }} 次生成，按时间倒序</template>
            <template v-else>生成结果会按时间倒序出现在这里</template>
          </p>
        </div>
        <el-button size="small" text :loading="exporting" @click="exportHistory">导出 ZIP</el-button>
      </header>

      <!-- 本次结果提示：结果本身已经在下面的流里，这里只交代花了多少 -->
      <div v-if="lastImages.length" class="pd-notice">
        <span v-if="lastFree">本次使用免费额度，余额 {{ lastBalance }} {{ status.token_name }}</span>
        <span v-else>本次扣除 {{ lastCost }} {{ status.token_name }}，余额 {{ lastBalance }}</span>
      </div>

      <StateBlock
        v-if="!history.length"
        state="empty"
        title="还没有作品"
        description="在左侧写好提示词，点「生成」即可。每次生成会在这里留下一张任务卡。"
      />

      <!-- 任务卡：每次生成一张，纵向堆叠 -->
      <article
        v-for="t in history"
        :key="t.id"
        class="pd-card"
        :class="{ 'is-latest': isLatest(t), 'is-failed': t.status === 'failed' }"
      >
        <header class="pd-card-head">
          <span class="pd-card-model">{{ t.model_name || 'AI 绘画' }}</span>
          <span class="pd-card-time">{{ formatTime(t.created_at) }}</span>
          <span v-if="t.is_public" class="pd-card-flag">已发布</span>
          <span v-if="t.status === 'failed'" class="pd-card-flag is-err">失败</span>
          <span v-if="t.cost" class="pd-card-cost">-{{ t.cost }} {{ status.token_name }}</span>
          <span class="pd-card-spacer" />
          <button class="pd-card-tool" type="button" @click="reusePrompt(t)">再来一张</button>
          <button class="pd-card-tool" type="button" @click="copyPrompt(t)">复制提示词</button>
          <button class="pd-card-tool" type="button" @click="openPublish(t)">
            {{ t.is_public ? '管理发布' : '发布' }}
          </button>
          <button class="pd-card-tool" type="button" @click="exportOne(t)">导出</button>
        </header>

        <p
          class="pd-card-prompt"
          :class="{ 'is-open': expandedIds.has(t.id) }"
          :title="expandedIds.has(t.id) ? '点击收起' : '点击展开'"
          @click="toggleExpand(t.id)"
        >
          {{ t.prompt || '（无提示词）' }}
        </p>

        <p v-if="t.status === 'failed' && t.error" class="pd-card-error">{{ t.error }}</p>

        <div class="pd-card-images" :class="`cols-${imgCount(t)}`">
          <img
            v-for="(img, i) in parseImages(t.result)"
            :key="i"
            :src="img"
            alt="AI 绘画结果"
            loading="lazy"
            @click="preview(img)"
          />
        </div>

        <footer v-if="attachmentsOf(t).length" class="pd-card-foot">
          参考图 {{ attachmentsOf(t).length }} 张
        </footer>
      </article>
    </section>

    <!-- 大图预览：自建遮罩层（按需导入模式下 el-image-viewer 不会自动注册） -->
    <div v-if="previewVisible" class="pd-preview" @click="previewVisible = false">
      <img :src="previewSrc" alt="预览" @click.stop />
      <button class="pd-preview-close" type="button" title="关闭" @click.stop="previewVisible = false">×</button>
    </div>

    <!-- 发布弹窗 -->
    <el-dialog v-model="publishVisible" title="发布到灵感广场" width="460px">
      <el-form label-width="64px">
        <el-form-item label="标题">
          <el-input v-model="publishForm.title" maxlength="128" show-word-limit placeholder="留空则取提示词前 40 字" />
        </el-form-item>
        <el-form-item label="风格">
          <el-input v-model="publishForm.style" maxlength="32" placeholder="如 写实摄影 / 国风 / 二次元（用于筛选）" />
        </el-form-item>
      </el-form>
      <p class="muted small">发布后作品与提示词对所有访客可见，可随时取消发布。</p>
      <template #footer>
        <el-button @click="publishVisible = false">取消</el-button>
        <el-button type="primary" :loading="publishing" @click="doPublish(true)">发布</el-button>
        <el-button v-if="publishForm.isPublic" type="danger" plain @click="doPublish(false)">取消发布</el-button>
      </template>
    </el-dialog>

    <AiPromptLibrary v-model="promptOpen" kind="image" :draft="prompt" @pick="onPickPrompt" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Collection } from '@element-plus/icons-vue';
import { aiHubApi, type AiHubModel, type AiHubStatus } from '@/api/aiHub';
import AiPromptLibrary from '@/components/ai/AiPromptLibrary.vue';
import StateBlock from '@/components/common/StateBlock.vue';

const props = defineProps<{ status: AiHubStatus; initialPrompt?: string }>();

const MODES = [
  { key: 't2i' as const, name: '文生图', desc: '文字直接出图' },
  { key: 'i2i' as const, name: '混图', desc: '参考图 + 描述再创作' },
  { key: 'faceswap' as const, name: '换脸', desc: '2 张图：目标人物 + 面部来源' },
];
const RATIOS = [
  { value: '1:1', label: '头像' },
  { value: '9:16', label: '竖版海报' },
  { value: '2:3', label: '手机壁纸' },
  { value: '3:4', label: '社交媒体' },
  { value: '4:3', label: '横版' },
];

const mode = ref<'t2i' | 'i2i' | 'faceswap'>('t2i');
const modelId = ref(0);
const prompt = ref('');
const ratio = ref('1:1');
const style = ref('');
const n = ref(1);
const generating = ref(false);
const refImages = ref<string[]>([]);
const lastImages = ref<string[]>([]);
const lastCost = ref(0);
const lastFree = ref(false);
const lastBalance = ref(0);
const history = ref<any[]>([]);
const promptOpen = ref(false);

const maxRefs = computed(() => (mode.value === 'faceswap' ? 2 : 3));
// 参考图同时上传一份到站内（attach_urls），这样历史里能看到「当时用了什么图」，也能随导出带走
const attachUrls = ref<string[]>([]);
const promptPlaceholder = computed(() => {
  if (mode.value === 'faceswap') return '描述换脸后想要的风格/光影；面部替换动作由前两张图决定';
  if (mode.value === 'i2i') return '描述要保留什么、改变什么，如：保留人物姿态，改为赛博朋克夜景霓虹光';
  return '描述你想生成的画面，如：雨夜霓虹下的旧书店，暖黄灯光，电影感';
});
const modeHint = computed(() =>
  mode.value === 'faceswap'
    ? '顺序很重要：第 1 张是目标人物底图，第 2 张提供面部'
    : '支持 1–3 张，仅接受本地上传（不走外链，防 SSRF）',
);

watch(
  () => props.status.image_models,
  (list) => {
    if (!modelId.value && list.length) modelId.value = list[0].id;
  },
  { immediate: true },
);
watch(mode, () => {
  if (refImages.value.length > maxRefs.value) refImages.value = refImages.value.slice(0, maxRefs.value);
});
// 灵感广场「复刻同款」注入提示词
watch(
  () => props.initialPrompt,
  (v) => { if (v) prompt.value = v; },
  { immediate: true },
);

const currentModel = computed<AiHubModel | undefined>(() =>
  props.status.image_models.find((m) => m.id === modelId.value),
);
const apiKind = computed(() => currentModel.value?.api_kind || 'openai');
const isMJ = computed(() => apiKind.value === 'mj');
watch(isMJ, (v) => { if (v) n.value = 1; });

function apiKindTag(m: AiHubModel) {
  const map: Record<string, string> = { mj: 'MJ 代理', gemini: 'Gemini', openai: 'OpenAI 兼容' };
  return map[m.api_kind || 'openai'] || '';
}

function onPickPrompt(content: string) {
  prompt.value = prompt.value.trim() ? `${prompt.value}\n\n${content}` : content;
}

function parseImages(result: string): string[] {
  try {
    return JSON.parse(result).images || [];
  } catch {
    return [];
  }
}

/** 本地图片 → data URL（仅前端读取，不上传服务器） */
function onPickRef(e: Event) {
  const input = e.target as HTMLInputElement;
  const f = input.files?.[0];
  input.value = '';
  if (!f) return;
  if (f.size > 8 * 1024 * 1024) {
    ElMessage.warning('单张参考图上限 8MB');
    return;
  }
  const reader = new FileReader();
  reader.onload = () => {
    const s = String(reader.result || '');
    if (!s.startsWith('data:image/')) {
      ElMessage.warning('仅支持图片文件');
      return;
    }
    refImages.value.push(s);
  };
  reader.readAsDataURL(f);
  // 附件落库（异步）：失败只提示，不阻断出图
  aiHubApi.attachMedia(f).then((res) => {
    const url = res.data?.data?.url;
    if (url) attachUrls.value.push(url);
  }).catch(() => {
    ElMessage.warning('参考图已用于生成，但附件未能保存（历史里看不到这张图）');
  });
}

function removeRef(i: number) {
  refImages.value.splice(i, 1);
  // data URL 与 attach URL 一一对应（同一顺序），同步删掉
  if (attachUrls.value.length > i) attachUrls.value.splice(i, 1);
}

async function generate(confirmed = false) {
  if (!modelId.value) {
    ElMessage.warning('请先选择模型');
    return;
  }
  if (mode.value === 'i2i' && !refImages.value.length) {
    ElMessage.warning('混图需先上传参考图');
    return;
  }
  if (mode.value === 'faceswap' && refImages.value.length < 2) {
    ElMessage.warning('换脸需上传 2 张图');
    return;
  }
  generating.value = true;
  try {
    const res = await aiHubApi.image({
      model_id: modelId.value,
      prompt: prompt.value.trim(),
      ratio: ratio.value,
      style: style.value.trim() || undefined,
      n: n.value,
      mode: mode.value,
      ref_images: refImages.value.length ? refImages.value : undefined,
      attach_urls: attachUrls.value.length ? attachUrls.value : undefined,
      confirm: confirmed || undefined,
    });
    if (res.data.code === 0) {
      lastImages.value = res.data.data.images || [];
      // 附件已随任务落库，清空避免串到下一次生成
      attachUrls.value = [];
      lastCost.value = res.data.data.cost;
      lastFree.value = !!res.data.data.free;
      lastBalance.value = res.data.data.balance_after;
      await loadHistory();
      ElMessage.success('生成成功');
    }
  } catch (e: any) {
    const code = e?.response?.data?.code;
    const msg = e?.response?.data?.message || '生成失败';
    if (code === 42801 && !confirmed) {
      generating.value = false;
      try {
        await ElMessageBox.confirm(msg, '确认消耗', { type: 'warning', confirmButtonText: '确认并继续', cancelButtonText: '取消' });
        await generate(true);
        return;
      } catch {
        ElMessage.info('已取消');
        return;
      }
    }
    ElMessage.error(msg);
  }
  generating.value = false;
}

// ---------- 发布到灵感广场 ----------
const publishVisible = ref(false);
const publishing = ref(false);
const publishForm = ref({ id: 0, title: '', style: '', isPublic: false });

function openPublish(t: any) {
  publishForm.value = { id: t.id, title: t.title || '', style: t.style || '', isPublic: !!t.is_public };
  publishVisible.value = true;
}

async function doPublish(publicFlag: boolean) {
  publishing.value = true;
  try {
    const res = await aiHubApi.publishMedia(publishForm.value.id, {
      public: publicFlag,
      title: publishForm.value.title,
      style: publishForm.value.style,
    });
    if (res.data.code === 0) {
      ElMessage.success((res.data as any).data?.message || '已更新');
      publishVisible.value = false;
      await loadHistory();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    publishing.value = false;
  }
}

// ── 导出与附件（gap-audit #17）──
const exporting = ref(false);

function attachmentsOf(t: any): any[] {
  try {
    const arr = JSON.parse(t.attachments || '[]');
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

function saveBlob(res: any, filename: string) {
  const blob = res.data instanceof Blob ? res.data : new Blob([res.data]);
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  a.click();
  URL.revokeObjectURL(a.href);
}

async function exportHistory() {
  exporting.value = true;
  try {
    const res = await aiHubApi.exportMedia('image');
    saveBlob(res, `ai-images-${Date.now()}.zip`);
    ElMessage.success('已导出（含 manifest 哈希清单）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败');
  } finally {
    exporting.value = false;
  }
}

async function exportOne(t: any) {
  try {
    const res = await aiHubApi.exportMediaTask(t.id, true);
    saveBlob(res, `ai-media-${t.id}.zip`);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败');
  }
}

// ---------- 作品流交互 ----------
const expandedIds = ref<Set<number>>(new Set());
function toggleExpand(id: number) {
  if (expandedIds.value.has(id)) expandedIds.value.delete(id);
  else expandedIds.value.add(id);
  // Set 是引用类型，Vue 不会因 mutate 触发更新，需换一个新实例
  expandedIds.value = new Set(expandedIds.value);
}

/** 最新一条（本次生成后 history 会重排，第一项就是它） */
function isLatest(t: any) {
  return history.value.length > 0 && history.value[0].id === t.id;
}

function imgCount(t: any) {
  const n = parseImages(t.result).length;
  return n >= 4 ? 4 : n === 2 ? 2 : 1;
}

function formatTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  const pad = (v: number) => String(v).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

async function copyPrompt(t: any) {
  try {
    await navigator.clipboard.writeText(t.prompt || '');
    ElMessage.success('提示词已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择');
  }
}

/** 再来一张：回填提示词与风格，滚动到左侧顶部（不自动扣费） */
function reusePrompt(t: any) {
  prompt.value = t.prompt || '';
  if (t.style) style.value = t.style;
  ElMessage.success('提示词已回填，确认参数后点「生成」');
}

// 大图预览
const previewVisible = ref(false);
const previewSrc = ref('');
function preview(src: string) {
  previewSrc.value = src;
  previewVisible.value = true;
}

async function loadHistory() {
  try {
    const res = await aiHubApi.mediaHistory('image', 1);
    if (res.data.code === 0) history.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

onMounted(loadHistory);
</script>

<style scoped>
/* ===== 布局：左 320 参数 + 右作品流（参照站 320 + 936） ===== */
.ai-paint {
  flex: 1;
  display: flex;
  min-height: 0;
  min-width: 0;
}

/* ---------- 左：参数 ---------- */
.pd-side {
  width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-card, #fff);
  border-right: 1px solid var(--border-color, #ebeef5);
  min-height: 0;
}
.pd-side-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-3, 12px);
  scrollbar-width: thin;
}
.pd-block { margin-bottom: var(--space-4, 16px); }
.pd-block-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-2, 8px);
  font-size: var(--text-xs, 12px);
  font-weight: 600;
  color: var(--text-light, #6b7280);
  letter-spacing: 0.02em;
}
.pd-block-sub { font-weight: 400; color: var(--text-light, #9ca3af); }
.pd-link {
  border: none;
  background: none;
  padding: 0;
  color: var(--el-color-primary, #6366f1);
  font-size: var(--text-xs, 12px);
  cursor: pointer;
  font-family: inherit;
}
.pd-empty {
  padding: 10px;
  font-size: var(--text-sm, 13px);
  color: var(--text-light, #9ca3af);
  text-align: center;
  border: 1px dashed var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 10px);
}

/* 模式 */
.pd-modes { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
.pd-mode {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: 8px 6px;
  cursor: pointer;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-card, #fff);
  font-family: inherit;
  text-align: center;
  transition: border-color 0.15s ease, background 0.15s ease;
}
.pd-mode b { font-size: var(--text-sm, 13px); color: var(--text-color, #1f2328); }
.pd-mode span { font-size: 10px; color: var(--text-light, #9ca3af); line-height: 1.3; }
.pd-mode.active {
  border-color: var(--el-color-primary, #6366f1);
  background: var(--el-color-primary-light-9, #eef2ff);
}
.pd-mode.active b { color: var(--el-color-primary, #6366f1); }

/* 模型 */
.pd-model {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  margin-bottom: 4px;
  cursor: pointer;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-card, #fff);
  font-family: inherit;
  text-align: left;
}
.pd-model:hover { border-color: var(--el-color-primary, #6366f1); }
.pd-model.active {
  border-color: var(--el-color-primary, #6366f1);
  background: var(--el-color-primary-light-9, #eef2ff);
}
.pd-model-main { display: flex; align-items: center; gap: 5px; min-width: 0; }
.pd-model-name {
  font-size: var(--text-sm, 13px);
  color: var(--text-color, #1f2328);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pd-model-tag {
  flex-shrink: 0;
  font-size: 10px;
  padding: 0 4px;
  border-radius: 3px;
  color: #8b5cf6;
  background: rgba(139, 92, 246, 0.12);
}
.pd-model-price {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--text-light, #9ca3af);
}

/* 比例 */
.pd-ratios { display: grid; grid-template-columns: repeat(5, 1fr); gap: 4px; }
.pd-ratio {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
  padding: 6px 2px;
  cursor: pointer;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 8px;
  background: var(--bg-card, #fff);
  font-family: inherit;
}
.pd-ratio b { font-size: 11px; color: var(--text-color, #1f2328); }
.pd-ratio span { font-size: 9px; color: var(--text-light, #9ca3af); transform: scale(0.92); }
.pd-ratio.active {
  border-color: var(--el-color-primary, #6366f1);
  background: var(--el-color-primary-light-9, #eef2ff);
}
.pd-ratio.active b { color: var(--el-color-primary, #6366f1); }

/* 参考图 */
.pd-refs { display: flex; flex-wrap: wrap; gap: 6px; }
.pd-ref {
  position: relative;
  width: 64px;
  height: 64px;
  border-radius: var(--radius-md, 10px);
  overflow: hidden;
  border: 1px solid var(--border-color, #e5e7eb);
}
.pd-ref img { width: 100%; height: 100%; object-fit: cover; display: block; }
.pd-ref-del {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 12px;
  line-height: 1;
  cursor: pointer;
}
.pd-ref-index {
  position: absolute;
  left: 3px;
  bottom: 2px;
  font-size: 10px;
  color: #fff;
  background: rgba(0, 0, 0, 0.5);
  border-radius: 3px;
  padding: 0 3px;
}
.pd-ref-add {
  width: 64px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--border-color, #d1d5db);
  border-radius: var(--radius-md, 10px);
  color: var(--text-light, #9ca3af);
  font-size: 18px;
  cursor: pointer;
}
.pd-ref-add:hover { border-color: var(--el-color-primary, #6366f1); color: var(--el-color-primary, #6366f1); }
.pd-hint {
  margin: 6px 0 0;
  font-size: 11px;
  line-height: 1.5;
  color: var(--text-light, #9ca3af);
}

/* 底部生成区 */
.pd-side-foot {
  flex-shrink: 0;
  padding: var(--space-3, 12px);
  border-top: 1px solid var(--border-color, #ebeef5);
  background: var(--bg-card, #fff);
}
.pd-cost {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 11px;
  color: var(--text-light, #6b7280);
}
.pd-cost-model { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pd-cost-balance { flex-shrink: 0; }
.pd-generate { width: 100%; }

/* ---------- 右：作品流 ---------- */
.pd-stream {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: var(--space-5, 24px);
  scrollbar-width: thin;
}
.pd-stream-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3, 12px);
  margin-bottom: var(--space-3, 12px);
}
.pd-stream-title {
  margin: 0 0 2px;
  font-size: var(--text-lg, 18px);
  font-weight: 600;
  color: var(--text-color, #1f2328);
}
.pd-stream-sub { margin: 0; font-size: var(--text-xs, 12px); color: var(--text-light, #9ca3af); }

.pd-notice {
  margin-bottom: var(--space-3, 12px);
  padding: 8px 12px;
  border-radius: var(--radius-md, 10px);
  font-size: var(--text-sm, 13px);
  color: #16a34a;
  background: rgba(22, 163, 74, 0.09);
  border: 1px solid rgba(22, 163, 74, 0.22);
}

/* 任务卡 */
.pd-card {
  margin-bottom: var(--space-4, 16px);
  padding: var(--space-3, 12px);
  border-radius: var(--radius-lg, 16px);
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
}
.pd-card.is-latest { border-color: var(--el-color-primary, #6366f1); box-shadow: 0 4px 16px rgba(99, 102, 241, 0.12); }
.pd-card.is-failed { border-color: rgba(239, 68, 68, 0.4); }

.pd-card-head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding-bottom: 8px;
  border-bottom: 1px dashed var(--border-color, #e5e7eb);
  font-size: 11px;
  color: var(--text-light, #9ca3af);
}
.pd-card-model { font-size: var(--text-sm, 13px); font-weight: 600; color: var(--text-color, #1f2328); }
.pd-card-flag {
  padding: 0 6px;
  border-radius: 3px;
  color: #16a34a;
  background: rgba(22, 163, 74, 0.1);
}
.pd-card-flag.is-err { color: #ef4444; background: rgba(239, 68, 68, 0.1); }
.pd-card-cost { color: var(--text-light, #9ca3af); }
.pd-card-spacer { flex: 1; }
.pd-card-tool {
  border: none;
  background: none;
  padding: 2px 6px;
  border-radius: 5px;
  font-size: 11px;
  color: var(--text-light, #6b7280);
  cursor: pointer;
  font-family: inherit;
}
.pd-card-tool:hover { background: var(--bg-page, #f5f6f8); color: var(--el-color-primary, #6366f1); }

.pd-card-prompt {
  margin: 8px 0 10px;
  font-size: var(--text-sm, 13px);
  line-height: 1.6;
  color: var(--text-color, #374151);
  cursor: pointer;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.pd-card-prompt.is-open { -webkit-line-clamp: unset; line-clamp: unset; display: block; }
.pd-card-error {
  margin: 0 0 10px;
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 12px;
  color: #b91c1c;
  background: rgba(239, 68, 68, 0.08);
}
.pd-card-images { display: grid; gap: 8px; justify-content: center; }
.pd-card-images.cols-1 { grid-template-columns: 1fr; }
.pd-card-images.cols-2 { grid-template-columns: repeat(2, 1fr); }
.pd-card-images.cols-4 { grid-template-columns: repeat(2, 1fr); }
.pd-card-images img {
  width: 100%;
  max-width: 820px;
  margin: 0 auto;
  border-radius: var(--radius-md, 10px);
  display: block;
  cursor: zoom-in;
  background: var(--bg-page, #f5f6f8);
  transition: transform 0.16s ease;
}
.pd-card-images img:hover { transform: scale(1.005); }
.pd-card-foot { margin-top: 8px; font-size: 11px; color: var(--text-light, #9ca3af); }

/* 大图预览 */
.pd-preview {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.86);
  cursor: zoom-out;
}
.pd-preview img {
  max-width: 92vw;
  max-height: 92vh;
  border-radius: var(--radius-md, 10px);
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.5);
  cursor: default;
}
.pd-preview-close {
  position: absolute;
  top: 18px;
  right: 22px;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
}
.pd-preview-close:hover { background: rgba(255, 255, 255, 0.28); }

/* ---------- 深色 ---------- */
[data-theme="dark"] .pd-side,
[data-theme="dark"] .pd-side-foot,
[data-theme="dark"] .pd-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .pd-mode,
[data-theme="dark"] .pd-model,
[data-theme="dark"] .pd-ratio { background: #0f172a; border-color: #334155; }
[data-theme="dark"] .pd-mode.active,
[data-theme="dark"] .pd-model.active,
[data-theme="dark"] .pd-ratio.active { background: rgba(99, 102, 241, 0.18); }
[data-theme="dark"] .pd-stream-title,
[data-theme="dark"] .pd-card-model,
[data-theme="dark"] .pd-card-prompt,
[data-theme="dark"] .pd-mode b,
[data-theme="dark"] .pd-model-name,
[data-theme="dark"] .pd-ratio b { color: #e2e8f0; }

/* ---------- 窄屏：上下堆叠 ---------- */
@media (max-width: 900px) {
  .ai-paint { flex-direction: column; }
  .pd-side { width: 100%; max-height: 52vh; border-right: none; border-bottom: 1px solid var(--border-color, #ebeef5); }
  .pd-stream { padding: var(--space-4, 16px); }
  .pd-card-images.cols-2, .pd-card-images.cols-4 { grid-template-columns: 1fr; }
}
</style>
