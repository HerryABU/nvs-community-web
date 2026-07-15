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

      <!-- 第一行：统计卡片 -->
      <el-row :gutter="20" class="stats-row">
        <el-col :xs="24" :sm="12" :md="6" v-for="(card, idx) in statCardDefs" :key="idx">
          <div class="glass-stat-card">
            <div class="glass-stat-icon" :style="{ background: card.gradient }">
              <el-icon :size="24"><component :is="card.icon" /></el-icon>
            </div>
            <div class="glass-stat-body">
              <AnimatedNumber
                :target="card.value"
                :duration="1400"
                class="glass-stat-value"
              />
              <div class="glass-stat-label">{{ card.label }}</div>
            </div>
          </div>
        </el-col>
      </el-row>

      <!-- 第二行：趋势图（多指标切换） -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :metrics="adminTrendMetrics" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :metrics="adminTrendMetrics" />
        </el-col>
      </el-row>

      <!-- 第三行：柱状图 + 饼图 -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :novel-bars="topNovelBars" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :category-pie="categoryPieData" />
        </el-col>
      </el-row>

      <!-- 底部：折叠面板 -->
      <div class="bottom-panels">
        <el-collapse v-model="activePanels" class="glass-collapse">
          <AdminSiteSettings />

           <!-- 隔离墙配置 -->
           <el-collapse-item title="隔离墙配置" name="wall">
             <div class="panel-body">
               <p class="hint-text">管理敏感分区列表及其确认流程。点击分区名称展开/收起详细配置。</p>
               <el-form label-width="100px">
                 <el-form-item label="启用隔离墙">
                   <el-switch v-model="wallEnabled" @change="saveWallConfig" />
                 </el-form-item>
                 <el-form-item label="跨域警告">
                   <el-switch v-model="wallCrossDomainWarning" @change="saveWallConfig" />
                 </el-form-item>
               </el-form>

               <!-- 敏感分区列表 + 详情 -->
               <div class="wall-zone-list">
                 <div v-for="(z, i) in wallZones" :key="i" class="wall-zone-card">
                   <div class="wzc-header" @click="toggleZoneDetail(i)">
                     <el-icon class="wzc-arrow" :class="{ rotated: expandedZoneIdx === i }"><ArrowRight /></el-icon>
                     <el-tag type="danger" size="large">{{ z }}</el-tag>
                     <el-button size="small" type="danger" text @click.stop="removeWallZone(i)" style="margin-left:auto">删除</el-button>
                   </div>
                   <div v-if="expandedZoneIdx === i" class="wzc-body">
                     <el-form label-width="110px" size="small">
                       <el-form-item label="确认步数">
                         <el-input-number v-model="zoneDetails[i].steps" :min="1" :max="5" />
                         <span class="hint-text" style="margin-left:8px">需要用户确认几次（1~5步，默认3）</span>
                       </el-form-item>
                       <el-form-item label="最终确认语">
                         <el-input v-model="zoneDetails[i].confirm_text" placeholder="如：我承诺承担全部阅读责任" />
                         <span class="hint-text" style="margin-left:8px">最后一步要求输入的文字（留空则不要求输入）</span>
                       </el-form-item>
                       <el-form-item label="跨域额外步数">
                         <el-input-number v-model="zoneDetails[i].cross_domain_extra" :min="0" :max="5" />
                         <span class="hint-text" style="margin-left:8px">从其他敏感区跨入时的额外步数</span>
                       </el-form-item>
                       <el-form-item label="介绍文案">
                         <el-input v-model="zoneDetails[i].intro_text" type="textarea" :rows="2" placeholder="第一步显示的介绍内容" />
                       </el-form-item>
                       <el-form-item label="警告列表">
                         <div class="warning-list">
                           <div v-for="(w, wi) in zoneDetails[i].warnings" :key="wi" class="warning-row">
                             <el-input v-model="zoneDetails[i].warnings[wi]" placeholder="警告内容" style="flex:1" />
                             <el-button size="small" type="danger" text @click="zoneDetails[i].warnings.splice(wi,1)"><el-icon><Delete /></el-icon></el-button>
                           </div>
                         </div>
                         <el-button size="small" @click="zoneDetails[i].warnings.push('')" :icon="Plus" style="margin-top:4px">添加警告</el-button>
                       </el-form-item>
                     </el-form>
                   </div>
                 </div>
               </div>

               <div class="add-zone-row" style="margin-top:12px">
                 <el-input v-model="newWallZone" placeholder="输入分区名称后回车添加" size="small" style="width:240px" @keyup.enter="addWallZone" />
                 <el-button size="small" type="primary" @click="addWallZone" :icon="Plus" style="margin-left:8px">添加分区</el-button>
                 <el-button size="small" type="success" @click="saveWallConfig" :loading="savingWall" style="margin-left:8px">保存配置</el-button>
               </div>
             </div>
           </el-collapse-item>

          <!-- 远程站点互通 -->
          <el-collapse-item title="远程站点互通" name="sites">
            <div class="panel-body">
              <p class="hint-text">模仿 AList 的站点互联：添加远程 NVS 站点后可同步作品列表。</p>
              <div style="margin-bottom:12px">
                <el-button type="primary" size="small" @click="showAddSiteDialog">添加站点</el-button>
              </div>
              <el-table :data="federatedSites" v-loading="loadingSites" style="width:100%">
                <el-table-column prop="name" label="站点名称" min-width="120" />
                <el-table-column prop="url" label="站点地址" min-width="180" show-overflow-tooltip />
                <el-table-column prop="api_url" label="API 地址" min-width="200" show-overflow-tooltip />
                <el-table-column prop="status" label="状态" width="90">
                  <template #default="{ row }">
                    <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
                      {{ row.status === 'active' ? '启用' : '停用' }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="novel_count" label="缓存作品" width="90" />
                <el-table-column label="操作" width="260" fixed="right">
                  <template #default="{ row }">
                    <el-button size="small" @click="syncSite(row)" :loading="syncingId === row.id">同步</el-button>
                    <el-button size="small" @click="editSite(row)">编辑</el-button>
                    <el-button size="small" type="danger" @click="deleteSite(row)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </el-collapse-item>

           <!-- 论坛板块管理 -->
           <el-collapse-item title="论坛板块管理" name="forums">
             <div class="panel-body">
               <div style="margin-bottom:12px">
                 <el-button type="primary" size="small" @click="showAddForum = true">新增板块</el-button>
               </div>
               <el-table :data="adminForums" v-loading="loadingForums" style="width:100%">
                 <el-table-column prop="id" label="ID" width="60" />
                 <el-table-column prop="name" label="板块名称" min-width="140" />
                 <el-table-column label="类型" width="110">
                   <template #default="{ row }">
                     <el-tag size="small" :type="forumTypeTag(row.type)">{{ forumTypeLabel(row.type) }}</el-tag>
                   </template>
                 </el-table-column>
                 <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip />
                 <el-table-column prop="thread_count" label="帖子数" width="80" />
                 <el-table-column prop="sort_order" label="排序" width="70" />
                 <el-table-column label="操作" width="160" fixed="right">
                   <template #default="{ row }">
                     <el-button size="small" @click="editForum(row)">编辑</el-button>
                     <el-button size="small" type="danger" @click="deleteForum(row)">删除</el-button>
                   </template>
                 </el-table-column>
               </el-table>
             </div>
            </el-collapse-item>
          <AdminCommunity />
         </el-collapse>
      </div>
     
      <!-- 添加/编辑站点弹窗 -->
      <el-dialog v-model="showAddSite" :title="editingSite ? '编辑站点' : '添加远程站点'" width="520px">
        <el-form :model="siteForm" label-width="100px">
          <el-form-item label="站点名称" required>
            <el-input v-model="siteForm.name" placeholder="如：星海文学" />
          </el-form-item>
          <el-form-item label="站点地址" required>
            <el-input v-model="siteForm.url" placeholder="如：https://nvs.example.com" />
          </el-form-item>
          <el-form-item label="API 地址" required>
            <el-input v-model="siteForm.api_url" placeholder="如：https://nvs.example.com/api" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="siteForm.description" type="textarea" :rows="2" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="showAddSite = false">取消</el-button>
          <el-button type="primary" @click="submitSite" :loading="submittingSite">
            {{ editingSite ? '保存' : '添加' }}
          </el-button>
        </template>
      </el-dialog>

      <!-- 添加/编辑论坛弹窗 -->
      <el-dialog v-model="showAddForum" :title="editingForum ? '编辑板块' : '新增板块'" width="480px">
        <el-form :model="forumForm" label-width="90px">
          <el-form-item label="板块名称" required>
            <el-input v-model="forumForm.name" placeholder="如：创作交流区" maxlength="64" />
          </el-form-item>
          <el-form-item label="板块类型" required>
            <el-select v-model="forumForm.type" style="width:100%">
              <el-option label="综合讨论 (general)" value="general" />
              <el-option label="读者区 (reader)" value="reader" />
              <el-option label="读者-作者 (reader_author)" value="reader_author" />
              <el-option label="作者区 (author)" value="author" />
              <el-option label="敏感区 (sensitive) ⚠" value="sensitive" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="forumForm.type === 'sensitive'" label="绑定分区">
            <el-input v-model="forumForm.zone" placeholder="如：同人区、政治文学区" maxlength="64" />
            <span class="hint-text">填写隔离墙分区名，进入论坛时将触发确认弹窗</span>
          </el-form-item>
          <el-form-item label="排序权重">
            <el-input-number v-model="forumForm.sort_order" :min="0" :max="999" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="forumForm.description" type="textarea" :rows="2" placeholder="可选：板块简介" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="showAddForum = false">取消</el-button>
          <el-button type="primary" @click="submitForum" :loading="submittingForum">
            {{ editingForum ? '保存' : '创建' }}
          </el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, nextTick } from 'vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  User, Document, ChatLineRound, Connection,
  Plus, Refresh, ArrowRight, Delete
} from '@element-plus/icons-vue';
import AnimatedNumber from '@/components/AnimatedNumber.vue';
import DashboardCharts from '@/components/DashboardCharts.vue';
import AdminSiteSettings from '@/components/admin/AdminSiteSettings.vue';
import AdminCommunity from '@/components/admin/AdminCommunity.vue';

