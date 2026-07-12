<template>
  <div class="dashboard-bg">
    <div class="dashboard-container">
      <!-- 顶部标题 -->
      <div class="dashboard-hero">
        <h1 class="dashboard-main-title">
          <span class="title-glow">管理员数据大屏</span>
        </h1>
        <p class="dashboard-subtitle">实时监控平台数据 · 管理站点配置</p>
        <el-button class="refresh-btn" @click="refreshAll" :loading="refreshing" size="default">
          <el-icon><Refresh /></el-icon>刷新数据
        </el-button>
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

      <!-- 第二行：折线图 -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :visitor-trend="userTrend" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :visitor-trend="chapterTrend" />
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
           <!-- 站点设置 -->
           <el-collapse-item title="站点设置" name="site">
             <div class="panel-body">
               <el-form :model="configForm" label-width="120px" @submit.prevent="saveConfig">
                 <el-form-item label="站点名称">
                   <el-input v-model="configForm.site_name" placeholder="星海文学" maxlength="64" />
                 </el-form-item>
                 <el-form-item label="VIP 付费">
                   <el-switch v-model="vipEnabled" active-text="开启" inactive-text="关闭" @change="onVipToggle" />
                   <span class="hint-text">关闭后，作者无法申请 VIP 付费功能</span>
                 </el-form-item>
                 <el-divider />
                 <h3 style="margin:0 0 12px 0;font-size:1rem;">邮件验证设置</h3>
                 <el-form-item label="邮箱验证">
                   <el-switch v-model="configForm.email_verify" active-text="开启" inactive-text="关闭" />
                   <span class="hint-text">开启后注册需验证邮箱，关闭则跳过验证</span>
                 </el-form-item>
                 <el-form-item label="SMTP 服务器">
                   <el-input v-model="configForm.smtp_host" placeholder="smtp.qq.com" style="width:220px" />
                 </el-form-item>
                 <el-form-item label="SMTP 端口">
                   <el-input v-model="configForm.smtp_port" placeholder="587" style="width:120px" />
                 </el-form-item>
                 <el-form-item label="发件邮箱">
                   <el-input v-model="configForm.smtp_user" placeholder="your-email@qq.com" style="width:280px" />
                 </el-form-item>
                 <el-form-item label="SMTP 密码/授权码">
                   <el-input v-model="configForm.smtp_password" type="password" show-password placeholder="授权码" style="width:260px" />
                 </el-form-item>
                 <el-form-item label="发件人地址">
                   <el-input v-model="configForm.smtp_from" placeholder="同发件邮箱" style="width:280px" />
                 </el-form-item>
                 <el-divider />
                 <h3 style="margin:0 0 12px 0;font-size:1rem;">安全设置</h3>
                 <el-form-item label="滑块验证码">
                   <el-switch v-model="configForm.captcha_enabled" active-text="开启" inactive-text="关闭" />
                   <span class="hint-text">在注册/登录时显示滑块验证码</span>
                 </el-form-item>
                 <el-form-item>
                   <el-button type="primary" @click="saveConfig" :loading="savingConfig">保存设置</el-button>
                 </el-form-item>
               </el-form>
             </div>
           </el-collapse-item>

          <!-- 分类管理 -->
          <el-collapse-item title="书目分类管理" name="category">
            <div class="panel-body">
              <p class="hint-text">定义平台的书目分类类型，读者可按分类浏览作品。双击标签重命名，点击 × 删除，修改后点击保存。</p>
              <div class="tag-list">
                <el-tag
                  v-for="(cat, idx) in editableCategories"
                  :key="idx"
                  closable
                  size="large"
                  class="category-tag-item"
                  @close="removeCategory(idx)"
                >
                  <template v-if="editingCatIdx === idx">
                    <el-input
                      v-model="editCatName"
                      size="small"
                      class="cat-edit-input"
                      @blur="saveCatName(idx)"
                      @keyup.enter="saveCatName(idx)"
                      @keyup.escape="cancelEditCat"
                      ref="catEditRef"
                    />
                  </template>
                  <template v-else>
                    <span @dblclick="startEditCat(idx, cat)">{{ cat }}</span>
                  </template>
                </el-tag>
              </div>
              <div style="margin-top: 12px">
                <el-button type="primary" @click="addCategory" :icon="Plus">添加分类</el-button>
                <el-button type="primary" @click="saveCategories" :loading="savingCategories">保存分类</el-button>
                <el-button @click="resetCategories" :disabled="savingCategories">重置</el-button>
              </div>
            </div>
          </el-collapse-item>

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
  labels: [],
  values: [],
  seriesName: '字数',
});

