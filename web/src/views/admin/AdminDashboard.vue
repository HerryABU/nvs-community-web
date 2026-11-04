<template>
  <div class="dashboard-bg">
    <div class="dashboard-container">
      <!-- 顶部标题 -->
      <div class="dashboard-hero">
        <h1 class="dashboard-main-title">
          <span class="title-glow">管理员数据大屏</span>
        </h1>
        <p class="dashboard-subtitle">实时监控平台数据 · 管理站点配置</p>
        <div class="hero-buttons">
          <el-button class="refresh-btn" @click="refreshAll" :loading="refreshing" size="default">
            <el-icon><Refresh /></el-icon>刷新数据
          </el-button>
          <el-button type="success" size="default" @click="$router.push('/admin/users')">
            <el-icon><User /></el-icon>用户管理
          </el-button>
        </div>
      </div>

      <!-- 备案状态提醒（176号令 第七条第一项，F7） -->
      <el-alert
        v-if="!filingReady"
        type="warning"
        show-icon
        :closable="false"
        style="margin-bottom: 20px"
        title="⚠️ 请尽快完成工信部ICP备案和公安联网备案，并在此配置备案号"
      >
        <template #default>
          <span style="font-size: 0.85rem">
            未完成备案的网站可能面临行政处罚。前往
            <el-link type="primary" @click="$router.push('/admin/settings')">站点设置 → 联网备案</el-link>
            填写备案号（填好后将自动显示在站点页脚）。
          </span>
        </template>
      </el-alert>

      <!-- 三幕切换：总览 / 内容与热词 / 用户与终端。
           数据量太大，一屏塞不下；分幕后每幕只回答一类问题，并写进 hash 便于收藏。 -->
      <div class="dash-tabs" role="tablist">
        <button
          v-for="s in scenes"
          :key="s.key"
          type="button"
          role="tab"
          class="dash-tab"
          :class="{ active: scene === s.key }"
          :aria-selected="scene === s.key"
          @click="switchScene(s.key)"
        >
          <span class="dash-tab-ico">{{ s.icon }}</span>{{ s.label }}
        </button>
      </div>

      <!-- ===================== 幕 1｜总览 ===================== -->
      <template v-if="scene === 'overview'">
        <!-- 统计卡片 -->
        <el-row :gutter="20" class="stats-row">
          <el-col :xs="24" :sm="12" :md="6" v-for="(card, idx) in statCardDefs" :key="idx">
            <div class="glass-stat-card">
              <div class="glass-stat-icon" :style="{ background: card.gradient }">
                <el-icon :size="24"><component :is="card.icon" /></el-icon>
              </div>
              <div class="glass-stat-body">
                <AnimatedNumber :target="card.value" :duration="1400" class="glass-stat-value" />
                <div class="glass-stat-label">{{ card.label }}</div>
              </div>
            </div>
          </el-col>
        </el-row>

        <!-- 趋势：左=用户增长，右=访问趋势（两张图数据不同） -->
        <el-row :gutter="20" class="chart-row">
          <el-col :xs="24" :md="12">
            <DashboardCharts :metrics="userOnlyMetrics" />
          </el-col>
          <el-col :xs="24" :md="12">
            <DashboardCharts :metrics="visitorMetrics" />
          </el-col>
        </el-row>

        <!-- 地域 + 活跃时段 -->
        <el-row :gutter="20" class="chart-row">
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <h4 class="chart-title">访问来源地域（近 30 天）</h4>
              <div v-if="geoSummary" class="geo-sum">
                <span>境内 <b>{{ geoSummary.cn_pct }}%</b>（{{ geoSummary.cn }}）</span>
                <span>境外 <b>{{ geoSummary.os_pct }}%</b>（{{ geoSummary.overseas }}）</span>
                <span class="muted">内网/保留 {{ geoSummary.reserved }} 不计入</span>
              </div>
              <StdMap
                :data="provinceHeat"
                :countries="countryHeat"
                note="基于 ip2region 离线库按省级着色；内网/保留地址与境外不计入省份排名（境外见「全球」视野）。"
              />
            </div>
          </el-col>
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <AdminActiveHoursHeatmap :matrix="activeHours" />
            </div>
          </el-col>
        </el-row>
      </template>

      <!-- ===================== 幕 2｜内容与热词 ===================== -->
      <template v-else-if="scene === 'content'">
        <!-- 违禁词拦截：三个数先给结论 -->
        <el-row :gutter="20" class="stats-row">
          <el-col :xs="24" :sm="8">
            <div class="glass-stat-card">
              <div class="glass-stat-icon" style="background: var(--gradient-rose, linear-gradient(135deg,#f43f5e,#e11d48))">🛡️</div>
              <div class="glass-stat-body">
                <AnimatedNumber :target="badword.blocked" :duration="1200" class="glass-stat-value" />
                <div class="glass-stat-label">违禁词命中（近 30 天，已留痕）</div>
              </div>
            </div>
          </el-col>
          <el-col :xs="24" :sm="8">
            <div class="glass-stat-card">
              <div class="glass-stat-icon" style="background: var(--gradient-amber, linear-gradient(135deg,#f59e0b,#d97706))">⏳</div>
              <div class="glass-stat-body">
                <AnimatedNumber :target="badword.pending" :duration="1200" class="glass-stat-value" />
                <div class="glass-stat-label">其中待人工复核</div>
              </div>
            </div>
          </el-col>
          <el-col :xs="24" :sm="8">
            <div class="glass-stat-card">
              <div class="glass-stat-icon" style="background: var(--gradient-blue, linear-gradient(135deg,#3b82f6,#2563eb))">📚</div>
              <div class="glass-stat-body">
                <AnimatedNumber :target="badword.library" :duration="1200" class="glass-stat-value" />
                <div class="glass-stat-label">词库总词数（启用 {{ badword.lib_enabled }}）</div>
              </div>
            </div>
          </el-col>
        </el-row>

        <el-row :gutter="20" class="chart-row">
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <h4 class="chart-title">违禁词词云（近 30 天命中频次）</h4>
              <p class="chart-sub">字号越大命中越多；只统计已留痕的命中，瞬时拒绝不在其内</p>
              <div v-if="badwordWords.length" class="wc">
                <span
                  v-for="(w, i) in badwordWords"
                  :key="w.word"
                  class="wc-item"
                  :style="wcStyle(w)"
                  :title="`${w.word}：${w.count} 次`"
                >{{ w.word }}</span>
              </div>
              <StateBlock
                v-else
                state="empty"
                title="近 30 天没有命中记录"
                description="内容命中违禁词并留痕后，这里会按词频生成词云。"
              />
            </div>
          </el-col>
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <h4 class="chart-title">搜索热词（近 7 天）</h4>
              <p class="chart-sub">读者在找什么 —— 需求最直接的信号</p>
              <div v-if="hotSearch.length" class="hot-list">
                <div v-for="(w, i) in hotSearch" :key="w.word" class="hot-row">
                  <span class="hot-rank" :class="{ top: i < 3 }">{{ i + 1 }}</span>
                  <span class="hot-word">{{ w.word }}</span>
                  <div class="hot-track"><div class="hot-fill" :style="{ width: hotPct(hotSearch, w.count) }"></div></div>
                  <span class="hot-count">{{ w.count }}</span>
                </div>
              </div>
              <StateBlock v-else state="empty" title="近 7 天没有搜索记录" description="搜索埋点会记录查询词与分词结果。" />
            </div>
          </el-col>
        </el-row>

        <el-row :gutter="20" class="chart-row">
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <h4 class="chart-title">评论区热词（近 7 天）</h4>
              <p class="chart-sub">评论 / 回帖分词后的高频词 —— 读者在聊什么</p>
              <div v-if="hotContent.length" class="hot-list">
                <div v-for="(w, i) in hotContent" :key="w.word" class="hot-row">
                  <span class="hot-rank" :class="{ top: i < 3 }">{{ i + 1 }}</span>
                  <span class="hot-word">{{ w.word }}</span>
                  <div class="hot-track"><div class="hot-fill" :style="{ width: hotPct(hotContent, w.count) }"></div></div>
                  <span class="hot-count">{{ w.count }}</span>
                </div>
              </div>
              <StateBlock v-else state="empty" title="近 7 天没有可分析的评论" description="评论内容分词后按词频排序。" />
            </div>
          </el-col>
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <h4 class="chart-title">作品标签分布（当前作品库）</h4>
              <p class="chart-sub">作者在写什么 —— 标签是作品固有属性，不按时间窗筛选</p>
              <div v-if="hotTag.length" class="hot-list">
                <div v-for="(w, i) in hotTag" :key="w.word" class="hot-row">
                  <span class="hot-rank" :class="{ top: i < 3 }">{{ i + 1 }}</span>
                  <span class="hot-word">{{ w.word }}</span>
                  <div class="hot-track"><div class="hot-fill" :style="{ width: hotPct(hotTag, w.count) }"></div></div>
                  <span class="hot-count">{{ w.count }}</span>
                </div>
              </div>
              <StateBlock v-else state="empty" title="近 7 天没有标签数据" description="作品填写标签后会计入这里（按作品库分布，不限时间窗）。" />
            </div>
          </el-col>
        </el-row>
      </template>

      <!-- ===================== 幕 3｜用户与终端 ===================== -->
      <template v-else>
        <el-row :gutter="20" class="chart-row">
          <el-col :xs="24" :md="12">
            <DashboardCharts :novel-bars="topNovelBars" />
          </el-col>
          <el-col :xs="24" :md="12">
            <DashboardCharts :category-pie="categoryPieData" />
          </el-col>
        </el-row>

        <!-- 客户端画像：条形改环形图，占比一眼可读 -->
        <el-row :gutter="20" class="chart-row">
          <el-col :xs="24" :lg="8" v-for="(grp, gi) in clientGroups" :key="gi">
            <div class="hm-card">
              <h4 class="chart-title">{{ grp.title }}</h4>
              <div v-if="!grp.items || grp.items.length === 0" class="client-empty">近 30 天暂无数据</div>
              <v-chart v-else :option="clientPieOption(grp)" autoresize class="client-pie" />
            </div>
          </el-col>
        </el-row>

        <!-- AI 使用画像：用户在点什么模型 / 请求实际打到哪条渠道 -->
        <el-row :gutter="20" class="chart-row">
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <h4 class="chart-title">模型选择热度（近 30 天）</h4>
              <p class="chart-sub">按模型主名分组、支链堆叠 —— 谁在承担主要流量，是哪个版本在扛</p>
              <v-chart v-if="aiModelFamily.length" :option="aiStackOption(aiModelFamily)" autoresize class="ai-bar" />
              <StateBlock v-else state="empty" title="近 30 天没有 AI 调用" description="AI 工作台有实际对话后，这里会显示模型热度。" />
            </div>
          </el-col>
          <el-col :xs="24" :lg="12">
            <div class="hm-card">
              <h4 class="chart-title">渠道路由频次（近 30 天）</h4>
              <p class="chart-sub">路由把请求实际打到了哪条上游 —— 哪条渠道在扛量</p>
              <v-chart v-if="aiTopChannels.length" :option="aiBarOption(aiTopChannels)" autoresize class="ai-bar" />
              <StateBlock v-else state="empty" title="没有渠道调用记录" description="路由命中后会按上游地址聚合到这里。" />
            </div>
          </el-col>
        </el-row>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { adminApi } from '@/api/admin';
