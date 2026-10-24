<template>
  <div class="reader-page" v-loading="loading" :style="{ '--reader-font-size': fontSizePx }">
    <!-- 阅读进度（长章节必备：知道还剩多少） -->
    <div class="reader-progress" :title="`已读 ${progressPct}%`">
      <span class="reader-progress-bar" :style="{ width: progressPct + '%' }" />
    </div>

    <!-- 顶部导航 -->
    <div class="reader-toolbar">
      <el-button text @click="$router.push(`/novel/${novelId}`)">
        <el-icon><ArrowLeft /></el-icon><span class="desktop-only"> 返回</span>
      </el-button>
      <el-button text @click="openToc">
        <el-icon><List /></el-icon><span class="desktop-only"> 目录</span>
      </el-button>
      <!-- 字号调节：小 / 中 / 大，按浏览器记住。中文长章节全靠这个 -->
      <el-tooltip :content="`正文字号：${FONT_LABELS[fontLevel]}（点击切换）`" placement="bottom">
        <el-button text class="reader-font-btn" @click="cycleFont">
          <span class="reader-font-glyph">A</span>
          <span class="reader-font-dot">{{ fontLevel + 1 }}</span>
        </el-button>
      </el-tooltip>
      <span class="reader-title">{{ novelTitle }} · 第{{ chapterNum }}章 {{ chapter?.title }}</span>
      <el-tag v-if="aiMarkLabel" size="small" type="warning" effect="plain" class="reader-ai-mark">{{ aiMarkLabel }}</el-tag>
      <!-- 精读：书签 / 批注 -->
      <template v-if="authStore.isLoggedIn && !isTrialExceeded">
        <el-button
          text
          :class="{ 'note-active': myBookmark }"
          :title="myBookmark ? '已收藏，点击删除书签' : '收藏本章（精读书签）'"
          @click="toggleBookmark"
        >
          <el-icon><Star /></el-icon><span class="desktop-only"> {{ myBookmark ? '已书签' : '书签' }}</span>
        </el-button>
        <el-button text :disabled="!hasSelection" :title="hasSelection ? '为选中内容添加批注' : '先选中正文文字再批注'" @click="openAnnotationDialog">
          <el-icon><EditPen /></el-icon><span class="desktop-only"> 批注</span>
        </el-button>
        <!-- 本章划线列表：原来只能靠"碰巧点到正文里的下划线"才发现公开批注，
             没人知道自己划了多少、别人划了什么。 -->
        <el-button text :class="{ 'note-active': showNotesPanel }" title="本章划线 / 批注" @click="showNotesPanel = !showNotesPanel">
          <el-icon><Notebook /></el-icon>
          <span class="desktop-only"> 划线</span>
          <span v-if="chapterNotes.length" class="note-count">{{ chapterNotes.length }}</span>
        </el-button>
        <el-button text title="我的精读（书签+批注）" @click="$router.push('/reading')">
          <el-icon><Collection /></el-icon><span class="desktop-only"> 精读</span>
        </el-button>
        <!-- 语音朗读（gap-audit #18）：服务端合成，产物落站内 uploads -->
        <el-button text :loading="ttsLoading" :title="ttsPlaying ? '停止朗读' : '朗读本章（消耗免费额度/代币）'"
          @click="toggleTTS">
          <el-icon><Microphone /></el-icon>
          <span class="desktop-only"> {{ ttsPlaying ? '停止' : '朗读' }}</span>
        </el-button>
      </template>
      <div class="reader-nav desktop-only" v-if="!isTrialExceeded">
        <el-button size="small" :disabled="chapterNum <= 1" @click="goChapter(chapterNum - 1)">
          上一章
        </el-button>
        <el-button size="small" :disabled="chapterNum >= totalChapters" @click="goChapter(chapterNum + 1)">
          下一章
        </el-button>
      </div>
    </div>

    <!-- 自定义模板Header区域 -->
    <div v-if="headerFrames.length > 0" class="template-zone template-header">
      <div v-for="f in headerFrames" :key="f.id" class="template-frame-wrapper">
        <!-- 非沙盒注入：作者本人页面直接同源嵌入（无 sandbox 限制） -->
        <iframe
          v-if="isInlineFrame(f)"
          :src="previewUrl(f.id) + '?chapter=' + chapterNum + '&novel=' + novelId"
          class="template-iframe"
          @load="onFrameLoaded($event)"
        />
        <!-- 沙盒模式：受限 iframe -->
        <iframe
          v-else
          :src="previewUrl(f.id) + '?chapter=' + chapterNum + '&novel=' + novelId"
          :sandbox="f.has_controls ? 'allow-scripts allow-forms allow-popups' : 'allow-scripts'"
          class="template-iframe"
          @load="onFrameLoaded($event)"
        />
      </div>
    </div>

    <!-- 章节目录抽屉 -->
    <el-drawer
      v-model="showToc"
      title="目录"
      direction="ltr"
      size="320px"
      :with-header="true"
      class="toc-drawer"
    >
      <div class="toc-header-info">
        <span>{{ novelTitle }}</span>
        <span class="toc-count">共 {{ chapters.length }} 章</span>
      </div>
      <div v-loading="tocLoading" class="toc-list">
        <div
          v-for="ch in chapters"
          :key="ch.chapter_number"
          class="toc-item"
          :class="{ active: ch.chapter_number === chapterNum }"
          @click="jumpToChapter(ch.chapter_number)"
        >
          <span class="toc-num">第{{ ch.chapter_number }}章</span>
          <span class="toc-title">{{ ch.title }}</span>
        </div>
        <el-empty v-if="!tocLoading && chapters.length === 0" description="暂无章节" :image-size="60" />
      </div>
    </el-drawer>

    <!-- 正文 -->
    <div class="reader-body">
      <div v-if="isTrialExceeded" class="trial-notice">
        <el-result icon="warning" title="试读已结束" sub-title="未登录用户仅可阅读前 {{ trialLimit }} 章（约30%）。请登录后继续阅读全文。">
          <template #extra>
            <el-button type="primary" @click="$router.push('/login')">去登录</el-button>
            <el-button @click="$router.push(`/novel/${novelId}`)">返回作品页</el-button>
          </template>
        </el-result>
      </div>
      <div v-else class="reader-content">
        <!-- 话本气泡体：气泡视图 / 原文视图切换 -->
        <div v-if="isScriptMode" class="reader-script-switch">
          <el-radio-group v-model="scriptView" size="small" @change="renderCurrent">
            <el-radio-button value="bubble">气泡对话</el-radio-button>
            <el-radio-button value="text">原文</el-radio-button>
          </el-radio-group>
          <span class="rs-tip">
            {{ effectiveMode === 'mixed' ? '混排作品：对话为气泡，其余段落按正文排版' : '话本作品：按「角色：台词」解析为对话气泡' }}
          </span>
        </div>
        <div v-if="chapter?.content" class="reader-md-wrap">
          <div class="reader-md markdown-body cherry-markdown cherry-previewer" data-code-block-theme="default" data-inline-code-theme="red" v-html="renderedHtml" ref="mdContainer"
            @click="onMdClick"
            @pointerdown="onBubbleDown"
            @pointerup="cancelBubblePress"
            @pointercancel="cancelBubblePress"
            @pointermove="onBubbleMove"
            @contextmenu="onBubbleContextMenu"></div>
        </div>
        <p v-else-if="chapter" style="text-align:center;color:var(--warning-color)">章节已加载但内容为空（content长度=0）</p>
        <p v-else style="text-align:center;color:var(--text-muted)">暂无内容（chapter 为 null）</p>
      </div>
    </div>

    <!-- ===== 本章划线 / 批注面板 =====
         自建侧栏而不是 el-drawer：本页是沉浸阅读场景，抽屉会挡住正文且要处理遮罩层级；
         这里做成固定右侧窄栏，点条目滚动到正文对应划线并闪一下。 -->
    <transition name="notes-slide">
      <aside v-if="showNotesPanel" class="reader-notes" aria-label="本章划线">
        <header class="rn-head">
          <span class="rn-title">本章划线 <b>{{ chapterNotes.length }}</b></span>
          <el-button text size="small" @click="showNotesPanel = false">收起</el-button>
        </header>

        <div v-if="chapterNotes.length === 0" class="rn-empty">
          <p>还没有划线。</p>
          <p class="rn-empty-hint">用鼠标选中正文里的一句话，在浮出的「批注」里写下想法 —— 公开批注别人也能看到。</p>
        </div>

        <ul v-else class="rn-list">
          <li
            v-for="n in chapterNotes"
            :key="n.id"
            class="rn-item"
            :class="{ 'is-mine': n.user_id === authStore.user?.id }"
            @click="focusNote(n)"
          >
            <p class="rn-quote">{{ n.selected_text }}</p>
            <p class="rn-content">{{ n.content || '（未填写想法）' }}</p>
            <div class="rn-meta">
              <span class="rn-who">{{ n.user_id === authStore.user?.id ? '我' : (n.username || '匿名用户') }}</span>
              <span v-if="n.visibility !== 'public'" class="rn-priv">仅自己可见</span>
              <span class="rn-time">{{ formatNoteTime(n.created_at) }}</span>
            </div>
          </li>
        </ul>
      </aside>
    </transition>

    <!-- 底部导航 -->
    <div v-if="!isTrialExceeded" class="reader-footer">
      <el-button :disabled="chapterNum <= 1" @click="goChapter(chapterNum - 1)">上一章</el-button>
      <el-button :disabled="chapterNum >= totalChapters" @click="goChapter(chapterNum + 1)">下一章</el-button>
    </div>

    <!-- 选区浮动批注按钮：鼠标/手指拉取选中正文文字后，出现在选区旁 -->
    <transition name="anno-float">
      <div
        v-if="showAnnoFloat && authStore.isLoggedIn && !isTrialExceeded"
        class="anno-float-btn"
        :style="{ left: annoFloatPos.x + 'px', top: annoFloatPos.y + 'px' }"
        @mousedown.prevent
        @touchstart.prevent.stop
        @click.stop="onAnnoFloatClick"
      >
        <el-icon :size="14"><EditPen /></el-icon>
        <span>批注</span>
      </div>
    </transition>

    <!-- 自定义模板Footer区域 -->
    <div v-if="footerFrames.length > 0" class="template-zone template-footer">
      <div v-for="f in footerFrames" :key="f.id" class="template-frame-wrapper">
        <iframe
          v-if="isInlineFrame(f)"
          :src="previewUrl(f.id) + '?chapter=' + chapterNum + '&novel=' + novelId"
          class="template-iframe"
          @load="onFrameLoaded($event)"
        />
        <iframe
          v-else
          :src="previewUrl(f.id) + '?chapter=' + chapterNum + '&novel=' + novelId"
          :sandbox="f.has_controls ? 'allow-scripts allow-forms allow-popups' : 'allow-scripts'"
          class="template-iframe"
          @load="onFrameLoaded($event)"
        />
      </div>
    </div>

    <!-- 评论 -->
    <div class="page-container" v-if="!isTrialExceeded">
      <h2 class="section-title">本章评论</h2>
      <CommentSection ref="commentSectionRef" :novel-id="novelId" :chapter-number="chapterNum" :novel-category="novelCategory" :owner-id="novelAuthorId" />
    </div>

    <!-- 书签对话框 -->
    <el-dialog v-model="showBookmarkDialog" title="🔖 添加书签" width="420px">
      <el-form label-width="72px">
        <el-form-item label="位置">第 {{ chapterNum }} 章 · {{ chapter?.title }}</el-form-item>
        <el-form-item label="备注">
          <el-input v-model="bookmarkContent" type="textarea" :rows="3" maxlength="500" placeholder="这本书写到哪里了？可留空" />
        </el-form-item>
        <el-form-item label="可见性">
          <el-switch v-model="bookmarkPublic" active-text="公开" inactive-text="私密" />
          <span class="note-hint">公开书签会展示在作品页/精读广场</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showBookmarkDialog = false">取消</el-button>
        <el-button type="primary" :loading="savingNote" @click="saveBookmark">保存书签</el-button>
      </template>
    </el-dialog>

    <!-- 批注对话框（新建） -->
    <el-dialog v-model="showAnnotationDialog" title="📝 添加批注" width="460px">
      <div class="note-selected">{{ annotationSelection }}</div>
      <el-input v-model="annotationContent" type="textarea" :rows="4" maxlength="2000" placeholder="写下你的批注/想法…" />
      <div class="note-vis-row">
        <el-switch v-model="annotationPublic" active-text="公开" inactive-text="私密" />
        <span class="note-hint">公开批注对所有人可见</span>
      </div>
      <template #footer>
        <el-button @click="showAnnotationDialog = false">取消</el-button>
        <el-button type="primary" :loading="savingNote" @click="saveAnnotation">保存批注</el-button>
      </template>
    </el-dialog>

    <!-- 批注详情（查看/编辑/删除） -->
    <el-dialog v-model="showNoteDetail" :title="detailNote?.type === 'bookmark' ? '🔖 书签' : '📝 批注'" width="460px">
      <div v-if="detailNote?.selected_text" class="note-selected">{{ detailNote.selected_text }}</div>
      <el-input v-if="detailNote?.user_id === authStore.user?.id" v-model="detailContent" type="textarea" :rows="4" maxlength="2000" />
      <p v-else class="note-detail-content">{{ detailNote?.content || '（无内容）' }}</p>
      <div v-if="detailNote?.user_id === authStore.user?.id" class="note-vis-row">
        <el-switch v-model="detailPublic" active-text="公开" inactive-text="私密" />
        <span class="note-hint">切换可见性</span>
      </div>
      <div class="note-meta">
        <span v-if="detailNote?.username">{{ detailNote.username }} · </span>
        {{ formatNoteTime(detailNote?.created_at) }}
      </div>
      <template #footer>
        <el-button v-if="detailNote?.user_id === authStore.user?.id" type="danger" plain @click="deleteDetailNote">删除</el-button>
        <el-button v-if="detailNote?.user_id === authStore.user?.id" type="primary" :loading="savingNote" @click="updateDetailNote">保存修改</el-button>
        <el-button @click="showNoteDetail = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 敏感分区确认弹窗 -->
    <SensitiveZoneGuard
      :visible="showZoneGuard"
      :zone-name="zoneGuardName"
      :is-cross-domain="zoneGuardCross"
      :custom-warning="novelWallWarning"
      @confirm="onZoneConfirmed"
      @cancel="onZoneCancelled"
    />
    <!-- 气泡长按互动（gap-audit #22）：复制 / 引用评论 / 收藏本章 -->
    <div v-if="bubbleSheet.visible" class="bubble-sheet-mask" @click.self="closeBubbleSheet">
      <div class="bubble-sheet">
        <div class="bs-head">
          <span class="bs-speaker">{{ bubbleSheet.speaker || '旁白' }}</span>
          <span class="bs-text">{{ bubbleSheet.text }}</span>
        </div>
        <div class="bs-actions">
          <button class="bs-btn" @click="bubbleCopy">📋 复制台词</button>
          <button class="bs-btn" @click="bubbleQuote">💬 引用评论</button>
          <button class="bs-btn" @click="bubbleBookmark">⭐ 收藏本章</button>
          <button class="bs-btn bs-cancel" @click="closeBubbleSheet">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import { useAiMark } from '@/composables/useAiMark';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, chapterApi, type Chapter } from '@/api/novel';