const refreshing = ref(false);
const activePanels = ref<string[]>([]);

const stats = reactive({ users: 0, novels: 0, comments: 0, forums: 0 });

// 统计卡片
const statCardDefs = computed(() => [
  { label: '用户数', value: stats.users, icon: User, gradient: 'var(--gradient-blue)' },
  { label: '作品数', value: stats.novels, icon: Document, gradient: 'var(--gradient-purple)' },
  { label: '评论数', value: stats.comments, icon: ChatLineRound, gradient: 'var(--gradient-teal)' },
  { label: '论坛数', value: stats.forums, icon: Connection, gradient: 'var(--gradient-accent)' },
]);

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
// ── 多指标选项：管理员趋势图 ──
// 用户增长 / 章节增长 / 评论增长（可切换）
const adminTrendMetrics = computed(() => {
  const opts: any[] = [
    { label: '用户增长趋势', data: userTrend.value },
    { label: '章节增长趋势', data: chapterTrend.value },
  ];
  if (wordCountTrend.value?.dates?.length) {
    opts.push({ label: '字数增长趋势', data: wordCountTrend.value });
  }
  if (commentTrend.value?.dates?.length) {
    opts.push({ label: '评论增长趋势', data: commentTrend.value });
  }
  return opts;
});

// 管理员评论趋势（从 API 加载）
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