import { opsApi } from '@/api/admin/ops';
import {
  User, Document, ChatLineRound, Connection,
  Refresh
} from '@element-plus/icons-vue';
import AnimatedNumber from '@/components/common/AnimatedNumber.vue';
import DashboardCharts from '@/components/admin/DashboardCharts.vue';
import StdMap from '@/components/maps/StdMap.vue';
import AdminActiveHoursHeatmap from '@/components/admin/AdminActiveHoursHeatmap.vue';
import StateBlock from '@/components/common/StateBlock.vue';

const refreshing = ref(false);

const stats = reactive({ users: 0, novels: 0, comments: 0, forums: 0 });

// 备案状态（176号令 第七条第一项，F7 首页提醒）
const filingReady = ref(true);
async function checkFiling() {
  try {
    const res = await adminApi.getConfig();
    const d = res.data.data || {};
    filingReady.value = !!(d.site_icp && d.site_psb);
  } catch { /* 保持默认（视为已配置，避免误报） */ }
}

// 统计卡片
const statCardDefs = computed(() => [
  { label: '用户数', value: stats.users, icon: User, gradient: 'var(--gradient-blue)' },
  { label: '作品数', value: stats.novels, icon: Document, gradient: 'var(--gradient-purple)' },
  { label: '评论数', value: stats.comments, icon: ChatLineRound, gradient: 'var(--gradient-teal)' },
  { label: '论坛数', value: stats.forums, icon: Connection, gradient: 'var(--gradient-accent)' },
]);