import { bookshelfApi } from '@/api/bookshelf';
import { readingApi, type ReadingNote } from '@/api/reading';
import { useAuthStore } from '@/stores/auth';
import { renderMarkdown, renderMermaidBlocks, renderChemfigBlocks } from '@/markdown/renderer';
import CommentSection from '@/components/CommentSection.vue';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import { shouldShowGuard, markZoneConfirmed, setLastZone } from '@/utils/sensitiveZone';
import { frameApi } from '@/api/frame';
import { ElMessage } from 'element-plus';
import { aiCostApi } from '@/api/costrisk';
import { ArrowLeft, List, Star, EditPen, Notebook, Microphone, Collection } from '@element-plus/icons-vue';

const authStore = useAuthStore();

const route = useRoute();
const router = useRouter();

const novelId = ref(Number(route.params.id));
const chapterNum = ref(Number(route.params.chapter));
const loading = ref(false);
const chapter = ref<Chapter | null>(null);
// AI 生成标识（章节详情徽标）
const { label: aiMarkLabel, load: loadAiMark } = useAiMark();
const htmlContent = ref('');
const novelTitle = ref('');
const novelCategory = ref('');
const novelCategories = ref<string[]>([]);
const novelAuthorId = ref(0);
const novelWallEnabled = ref(true);
const novelWallWarning = ref('');
const totalChapters = ref(0);