// 饼图数据
const categoryPieData = ref<any>({
  title: '分类分布',
  data: [],
});

// 站点设置
const vipEnabled = ref(true);
const configForm = reactive({
  site_name: '星海文学',
  email_verify: false,
  captcha_enabled: false,
  smtp_host: '',
  smtp_port: '',
  smtp_user: '',
  smtp_password: '',
  smtp_from: '',
});
const savingConfig = ref(false);

// 分类管理
const editableCategories = ref<string[]>([]);
const originalCategories = ref<string[]>([]);
const savingCategories = ref(false);
const editingCatIdx = ref<number | null>(null);
const editCatName = ref('');
const catEditRef = ref<any>(null);

// 远程站点
const federatedSites = ref<any[]>([]);
const loadingSites = ref(false);
const syncingId = ref<number | null>(null);
const showAddSite = ref(false);
const editingSite = ref<any>(null);
const siteForm = reactive({ name: '', url: '', api_url: '', description: '' });
const submittingSite = ref(false);

// 隔离墙
const wallEnabled = ref(false);
const wallCrossDomainWarning = ref(false);
const wallZones = ref<string[]>([]);
const newWallZone = ref('');
const savingWall = ref(false);
const expandedZoneIdx = ref<number | null>(null);
interface ZoneDetail {
  name: string;
  steps: number;
  confirm_text: string;
  warnings: string[];
  intro_text: string;
  cross_domain_extra: number;
}
const zoneDetails = ref<ZoneDetail[]>([]);

// ===== 数据加载 =====
async function loadDashboard() {
  try {
    const res = await adminApi.getDashboardStats();
    if (res.data.code === 0) {
      const d = res.data.data;

      if (d.stats) {
        Object.assign(stats, {
          users: d.stats.users || 0,
          novels: d.stats.novels || 0,
          comments: d.stats.comments || 0,
          forums: d.stats.forums || 0,
        });
      }

      if (d.user_trend) {
        userTrend.value = {
          title: '用户增长趋势（近7天）',
          dates: d.user_trend.dates || [],
          values: d.user_trend.new_users || [],
          seriesName: '新增用户',
          secondValues: d.user_trend.active_users || [],
          secondName: '活跃用户',
        };
      }

      if (d.chapter_trend) {
        chapterTrend.value = {
          title: '章节增长趋势（近7天）',
          dates: d.chapter_trend.dates || [],
          values: d.chapter_trend.counts || [],
          seriesName: '新增章节',
        };
      }

      if (d.top_novels) {
        topNovelBars.value = {
          title: 'Top 6 作品字数对比',
          labels: d.top_novels.map((n: any) => n.title || '未命名'),
          values: d.top_novels.map((n: any) => n.total_words || 0),
          seriesName: '字数',
        };
      }

      if (d.category_distribution) {
        categoryPieData.value = {
          title: '分类分布',
          data: d.category_distribution.map((c: any) => ({
            name: c.name || '未分类',
            value: c.count || 0,
          })),
        };
      }
    }
  } catch {
    // fallback: load stats separately
    await loadStats();
  }
}

async function loadStats() {
  try {
    const res = await adminApi.getStats();
    if (res.data.code === 0) {
      const d = res.data.data;
      Object.assign(stats, {
        users: d.users || 0,
        novels: d.novels || 0,
        comments: d.comments || 0,
        forums: d.forums || 0,
      });
    }
  } catch { /* ignore */ }
}

async function loadConfig() {
  try {
    const res = await adminApi.getConfig();
    if (res.data.code === 0) {
      const cfg = res.data.data;
      configForm.site_name = cfg.site_name || '星海文学';
      vipEnabled.value = cfg.vip_enabled !== 'false';
      configForm.email_verify = cfg.email_verify === 'true';
      configForm.captcha_enabled = cfg.captcha_enabled === 'true';
      configForm.smtp_host = cfg.smtp_host || '';
      configForm.smtp_port = cfg.smtp_port || '';
      configForm.smtp_user = cfg.smtp_user || '';
      configForm.smtp_password = cfg.smtp_password || '';
      configForm.smtp_from = cfg.smtp_from || '';
      if (cfg.categories) {
        try {
          const parsed = JSON.parse(cfg.categories);
          if (Array.isArray(parsed)) {
            editableCategories.value = [...parsed];
            originalCategories.value = [...parsed];
          }
        } catch { /* ignore */ }
      }
      if (editableCategories.value.length === 0) {
        editableCategories.value = ['硬科幻', '奇幻', '推演文学', '架空历史', '现实主义', '悬疑推理', '实验文学', '同人区', '其他'];
        originalCategories.value = [...editableCategories.value];
      }
    }
  } catch { /* ignore */ }
}