// 管理功能入口已抽离至 @/components/admin/AdminMenuPanel.vue（分类折叠 + 深链定位）

// 折线图数据
const userTrend = ref<any>({
  title: '用户增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增用户',
  secondValues: [],
  secondName: '活跃用户',
});

const chapterTrend = ref<any>({
  title: '章节增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增章节',
});

// 柱状图数据
const topNovelBars = ref<any>({
  title: 'Top 6 作品字数对比',
  labels: [] as string[],
  values: [] as number[],
  seriesName: '字数',
});

// 饼图数据
const categoryPieData = ref<any>({
  title: '分类分布',
  data: [],
});

// 热力图数据
const provinceHeat = ref<any[]>([]); // [{name:'广东省', value: N}] → ChinaGeoMap 省级着色
// 境外按国家聚合（同一个 ip2region 离线库，本地解析，不调任何外部 API）
const countryHeat = ref<any[]>([]); // [{code:'US', name:'United States', value:N, cn:false}]
const geoSummary = ref<any>(null);  // {cn, overseas, reserved, cn_pct, os_pct}
const activeHours = ref<number[][]>([]); // 7×24 星期×小时 互动矩阵

// AI 使用画像：模型热度 / 渠道路由频次（来自 AiCallLog 聚合，与「模型统计」同口径）
const aiTopModels = ref<any[]>([]);
// 主名 → 支链堆叠（family / total / variants[{name,requests}]）
const aiModelFamily = ref<any[]>([]);
const aiTopChannels = ref<any[]>([]);

