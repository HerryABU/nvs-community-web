<template>
  <div class="admin-page">
    <AdminHeader title="热词统计" subtitle="内容热词 · 搜索热词 · 标签热词 · 内容治理与舆情观测" :icon="Histogram" />
    <div class="admin-page-body">

      <!-- 类型切换卡片 -->
      <div class="hw-type-cards">
        <div
          v-for="t in typeCards"
          :key="t.value"
          class="hw-type-card"
          :class="{ active: activeType === t.value }"
          @click="switchType(t.value)"
        >
          <div class="hw-type-icon" :style="{ background: t.gradient }">{{ t.icon }}</div>
          <div class="hw-type-body">
            <div class="hw-type-name">{{ t.name }}</div>
            <div class="hw-type-desc">{{ t.desc }}</div>
          </div>
          <span v-if="activeType === t.value" class="hw-type-badge" :style="{ background: t.gradient }">
            {{ activeCardCount }} 词
          </span>
        </div>
      </div>

      <!-- 概览卡片 -->
      <div class="admin-panel">
        <SectionHeader :title="`${currentTypeName}概览`" :subtitle="currentTypeSubtitle" :icon="currentTypeIcon" gradient="primary" />
        <div class="admin-stat-grid" v-loading="loading">
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-primary)">∑</div>
            <div>
              <div class="admin-stat-value">{{ fmtNum(stats.total) }}</div>
              <div class="admin-stat-label">{{ stats.totalLabel }}</div>
            </div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-amber)">🔥</div>
            <div>
              <div class="admin-stat-value hw-top-word" :title="stats.topWord">{{ stats.topWord || '—' }}</div>
              <div class="admin-stat-label">{{ stats.topLabel }}</div>
              <div v-if="stats.topShare" class="admin-stat-sub">占总量 {{ stats.topShare }}</div>
            </div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-success)">📈</div>
            <div>
              <div class="admin-stat-value">{{ fmtNum(stats.topCount) }}</div>
              <div class="admin-stat-label">{{ stats.countLabel }}</div>
            </div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-blue)">🕐</div>
            <div>
              <div class="admin-stat-value">{{ days }} 天</div>
              <div class="admin-stat-label">统计窗口</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 明细 -->
      <div class="admin-panel">
        <SectionHeader :title="`${currentTypeName}明细`" subtitle="按出现次数降序 · 支持关键词筛选与 CSV 导出" :icon="currentTypeIcon" :gradient="currentTypeGradient" />

        <div class="admin-toolbar">
          <span class="filter-label">统计窗口</span>
          <el-radio-group v-model="days" size="small" @change="reload">
            <el-radio-button :value="7">近 7 天</el-radio-button>
            <el-radio-button :value="30">近 30 天</el-radio-button>
            <el-radio-button :value="90">近 90 天</el-radio-button>
          </el-radio-group>
          <el-select v-model="limit" size="small" style="width:130px" @change="reload">
            <el-option label="前 30 词" :value="30" />
            <el-option label="前 50 词" :value="50" />
            <el-option label="前 100 词" :value="100" />
            <el-option label="前 200 词" :value="200" />
          </el-select>
          <el-input
            v-model="keyword"
            placeholder="筛选热词"
            size="small"
            style="width:180px"
            clearable
            :prefix-icon="Search"
          />
          <el-button size="small" :icon="RefreshLeft" @click="reload">刷新</el-button>
          <el-button size="small" type="primary" :icon="Download" :disabled="!currentRows.length" @click="exportCsv">
            导出 CSV
          </el-button>
          <el-button
            v-if="activeType === 'search'"
            size="small"
            type="danger"
            plain
            :icon="Delete"
            @click="confirmCleanup"
          >
            清空搜索日志
          </el-button>
        </div>

        <p class="hint-text">
          <template v-if="activeType === 'content'">
            轻量分词聚合：英文整词 + 中文短段（≤4 字）整取 / 长段二元切分，已过滤停用词；仅统计出现 ≥2 次的词。数据按需实时聚合，不落库。单表扫描上限 5000 条。
          </template>
          <template v-else-if="activeType === 'search'">
            埋点聚合：统一搜索 / 作品搜索 / 作者搜索三处入口的 query（异步批量落库，单表扫描上限 5000 条）。「原始搜索短语」单次即计入；「搜索词分词」需 ≥2 次。写入不阻塞搜索请求，channel 满时丢弃（背压保护）。
          </template>
          <template v-else>
            标签聚合：博客结构化标签（#tag，窗口内新建博客）+ 作品 JSON 标签，按使用次数降序，单次出现即计入。
          </template>
        </p>

        <!-- 搜索热词：原始短语 + 分词双区 -->
        <template v-if="activeType === 'search'">
          <div class="hw-section" v-loading="loading">
            <div class="hw-section-title">
              <span class="hw-section-dot" style="background: var(--gradient-blue)" />
              原始搜索短语 <span class="hw-section-sub">用户完整输入，大小写不敏感</span>
            </div>
            <div v-for="(w, i) in filteredQueries" :key="'q' + w.word" class="hw-row" title="点击复制" @click="copyWord(w.word)">
              <span class="hw-rank" :class="rankClass(i)">{{ i + 1 }}</span>
              <span class="hw-word" :title="w.word">{{ w.word }}</span>
              <span class="hw-bar-wrap">
                <span class="hw-bar hw-bar-blue" :style="{ width: barWidth(w.count, maxQueryCount) }" />
              </span>
              <span class="hw-count">{{ w.count }}</span>
            </div>
            <EmptyState v-if="!loading && !filteredQueries.length" icon="🔍" text="暂无搜索记录" description="该窗口内用户还没有产生站内搜索行为" />
          </div>
          <div class="hw-section" v-loading="loading">
            <div class="hw-section-title">
              <span class="hw-section-dot" style="background: var(--gradient-teal)" />
              搜索词分词 <span class="hw-section-sub">中英混合 tokenizer 切分，≥2 次</span>
            </div>
            <div v-for="(w, i) in filteredWords" :key="'w' + w.word" class="hw-row" title="点击复制" @click="copyWord(w.word)">
              <span class="hw-rank" :class="rankClass(i)">{{ i + 1 }}</span>
              <span class="hw-word" :title="w.word">{{ w.word }}</span>
              <span class="hw-bar-wrap">
                <span class="hw-bar hw-bar-teal" :style="{ width: barWidth(w.count, maxWordCount) }" />
              </span>
              <span class="hw-count">{{ w.count }}</span>
            </div>
            <EmptyState v-if="!loading && !filteredWords.length" icon="✂️" text="暂无搜索热词" description="该窗口内的搜索短语尚未切分出高频词" />
          </div>
        </template>

        <!-- 内容热词 / 标签热词：单列表 -->
        <div v-else class="hw-section" v-loading="loading">
          <div class="hw-section-title">
            <span class="hw-section-dot" :style="{ background: activeType === 'tag' ? 'var(--gradient-purple)' : 'var(--gradient-primary)' }" />
            {{ activeType === 'tag' ? '标签使用排行' : '热词出现排行' }}
            <span class="hw-section-sub">{{ activeType === 'tag' ? '单次出现即计入' : '仅统计出现 ≥2 次的词' }}</span>
          </div>
          <div v-for="(w, i) in filteredWords" :key="w.word" class="hw-row" title="点击复制" @click="copyWord(w.word)">
            <span class="hw-rank" :class="rankClass(i)">{{ i + 1 }}</span>
            <span class="hw-word" :title="w.word">{{ w.word }}</span>
            <span class="hw-bar-wrap">
              <span class="hw-bar" :class="activeType === 'tag' ? 'hw-bar-purple' : 'hw-bar-primary'" :style="{ width: barWidth(w.count, maxWordCount) }" />
            </span>
            <span class="hw-count">{{ w.count }}</span>
          </div>
          <EmptyState v-if="!loading && !filteredWords.length" :icon="activeType === 'tag' ? '🏷️' : '💬'" :text="activeType === 'tag' ? '暂无标签' : '暂无热词'" :description="activeType === 'tag' ? '该窗口内没有新建带标签的博客或作品' : '该窗口内评论/回帖/帖题尚未聚合出高频词'" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Histogram, Search, RefreshLeft, Download, Delete } from '@element-plus/icons-vue';