// ===== 隔离墙 =====
function toggleZoneDetail(idx: number) {
  expandedZoneIdx.value = expandedZoneIdx.value === idx ? null : idx;
}

async function saveWallConfig() {
  savingWall.value = true;
  try {
    // 同步 zone_details 的 name 与 zones 列表
    const details = wallZones.value.map((z, i) => {
      const existing = zoneDetails.value[i] || { name: z, steps: 3, confirm_text: '我承诺承担全部阅读责任', warnings: ['该分区内容属于敏感题材。'], intro_text: '', cross_domain_extra: 2 };
      return { name: z, steps: existing.steps || 3, confirm_text: existing.confirm_text || '', warnings: existing.warnings?.filter((w: string) => w.trim()) || [], intro_text: existing.intro_text || '', cross_domain_extra: existing.cross_domain_extra ?? 2 };
    });
    zoneDetails.value = details;
    await adminApi.updateWallConfig({
      zones: wallZones.value,
      zone_details: details,
      enabled: wallEnabled.value,
      cross_domain_warning: wallCrossDomainWarning.value,
    });
    ElMessage.success('隔离墙配置已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingWall.value = false;
}

function addWallZone() {
  const name = newWallZone.value.trim();
  if (!name) return;
  if (wallZones.value.includes(name)) { ElMessage.warning('该分区已存在'); return; }
  wallZones.value.push(name);
  zoneDetails.value.push({ name, steps: 3, confirm_text: '我承诺承担全部阅读责任', warnings: ['该分区内容属于敏感题材。'], intro_text: '', cross_domain_extra: 2 });
  newWallZone.value = '';
  saveWallConfig();
}

function removeWallZone(idx: number) {
  wallZones.value.splice(idx, 1);
  zoneDetails.value.splice(idx, 1);
  if (expandedZoneIdx.value === idx) expandedZoneIdx.value = null;
  else if (expandedZoneIdx.value !== null && expandedZoneIdx.value > idx) expandedZoneIdx.value--;
  saveWallConfig();
}

// ===== 远程站点 =====
function showAddSiteDialog() { resetSiteForm(); showAddSite.value = true; }

async function syncSite(site: any) {
  syncingId.value = site.id;
  try {
    const res = await adminApi.syncSite(site.id);
    if (res.data.code === 0) {
      ElMessage.success(`同步完成，新增 ${res.data.data.synced} 部作品，共 ${res.data.data.total} 部`);
      await loadSites();
    } else {
      ElMessage.error(res.data.message || '同步失败');
    }
  } catch (e: any) {
    ElMessage.error('同步失败：' + (e?.response?.data?.message || e.message));
  }
  syncingId.value = null;
}

function editSite(site: any) {
  editingSite.value = site;
  siteForm.name = site.name;
  siteForm.url = site.url;
  siteForm.api_url = site.api_url;
  siteForm.description = site.description || '';
  showAddSite.value = true;
}

async function submitSite() {
  if (!siteForm.name || !siteForm.url || !siteForm.api_url) { ElMessage.warning('请填写站点名称、地址和API地址'); return; }
  submittingSite.value = true;
  try {
    if (editingSite.value) {
      await adminApi.updateSite(editingSite.value.id, { ...siteForm });
      ElMessage.success('站点已更新');
    } else {
      await adminApi.createSite({ ...siteForm });
      ElMessage.success('站点已添加');
    }
    showAddSite.value = false;
    resetSiteForm();
    await loadSites();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  submittingSite.value = false;
}

function resetSiteForm() {
  editingSite.value = null;
  siteForm.name = '';
  siteForm.url = '';
  siteForm.api_url = '';
  siteForm.description = '';
}

async function deleteSite(site: any) {
  try {
    await ElMessageBox.confirm(`确定要删除站点「${site.name}」吗？`, '确认删除', { type: 'warning' });
    await adminApi.deleteSite(site.id);
    ElMessage.success('已删除');
    await loadSites();
  } catch { /* cancelled */ }
}

// ===== 论坛板块管理 =====
const adminForums = ref<any[]>([]);
const loadingForums = ref(false);
const showAddForum = ref(false);
const editingForum = ref<any>(null);
const submittingForum = ref(false);
const forumForm = reactive({ name: '', type: 'general', zone: '', description: '', sort_order: 0 });

async function loadForums() {
  loadingForums.value = true;
  try {
    const res = await adminApi.getForums();
    adminForums.value = res.data.data || [];
  } catch { /* ignore */ }
  loadingForums.value = false;
}

function editForum(forum: any) {
  editingForum.value = forum;
  forumForm.name = forum.name || '';
  forumForm.type = forum.type || 'general';
  forumForm.zone = forum.zone || '';
  forumForm.description = forum.description || '';
  forumForm.sort_order = forum.sort_order || 0;
  showAddForum.value = true;
}

async function submitForum() {
  if (!forumForm.name) { ElMessage.warning('请填写板块名称'); return; }
  submittingForum.value = true;
  try {
    if (editingForum.value) {
      await adminApi.updateForum(editingForum.value.id, {
        name: forumForm.name,
        type: forumForm.type,
        zone: forumForm.zone,
        description: forumForm.description,
        sort_order: forumForm.sort_order,
      });
      ElMessage.success('板块已更新');
    } else {
      await adminApi.createForum({
        name: forumForm.name,
        type: forumForm.type,
        zone: forumForm.zone,
        description: forumForm.description,
        sort_order: forumForm.sort_order,
      });
      ElMessage.success('板块已创建');
    }
    showAddForum.value = false;
    resetForumForm();
    await loadForums();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '操作失败');
  }
  submittingForum.value = false;
}

function resetForumForm() {
  editingForum.value = null;
  forumForm.name = '';
  forumForm.type = 'general';
  forumForm.zone = '';
  forumForm.description = '';
  forumForm.sort_order = 0;
}

async function deleteForum(forum: any) {
  try {
    await ElMessageBox.confirm(`确定要删除板块「${forum.name}」吗？关联帖子将保留但不可见。`, '确认删除', { type: 'warning' });
    await adminApi.deleteForum(forum.id);
    ElMessage.success('已删除');
    await loadForums();
  } catch { /* cancelled */ }
}

function forumTypeTag(t: string) {
  const map: Record<string, string> = { general: 'info', reader: 'success', reader_author: 'warning', author: 'primary', sensitive: 'danger' };
  return map[t] || 'info';
}

function forumTypeLabel(t: string) {
  const map: Record<string, string> = { general: '综合讨论', reader: '读者区', reader_author: '读者-作者', author: '作者区', sensitive: '敏感区' };
  return map[t] || t;
}


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
  } catch { /* ignore */ }
}