// 客户端画像（结构化 UA 解析：OS / 浏览器 / 设备类型）
const clientData = ref<{ os: any[]; browser: any[]; device: any[] }>({ os: [], browser: [], device: [] });
const clientGroups = computed(() => [
  { key: 'os', title: '操作系统分布（近30天）', items: clientData.value.os || [] },
  { key: 'browser', title: '浏览器分布（近30天）', items: clientData.value.browser || [] },
  { key: 'device', title: '设备类型分布（近30天）', items: clientData.value.device || [] },
]);
function deviceName(n: string) {
  return ({ desktop: '桌面端', phone: '手机端', tablet: '平板', unknown: '未知' } as Record<string, string>)[n] || n;
}
function barPct(items: any[], cnt: number) {
  const max = Math.max(1, ...items.map((x: any) => Number(x.cnt) || 0));
  return Math.round((Number(cnt) / max) * 100) + '%';
}

// ==================== 三幕切换 ====================
// 数据量大，一屏塞不下：幕 1 总览（趋势与地域）、幕 2 内容与热词、幕 3 用户与终端。
// 用 v-if 而不是 v-show：ECharts 在 display:none 的容器里挂不上 canvas，
// 隐藏幕里的图表会变成空白，切回来也不会自己画（真实踩过）。
type Scene = 'overview' | 'content' | 'clients';
const SCENE_KEYS: Scene[] = ['overview', 'content', 'clients'];
const scenes = [
  { key: 'overview' as Scene, label: '总览', icon: '📊' },
  { key: 'content' as Scene, label: '内容与热词', icon: '🔥' },
  { key: 'clients' as Scene, label: '用户与终端', icon: '💻' },
];

const route = useRoute();
const router = useRouter();

// 幕用 query 参数（?scene=content）：能随路由前进/后退、可直接分享到某一幕，
// 与后台其它页的 ?item= 深链风格一致，不用 #hash 另立一套。
function readScene(): Scene {
  const q = String(route.query.scene || '');
  return (SCENE_KEYS as string[]).includes(q) ? (q as Scene) : 'overview';
}
const scene = ref<Scene>(readScene());

function switchScene(k: Scene) {
  scene.value = k;
  // 保留其它 query，只覆盖 scene；replace 避免切幕堆历史
  router.replace({ query: { ...route.query, scene: k } });
}

// 外部改动地址（粘贴链接 / 前进后退）时同步
watch(
  () => route.query.scene,
  () => {
    const k = readScene();
    if (k !== scene.value) scene.value = k;
  },
);

// 客户端分布：环形图（原先只有进度条，看不出占比关系）
const PIE_COLORS = ['#6366f1', '#14b8a6', '#f59e0b', '#ef4444', '#8b5cf6', '#0ea5e9', '#84cc16', '#f472b6'];
function clientPieOption(grp: { key: string; items: any[] }) {
  const items = (grp.items || []).map((it: any) => ({
    name: grp.key === 'device' ? deviceName(it.name) : it.name,
    value: Number(it.cnt) || 0,
  }));
  return {
    tooltip: { trigger: 'item', formatter: '{b}：{c}（{d}%）' },
    legend: { bottom: 0, type: 'scroll', itemWidth: 9, itemHeight: 9, textStyle: { fontSize: 11 } },
    color: PIE_COLORS,
    series: [
      {
        type: 'pie',
        radius: ['46%', '68%'],
        center: ['50%', '44%'],
        avoidLabelOverlap: true,
        itemStyle: { borderColor: 'var(--bg-card, #fff)', borderWidth: 2 },
        label: { show: false },
        labelLine: { show: false },
        emphasis: { label: { show: true, fontSize: 13, fontWeight: 'bold', formatter: '{b}\n{d}%' } },
        data: items,
      },
    ],
  };
}

// ==================== 幕 2：违禁词与热词 ====================
const badword = reactive({ blocked: 0, pending: 0, confirmed: 0, library: 0, lib_enabled: 0 });
const badwordWords = ref<{ word: string; count: number }[]>([]);
const hotSearch = ref<{ word: string; count: number }[]>([]);
const hotContent = ref<{ word: string; count: number }[]>([]);
const hotTag = ref<{ word: string; count: number }[]>([]);

// 词云不是 ECharts 插件（项目没装 echarts-wordcloud，不为此引入新依赖）：
// 按词频映射字号 + 分档配色，纯 CSS 排版即可，且不增加打包体积。
const wcMax = computed(() => Math.max(1, ...badwordWords.value.map((w) => w.count)));
function wcStyle(w: { word: string; count: number }) {
  const ratio = w.count / wcMax.value;
  const size = Math.round(13 + ratio * 21); // 13 ~ 34px
  // 高频词用暖色高亮，低频词降饱和，视觉上自然分层
  const palette = ['#dc2626', '#ea580c', '#d97706', '#7c3aed', '#2563eb', '#0f766e', '#475569'];
  const idx = Math.min(palette.length - 1, Math.floor((1 - ratio) * palette.length));
  return {
    fontSize: `${size}px`,
    color: palette[idx],
    fontWeight: ratio > 0.6 ? '700' : '500',
    opacity: String(0.55 + ratio * 0.45),
  };
}