type HotWordType = 'content' | 'search' | 'tag';
interface HotWord { word: string; count: number }

const activeType = ref<HotWordType>('content');
const days = ref(7);
const limit = ref(50);
const keyword = ref('');
const loading = ref(false);
const words = ref<HotWord[]>([]);
const queries = ref<HotWord[]>([]);
const searchTotal = ref(0);

// ===== 类型卡片 =====
const typeCards: { value: HotWordType; name: string; desc: string; icon: string; gradient: string }[] = [
  { value: 'content', name: '内容热词', desc: '评论 / 回帖 / 帖题分词高频词', icon: '💬', gradient: 'var(--gradient-primary)' },
  { value: 'search', name: '搜索热词', desc: '站内搜索 query 埋点 + 分词', icon: '🔍', gradient: 'var(--gradient-blue)' },
  { value: 'tag', name: '标签热词', desc: '博客 #tag 与作品标签聚合', icon: '🏷️', gradient: 'var(--gradient-purple)' },
];
function switchType(t: HotWordType) {
  if (activeType.value === t) return;
  activeType.value = t;
  reload();
}

const currentTypeName = computed(() => typeCards.find((t) => t.value === activeType.value)?.name || '');
const currentTypeIcon = computed(() => typeCards.find((t) => t.value === activeType.value)?.icon || '📊');
const currentTypeGradient = computed(() => {
  const g = typeCards.find((t) => t.value === activeType.value)?.gradient || '';
  return g.replace('var(--gradient-', '').replace(')', '');
});
const currentTypeSubtitle = computed(() => typeCards.find((t) => t.value === activeType.value)?.desc || '');