async function loadSites() {
  loadingSites.value = true;
  try {
    const res = await adminApi.getSites();
    if (res.data.code === 0) federatedSites.value = res.data.data;
  } catch { /* ignore */ }
  loadingSites.value = false;
}

async function loadWallConfig() {
  try {
    const res = await adminApi.getWallConfig();
    if (res.data.code === 0) {
      const cfg = res.data.data;
      wallEnabled.value = !!cfg.enabled;
      wallCrossDomainWarning.value = !!cfg.cross_domain_warning;
      wallZones.value = Array.isArray(cfg.zones) ? [...cfg.zones] : [];
      // 加载 zone_details
      if (Array.isArray(cfg.zone_details)) {
        zoneDetails.value = cfg.zone_details.map((d: any) => ({
          name: d.name || '',
          steps: d.steps || 3,
          confirm_text: d.confirm_text || '我承诺承担全部阅读责任',
          warnings: Array.isArray(d.warnings) ? [...d.warnings] : ['该分区内容属于敏感题材。'],
          intro_text: d.intro_text || '',
          cross_domain_extra: d.cross_domain_extra ?? 2,
        }));
      } else {
        // 为现有 zones 创建默认 detail
        zoneDetails.value = wallZones.value.map(z => ({
          name: z, steps: 3, confirm_text: '我承诺承担全部阅读责任',
          warnings: ['该分区内容属于敏感题材。'], intro_text: '', cross_domain_extra: 2,
        }));
      }
    }
  } catch { /* ignore */ }
}

async function refreshAll() {
  refreshing.value = true;
  await Promise.all([loadDashboard(), loadConfig(), loadSites(), loadWallConfig()]);
  refreshing.value = false;
  ElMessage.success('已刷新');
}

// ===== 站点设置 =====
async function saveConfig() {
  savingConfig.value = true;
  try {
    await adminApi.updateConfig({
      site_name: configForm.site_name,
      vip_enabled: vipEnabled.value ? 'true' : 'false',
      email_verify: configForm.email_verify ? 'true' : 'false',
      captcha_enabled: configForm.captcha_enabled ? 'true' : 'false',
      smtp_host: configForm.smtp_host,
      smtp_port: configForm.smtp_port,
      smtp_user: configForm.smtp_user,
      smtp_password: configForm.smtp_password,
      smtp_from: configForm.smtp_from,
    });
    ElMessage.success('设置已保存');
  } catch { ElMessage.error('保存失败'); }
  savingConfig.value = false;
}

function onVipToggle(val: boolean) {
  vipEnabled.value = val;
}

// ===== 分类管理 =====
function addCategory() {
  editableCategories.value.push('新分类');
}

function removeCategory(idx: number) {
  editableCategories.value.splice(idx, 1);
  if (editingCatIdx.value === idx) editingCatIdx.value = null;
}

async function startEditCat(idx: number, name: string) {
  editingCatIdx.value = idx;
  editCatName.value = name;
  await nextTick();
  catEditRef.value?.focus?.();
}

function saveCatName(idx: number) {
  const trimmed = editCatName.value.trim();
  if (trimmed) editableCategories.value[idx] = trimmed;
  editingCatIdx.value = null;
  editCatName.value = '';
}

function cancelEditCat() {
  editingCatIdx.value = null;
  editCatName.value = '';
}

async function saveCategories() {
  if (editableCategories.value.length === 0) { ElMessage.warning('至少保留一个分类'); return; }
  savingCategories.value = true;
  try {
    await adminApi.updateConfig({ categories: JSON.stringify(editableCategories.value) });
    originalCategories.value = [...editableCategories.value];
    ElMessage.success('分类已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingCategories.value = false;
}

function resetCategories() {
  editableCategories.value = [...originalCategories.value];
  editingCatIdx.value = null;
  editCatName.value = '';
}

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

onMounted(() => {
  loadDashboard();
  loadConfig();
  loadSites();
  loadWallConfig();
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