// 模型热度：主名分组 + 支链堆叠。
// 为什么不是「每个完整模型一根柱」：deepseek-v4.1-flash / -pro 会各占一列，
// 十几列之后完全看不出「主力是哪个家族」；堆叠后一眼看到家族总量，再看支链占比。
// 支链少的（1~2 个）看起来仍是普通柱，不会因为堆叠而变乱。
function aiStackOption(list: { family: string; total: number; variants: { name: string; requests: number }[] }[]) {
  const isDark = document.documentElement.classList.contains('dark');
  const axis = isDark ? '#94a3b8' : '#64748b';
  const split = isDark ? 'rgba(148,163,184,0.18)' : 'rgba(148,163,184,0.22)';
  // 支链颜色：按出现顺序取调色板，同一支链在不同家族里颜色一致
  const palette = ['#6366f1', '#14b8a6', '#f59e0b', '#ef4444', '#8b5cf6', '#0ea5e9', '#84cc16'];
  const variantNames: string[] = [];
  list.forEach((f) => f.variants.forEach((v) => { if (!variantNames.includes(v.name)) variantNames.push(v.name); }));

  return {
    grid: { left: 8, right: 16, top: 28, bottom: 8, containLabel: true },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { top: 0, textStyle: { color: axis, fontSize: 10 }, itemHeight: 8, itemWidth: 10 },
    xAxis: {
      type: 'category',
      data: list.map((f) => f.family),
      axisLabel: { color: axis, fontSize: 10, interval: 0, rotate: list.length > 4 ? 20 : 0 },
      axisLine: { lineStyle: { color: split } },
      axisTick: { show: false },
    },
    yAxis: { type: 'value', axisLabel: { color: axis, fontSize: 10 }, splitLine: { lineStyle: { color: split } } },
    series: variantNames.map((vn, i) => ({
      name: vn,
      type: 'bar',
      stack: 'ai-model',
      barMaxWidth: 34,
      itemStyle: { color: palette[i % palette.length], borderRadius: i === variantNames.length - 1 ? [4, 4, 0, 0] : 0 },
      data: list.map((f) => f.variants.find((v) => v.name === vn)?.requests || 0),
    })),
  };
}

// AI 排行图：横向柱状 —— 模型名/渠道地址普遍很长，纵向柱子会把标签压成省略号。
// 倒序传入让「第一名在最上面」，符合排行榜直觉；数值直接标在柱尾，不用悬停才看到。
function aiBarOption(list: { name: string; requests: number; tokens?: number }[]) {
  const rows = [...list].sort((a, b) => a.requests - b.requests);
  const isDark = document.documentElement.classList.contains('dark');
  const axis = isDark ? '#94a3b8' : '#64748b';
  const split = isDark ? 'rgba(148,163,184,0.18)' : 'rgba(148,163,184,0.22)';
  return {
    grid: { left: 8, right: 56, top: 8, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (ps: any[]) => {
        const p = ps[0];
        const row = rows[p.dataIndex];
        const tok = row.tokens ? `<br/>tokens ${row.tokens.toLocaleString()}` : '';
        return `${p.name}<br/>请求 ${row.requests}${tok}`;
      },
    },
    xAxis: { type: 'value', axisLabel: { color: axis, fontSize: 10 }, splitLine: { lineStyle: { color: split } } },
    yAxis: {
      type: 'category',
      data: rows.map((r) => r.name),
      axisLabel: { color: axis, fontSize: 10, width: 150, overflow: 'truncate' },
      axisLine: { lineStyle: { color: split } },
      axisTick: { show: false },
    },
    series: [{
      type: 'bar',
      data: rows.map((r) => r.requests),
      barMaxWidth: 14,
      itemStyle: {
        borderRadius: [0, 6, 6, 0],
        color: { type: 'linear', x: 0, y: 0, x2: 1, y2: 0,
          colorStops: [{ offset: 0, color: '#6366f1' }, { offset: 1, color: '#14b8a6' }] },
      },
      label: { show: true, position: 'right', color: axis, fontSize: 10 },
    }],
  };
}

function hotPct(list: { count: number }[], count: number) {
  const max = Math.max(1, ...list.map((x) => x.count));
  return Math.max(4, Math.round((count / max) * 100)) + '%';
}

// 左上图：只看「用户增长」。此前左右两列渲染的是同一个 adminTrendMetrics，
// 两张折线图数据与形状完全一致（复制粘贴留下的缺陷）。
const userOnlyMetrics = computed(() => [
  { label: '用户增长趋势（近7天）', data: userTrend.value },
]);