// 出现次数总和（用于 top 词占比）
const wordsTotalCount = computed(() => words.value.reduce((s, w) => s + w.count, 0));
const queriesTotalCount = computed(() => queries.value.reduce((s, w) => s + w.count, 0));

function shareOf(topCount: number, total: number): string {
  if (!topCount || !total) return '';
  const pct = (topCount / total) * 100;
  return pct >= 10 ? `${pct.toFixed(0)}%` : `${pct.toFixed(1)}%`;
}

const stats = computed(() => {
  const top = words.value[0];
  if (activeType.value === 'search') {
    const q = queries.value[0];
    return {
      total: searchTotal.value,
      totalLabel: '搜索总次数',
      topWord: q?.word || top?.word || '',
      topLabel: '最热搜索词',
      topCount: q?.count || top?.count || 0,
      countLabel: '最高搜索次数',
      topShare: shareOf(q?.count || top?.count || 0, queriesTotalCount.value),
    };
  }
  if (activeType.value === 'tag') {
    return {
      total: words.value.length,
      totalLabel: '标签总数',
      topWord: top?.word || '',
      topLabel: '最热标签',
      topCount: top?.count || 0,
      countLabel: '最高使用次数',
      topShare: shareOf(top?.count || 0, wordsTotalCount.value),
    };
  }
  return {
    total: words.value.length,
    totalLabel: '热词总数（≥2次）',
    topWord: top?.word || '',
    topLabel: '当前最热词',
    topCount: top?.count || 0,
    countLabel: '最高出现次数',
    topShare: shareOf(top?.count || 0, wordsTotalCount.value),
  };
});

const filteredWords = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return words.value;
  return words.value.filter((w) => w.word.toLowerCase().includes(kw));
});

const filteredQueries = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return queries.value;
  return queries.value.filter((w) => w.word.toLowerCase().includes(kw));
});

// 当前主列表（导出/禁用判断用）：搜索态取原始短语，其余取 words
const currentRows = computed(() => (activeType.value === 'search' ? filteredQueries.value : filteredWords.value));

// 激活类型卡上的词数徽标
const activeCardCount = computed(() => currentRows.value.length);

// 千分位格式化
function fmtNum(n: number): string {
  return (n ?? 0).toLocaleString('zh-CN');
}

// 点击热词行复制（便于拿到词条去站内搜索/联动排查）
async function copyWord(word: string) {
  try {
    await navigator.clipboard.writeText(word);
    ElMessage.success(`已复制「${word}」`);
  } catch {
    ElMessage.warning('复制失败，请手动选择文本');
  }
}

const maxWordCount = computed(() => words.value.reduce((m, w) => Math.max(m, w.count), 0));
const maxQueryCount = computed(() => queries.value.reduce((m, w) => Math.max(m, w.count), 0));

function barWidth(count: number, max: number): string {
  if (!max) return '0%';
  return `${Math.max(4, Math.round((count / max) * 100))}%`;
}

function rankClass(i: number): string {
  if (i === 0) return 'rk-gold';
  if (i === 1) return 'rk-silver';
  if (i === 2) return 'rk-bronze';
  return '';
}

async function reload() {
  loading.value = true;
  try {
    const res = await adminApi.hotWords({ days: days.value, limit: limit.value, type: activeType.value });
    words.value = res.data?.words || [];
    queries.value = res.data?.queries || [];
    searchTotal.value = res.data?.search_total || 0;
  } catch {
    words.value = [];
    queries.value = [];
    searchTotal.value = 0;
  } finally {
    loading.value = false;
  }
}

async function confirmCleanup() {
  try {
    await ElMessageBox.confirm(
      '将清空全部搜索埋点日志（含历史 query 与计数），此操作不可恢复。仅影响搜索热词统计，不影响站内搜索功能。确定继续？',
      '清空搜索日志',
      { type: 'warning', confirmButtonText: '全部清空', cancelButtonText: '取消' },
    );
  } catch {
    return; // 用户取消
  }
  try {
    await adminApi.clearSearchLogs();
    ElMessage.success('搜索日志已清空');
    await reload();
  } catch {
    ElMessage.error('清空失败');
  }
}