// 章节目录抽屉
const showToc = ref(false);
const tocLoading = ref(false);
const chapters = ref<Chapter[]>([]);

async function openToc() {
  showToc.value = true;
  if (chapters.value.length === 0) {
    tocLoading.value = true;
    try {
      const res = await chapterApi.getChapters(novelId.value);
      chapters.value = res.data.data || [];
      await nextTick();
      scrollToActiveChapter();
    } catch { /* 目录加载失败静默 */ }
    tocLoading.value = false;
  } else {
    await nextTick();
    scrollToActiveChapter();
  }
}

function scrollToActiveChapter() {
  const el = document.querySelector('.toc-item.active');
  if (el) el.scrollIntoView({ block: 'center' });
}

function jumpToChapter(num: number) {
  showToc.value = false;
  if (num === chapterNum.value) return;
  goChapter(num);
}

// 自定义模板
const headerFrames = ref<any[]>([]);
const footerFrames = ref<any[]>([]);
const frameLoaded = ref<Record<number, boolean>>({});

function previewUrl(frameId: number) { return frameApi.getPreview(frameId); }

// 非沙盒注入判定：仅当模板声明 inline 且当前访问者是模板作者本人时生效
function isInlineFrame(f: any): boolean {
  return f.run_mode === 'inline' && authStore.user?.id === f.user_id;
}

function onFrameLoaded(e: Event) {
  const iframe = e.target as HTMLIFrameElement;
  try {
    // 自适应高度：根据iframe内部内容调整
    const body = iframe.contentDocument?.body;
    if (body) {
      const h = body.scrollHeight;
      if (h > 60) iframe.style.height = h + 'px';
    }
  } catch {
    // 跨域限制下无法读取contentDocument，使用默认高度
    iframe.style.height = '200px';
  }
}

async function loadFrames() {
  try {
    const res = await frameApi.getByNovel(novelId.value);
    if (res.data.code === 0) {
      const frames = res.data.data || [];
      // position 为 footer 的模板放正文下方；空值或其它值默认 header（向后兼容）
      headerFrames.value = frames.filter((f: any) => f.position !== 'footer');
      footerFrames.value = frames.filter((f: any) => f.position === 'footer');
    }
  } catch { /* 静默失败，模板非必需 */ }
}

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);

const trialLimit = computed(() => Math.min(5, Math.ceil(totalChapters.value * 0.3)));
const isTrialExceeded = computed(() => !authStore.isLoggedIn && chapterNum.value > trialLimit.value);

function goChapter(num: number) {
  router.push(`/author/${novelAuthorId.value}/novel/${novelId.value}/read/${num}`);
}