// 右上图：访问趋势（PV/UV，来自访客统计）+ 章节/评论增长，
// 与左上「用户增长」数据来源完全不同，两张图不会再撞脸。
const visitorTrend = ref<any>({ title: '访问趋势（近7天）', dates: [], values: [], seriesName: 'PV' });
const visitorMetricKey = ref<'pv' | 'uv'>('pv');

const visitorMetrics = computed(() => {
  const opts: any[] = [
    {
      label: visitorMetricKey.value === 'pv' ? '访问趋势·PV（近7天）' : '访问趋势·UV（近7天）',
      data: visitorTrend.value,
    },
  ];
  if (chapterTrend.value?.dates?.length) {
    opts.push({ label: '章节增长趋势（近7天）', data: chapterTrend.value });
  }
  if (commentTrend.value?.dates?.length) {
    opts.push({ label: '评论增长趋势（近7天）', data: commentTrend.value });
  }
  if (wordCountTrend.value?.dates?.length) {
    opts.push({ label: '字数增长趋势（近7天）', data: wordCountTrend.value });
  }
  return opts;
});

async function loadVisitorTrend() {
  try {
    const res = await opsApi.adminVisit();
    const stats = res.data?.data?.stats || [];
    const dates = stats.map((s: any) => s.date);
    const values = stats.map((s: any) => Number(s[visitorMetricKey.value]) || 0);
    visitorTrend.value = {
      title: visitorMetricKey.value === 'pv' ? '访问趋势·PV（近7天）' : '访问趋势·UV（近7天）',
      dates,
      values,
      seriesName: visitorMetricKey.value.toUpperCase(),
    };
  } catch {
    // 访客统计拿不到就保持空图，不影响其它卡片
  }
}

const commentTrend = ref<any>({
  title: '评论增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增评论',
});

const wordCountTrend = ref<any>({
  title: '字数增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增字数',
});

// ===== 仪表盘数据 =====
async function loadDashboard() {
  try {
    const res = await adminApi.getDashboardStats();
    const d = res.data.data;
    if (d?.stats) {
      stats.users = d.stats.users || 0;
      stats.novels = d.stats.novels || 0;
      stats.comments = d.stats.comments || 0;
      stats.forums = d.stats.forums || 0;
    }
    const ut = d?.user_trend;
    if (ut) {
      userTrend.value = { title: '用户增长趋势（近7天）', dates: ut.dates || [], values: ut.new_users || [], seriesName: '新增用户' };
    }
    const ct = d?.chapter_trend;
    if (ct) {
      chapterTrend.value = { title: '章节增长趋势（近7天）', dates: ct.dates || [], values: ct.counts || [], seriesName: '新增章节' };
    }
    const cmt = d?.comment_trend;
    if (cmt) {
      commentTrend.value = { title: '评论增长趋势（近7天）', dates: cmt.dates || [], values: cmt.counts || [], seriesName: '新增评论' };
    }
    const wct = d?.word_count_trend;
    if (wct) {
      wordCountTrend.value = { title: '字数增长趋势（近7天）', dates: wct.dates || [], values: wct.counts || [], seriesName: '每日新增', secondValues: wct.cumulative || [], secondName: '累计总字数' };
    }
    const tn = d?.top_novels;
    if (tn) {
      topNovelBars.value = { title: 'Top 6 作品字数对比', labels: tn.map((n: any) => n.title), values: tn.map((n: any) => n.total_words), seriesName: '字数' };
    }
    const cd = d?.category_distribution;
    if (cd) {
      categoryPieData.value = { title: '分类分布', data: cd.map((c: any) => ({ name: c.name, value: c.count })) };
    }
    const ph = d?.province_heat;
    if (Array.isArray(ph)) provinceHeat.value = ph;
    const ch = d?.country_heat;
    if (Array.isArray(ch)) countryHeat.value = ch;
    if (d?.geo_summary?.available) geoSummary.value = d.geo_summary;
    const ah = d?.active_hours;
    if (Array.isArray(ah)) activeHours.value = ah;
    if (d?.client) clientData.value = d.client;
    if (Array.isArray(d?.ai_top_models)) aiTopModels.value = d.ai_top_models;
    if (Array.isArray(d?.ai_model_family)) aiModelFamily.value = d.ai_model_family;
    if (Array.isArray(d?.ai_top_channels)) aiTopChannels.value = d.ai_top_channels;

    // 幕 2：违禁词命中与词云 + 三类热词（后端同一次响应带回，避免三个额外请求）
    const bw = d?.badword;
    if (bw?.available) {
      badword.blocked = bw.blocked || 0;
      badword.pending = bw.pending || 0;
      badword.confirmed = bw.confirmed || 0;
      badword.library = bw.library || 0;
      badword.lib_enabled = bw.lib_enabled || 0;
      badwordWords.value = Array.isArray(bw.words) ? bw.words : [];
    }
    const hw = d?.hotwords;
    if (hw) {
      hotSearch.value = Array.isArray(hw.search) ? hw.search : [];
      hotContent.value = Array.isArray(hw.content) ? hw.content : [];
      hotTag.value = Array.isArray(hw.tag) ? hw.tag : [];
    }
  } catch { /* ignore */ }
}