async function refreshAll() {
  refreshing.value = true;
  await loadDashboard();
  await loadSites();
  await loadForums();
  refreshing.value = false;
}

// ===== 远程站点 =====
const sites = ref<any[]>([]);
const loadingSites = ref(false);
const showAddSite = ref(false);
const editingSite = ref<any>(null);
const submittingSite = ref(false);
const syncingId = ref(0);
const siteForm = reactive({ name: '', url: '', api_url: '', description: '' });

async function loadSites() {
  try { const res = await adminApi.getSites(); sites.value = res.data.data || []; } catch { /* ignore */ }
}

// ===== 隔离墙配置 =====
const wallZones = ref<string[]>([]);
const wallEnabled = ref(false);
const wallCrossDomainWarning = ref(false);
const savingWall = ref(false);
const expandedZoneIdx = ref<number | null>(null);
const newWallZone = ref('');
const zoneDetails = ref<any[]>([]);

async function loadWallConfig() {
  try {
    const res = await adminApi.getWallConfig();
    const cfg = res.data.data || res.data || {};
    wallZones.value = cfg.zones || [];
    wallEnabled.value = cfg.enabled !== false;
    wallCrossDomainWarning.value = cfg.cross_domain_warning !== false;
    zoneDetails.value = (cfg.zone_details || wallZones.value.map((z: string) => ({ name: z, warnings: [] })));
  } catch { /* ignore */ }
}