function exportCsv() {
  const isSearch = activeType.value === 'search';
  const rows: string[][] = isSearch
    ? [['排名', '搜索短语', '搜索次数']]
    : activeType.value === 'tag'
      ? [['排名', '标签', '使用次数']]
      : [['排名', '热词', '出现次数']];
  const src = currentRows.value;
  src.forEach((w, i) => rows.push([String(i + 1), w.word, String(w.count)]));
  if (!src.length) {
    ElMessage.warning('当前无可导出的数据');
    return;
  }
  // CSV 单元格转义：含逗号/引号/换行时加双引号并翻倍内部引号
  const csv = '\uFEFF' + rows
    .map((r) => r.map((c) => (/[",\n]/.test(c) ? `"${c.replace(/"/g, '""')}"` : c)).join(','))
    .join('\r\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `hotwords_${activeType.value}_${days.value}d_${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
  ElMessage.success('已导出 CSV');
}

onMounted(reload);
</script>

<style scoped>
.admin-page { --admin-content-max: 980px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
[data-theme="dark"] .admin-page { background: #0f172a; }
.hw-top-word { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.admin-stat-sub { font-size: 11px; color: var(--text-light, #999); margin-top: 2px; }

/* 类型切换卡片 */
.hw-type-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; margin-bottom: 16px; }
.hw-type-card { position: relative; display: flex; align-items: center; gap: 12px; padding: 14px 16px; border: 2px solid var(--border-color); border-radius: 10px; background: var(--bg-card); cursor: pointer; transition: border-color .15s, transform .15s; }
.hw-type-card:hover { transform: translateY(-1px); }
.hw-type-card.active { border-color: var(--primary-color); }
.hw-type-icon { width: 40px; height: 40px; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 1.2rem; flex-shrink: 0; }
.hw-type-badge { position: absolute; top: 8px; right: 8px; padding: 1px 8px; border-radius: 9px; font-size: 11px; font-weight: 700; color: #fff; }
.hw-type-name { font-weight: 600; font-size: 0.92rem; color: var(--text-color); }
.hw-type-desc { font-size: 0.76rem; color: var(--text-muted); margin-top: 2px; }

.filter-label { font-size: 13px; color: var(--el-text-color-secondary); }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin: 0 0 12px; }

/* 明细分区 */
.hw-section {
  border: 1px solid var(--border-color);
  border-radius: 10px;
  padding: 6px 16px 10px;
  margin-bottom: 16px;
  min-height: 120px;
  background: var(--bg-card);
}
.hw-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
  padding: 10px 0 8px;
}
.hw-section-dot { width: 10px; height: 10px; border-radius: 3px; flex-shrink: 0; }
.hw-section-sub { font-size: 12px; font-weight: 400; color: var(--text-muted); }
.hw-row { display: flex; align-items: center; gap: 12px; padding: 8px 6px; margin: 0 -6px; border-bottom: 1px solid var(--border-color); cursor: pointer; border-radius: 6px; transition: background .12s; }
.hw-row:hover { background: var(--bg-soft, rgba(0,0,0,.03)); }
.hw-row:last-child { border-bottom: none; }
.hw-rank {
  width: 26px; height: 22px; border-radius: 6px;
  display: inline-flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700; color: var(--text-muted);
  background: var(--bg-soft); flex-shrink: 0;
}
.hw-rank.rk-gold { background: #f7d774; color: #7a5b00; }
.hw-rank.rk-silver { background: #dfe3e8; color: #4a5568; }
.hw-rank.rk-bronze { background: #e8c39e; color: #7a4a1d; }
.hw-word { width: 180px; flex-shrink: 0; font-size: 14px; color: var(--text-color); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.hw-bar-wrap { flex: 1; height: 10px; background: var(--bg-soft); border-radius: 5px; overflow: hidden; }
.hw-bar { display: block; height: 100%; border-radius: 5px; transition: width .3s ease; }
.hw-bar-primary { background: linear-gradient(90deg, var(--el-color-primary-light-4), var(--el-color-primary)); }
.hw-bar-blue { background: linear-gradient(90deg, #93c5fd, #3b82f6); }
.hw-bar-teal { background: linear-gradient(90deg, #5eead4, #14b8a6); }
.hw-bar-purple { background: linear-gradient(90deg, #c4b5fd, #8b5cf6); }
.hw-count { width: 56px; text-align: right; font-variant-numeric: tabular-nums; font-size: 13px; font-weight: 600; color: var(--text-color); flex-shrink: 0; }
</style>