async function refreshAll() {
  refreshing.value = true;
  await loadDashboard();
  refreshing.value = false;
}

onMounted(() => { loadDashboard(); checkFiling(); loadVisitorTrend(); });
</script>

<style scoped>
/* ===== 全局背景 ===== */
.dashboard-bg {
  min-height: 100vh;
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 40%, #0f172a 100%);
  position: relative;
}

[data-theme="light"] .dashboard-bg {
  background: linear-gradient(135deg, #e2e8f0 0%, #f1f5f9 40%, #e2e8f0 100%);
}

.dashboard-bg::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background:
    radial-gradient(ellipse at 20% 20%, rgba(99, 102, 241, 0.08) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(139, 92, 246, 0.06) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(52, 211, 153, 0.04) 0%, transparent 50%);
  pointer-events: none;
  z-index: 0;
}

[data-theme="light"] .dashboard-bg::before {
  background:
    radial-gradient(ellipse at 20% 20%, rgba(99, 102, 241, 0.04) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(139, 92, 246, 0.03) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(52, 211, 153, 0.02) 0%, transparent 50%);
}

/* ===== 三幕切换（顶部按钮） ===== */
.dash-tabs {
  display: flex; gap: 8px; flex-wrap: wrap;
  margin-bottom: 18px;
  padding: 6px;
  border-radius: 14px;
  background: var(--bg-card, rgba(255, 255, 255, 0.72));
  border: 1px solid var(--border-color, #e8eaf0);
  backdrop-filter: blur(8px);
}
.dash-tab {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 9px 20px; border: none; border-radius: 10px;
  background: transparent; cursor: pointer;
  font-size: 0.88rem; font-weight: 600;
  color: var(--text-light, #64748b);
  transition: background 0.16s, color 0.16s, box-shadow 0.16s;
}
.dash-tab:hover { background: var(--bg-subtle, rgba(15, 23, 42, 0.04)); color: var(--text-main, #334155); }
.dash-tab.active {
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  color: #fff;
  box-shadow: 0 6px 16px rgba(99, 102, 241, 0.28);
}
.dash-tab-ico { font-size: 1rem; line-height: 1; }

/* ===== 图表副标题（热词/词云的说明行） ===== */
.chart-sub {
  margin: -2px 0 10px; font-size: 0.74rem; line-height: 1.6;
  color: var(--text-light, #94a3b8);
}

/* ===== 违禁词词云 ===== */
.wc {
  display: flex; flex-wrap: wrap; align-items: center; justify-content: center;
  gap: 6px 14px; padding: 16px 8px; min-height: 168px;
  line-height: 1.4;
}
.wc-item {
  cursor: default;
  transition: transform 0.14s ease, filter 0.14s ease;
  font-variant-numeric: tabular-nums;
}
.wc-item:hover { transform: scale(1.12); filter: brightness(0.85); }

/* ===== 热词排行（搜索 / 评论 / 标签共用） ===== */
.hot-list { display: flex; flex-direction: column; gap: 7px; }
.hot-row { display: flex; align-items: center; gap: 8px; font-size: 0.8rem; }
.hot-rank {
  width: 20px; height: 20px; flex: none;
  display: flex; align-items: center; justify-content: center;
  border-radius: 6px; font-size: 0.7rem; font-weight: 700;
  background: var(--bg-subtle, #f1f5f9); color: var(--text-light, #94a3b8);
}
.hot-rank.top { background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6)); color: #fff; }
.hot-word {
  min-width: 84px; max-width: 150px; flex: none;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  color: var(--text-main, #334155);
}
.hot-track {
  flex: 1; height: 8px; border-radius: 999px;
  background: var(--bg-page, #eef2f7); overflow: hidden;
}
.hot-fill {
  height: 100%; border-radius: 999px;
  background: linear-gradient(90deg, #6366f1, #14b8a6);
  transition: width 0.4s ease;
}
.hot-count {
  width: 46px; text-align: right; flex: none;
  color: var(--text-light, #64748b); font-variant-numeric: tabular-nums;
}

/* AI 排行横向柱状图：高度随条目数自适应（最多 8 条） */
.ai-bar { width: 100%; height: 248px; }

/* 渠道地址较长：等宽 + 省略，避免把进度条挤没 */
.hot-word.host-text { font-family: var(--font-mono, ui-monospace, Menlo, monospace); font-size: 0.74rem; }

/* 客户端分布环形图容器 */
.client-pie { width: 100%; height: 232px; }

.dashboard-container {
  max-width: 1280px;
  margin: 0 auto;
  padding: 40px 24px 60px;
  position: relative;
  z-index: 1;
}

/* ===== Hero ===== */
.dashboard-hero {
  text-align: center;
  margin-bottom: 40px;
}

.dashboard-main-title {
  font-size: 2.4rem;
  font-weight: 800;
  margin: 0 0 8px;
  line-height: 1.3;
}

.title-glow {
  background: linear-gradient(135deg, #818cf8, #c084fc, #34d399);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  text-shadow: none;
  filter: drop-shadow(0 0 18px rgba(129, 140, 248, 0.35));
}

.dashboard-subtitle {
  color: var(--text-light);
  font-size: 0.95rem;
  margin: 0 0 16px;
}

.refresh-btn {
  background: rgba(255, 255, 255, 0.08) !important;
  border: 1px solid rgba(255, 255, 255, 0.15) !important;
  color: #e2e8f0 !important;
  backdrop-filter: blur(8px);
}

[data-theme="light"] .refresh-btn {
  background: rgba(0, 0, 0, 0.04) !important;
  border-color: rgba(0, 0, 0, 0.1) !important;
  color: var(--text-color) !important;
}

.refresh-btn:hover {
  background: rgba(255, 255, 255, 0.14) !important;
  border-color: rgba(255, 255, 255, 0.25) !important;
}

[data-theme="light"] .refresh-btn:hover {
  background: rgba(0, 0, 0, 0.08) !important;
  border-color: rgba(0, 0, 0, 0.15) !important;
}

/* ===== 统计卡片 ===== */
.stats-row {
  margin-bottom: 24px;
}

.glass-stat-card {
  background: rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  padding: 24px 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease;
}

[data-theme="light"] .glass-stat-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.glass-stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
  border-color: rgba(255, 255, 255, 0.2);
}

[data-theme="light"] .glass-stat-card:hover {
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.glass-stat-icon {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}

.glass-stat-body {
  flex: 1;
  min-width: 0;
}

.glass-stat-value :deep(.animated-number-value) {
  background: linear-gradient(135deg, #e2e8f0, #f1f5f9);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-size: 2rem;
}

[data-theme="light"] .glass-stat-value :deep(.animated-number-value) {
  background: var(--gradient-primary);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.glass-stat-label {
  font-size: 0.82rem;
  color: rgba(255, 255, 255, 0.5);
  margin-top: 2px;
}

[data-theme="light"] .glass-stat-label {
  color: var(--text-light);
}

/* ===== 图表行 ===== */
.chart-row {
  margin-bottom: 24px;
}

/* ===== 热力图卡片 ===== */
.hm-card {
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, rgba(0,0,0,0.06));
  border-radius: 16px;
  padding: 18px 20px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.04);
  height: 100%;
}
[data-theme="dark"] .hm-card {
  background: rgba(30,41,59,0.8);
  border-color: rgba(71,85,105,0.3);
}
.hm-card .chart-title { margin-bottom: 6px; }

/* 境内/境外/内网 三口径占比（放在地图上方，先给结论再看图） */
.geo-sum {
  display: flex; flex-wrap: wrap; gap: 14px;
  margin-bottom: 8px; font-size: 0.78rem; color: var(--text-light, #6b7280);
}
.geo-sum b { color: var(--text-main, #1f2937); font-variant-numeric: tabular-nums; }

/* ===== 客户端分布条形 ===== */
.client-empty { color: var(--text-light, #94a3b8); font-size: 0.82rem; padding: 8px 0; }
.client-bar-row { display: flex; align-items: center; gap: 8px; margin: 7px 0; font-size: 0.82rem; }
.client-bar-label { width: 56px; flex-shrink: 0; color: var(--text-color, #1e293b); text-align: right; }
.client-bar-track { flex: 1; height: 10px; background: rgba(99,102,241,0.12); border-radius: 6px; overflow: hidden; }
.client-bar-fill { height: 100%; background: linear-gradient(90deg, #6366f1, #8b5cf6); border-radius: 6px; transition: width 0.4s ease; }
.client-bar-count { width: 48px; flex-shrink: 0; color: var(--text-light, #94a3b8); text-align: left; }

/* ====== 移动端管理员仪表盘适配 ====== */
@media (max-width: 768px) {
  .dashboard-container {
    padding: 20px 12px 40px;
  }
  .dashboard-hero {
    margin-bottom: 24px;
  }
  .dashboard-main-title {
    font-size: 1.5rem;
  }
  .dashboard-subtitle {
    font-size: 0.85rem;
  }
  .hero-buttons {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }
  .hero-buttons .el-button {
    flex: 1;
    min-width: 120px;
    margin-left: 0 !important;
  }
  .stats-row .el-col,
  .chart-row .el-col {
    margin-bottom: 14px;
  }
  .stats-row .glass-stat-card {
    padding: 14px;
  }
  .menu-section {
    padding: 0 2px;
  }
}
</style>
