<template>
  <div class="admin-page">
    <AdminHeader title="热词统计" subtitle="内容热词 · 搜索热词 · 标签热词 · 内容治理与舆情观测" :icon="Histogram" />
    <div class="admin-page-body admin-panel">

      <!-- 概览卡片 -->
      <div class="stat-cards" v-loading="loading">
        <div class="stat-card">
          <div class="stat-value">{{ stats.total }}</div>
          <div class="stat-label">{{ stats.totalLabel }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.topWord || '—' }}</div>
          <div class="stat-label">{{ stats.topLabel }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.topCount }}</div>
          <div class="stat-label">{{ stats.countLabel }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ days }} 天</div>
          <div class="stat-label">统计窗口</div>
        </div>
      </div>

      <!-- 筛选区 -->
      <div class="filter-bar">
        <span class="filter-label">热词类型</span>
        <el-radio-group v-model="activeType" size="small" @change="reload">
          <el-radio-button value="content">内容热词</el-radio-button>
          <el-radio-button value="search">搜索热词</el-radio-button>
          <el-radio-button value="tag">标签热词</el-radio-button>
        </el-radio-group>
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
        <el-button size="small" @click="reload" :icon="RefreshLeft">刷新</el-button>
        <el-button size="small" type="primary" :icon="Download" :disabled="!filteredWords.length" @click="exportCsv">
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
            原始搜索短语 <span class="hw-section-sub">用户完整输入，大小写不敏感</span>
          </div>
          <div v-for="(w, i) in filteredQueries" :key="'q' + w.word" class="hw-row">
            <span class="hw-rank" :class="rankClass(i)">{{ i + 1 }}</span>
            <span class="hw-word" :title="w.word">{{ w.word }}</span>
            <span class="hw-bar-wrap">
              <span class="hw-bar" :style="{ width: barWidth(w.count, maxQueryCount) }" />
            </span>
            <span class="hw-count">{{ w.count }}</span>
          </div>
          <el-empty v-if="!loading && !filteredQueries.length" description="该窗口内暂无搜索记录" :image-size="80" />
        </div>
        <div class="hw-section" v-loading="loading">
          <div class="hw-section-title">
            搜索词分词 <span class="hw-section-sub">中英混合 tokenizer 切分，≥2 次</span>
          </div>
          <div v-for="(w, i) in filteredWords" :key="'w' + w.word" class="hw-row">
            <span class="hw-rank" :class="rankClass(i)">{{ i + 1 }}</span>
            <span class="hw-word" :title="w.word">{{ w.word }}</span>
            <span class="hw-bar-wrap">
              <span class="hw-bar" :style="{ width: barWidth(w.count, maxWordCount) }" />
            </span>
            <span class="hw-count">{{ w.count }}</span>
          </div>
          <el-empty v-if="!loading && !filteredWords.length" description="该窗口内暂无搜索热词" :image-size="80" />
        </div>
      </template>

      <!-- 内容热词 / 标签热词：单列表 -->
      <div v-else class="hw-list" v-loading="loading">
        <div v-for="(w, i) in filteredWords" :key="w.word" class="hw-row">
          <span class="hw-rank" :class="rankClass(i)">{{ i + 1 }}</span>
          <span class="hw-word" :title="w.word">{{ w.word }}</span>
          <span class="hw-bar-wrap">
            <span class="hw-bar" :style="{ width: barWidth(w.count, maxWordCount) }" />
          </span>
          <span class="hw-count">{{ w.count }}</span>
        </div>
        <el-empty v-if="!loading && !filteredWords.length" :description="activeType === 'tag' ? '该窗口内暂无标签' : '该窗口内暂无热词'" :image-size="80" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import AdminHeader from '@/components/admin/AdminHeader.vue';
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

const stats = computed(() => {
  const top = words.value[0];
  if (activeType.value === 'search') {
    return {
      total: searchTotal.value,
      totalLabel: '搜索总次数',
      topWord: queries.value[0]?.word || top?.word || '',
      topLabel: '最热搜索词',
      topCount: queries.value[0]?.count || top?.count || 0,
      countLabel: '最高搜索次数',
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
    };
  }
  return {
    total: words.value.length,
    totalLabel: '热词总数（≥2次）',
    topWord: top?.word || '',
    topLabel: '当前最热词',
    topCount: top?.count || 0,
    countLabel: '最高出现次数',
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
  const src = isSearch ? filteredQueries.value : filteredWords.value;
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
.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.filter-label { font-size: 13px; color: var(--el-text-color-secondary); }

.hw-list, .hw-section {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 8px 12px;
  min-height: 120px;
}
.hw-section { margin-bottom: 16px; }
.hw-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  padding: 8px 0 6px;
}
.hw-section-sub {
  font-size: 12px;
  font-weight: 400;
  color: var(--el-text-color-secondary);
  margin-left: 8px;
}
.hw-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 7px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.hw-row:last-child { border-bottom: none; }
.hw-rank {
  width: 26px;
  height: 22px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  flex-shrink: 0;
}
.hw-rank.rk-gold { background: #f7d774; color: #7a5b00; }
.hw-rank.rk-silver { background: #dfe3e8; color: #4a5568; }
.hw-rank.rk-bronze { background: #e8c39e; color: #7a4a1d; }
.hw-word {
  width: 180px;
  flex-shrink: 0;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hw-bar-wrap {
  flex: 1;
  height: 10px;
  background: var(--el-fill-color-light);
  border-radius: 5px;
  overflow: hidden;
}
.hw-bar {
  display: block;
  height: 100%;
  border-radius: 5px;
  background: linear-gradient(90deg, var(--el-color-primary-light-4), var(--el-color-primary));
  transition: width .3s ease;
}
.hw-count {
  width: 56px;
  text-align: right;
  font-variant-numeric: tabular-nums;
  font-size: 13px;
  color: var(--el-text-color-regular);
  flex-shrink: 0;
}
</style>