onMounted(() => {
  loadDashboard();
  loadSites();
  loadWallConfig();
  loadForums();
});
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

/* ===== 底部面板 ===== */
.bottom-panels {
  margin-top: 8px;
}

.glass-collapse {
  border: none !important;
  background: transparent !important;
}

.glass-collapse :deep(.el-collapse-item) {
  background: rgba(255, 255, 255, 0.05) !important;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.08) !important;
  border-radius: 14px !important;
  margin-bottom: 16px !important;
  overflow: hidden;
}

[data-theme="light"] .glass-collapse :deep(.el-collapse-item) {
  background: #fff !important;
  border-color: var(--border-color) !important;
}

.glass-collapse :deep(.el-collapse-item__header) {
  background: rgba(255, 255, 255, 0.04) !important;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06) !important;
  color: #e2e8f0 !important;
  font-size: 1rem !important;
  font-weight: 600 !important;
  padding: 0 24px !important;
  height: 52px !important;
  border-radius: 14px 14px 0 0 !important;
}

[data-theme="light"] .glass-collapse :deep(.el-collapse-item__header) {
  background: var(--bg-color) !important;
  border-bottom-color: var(--border-color) !important;
  color: var(--primary-color) !important;
}

.glass-collapse :deep(.el-collapse-item__header.is-active) {
  border-bottom-color: rgba(255, 255, 255, 0.1) !important;
}