async function loadChapter() {
  loading.value = true;
  try {
    const [chRes, novelRes] = await Promise.all([
      chapterApi.getChapter(novelId.value, chapterNum.value),
      novelApi.getNovel(novelId.value),
    ]);
    const detail = chRes.data.data;
    chapter.value = detail?.chapter || null;
    // 章节加载后取 AI 生成标识（无标记则徽标不显示）
    if (chapter.value?.id) loadAiMark('novel_chapter', chapter.value.id);
    htmlContent.value = detail?.html_content || '';
    novelMode.value = (detail as any)?.novel_mode || novelRes.data.data.mode || 'prose';
    chapterMode.value = (detail as any)?.chapter_mode || '';
    if (!chapter.value) {
      console.error('章节内容为空', detail);
    }
    novelTitle.value = novelRes.data.data.title;
    novelCategory.value = novelRes.data.data.category || '';
    novelCategories.value = novelRes.data.data.categories || (novelRes.data.data.category ? [novelRes.data.data.category] : []);
    novelAuthorId.value = novelRes.data.data.author_id || 0;
    novelWallEnabled.value = novelRes.data.data.wall_enabled !== false;
    novelWallWarning.value = novelRes.data.data.wall_warning || '';
    totalChapters.value = novelRes.data.data.total_chapters;
    localStorage.setItem(
      `reading-progress-${novelId.value}`,
      JSON.stringify({ chapter: chapterNum.value, time: Date.now() })
    );
    window.scrollTo(0, 0);

    // 加载自定义模板
    loadFrames();

    // 同步阅读进度到后端书架（静默失败）
    if (authStore.isLoggedIn) {
      try {
        await bookshelfApi.updateProgress(novelId.value, chapterNum.value);
      } catch {
        // 书架同步失败不阻断阅读
      }
    }

    // 敏感分区检查
    await checkSensitiveZone();

    // 精读：加载本章书签/批注 + 公开批注（渲染后高亮）
    loadNotes();
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function checkSensitiveZone() {
  // author custom wall: if wall_warning present and wall_enabled, trigger directly
  if (novelWallEnabled.value !== false && novelWallWarning.value) {
    zoneGuardName.value = novelCategory.value || novelTitle.value || 'wall';
    zoneGuardCross.value = false;
    showZoneGuard.value = true;
    return;
  }
  const cats = novelCategories.value.length > 0 ? novelCategories.value : (novelCategory.value ? [novelCategory.value] : []);
  if (cats.length === 0) return;
  const guard = await shouldShowGuard(cats, {
    authorId: novelAuthorId.value,
    userId: authStore.user?.id, wallEnabled: novelWallEnabled.value,
  });
  if (guard?.needed) {
    zoneGuardName.value = guard.zoneName;
    zoneGuardCross.value = guard.isCrossDomain;
    showZoneGuard.value = true;
  }
}

function onZoneConfirmed() {
  markZoneConfirmed(zoneGuardName.value);
  setLastZone(zoneGuardName.value);
  showZoneGuard.value = false;
}

function onZoneCancelled() {
  showZoneGuard.value = false;
  router.push('/');
}

watch(
  () => route.params.chapter,
  (newVal) => {
    chapterNum.value = Number(newVal);
    loadChapter();
  }
);

// ── Markdown 渲染 + Mermaid 异步处理 ──
const mdContainer = ref<HTMLElement | null>(null);
const renderedHtml = ref('');

// ---------- 语音朗读（服务端合成 POST /api/ai/speech，分段顺序播放）----------
// 后端限制单次 ≤2000 字且按次计费，所以按句切段（每段 ≤1800 字）、一次最多 8 段，
// 避免一次点击就把免费额度/代币刷光；播放中可以随时停止，离开页面必须 stopTTS 释放音频。
const TTS_MAX_RUNES = 1800;
const TTS_MAX_CHUNKS = 8;
const ttsLoading = ref(false);
const ttsPlaying = ref(false);
const ttsQueue = ref<string[]>([]);
let ttsAudio: HTMLAudioElement | null = null;
let ttsCursor = 0;

/** 取可朗读的纯文本：剔除代码块/图片/HTML 标签/Markdown 标记（代码念出来没有意义） */
function ttsPlainText(): string {
  const raw = chapter.value?.content || '';
  return raw
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/<[^>]+>/g, ' ')
    .replace(/[#>*_~`|]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
}

/** 按句切段（不用 lookbehind 正则，避免旧 Safari 解析期报错） */
function ttsSplit(text: string): string[] {
  const sentences = text.match(/[^。！？；.!?;]+[。！？；.!?;]?/g) || [text];
  const out: string[] = [];
  let buf = '';
  for (const sRaw of sentences) {
    let s = sRaw.trim();
    if (!s) continue;
    while (s.length > TTS_MAX_RUNES) {
      if (buf) { out.push(buf); buf = ''; }
      out.push(s.slice(0, TTS_MAX_RUNES));
      s = s.slice(TTS_MAX_RUNES);
    }
    if ((buf + s).length > TTS_MAX_RUNES) {
      if (buf) out.push(buf);
      buf = s;
    } else {
      buf += s;
    }
  }
  if (buf.trim()) out.push(buf);
  return out.slice(0, TTS_MAX_CHUNKS);
}

function stopTTS() {
  ttsPlaying.value = false;
  ttsLoading.value = false;
  ttsQueue.value = [];
  ttsCursor = 0;
  if (ttsAudio) {
    ttsAudio.onended = null;
    ttsAudio.onerror = null;
    ttsAudio.pause();
    ttsAudio.src = '';
    ttsAudio = null;
  }
}

async function playTTSAt(i: number) {
  const queue = ttsQueue.value;
  if (i >= queue.length) { stopTTS(); return; }
  ttsCursor = i;
  ttsLoading.value = true;
  try {
    const res = await aiCostApi.speech({ text: queue[i], kind: 'tts' });
    const url = res.data?.data?.url;
    if (!url) throw new Error('未返回音频地址');
    if (ttsCursor !== i) return; // 期间被停止/重新开始
    const audio = new Audio(url);
    ttsAudio = audio;
    audio.onended = () => { if (ttsAudio === audio) playTTSAt(i + 1); };
    audio.onerror = () => { ElMessage.error('音频播放失败'); stopTTS(); };
    await audio.play();
    ttsPlaying.value = true;
  } catch (e: any) {
    stopTTS();
    const msg = e?.response?.data?.message || e?.message || '语音合成失败';
    ElMessage.error(msg);
  } finally {
    ttsLoading.value = false;
  }
}

async function toggleTTS() {
  if (ttsPlaying.value || ttsLoading.value) { stopTTS(); return; }
  if (!authStore.isLoggedIn) {
    ElMessage.warning('登录后才能使用朗读');
    router.push('/login');
    return;
  }
  const text = ttsPlainText();
  if (!text) { ElMessage.warning('本章没有可朗读的正文'); return; }
  const chunks = ttsSplit(text);
  if (!chunks.length) { ElMessage.warning('本章没有可朗读的正文'); return; }
  ttsQueue.value = chunks;
  await playTTSAt(0);
}

// 叙事形态：章节形态优先（script 话本 / mixed 散文与话本混排），未设置则继承作品
// ---------- 阅读字号（小 / 中 / 大，持久化）----------
const FONT_SIZES = ['0.95rem', '1.05rem', '1.2rem'];
const FONT_LABELS = ['小', '中', '大'];
const FONT_KEY = 'nvs_reader_font_level';
const fontLevel = ref(Math.min(2, Math.max(0, Number(localStorage.getItem(FONT_KEY) ?? 1) || 0)));
const fontSizePx = computed(() => FONT_SIZES[fontLevel.value]);

function cycleFont() {
  fontLevel.value = (fontLevel.value + 1) % FONT_SIZES.length;
  localStorage.setItem(FONT_KEY, String(fontLevel.value));
}

// ---------- 阅读进度 ----------
const progressPct = ref(0);
function updateProgress() {
  const doc = document.documentElement;
  const total = doc.scrollHeight - window.innerHeight;
  progressPct.value = total <= 0 ? 0 : Math.min(100, Math.max(0, Math.round((window.scrollY / total) * 100)));
}

const novelMode = ref('prose');
const chapterMode = ref('');
const effectiveMode = computed(() => chapterMode.value || novelMode.value || 'prose');
const isScriptMode = computed(() => effectiveMode.value === 'script' || effectiveMode.value === 'mixed');
const scriptView = ref<'bubble' | 'text'>('bubble');

// 章节标题大字（明确需求，不可去掉）：
// 阅读器正文必须与编辑器/管理列表一样显示章节标题的大字（一级标题）。
// 规则：
//   1. 参照上传的 md 文件——若正文首个非空行已是标题且包含章节标题 → 原样保留（不重复、不删除）；
//   2. 用户输入/存量数据库章节——正文没有标题时，渲染期前置注入「# 第N章 标题」一级标题。
// 说明：注入发生在渲染期，不改写数据库内容文件，不影响哈希/签名校验；
// 因此「之前已在数据库的章节」同样自动生效，无需逐一改数据。
function ensureChapterTitle(md: string, num: number, title: string): string {
  const t = (title || '').trim();
  if (!t || !md) return md;
  // 首个非空行
  const firstLine = (md.replace(/^\s+/, '').split('\n').find((l) => l.trim() !== '') || '').trim();
  if (/^#{1,6}\s+/.test(firstLine)) {
    const heading = firstLine.replace(/^#{1,6}\s+/, '').replace(/[*_`~#]/g, '').trim();
    if (heading && (heading.includes(t) || t.includes(heading))) {
      return md; // md 已自带标题（上传文件场景），保持原样
    }
  }
  // 前置注入一级标题（用户输入/存量数据兜底）
  const prefix = `第${num}章`;
  const display = t.startsWith(prefix) ? t : `${prefix} ${t}`;
  return `# ${display}\n\n${md}`;
}

// 渲染入口：话本气泡视图直接用服务端渲染结果，其余走 Markdown
function renderCurrent() {
  const content = chapter.value?.content || '';
  if (!content) { renderedHtml.value = ''; return; }
  if (isScriptMode.value && scriptView.value === 'bubble') {
    renderedHtml.value = htmlContent.value || '';
    return;
  }
  try {
    const md = ensureChapterTitle(content, chapterNum.value, chapter.value?.title || '');
    const html = renderMarkdown(md);
    renderedHtml.value = html;
  } catch (e: any) {
    console.error('[NVS] Reader: renderMarkdown error:', e);
    renderedHtml.value = '<p style=color:red>渲染失败: ' + (e.message || String(e)) + '</p>';
  }
  nextTick(() => {
    if (mdContainer.value) {
      renderMermaidBlocks(mdContainer.value);
      renderChemfigBlocks(mdContainer.value);
    }
  });
}

watch(() => chapter.value?.content, () => renderCurrent());

// ── 精读：书签 + 批注（公开/私密） ──
const myBookmark = ref<ReadingNote | null>(null);
const myAnnotations = ref<ReadingNote[]>([]);
const publicNotes = ref<ReadingNote[]>([]);
const showNotesPanel = ref(false);

/**
 * 本章划线（自己的 + 公开的，按创建时间倒序）。
 * 去重：自己已公开的批注会同时出现在 myAnnotations 与 publicNotes 里，
 * 不去重会出现两条一模一样。
 */
const chapterNotes = computed<ReadingNote[]>(() => {
  const seen = new Set<number>();
  const all: ReadingNote[] = [];
  for (const n of [...myAnnotations.value, ...publicNotes.value]) {
    if (!n.selected_text || seen.has(n.id)) continue;
    seen.add(n.id);
    all.push(n);
  }
  return all.sort((a, b) => (b.created_at || '').localeCompare(a.created_at || ''));
});

/** 点侧栏条目：滚动到正文对应划线并闪一下（找不到就提示，不要静默失败） */
async function focusNote(n: ReadingNote) {
  const el = mdContainer.value?.querySelector(`mark.note-ann[data-note-id="${n.id}"]`) as HTMLElement | null;
  if (!el) {
    ElMessage.warning('正文里没找到这条划线（可能改写后文本已变化）');
    return;
  }
  el.scrollIntoView({ block: 'center', behavior: 'smooth' });
  el.classList.add('is-flash');
  window.setTimeout(() => el.classList.remove('is-flash'), 1400);
}

const showBookmarkDialog = ref(false);
const bookmarkContent = ref('');
const bookmarkPublic = ref(false);
const showAnnotationDialog = ref(false);
const annotationSelection = ref('');
const annotationContent = ref('');
const annotationPublic = ref(false);
const showNoteDetail = ref(false);
const detailNote = ref<ReadingNote | null>(null);
const detailContent = ref('');
const detailPublic = ref(false);
const savingNote = ref(false);

// 是否已选中正文文字（批注按钮可用）
const hasSelection = computed(() => {
  const sel = window.getSelection?.();
  const txt = sel?.toString()?.trim();
  return !!txt && !!mdContainer.value && mdContainer.value.contains(sel!.anchorNode);
});

// ====== 选区浮动批注按钮 ======
// 用户用光标/手指拉取选中正文中的一段文字后，在选区旁弹出「批注」按钮，
// 点击即打开批注弹窗（无需到顶部工具栏找灰色按钮）。
const showAnnoFloat = ref(false);
const annoFloatPos = ref({ x: 0, y: 0 });

function updateAnnoFloat() {
  const sel = window.getSelection?.();
  const txt = sel?.toString()?.trim();
  if (!txt || !mdContainer.value || !mdContainer.value.contains(sel!.anchorNode)) {
    showAnnoFloat.value = false;
    return;
  }
  try {
    const range = sel!.getRangeAt(0);
    const rect = range.getBoundingClientRect();
    // 选区上方：x 对齐选区末尾，y 在选区上方 8px
    let x = rect.right;
    let y = rect.top - 8;
    // 超出视口右/上边缘时折回，保证按钮可见
    if (x > window.innerWidth - 90) x = rect.left - 8;
    if (y < 70) y = rect.bottom + 10; // 顶部 70px 为工具栏区域
    annoFloatPos.value = { x: Math.max(8, x), y: Math.max(70, y) };
    showAnnoFloat.value = true;
  } catch {
    showAnnoFloat.value = false;
  }
}

function onReaderMouseUp(e: MouseEvent) {
  // 点击批注划线等非选区操作不弹出
  if ((e.target as HTMLElement)?.closest?.('mark.note-ann')) return;
  // 稍作延迟，等待浏览器确定选区
  setTimeout(updateAnnoFloat, 20);
}

// ====== 气泡对话长按菜单 ======
// ⚠️ 模板与样式早就写好了，脚本里却一直缺这组状态与方法 —— 渲染时访问 bubbleSheet.visible
// 直接抛 TypeError，整个阅读器白屏（本页对所有人不可用）。这里补齐。
// 评论区实例引用：模板里 ref="commentSectionRef" 需要脚本里有同名 ref 才绑得上
const commentSectionRef = ref<{ setQuote?: (s: string) => void } | null>(null);
const bubbleSheet = reactive({ visible: false, speaker: '', text: '' });
let bubblePressTimer: number | undefined;
let bubblePressFrom: { x: number; y: number } | null = null;

function bubbleElFrom(t: EventTarget | null): HTMLElement | null {
  return ((t as HTMLElement | null)?.closest?.('.script-bubble') as HTMLElement | null) || null;
}

function openBubbleSheet(el: HTMLElement) {
  // 话本气泡结构：.sb-name 角色名 / .sb-text 台词 / .sb-emote 动作描写
  const speaker = el.querySelector('.sb-name')?.textContent?.trim() || '';
  const text = el.querySelector('.sb-text')?.textContent?.trim() || '';
  const emote = el.querySelector('.sb-emote')?.textContent?.trim() || '';
  if (!text && !emote) return;
  bubbleSheet.speaker = speaker;
  bubbleSheet.text = [text, emote].filter(Boolean).join(' ');
  bubbleSheet.visible = true;
  showAnnoFloat.value = false; // 与选区浮动按钮互斥
}

function onBubbleDown(e: PointerEvent) {
  const el = bubbleElFrom(e.target);
  if (!el) return;
  bubblePressFrom = { x: e.clientX, y: e.clientY };
  if (bubblePressTimer) window.clearTimeout(bubblePressTimer);
  // 长按 480ms 弹菜单；期间手指/鼠标移动超过 8px 视为滑动，取消
  bubblePressTimer = window.setTimeout(() => {
    bubblePressTimer = undefined;
    openBubbleSheet(el);
  }, 480);
}

function cancelBubblePress() {
  if (bubblePressTimer) {
    window.clearTimeout(bubblePressTimer);
    bubblePressTimer = undefined;
  }
  bubblePressFrom = null;
}

function onBubbleMove(e: PointerEvent) {
  if (!bubblePressFrom) return;
  if (Math.abs(e.clientX - bubblePressFrom.x) > 8 || Math.abs(e.clientY - bubblePressFrom.y) > 8) {
    cancelBubblePress();
  }
}

/** 桌面端：右键气泡也能唤出同一菜单（长按是移动端习惯） */
function onBubbleContextMenu(e: MouseEvent) {
  const el = bubbleElFrom(e.target);
  if (!el) return;
  e.preventDefault();
  cancelBubblePress();
  openBubbleSheet(el);
}

function closeBubbleSheet() {
  bubbleSheet.visible = false;
}

async function bubbleCopy() {
  const t = bubbleSheet.text;
  closeBubbleSheet();
  try {
    await navigator.clipboard.writeText(t);
    ElMessage.success('台词已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择');
  }
}

/** 引用评论：把台词填进下方评论区（CommentSection 暴露了 setQuote） */
function bubbleQuote() {
  const t = bubbleSheet.text;
  closeBubbleSheet();
  const cmp = commentSectionRef.value as { setQuote?: (s: string) => void } | null;
  if (cmp?.setQuote) {
    cmp.setQuote(t);
    ElMessage.success('已填入评论框');
  } else {
    ElMessage.info('已复制台词，可在评论区粘贴');
    navigator.clipboard?.writeText(t).catch(() => {});
  }
}

function bubbleBookmark() {
  closeBubbleSheet();
  toggleBookmark();
}

function onAnnoFloatClick() {
  showAnnoFloat.value = false;
  openAnnotationDialog();
}

// 点击页面其他区域时收起浮动按钮（正文容器外）
function onGlobalMouseDown() {
  showAnnoFloat.value = false;
}

function formatNoteTime(iso?: string) {
  if (!iso) return '';
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

// 加载本章我的书签/批注 + 公开批注，并注入高亮
async function loadNotes() {
  try {
    if (authStore.isLoggedIn) {
      const [mineRes, pubRes] = await Promise.all([
        readingApi.listMine({ novel_id: novelId.value, chapter_number: chapterNum.value }),
        readingApi.listPublic({ novel_id: novelId.value, chapter_number: chapterNum.value }),
      ]);
      const mine = mineRes.data.data || [];
      myBookmark.value = mine.find((n: ReadingNote) => n.type === 'bookmark') || null;
      myAnnotations.value = mine.filter((n: ReadingNote) => n.type === 'annotation');
      publicNotes.value = pubRes.data.data || [];
    } else {
      const pubRes = await readingApi.listPublic({ novel_id: novelId.value, chapter_number: chapterNum.value });
      publicNotes.value = pubRes.data.data || [];
      myBookmark.value = null;
      myAnnotations.value = [];
    }
  } catch {
    publicNotes.value = [];
    myAnnotations.value = [];
    myBookmark.value = null;
  }
  await nextTick();
  applyNoteHighlights();
}

// 在渲染后的正文中注入 <mark> 划线（每笔记标第一个匹配片段）
// 划线批注语义：选中某句子 → 生成下划线标记；公开批注对所有人可见，
// 点击划线可查看「批注内容 + 批注人 + 时间」（悬浮 title 也显示批注人）。
function applyNoteHighlights() {
  const root = mdContainer.value;
  if (!root) return;
  // 清除旧标记（还原文本）
  root.querySelectorAll('mark.note-ann').forEach((el) => {
    const parent = el.parentNode;
    if (parent) {
      parent.replaceChild(document.createTextNode(el.textContent || ''), el);
      parent.normalize();
    }
  });
  const anns = [...myAnnotations.value, ...publicNotes.value];
  // 同一段文字在本章可能出现多次（例如"他笑了"）：只取第一处会导致两条不同批注
  // 叠在同一个位置、后面那条把前面那条顶掉。这里按出现次序依次落位。
  const usedCount = new Map<string, number>();
  for (const n of anns) {
    const target = (n.selected_text || '').trim();
    if (!target) continue;
    const nth = usedCount.get(target) ?? 0;
    usedCount.set(target, nth + 1);

    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let node: Text | null;
    let remain = nth; // 跳过前面 nth 处
    while ((node = walker.nextNode() as Text | null)) {
      let from = 0;
      let idx = -1;
      // 单个文本节点里也可能出现多次
      for (;;) {
        const found = node.textContent?.indexOf(target, from) ?? -1;
        if (found < 0) break;
        if (remain === 0) { idx = found; break; }
        remain -= 1;
        from = found + target.length;
      }
      if (idx < 0) continue;
      const range = document.createRange();
      range.setStart(node, idx);
      range.setEnd(node, idx + target.length);
      const mark = document.createElement('mark');
      mark.className = 'note-ann' + (n.user_id === authStore.user?.id ? ' note-mine' : ' note-public');
      mark.dataset.noteId = String(n.id);
      // 划线悬浮提示：公开批注显示批注人，私有批注标注"仅自己可见"
      const who = n.username || '匿名用户';
      mark.title = n.visibility === 'public'
        ? `✏️ ${who} 的公开批注（点击查看）`
        : `✏️ ${who} 的批注（仅自己可见）`;
      mark.textContent = target;
      try {
        range.deleteContents();
        range.insertNode(mark);
      } catch { /* 忽略单个失败 */ }
      break;
    }
  }
}

// 正文点击委托：点中批注高亮 → 打开详情
function onMdClick(e: MouseEvent) {
  const el = e.target as HTMLElement;
  const mark = el.closest?.('mark.note-ann') as HTMLElement | null;
  if (!mark) return;
  const id = Number(mark.dataset.noteId);
  const note = [...myAnnotations.value, ...publicNotes.value].find((n) => n.id === id);
  if (!note) return;
  detailNote.value = note;
  detailContent.value = note.content || '';
  detailPublic.value = note.visibility === 'public';
  showNoteDetail.value = true;
}

// 书签：已收藏则删除，未收藏则弹窗
function toggleBookmark() {
  if (myBookmark.value) {
    readingApi.remove(myBookmark.value.id).then(() => {
      myBookmark.value = null;
      ElMessage.success('已取消书签');
    }).catch(() => ElMessage.error('操作失败'));
    return;
  }
  bookmarkContent.value = '';
  bookmarkPublic.value = false;
  showBookmarkDialog.value = true;
}

async function saveBookmark() {
  savingNote.value = true;
  try {
    await readingApi.create({
      novel_id: novelId.value, chapter_number: chapterNum.value, type: 'bookmark',
      content: bookmarkContent.value, visibility: bookmarkPublic.value ? 'public' : 'private',
    });
    ElMessage.success('书签已保存');
    showBookmarkDialog.value = false;
    await loadNotes();
  } catch { ElMessage.error('保存失败'); }
  finally { savingNote.value = false; }
}

function openAnnotationDialog() {
  const sel = window.getSelection?.();
  const txt = sel?.toString()?.trim() || '';
  if (!txt || !mdContainer.value?.contains(sel!.anchorNode)) {
    ElMessage.warning('请先在正文中选中文字，再点「批注」');
    return;
  }
  annotationSelection.value = txt;
  annotationContent.value = '';
  annotationPublic.value = false;
  showAnnotationDialog.value = true;
}

async function saveAnnotation() {
  if (!annotationContent.value.trim()) { ElMessage.warning('请填写批注内容'); return; }
  savingNote.value = true;
  try {
    await readingApi.create({
      novel_id: novelId.value, chapter_number: chapterNum.value, type: 'annotation',
      content: annotationContent.value, selected_text: annotationSelection.value,
      visibility: annotationPublic.value ? 'public' : 'private',
    });
    ElMessage.success('批注已保存');
    showAnnotationDialog.value = false;
    await loadNotes();
  } catch { ElMessage.error('保存失败'); }
  finally { savingNote.value = false; }
}

async function updateDetailNote() {
  if (!detailNote.value) return;
  savingNote.value = true;
  try {
    await readingApi.update(detailNote.value.id, {
      content: detailContent.value,
      visibility: detailPublic.value ? 'public' : 'private',
    });
    ElMessage.success('已更新');
    showNoteDetail.value = false;
    await loadNotes();
  } catch { ElMessage.error('更新失败'); }
  finally { savingNote.value = false; }
}

async function deleteDetailNote() {
  if (!detailNote.value) return;
  try {
    await readingApi.remove(detailNote.value.id);
    ElMessage.success('已删除');
    showNoteDetail.value = false;
    await loadNotes();
  } catch { ElMessage.error('删除失败'); }
}

onMounted(() => {
  // 🔀 旧URL格式重定向：/novel/:id → 先获取作者再跳转
  if (!route.params.authorId) {
    novelApi.getNovel(novelId.value).then(res => {
      const aid = res.data?.data?.author_id;
      if (aid) router.replace(`/author/${aid}/novel/${novelId.value}/read/${chapterNum.value}`);
    }).catch(() => {});
  }
  loadChapter();
  // 选区浮动批注按钮：正文 mouseup 检测选区，document mousedown 收起
  document.addEventListener('mouseup', onReaderMouseUp);
  document.addEventListener('mousedown', onGlobalMouseDown);
  // 阅读进度条
  updateProgress();
  window.addEventListener('scroll', updateProgress, { passive: true });
  window.addEventListener('resize', updateProgress);
});

onBeforeUnmount(() => {
  stopTTS();
  document.removeEventListener('mouseup', onReaderMouseUp);
  document.removeEventListener('mousedown', onGlobalMouseDown);
  window.removeEventListener('scroll', updateProgress);
  window.removeEventListener('resize', updateProgress);
});
</script>

<style scoped>
/* 气泡长按互动弹层 */
.bubble-sheet-mask {
  position: fixed; inset: 0; z-index: 3000;
  background: rgb(0 0 0 / 35%);
  display: flex; align-items: flex-end; justify-content: center;
}
.bubble-sheet {
  width: 100%; max-width: 560px;
  background: var(--bg-card);
  border-radius: 14px 14px 0 0;
  padding: 14px 16px calc(14px + env(safe-area-inset-bottom, 0px));
  box-shadow: 0 -6px 24px rgb(0 0 0 / 18%);
}
.bs-head { display: flex; gap: 8px; align-items: baseline; margin-bottom: 10px; }
.bs-speaker { font-weight: 700; font-size: 0.86rem; white-space: nowrap; }
.bs-text {
  font-size: 0.84rem; color: var(--text-muted); line-height: 1.5;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}
.bs-actions { display: grid; grid-template-columns: repeat(2, 1fr); gap: 8px; }
.bs-btn {
  border: 1px solid var(--border-color); background: var(--bg-muted);
  border-radius: 10px; padding: 10px 12px; font-size: 0.86rem; cursor: pointer;
  color: var(--text-main);
}
.bs-btn:hover { border-color: var(--el-color-primary); }
.bs-cancel { grid-column: span 2; color: var(--text-muted); }

.reader-page {
  background: var(--reader-bg);
  min-height: calc(100vh - 60px);
}

.reader-toolbar {
  position: sticky;
  top: 60px;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 24px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-color);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.05);
}

[data-theme="dark"] .reader-toolbar {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
}

.reader-ai-mark { margin-left: 8px; }
.reader-title {
  flex: 1;
  font-weight: 500;
  color: var(--text-color);
}

.reader-body {
  padding: 40px 16px;
}

.template-zone {
  max-width: 800px;
  margin: 0 auto 12px;
}
.template-header { padding: 0 16px; margin-top: 8px; }
.template-footer { padding: 0 16px; margin-bottom: 16px; }
.template-frame-wrapper {
  border-radius: 8px; overflow: hidden;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  margin-bottom: 8px;
}
.template-iframe {
  width: 100%; min-height: 150px; border: none;
  display: block;
}

/* ===== 阅读进度条（贴顶，2px，不抢视线） ===== */
.reader-progress {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  z-index: 60;
  background: transparent;
  pointer-events: none;
}
.reader-progress-bar {
  display: block;
  height: 100%;
  width: 0;
  background: var(--gradient-primary, linear-gradient(90deg, #6366f1, #8b5cf6));
  transition: width 0.12s linear;
}

/* ===== 字号按钮 ===== */
.reader-font-btn { display: inline-flex; align-items: center; gap: 1px; }
.reader-font-glyph { font-size: 15px; font-weight: 600; line-height: 1; }
.reader-font-dot {
  font-size: 9px;
  line-height: 1;
  padding: 1px 3px;
  border-radius: 3px;
  background: var(--bg-page, #f0f1f4);
  color: var(--text-light, #6b7280);
}

.reader-content {
  max-width: 800px;
  margin: 0 auto;
  padding: 32px;
  background: var(--bg-card);
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
}

[data-theme="dark"] .reader-content {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
}

.reader-md-wrap {
  min-height: 200px;
}

/* ====== 话本气泡体（script mode）：服务端渲染的 .script-chapter 结构 ====== */
.reader-script-switch {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.rs-tip {
  font-size: 0.78rem;
  color: var(--text-muted);
}

.reader-md :deep(.script-chapter) {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 0;
}

/* 场景/动作提示：居中灰字斜体 */
.reader-md :deep(.script-stage) {
  text-align: center;
  font-size: 0.85rem;
  color: var(--text-muted);
  font-style: italic;
  padding: 6px 10px;
  background: var(--bg-soft);
  border-radius: 6px;
}

/* 旁白：居中正文 */
.reader-md :deep(.script-narration) {
  text-align: center;
  font-size: 0.95rem;
  line-height: 1.9;
  color: var(--text-color);
  opacity: 0.9;
  padding: 4px 12px;
}

/* 对话气泡：左右分侧 */
.reader-md :deep(.script-bubble) {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  max-width: 82%;
}
.reader-md :deep(.script-bubble.sb-right) {
  flex-direction: row-reverse;
  margin-left: auto;
}
.reader-md :deep(.sb-avatar) {
  flex-shrink: 0;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.95rem;
  font-weight: 600;
  color: #fff;
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
}
.reader-md :deep(.sb-right .sb-avatar) {
  background: var(--gradient-accent, linear-gradient(135deg, #f59e0b, #ef4444));
}
.reader-md :deep(.sb-body) {
  padding: 8px 12px;
  border-radius: 12px;
  background: var(--bg-soft);
  border: 1px solid var(--border-color);
  position: relative;
}
.reader-md :deep(.sb-right .sb-body) {
  background: var(--el-color-primary-light-9);
}
.reader-md :deep(.sb-name) {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-bottom: 2px;
}
.reader-md :deep(.sb-right .sb-name) {
  text-align: right;
}
.reader-md :deep(.sb-text) {
  font-size: 1rem;
  line-height: 1.8;
  word-break: break-word;
}

/* 气泡内表情 */
.reader-md :deep(.sb-emote) {
  font-size: 1.25em;
  line-height: 1;
  margin: 0 2px;
  vertical-align: -0.1em;
}

/* 图片气泡 */
.reader-md :deep(.script-image-block) {
  align-self: center;
  max-width: 100%;
  text-align: center;
  margin: 4px auto;
}
.reader-md :deep(.script-image-block.sb-invalid) {
  font-size: 0.8rem;
  color: var(--text-muted);
  border: 1px dashed var(--border-color);
  border-radius: 8px;
  padding: 8px 12px;
}
.reader-md :deep(.sb-image) {
  max-width: 100%;
  border-radius: 10px;
  border: 1px solid var(--border-color);
}
.reader-md :deep(.sb-image-caption) {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-top: 4px;
}

/* 混排：散文块（Markdown 渲染） */
.reader-md :deep(.script-prose) {
  padding: 0 6px;
}

[data-theme="dark"] .reader-md :deep(.sb-body) {
  background: rgba(255, 255, 255, 0.05);
}
[data-theme="dark"] .reader-md :deep(.sb-right .sb-body) {
  background: rgba(66, 133, 244, 0.18);
}

/* 正文渲染完全跟随 Cherry 默认（与编辑器预览一致）：字体/字号/行距/段落样式
   由 cherry-markdown.min.css 提供，这里不再覆盖。
   标题大字（h1-h6 2em→0.9em 居中）由 global-fix.css 以 !important 统一控制，
   阅读器 / 博客详情 / 评论 / 论坛全部一致，任何 scoped 样式都无法覆盖。 */

.reader-md :deep(ul),
.reader-md :deep(ol) {
  padding-left: 1.5em;
  margin-bottom: 1em;
}

.reader-md :deep(li) {
  margin-bottom: 0.3em;
}

/* 引用块 / 表格 / 分隔线 / 链接 / 加粗：与 Cherry 编辑器预览统一，
   样式由 cherry-markdown.min.css + global-fix.css 提供（.cherry-markdown 类），
   此处不再单独覆盖，保证浏览端与编辑器「所见即所得」。 */

/* Markdown 正文排版（基于 .markdown-body，与 Cherry 编辑器预览统一） */
.markdown-body {
  color: var(--text-color);
  /* 跟随工具栏的字号档位（--reader-font-size 由 .reader-page 注入） */
  font-size: var(--reader-font-size, 1.05rem);
  line-height: 2;
  /* 中文长正文：首行不缩进、段间距由 Cherry 提供，这里只保证行宽舒适 */
  word-break: break-word;
}
[data-theme="dark"] .markdown-body {
  color: #e2e8f0;
}

/* KaTeX 公式 */
.markdown-body .katex { font-size: 1.1em; text-indent: 0; }
.markdown-body .katex-display { margin: 1.2em 0; text-align: center; }
.markdown-body .katex-display > .katex { display: inline-block; }

/* Mermaid 图表容器 */
.markdown-body .mermaid-container {
  text-align: center;
  margin: 1em 0;
  overflow-x: auto;
  background: var(--bg-card);
}
.markdown-body .mermaid-container svg { max-width: 100%; height: auto; }
[data-theme="dark"] .markdown-body .mermaid-container { background: #1e293b; }

/* 代码块：统一走 Cherry 默认主题（cherry-markdown 类），亮色浅底 + Prism token 高亮与编辑器一致；
   仅保留圆角/边距/横滚等布局微调，颜色样式完全跟随 Cherry（所见即所得）。 */
.markdown-body pre.cherry-code-block-wrapper,
.cherry-markdown div[data-type="codeBlock"] > pre {
  border-radius: 8px;
  overflow-x: auto;
  margin: 14px 0;
}
.markdown-body pre code {
  font-family: 'Fira Code', 'Cascadia Code', 'JetBrains Mono', Consolas, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
}

.reader-footer {
  display: flex;
  justify-content: center;
  gap: 24px;
  padding: 24px;
}

/* ====== 选区浮动批注按钮 ====== */
.anno-float-btn {
  position: fixed;
  transform: translateY(-100%);
  z-index: 60;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  border-radius: 999px;
  background: var(--gradient-primary);
  color: #fff;
  font-size: 0.82rem;
  font-weight: 500;
  cursor: pointer;
  user-select: none;
  box-shadow: 0 4px 16px rgba(99, 102, 241, 0.35);
  transition: transform 0.15s, box-shadow 0.15s;
  white-space: nowrap;
}
.anno-float-btn:hover {
  transform: translateY(-100%) scale(1.05);
  box-shadow: 0 6px 20px rgba(99, 102, 241, 0.45);
}
.anno-float-enter-active,
.anno-float-leave-active {
  transition: opacity 0.15s, transform 0.15s;
}
.anno-float-enter-from,
.anno-float-leave-to {
  opacity: 0;
  transform: translateY(-100%) scale(0.85);
}

.trial-notice {
  display: flex;
  justify-content: center;
  padding: 60px 16px;
}

/* 章节目录抽屉 */
.toc-header-info {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  padding: 4px 4px 12px;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--text-color);
}
.toc-count {
  font-size: 0.8rem;
  color: var(--text-light);
  font-weight: 400;
}
.toc-list {
  max-height: calc(100vh - 160px);
  overflow-y: auto;
}
.toc-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
}
.toc-item:hover {
  background: rgba(0, 0, 0, 0.04);
}
[data-theme="dark"] .toc-item:hover {
  background: rgba(255, 255, 255, 0.06);
}
.toc-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}
[data-theme="dark"] .toc-item.active {
  background: rgba(96, 165, 250, 0.16);
  color: #60a5fa;
}
.toc-num {
  flex-shrink: 0;
  font-size: 0.85rem;
  color: var(--text-light);
}
.toc-item.active .toc-num {
  color: inherit;
}
.toc-title {
  flex: 1;
  min-width: 0;
  font-size: 0.9rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .reader-toolbar {
    flex-wrap: wrap;
    gap: 4px;
    padding: 6px 10px;
  }
  .reader-toolbar .el-button {
    padding: 8px 9px;
    font-size: 1.05rem;
  }
  .reader-title {
    width: 100%;
    order: -1;
    text-align: center;
    font-size: 0.9rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .reader-body {
    padding: 24px 10px;
  }
  .reader-content {
    padding: 20px 14px;
    border-radius: 6px;
  }
  /* 移动端阅读字号与行距优化 */
  .reader-md,
  .markdown-body {
    font-size: 1.1rem;
    line-height: 2.05;
  }
  /* 标题大字由 global-fix.css !important 统一控制（2em），不在此覆盖 */
  /* 底部翻页按钮：大按钮均分全宽，方便拇指点击 */
  .reader-footer {
    gap: 12px;
    padding: 14px 12px calc(20px + env(safe-area-inset-bottom));
  }
  .reader-footer .el-button {
    flex: 1;
    height: 44px;
    font-size: 0.95rem;
  }
  /* 目录抽屉标题区域 */
  .toc-header-info {
    padding: 2px 4px 10px;
  }
  .toc-item {
    padding: 12px 10px;
  }
  /* 模板 iframe 容器窄屏留边 */
  .template-header,
  .template-footer {
    padding: 0 10px;
  }
  /* 精读弹窗备注提示换行 */
  .note-hint {
    display: block;
    margin: 6px 0 0;
  }
}

/* ====== 精读：书签 / 批注 ====== */
.note-active { color: #f59e0b !important; }
/* ====== 划线批注：选中句子 → 下划线标记 ======
   我的批注：蓝色实线；公开批注（他人）：琥珀色点线；
   文字保持原色（不遮背景），悬浮浅色底提示可点击查看批注内容与批注人 */
/* ===== 本章划线面板 ===== */
.note-count {
  margin-left: 3px;
  padding: 0 5px;
  border-radius: 8px;
  font-size: 11px;
  line-height: 16px;
  color: #fff;
  background: var(--el-color-primary, #6366f1);
}
.reader-notes {
  position: fixed;
  top: 92px;
  right: 18px;
  bottom: 80px;
  width: 300px;
  z-index: 40;
  display: flex;
  flex-direction: column;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 14px);
  box-shadow: var(--shadow-card, 0 8px 24px rgba(0, 0, 0, 0.08));
  overflow: hidden;
}
.rn-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border-color, #f0f1f4);
  font-size: var(--text-sm, 13px);
}
.rn-title { color: var(--text-color, #1f2328); font-weight: 600; }
.rn-title b { color: var(--el-color-primary, #6366f1); }
.rn-empty { padding: 18px 14px; color: var(--text-light, #6b7280); font-size: var(--text-sm, 13px); line-height: 1.7; }
.rn-empty-hint { margin-top: 6px; font-size: var(--text-xs, 12px); opacity: 0.85; }
.rn-list { list-style: none; margin: 0; padding: 6px; overflow-y: auto; flex: 1; }
.rn-item {
  padding: 8px 10px;
  border-radius: var(--radius-md, 10px);
  border-left: 3px solid var(--border-color, #e5e7eb);
  cursor: pointer;
  transition: background 0.14s ease;
}
.rn-item:hover { background: var(--bg-page, #f7f8fa); }
.rn-item.is-mine { border-left-color: var(--el-color-primary, #6366f1); }
.rn-quote {
  margin: 0 0 4px;
  padding-left: 8px;
  border-left: 2px solid var(--el-color-primary, #6366f1);
  font-size: var(--text-sm, 13px);
  color: var(--text-color, #1f2328);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.rn-content {
  margin: 0 0 4px;
  font-size: var(--text-sm, 13px);
  color: var(--text-light, #4b5563);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.rn-meta { display: flex; align-items: center; gap: 8px; font-size: 11px; color: var(--text-light, #9ca3af); }
.rn-priv { padding: 0 4px; border-radius: 3px; background: var(--bg-page, #f1f5f9); }
.rn-time { margin-left: auto; }

/* 侧栏进出动画 */
.notes-slide-enter-active, .notes-slide-leave-active { transition: transform 0.2s ease, opacity 0.2s ease; }
.notes-slide-enter-from, .notes-slide-leave-to { transform: translateX(16px); opacity: 0; }

/* 从侧栏定位到划线时闪一下，否则用户不知道该看哪 */
mark.note-ann.is-flash {
  background: rgba(251, 191, 36, 0.55);
  box-shadow: 0 0 0 3px rgba(251, 191, 36, 0.35);
  transition: background 0.2s ease, box-shadow 0.2s ease;
}

@media (max-width: 768px) {
  /* 手机上改成底部抽屉，避免窄屏遮挡正文 */
  .reader-notes {
    top: auto;
    left: 0;
    right: 0;
    bottom: 0;
    width: auto;
    max-height: 55vh;
    border-radius: 14px 14px 0 0;
  }
}

mark.note-ann {
  background: transparent;
  border-bottom: 2px solid rgba(245, 158, 11, 0.55);
  border-radius: 0;
  padding: 0;
  color: inherit;
  cursor: pointer;
  transition: background 0.15s;
}
mark.note-mine {
  border-bottom: 2px solid #3b82f6; /* 我的批注：蓝色实线 */
}
mark.note-public {
  border-bottom: 2px dotted #f59e0b; /* 他人公开批注：琥珀色点线 */
}
mark.note-ann:hover,
mark.note-ann:active {
  background: rgba(245, 158, 11, 0.14);
}
[data-theme="dark"] mark.note-public { border-bottom-color: #fbbf24; }
[data-theme="dark"] mark.note-mine { border-bottom-color: #60a5fa; }
[data-theme="dark"] mark.note-ann:hover,
[data-theme="dark"] mark.note-ann:active { background: rgba(251, 191, 36, 0.16); }
.note-hint { margin-left: 8px; font-size: 0.75rem; color: var(--text-muted); }
.note-selected {
  background: var(--bg-soft); border-left: 3px solid #f59e0b; padding: 8px 12px;
  margin-bottom: 12px; color: #666; font-size: 0.85rem; border-radius: 4px;
  max-height: 80px; overflow: auto; white-space: pre-wrap;
}
.note-vis-row { margin-top: 10px; display: flex; align-items: center; }
.note-meta { margin-top: 10px; font-size: 0.78rem; color: var(--text-muted); }
.note-detail-content { white-space: pre-wrap; color: var(--text-color, #333); line-height: 1.7; }
[data-theme="dark"] .note-selected { background: rgba(255,255,255,0.06); color: #cbd5e1; border-left-color: #f59e0b; }
</style>

<style>
[data-theme="dark"] .reader-body {
  background: transparent;
}
[data-theme="dark"] .trial-notice .el-result__title {
  color: #e2e8f0;
}
[data-theme="dark"] .trial-notice .el-result__subtitle {
  color: #94a3b8;
}
/* Mermaid / KaTeX 背景/颜色修复已统一在 markdown/global-fix.css */
</style>