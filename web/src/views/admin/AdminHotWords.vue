<template>
  <div class="admin-page">
    <AdminHeader title="热词统计" subtitle="评论 / 回帖 / 帖题高频词聚合 · 内容治理与舆情观测" :icon="Histogram" />
    <div class="admin-page-body admin-panel">

      <!-- 概览卡片 -->
      <div class="stat-cards" v-loading="loading">
        <div class="stat-card">
          <div class="stat-value">{{ stats.total }}</div>
          <div class="stat-label">热词总数（≥2次）</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.topWord || '—' }}</div>
          <div class="stat-label">当前最热词</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.topCount }}</div>
          <div class="stat-label">最高出现次数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ days }} 天</div>
          <div class="stat-label">统计窗口</div>
        </div>
      </div>

      <!-- 筛选区 -->
      <div class="filter-bar">
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
      </div>

      <p class="hint-text">
        轻量分词聚合：英文整词 + 中文二元切分（bigram），已过滤停用词；仅统计出现 ≥2 次的词。数据按需实时聚合，不落库。
        单表扫描上限 5000 条，超长历史请以数据库为准。
      </p>

      <!-- 热词列表 -->
      <div class="hw-list" v-loading="loading">
        <div v-for="(w, i) in filteredWords" :key="w.word" class="hw-row">
          <span class="hw-rank" :class="rankClass(i)">{{ i + 1 }}</span>
          <span class="hw-word" :title="w.word">{{ w.word }}</span>
          <span class="hw-bar-wrap">
            <span class="hw-bar" :style="{ width: barWidth(w.count) }" />
          </span>
          <span class="hw-count">{{ w.count }}</span>
        </div>
        <el-empty v-if="!loading && !filteredWords.length" description="该窗口内暂无热词" :image-size="80" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { ElMessage } from 'element-plus';
import { Histogram, Search, RefreshLeft, Download } from '@element-plus/icons-vue';

interface HotWord { word: string; count: number }

const days = ref(7);
const limit = ref(50);
const keyword = ref('');
const loading = ref(false);
const words = ref<HotWord[]>([]);

const stats = computed(() => {
  const top = words.value[0];
  return {
    total: words.value.length,
    topWord: top?.word || '',
    topCount: top?.count || 0,
  };
});

const filteredWords = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return words.value;
  return words.value.filter((w) => w.word.toLowerCase().includes(kw));
});

const maxCount = computed(() => words.value.reduce((m, w) => Math.max(m, w.count), 0));

function barWidth(count: number): string {
  if (!maxCount.value) return '0%';
  return `${Math.max(4, Math.round((count / maxCount.value) * 100))}%`;
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
    const res = await adminApi.hotWords({ days: days.value, limit: limit.value });
    words.value = res.data?.words || [];
  } catch {
    words.value = [];
  } finally {
    loading.value = false;
  }
}

function exportCsv() {
  const rows = [['排名', '热词', '出现次数']];
  filteredWords.value.forEach((w, i) => rows.push([String(i + 1), w.word, String(w.count)]));
  // CSV 单元格转义：含逗号/引号/换行时加双引号并翻倍内部引号
  const csv = '\uFEFF' + rows
    .map((r) => r.map((c) => (/[",\n]/.test(c) ? `"${c.replace(/"/g, '""')}"` : c)).join(','))
    .join('\r\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `hotwords_${days.value}d_${new Date().toISOString().slice(0, 10)}.csv`;
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

.hw-list {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 8px 12px;
  min-height: 120px;
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