[data-theme="light"] .glass-collapse :deep(.el-collapse-item__header.is-active) {
  border-bottom-color: var(--border-color) !important;
}

.glass-collapse :deep(.el-collapse-item__wrap) {
  background: transparent !important;
  border: none !important;
}

.glass-collapse :deep(.el-collapse-item__content) {
  color: #cbd5e1 !important;
  padding: 24px !important;
}

[data-theme="light"] .glass-collapse :deep(.el-collapse-item__content) {
  color: var(--text-color) !important;
}

.panel-body {
  color: #cbd5e1;
}

[data-theme="light"] .panel-body {
  color: var(--text-color);
}

.panel-body .hint-text {
  color: rgba(255, 255, 255, 0.45);
  font-size: 0.85rem;
  margin-bottom: 14px;
}

[data-theme="light"] .panel-body .hint-text {
  color: var(--text-light);
}

/* ===== 其他 ===== */
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.category-tag-item {
  cursor: pointer;
  user-select: none;
}

.cat-edit-input {
  width: 100px;
}

/* ===== 隔离墙 Zone 卡片 ===== */
.wall-zone-list {
  margin: 12px 0;
}

.wall-zone-card {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  margin-bottom: 8px;
  overflow: hidden;
}

[data-theme="light"] .wall-zone-card {
  background: #fafafa;
  border-color: #e5e7eb;
}

.wzc-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}

.wzc-header:hover {
  background: rgba(255, 255, 255, 0.06);
}

[data-theme="light"] .wzc-header:hover {
  background: #f0f0f0;
}

.wzc-arrow {
  transition: transform 0.2s;
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.5);
}

.wzc-arrow.rotated {
  transform: rotate(90deg);
}

[data-theme="light"] .wzc-arrow {
  color: #999;
}

.wzc-body {
  padding: 14px 18px 18px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(0, 0, 0, 0.12);
}

[data-theme="light"] .wzc-body {
  border-top-color: #e5e7eb;
  background: #f5f5f5;
}

.warning-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.warning-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.add-zone-row {
  display: flex;
  align-items: center;
  margin-top: 8px;
}
</style>