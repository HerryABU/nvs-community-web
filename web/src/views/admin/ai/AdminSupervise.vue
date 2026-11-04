<template>
  <div class="admin-page">
    <AdminHeader
      title="绥靖名单 · 以疏代堵"
      subtitle="绥靖模式下被静默记入的用户；可疑 → 恶意自动升级，亦可人工改档 / 清除 / 加注"
      :icon="View"
    >
      <template #actions>
        <el-button size="small" :loading="loading" @click="loadAll">刷新</el-button>
      </template>
    </AdminHeader>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="以疏代堵"
      description="本名单由 AI 安全闸的「绥靖模式」自动写入：用户无感，危险内容被改写为引导内容，行为静默入档。可疑名单在监管模型 B 判定分越线时自动升级为恶意名单。此处仅站长可见，可人工改档、清除、加注。导出取证包请到警方取证审批流程。"
      style="margin-bottom: 16px"
    />

    <!-- 概览 -->
    <div class="sp-stats">
      <div class="sp-stat sp-stat--warn">
        <span class="sp-stat-num">{{ stats.by_tier?.suspect ?? 0 }}</span>
        <span class="sp-stat-label">可疑名单</span>
      </div>
      <div class="sp-stat sp-stat--danger">
        <span class="sp-stat-num">{{ stats.by_tier?.malicious ?? 0 }}</span>
        <span class="sp-stat-label">恶意名单</span>
      </div>
      <div class="sp-stat">
        <span class="sp-stat-num">{{ stats.cleared ?? 0 }}</span>
        <span class="sp-stat-label">已清除（留痕）</span>
      </div>
    </div>

    <!-- 筛选条 -->
    <div class="sp-toolbar">
      <el-select v-model="filterTier" placeholder="全部档位" clearable style="width: 150px" @change="page = 1; loadList()">
        <el-option label="可疑" value="suspect" />
        <el-option label="恶意" value="malicious" />
      </el-select>
      <el-select v-model="filterStatus" placeholder="全部状态" clearable style="width: 150px" @change="page = 1; loadList()">
        <el-option label="在档" value="active" />
        <el-option label="已清除" value="cleared" />
      </el-select>
      <el-input
        v-model="keyword"
        placeholder="搜索用户名 / 用户ID"
        clearable
        style="width: 220px"
        :prefix-icon="Search"
        @keyup.enter="page = 1; loadList()"
        @clear="page = 1; loadList()"
      />
      <el-button type="primary" :loading="loading" @click="page = 1; loadList()">查询</el-button>
    </div>

    <!-- 列表 -->
    <StateBlock v-if="loading" state="loading" skeleton="table" :rows="6" />
    <StateBlock v-else-if="loadError" state="error" title="加载失败" description="请稍后重试" action-text="重试" @action="loadList" />
    <template v-else>
      <el-table :data="items" stripe border style="width: 100%">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column label="档位" width="100">
          <template #default="{ row }">
            <el-tag :type="row.tier === 'malicious' ? 'danger' : 'warning'" effect="dark" size="small">
              {{ row.tier === 'malicious' ? '恶意' : '可疑' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="score" label="评分" width="80" />
        <el-table-column prop="source" label="来源" width="110" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? '在档' : '已清除' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="first_seen" label="首次入档" width="150">
          <template #default="{ row }">{{ fmt(row.first_seen) }}</template>
        </el-table-column>
        <el-table-column prop="last_event" label="最近事件" width="150">
          <template #default="{ row }">{{ fmt(row.last_event) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openDetail(row)">详情</el-button>
            <el-button size="small" type="warning" plain :disabled="row.status === 'cleared'" @click="openSetTier(row)">改档</el-button>
            <el-button size="small" type="danger" plain :disabled="row.status === 'cleared'" @click="openClear(row)">清除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="sp-pager">
        <el-pagination
          v-model:current-page="page"
          :page-size="size"
          :total="total"
          layout="total, prev, pager, next"
          background
          @current-change="loadList"
        />
      </div>
    </template>

    <!-- 详情抽屉 -->
    <el-drawer v-model="detailVisible" :title="`用户 #${detail?.user_id} ${detail?.username ?? ''} 详情`" size="46%">
      <template v-if="detail">
        <div class="sp-detail-block">
          <div class="sp-detail-row"><span>档位</span>
            <el-tag :type="detail.tier === 'malicious' ? 'danger' : 'warning'" effect="dark" size="small">
              {{ detail.tier === 'malicious' ? '恶意' : '可疑' }}
            </el-tag>
          </div>
          <div class="sp-detail-row"><span>状态</span>
            <el-tag :type="detail.status === 'active' ? 'success' : 'info'" size="small">
              {{ detail.status === 'active' ? '在档' : '已清除' }}
            </el-tag>
          </div>
          <div class="sp-detail-row"><span>评分</span><b>{{ detail.score }}</b></div>
          <div class="sp-detail-row"><span>来源</span><code>{{ detail.source }}</code></div>
          <div class="sp-detail-row"><span>首次入档</span>{{ fmt(detail.first_seen) }}</div>
          <div class="sp-detail-row"><span>最近事件</span>{{ fmt(detail.last_event) }}</div>
        </div>

        <SectionHeader title="入档原因" :icon="'📋'" gradient="blue" />
        <ul class="sp-reasons">
          <li v-for="(r, i) in parseArr(detail.reasons)" :key="i">{{ r }}</li>
          <li v-if="!parseArr(detail.reasons).length" class="muted">（无）</li>
        </ul>

        <SectionHeader title="备注（站长加注）" :icon="'✏️'" gradient="teal" />
        <p class="sp-note">{{ detail.note || '（无）' }}</p>

        <SectionHeader title="处置日志" :icon="'🧾'" gradient="purple" />
        <el-timeline v-if="logs.length">
          <el-timeline-item
            v-for="l in logs"
            :key="l.id"
            :timestamp="fmt(l.created_at)"
            placement="top"
          >
            <div class="sp-log-action">
              <b>{{ actionLabel(l.action) }}</b>
              <span v-if="l.actor_name" class="muted"> · 操作人 {{ l.actor_name }}</span>
            </div>
            <div v-if="l.from_to" class="sp-log-fromto">{{ l.from_to }}</div>
            <div v-if="l.note" class="sp-log-note">{{ l.note }}</div>
          </el-timeline-item>
        </el-timeline>
        <StateBlock v-else state="empty" title="暂无日志" description="系统写入或人工干预都会留痕" />

        <div class="sp-detail-actions">
          <el-button type="warning" plain :disabled="detail.status === 'cleared'" @click="openSetTier(detail)">改档</el-button>
          <el-button type="danger" plain :disabled="detail.status === 'cleared'" @click="openClear(detail)">清除</el-button>
          <el-button @click="openAddNote(detail)">加注</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 改档对话框 -->
    <el-dialog v-model="setTierVisible" title="改档" width="460px">
      <el-form label-width="80px">
        <el-form-item label="目标档位">
          <el-radio-group v-model="setTierForm.tier">
            <el-radio-button value="suspect">可疑</el-radio-button>
            <el-radio-button value="malicious">恶意</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="理由">
          <el-input v-model="setTierForm.note" type="textarea" :rows="3" placeholder="必填：人工改档需留痕" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="setTierVisible = false">取消</el-button>
        <el-button type="primary" :loading="acting" @click="doSetTier">确认改档</el-button>
      </template>
    </el-dialog>

    <!-- 清除对话框 -->
    <el-dialog v-model="clearVisible" title="清除在档记录" width="460px">
      <el-alert type="warning" :closable="false" show-icon title="提示" description="清除后该用户不在档，但历史原因与日志会保留（不删除）。" style="margin-bottom:12px" />
      <el-form label-width="80px">
        <el-form-item label="理由">
          <el-input v-model="clearForm.note" type="textarea" :rows="3" placeholder="必填：清除需留痕" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="clearVisible = false">取消</el-button>
        <el-button type="danger" :loading="acting" @click="doClear">确认清除</el-button>
      </template>
    </el-dialog>

    <!-- 加注对话框 -->
    <el-dialog v-model="noteVisible" title="加注" width="460px">
      <el-form label-width="80px">
        <el-form-item label="备注">
          <el-input v-model="noteForm.note" type="textarea" :rows="3" placeholder="追加站长备注" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="noteVisible = false">取消</el-button>
        <el-button type="primary" :loading="acting" @click="doAddNote">确认加注</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { View, Search } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiSafetyApi, type SupervisedUser, type SuperviseLog, type SuperviseStats } from '@/api/ai/aiSafety';

const loading = ref(false);
const loadError = ref(false);
const items = ref<SupervisedUser[]>([]);
const total = ref(0);
const page = ref(1);
const size = 20;
const filterTier = ref('');
const filterStatus = ref('');
const keyword = ref('');

const stats = ref<SuperviseStats>({ by_tier: {}, cleared: 0 });

const detailVisible = ref(false);
const detail = ref<SupervisedUser | null>(null);
const logs = ref<SuperviseLog[]>([]);

const setTierVisible = ref(false);
const clearVisible = ref(false);
const noteVisible = ref(false);
const acting = ref(false);
const setTierForm = ref({ id: 0, tier: 'malicious', note: '' });
const clearForm = ref({ id: 0, note: '' });
const noteForm = ref({ id: 0, note: '' });

function fmt(v?: string | null) {
  if (!v) return '—';
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
function parseArr(s?: string): string[] {
  if (!s) return [];
  try {
    const v = JSON.parse(s);
    return Array.isArray(v) ? v.map(String) : [String(v)];
  } catch {
    return s ? [s] : [];
  }
}
function actionLabel(a: string) {
  return (
    { upsert: '入档', escalate: '升级恶意', clear: '清除', set_tier: '改档', note: '加注' } as Record<string, string>
  )[a] || a;
}

async function loadStats() {
  try {
    const res = await aiSafetyApi.superviseStats();
    if (res.data.code === 0 && res.data.data) stats.value = res.data.data;
  } catch {
    /* 统计失败不阻断 */
  }
}
async function loadList() {
  loading.value = true;
  loadError.value = false;
  try {
    const res = await aiSafetyApi.superviseList({
      page: page.value,
      page_size: size,
      tier: filterTier.value || undefined,
      status: filterStatus.value || undefined,
    });
    if (res.data.code === 0 && res.data.data) {
      let rows = res.data.data.items;
      if (keyword.value.trim()) {
        const k = keyword.value.trim().toLowerCase();
        rows = rows.filter(
          (r) => r.username.toLowerCase().includes(k) || String(r.user_id).includes(k),
        );
      }
      items.value = rows;
      total.value = keyword.value.trim() ? rows.length : res.data.data.total;
    }
  } catch {
    loadError.value = true;
  } finally {
    loading.value = false;
  }
}
async function loadAll() {
  await Promise.all([loadStats(), loadList()]);
}

async function openDetail(row: SupervisedUser) {
  detail.value = row;
  logs.value = [];
  detailVisible.value = true;
  try {
    const res = await aiSafetyApi.superviseGet(row.id);
    if (res.data.code === 0 && res.data.data) {
      detail.value = res.data.data.record;
      logs.value = res.data.data.logs;
    }
  } catch {
    ElMessage.error('加载详情失败');
  }
}

function openSetTier(row: SupervisedUser) {
  setTierForm.value = { id: row.id, tier: row.tier === 'malicious' ? 'suspect' : 'malicious', note: '' };
  setTierVisible.value = true;
}
function openClear(row: SupervisedUser) {
  clearForm.value = { id: row.id, note: '' };
  clearVisible.value = true;
}
function openAddNote(row: SupervisedUser) {
  noteForm.value = { id: row.id, note: '' };
  noteVisible.value = true;
}

async function doSetTier() {
  if (!setTierForm.value.note.trim()) {
    ElMessage.warning('改档须填写理由');
    return;
  }
  acting.value = true;
  try {
    await aiSafetyApi.superviseSetTier(setTierForm.value.id, setTierForm.value.tier, setTierForm.value.note);
    ElMessage.success('已改档');
    setTierVisible.value = false;
    await loadAll();
    if (detail.value) await openDetail(detail.value);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '改档失败');
  } finally {
    acting.value = false;
  }
}
async function doClear() {
  if (!clearForm.value.note.trim()) {
    ElMessage.warning('清除须填写理由');
    return;
  }
  acting.value = true;
  try {
    await aiSafetyApi.superviseClear(clearForm.value.id, clearForm.value.note);
    ElMessage.success('已清除');
    clearVisible.value = false;
    await loadAll();
    detailVisible.value = false;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '清除失败');
  } finally {
    acting.value = false;
  }
}
async function doAddNote() {
  if (!noteForm.value.note.trim()) {
    ElMessage.warning('备注不能为空');
    return;
  }
  acting.value = true;
  try {
    await aiSafetyApi.superviseAddNote(noteForm.value.id, noteForm.value.note);
    ElMessage.success('已加注');
    noteVisible.value = false;
    if (detail.value) await openDetail(detail.value);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加注失败');
  } finally {
    acting.value = false;
  }
}

onMounted(loadAll);
</script>

<style scoped>
.sp-stats {
  display: flex;
  gap: 14px;
  margin-bottom: 16px;
}
.sp-stat {
  flex: 1;
  background: var(--bg-soft, #f6f8fa);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
  padding: 16px;
  text-align: center;
}
.sp-stat-num {
  font-size: 28px;
  font-weight: 700;
}
.sp-stat-label {
  color: var(--text-muted, #6b7280);
  font-size: 13px;
  margin-top: 4px;
  display: block;
}
.sp-stat--warn .sp-stat-num {
  color: #d97706;
}
.sp-stat--danger .sp-stat-num {
  color: #dc2626;
}
.sp-toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.sp-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
.sp-detail-block {
  background: var(--bg-soft, #f6f8fa);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
  padding: 12px 16px;
  margin-bottom: 16px;
}
.sp-detail-row {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 4px 0;
  font-size: 14px;
}
.sp-detail-row > span:first-child {
  width: 80px;
  color: var(--text-muted, #6b7280);
}
.sp-detail-row code {
  background: #eef2f7;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 12px;
}
.sp-reasons {
  margin: 0 0 6px;
  padding-left: 18px;
}
.sp-note {
  background: #eef2f7;
  border-radius: 6px;
  padding: 8px 12px;
  white-space: pre-wrap;
}
.sp-log-action {
  font-size: 14px;
}
.sp-log-fromto {
  color: var(--text-muted, #6b7280);
  font-size: 12px;
}
.sp-log-note {
  margin-top: 2px;
  font-size: 13px;
  white-space: pre-wrap;
}
.sp-detail-actions {
  display: flex;
  gap: 10px;
  margin-top: 18px;
}
</style>
