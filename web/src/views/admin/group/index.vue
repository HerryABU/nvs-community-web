<template>
  <div class="ag">
    <div class="ag-head">
      <div>
        <h1 class="page-title">群组管理</h1>
        <p class="ag-sub">封禁/解散群组、配置建群额度（全局按角色 + 按人覆盖）</p>
      </div>
      <el-button size="small" @click="loadAll">刷新</el-button>
    </div>

    <!-- 额度配置 -->
    <el-card shadow="never" class="ag-card">
      <div class="ag-card-head">
        <span>建群额度</span>
        <el-button size="small" type="primary" :loading="savingCfg" @click="saveCfg">保存</el-button>
      </div>
      <div class="ag-row">
        <div class="ag-item">
          <div class="ag-name">群组功能总开关</div>
          <div class="ag-desc">关闭后全站无法创建/浏览群组</div>
        </div>
        <el-switch v-model="cfg.enabled" />
      </div>
      <div class="ag-row">
        <div class="ag-item">
          <div class="ag-name">单群人数上限</div>
          <div class="ag-desc">所有群组的人数硬顶（2~10000）</div>
        </div>
        <el-input-number v-model="cfg.member_limit" :min="2" :max="10000" size="small" controls-position="right" />
      </div>
      <div class="ag-row">
        <div class="ag-item">
          <div class="ag-name">每人每日群组发帖上限</div>
          <div class="ag-desc">跨群累计；0 = 不限（仍计入全站发文额度）</div>
        </div>
        <el-input-number v-model="cfg.daily_posts" :min="0" :max="500" size="small" controls-position="right" />
      </div>
      <div class="ag-row">
        <div class="ag-item">
          <div class="ag-name">未列角色兜底额度</div>
          <div class="ag-desc">0 = 不限；-1 = 禁止建群</div>
        </div>
        <el-input-number v-model="cfg.max_per_user" :min="0" :max="1000" size="small" controls-position="right" />
      </div>

      <div class="ag-roles">
        <div v-for="r in roleRows" :key="r.key" class="ag-role">
          <span class="ag-role-name">{{ r.label }}</span>
          <el-input-number v-model="cfg.roles[r.key]" :min="-1" :max="1000" size="small" controls-position="right" />
          <span class="ag-role-hint">{{ r.hint }}</span>
        </div>
      </div>
    </el-card>

    <!-- 按人覆盖 -->
    <el-card shadow="never" class="ag-card">
      <div class="ag-card-head">
        <span>按人覆盖（-1 跟随默认 / 0 禁止建群）</span>
      </div>
      <div class="ag-add">
        <el-input v-model="overrideUser" size="small" placeholder="用户名或用户 ID" style="width:180px" />
        <el-input-number v-model="overrideMax" :min="-1" :max="1000" size="small" controls-position="right" />
        <el-input v-model="overrideNote" size="small" placeholder="备注（可选）" style="width:180px" />
        <el-button size="small" type="primary" :loading="savingOverride" @click="saveOverride">保存</el-button>
      </div>
      <el-table :data="overrides" size="small">
        <el-table-column prop="username" label="用户" width="150" />
        <el-table-column prop="max_groups" label="建群上限" width="110">
          <template #default="{ row }">{{ row.max_groups === -1 ? '跟随默认' : row.max_groups === 0 ? '禁止' : row.max_groups }}</template>
        </el-table-column>
        <el-table-column prop="note" label="备注" show-overflow-tooltip />
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button size="small" text type="danger" @click="removeOverride(row)">移除</el-button>
          </template>
        </el-table-column>
        <template #empty><EmptyState icon="🎯" text="暂无按人覆盖" /></template>
      </el-table>
    </el-card>

    <!-- 群列表 -->
    <el-card shadow="never" class="ag-card">
      <div class="ag-card-head">
        <span>全部群组（{{ total }}）</span>
        <div class="ag-filters">
          <el-select v-model="filters.status" size="small" clearable placeholder="状态" style="width:110px" @change="loadList">
            <el-option label="正常" value="active" />
            <el-option label="已封禁" value="banned" />
            <el-option label="已归档" value="archived" />
          </el-select>
          <el-select v-model="filters.visibility" size="small" clearable placeholder="可见性" style="width:120px" @change="loadList">
            <el-option label="公开" value="public" />
            <el-option label="半公开" value="semi" />
            <el-option label="私密" value="private" />
          </el-select>
          <el-input v-model="filters.q" size="small" placeholder="群名 / 群号" clearable style="width:180px" @keyup.enter="loadList" />
          <el-button size="small" @click="loadList">搜索</el-button>
        </div>
      </div>
      <el-table :data="list" size="small" v-loading="loading">
        <el-table-column label="群组" min-width="200">
          <template #default="{ row }">
            <div class="ag-gname">{{ row.name }}</div>
            <div class="ag-gno">群号 {{ row.group_no }}</div>
          </template>
        </el-table-column>
        <el-table-column label="可见性" width="90">
          <template #default="{ row }">{{ visLabel(row.visibility) }}</template>
        </el-table-column>
        <el-table-column prop="owner_name" label="群主" width="120" />
        <el-table-column prop="member_count" label="人数" width="80" />
        <el-table-column prop="post_count" label="帖子" width="80" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'banned' ? 'danger' : row.status === 'archived' ? 'info' : 'success'">
              {{ row.status === 'banned' ? '已封禁' : row.status === 'archived' ? '已归档' : '正常' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="$router.push(`/group/${row.id}`)">查看</el-button>
            <el-button size="small" text :type="row.status === 'banned' ? 'success' : 'warning'" @click="toggleBan(row)">
              {{ row.status === 'banned' ? '解封' : '封禁' }}
            </el-button>
            <el-button size="small" text type="danger" @click="remove(row)">解散</el-button>
          </template>
        </el-table-column>
        <template #empty><EmptyState icon="👥" text="暂无群组" /></template>
      </el-table>
      <div class="ag-pager">
        <el-pagination layout="prev, pager, next" :total="total" :page-size="filters.size" :current-page="filters.page"
          @current-change="(p: number) => { filters.page = p; loadList(); }" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { groupApi, type Group } from '@/api/group';
import EmptyState from '@/components/common/EmptyState.vue';

const list = ref<Group[]>([]);
const total = ref(0);
const loading = ref(false);
const filters = reactive({ status: '', visibility: '', q: '', page: 1, size: 20 });

const cfg = reactive<any>({
  enabled: true, max_per_user: 3, member_limit: 200, daily_posts: 20,
  roles: { reader: 1, author: 3, vip_author: 10, arbitrator: 3, police: 20, admin: 0 },
});
const roleRows = [
  { key: 'reader', label: '普通读者', hint: '0 不限 / -1 禁止' },
  { key: 'author', label: '作者', hint: '' },
  { key: 'vip_author', label: 'VIP 作者', hint: '' },
  { key: 'arbitrator', label: '仲裁', hint: '' },
  { key: 'police', label: '警方账号', hint: '' },
  { key: 'admin', label: '管理员', hint: '不填则不限' },
];
const savingCfg = ref(false);
const overrides = ref<any[]>([]);
const overrideUser = ref('');
const overrideMax = ref(3);
const overrideNote = ref('');
const savingOverride = ref(false);

function visLabel(v: string) {
  return v === 'semi' ? '半公开' : v === 'private' ? '私密' : '公开';
}

async function loadList() {
  loading.value = true;
  try {
    const res = await groupApi.adminList({
      status: filters.status || undefined,
      visibility: filters.visibility || undefined,
      q: filters.q || undefined,
      page: filters.page,
      size: filters.size,
    });
    if (res.data.code === 0) {
      list.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  }
  loading.value = false;
}

async function loadQuota() {
  try {
    const res = await groupApi.adminQuota();
    if (res.data.code === 0) {
      Object.assign(cfg, res.data.data.config);
      overrides.value = res.data.data.overrides || [];
    }
  } catch { /* ignore */ }
}

async function loadAll() {
  await Promise.all([loadList(), loadQuota()]);
}

async function saveCfg() {
  savingCfg.value = true;
  try {
    await groupApi.adminSetQuota({ ...cfg });
    ElMessage.success('已保存（立即生效）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingCfg.value = false;
}

async function toggleBan(row: Group) {
  const banned = row.status !== 'banned';
  let reason = '';
  if (banned) {
    try {
      const r = await ElMessageBox.prompt('封禁原因（会通知群主）', '封禁群组', {
        confirmButtonText: '封禁', cancelButtonText: '取消', inputValue: '违反平台规范',
      });
      reason = r.value || '';
    } catch { return; }
  }
  try {
    await groupApi.adminBan(row.id, banned, reason);
    ElMessage.success(banned ? '已封禁' : '已解封');
    await loadList();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function remove(row: Group) {
  try {
    await ElMessageBox.confirm(`强制解散「${row.name}」？软删除可追溯。`, '解散群组', { type: 'warning' });
  } catch { return; }
  try {
    await groupApi.adminDelete(row.id);
    ElMessage.success('已解散');
    await loadList();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function saveOverride() {
  const key = overrideUser.value.trim();
  if (!key) {
    ElMessage.warning('请填写用户名或用户 ID');
    return;
  }
  savingOverride.value = true;
  try {
    // 数字 → 用户 ID；否则先解析用户名
    let uid = Number(key);
    if (!uid) {
      const r = await import('@/api/admin').then((m) => m.adminApi.resolveUser(key));
      uid = r.data.data?.id || 0;
    }
    if (!uid) {
      ElMessage.warning('未找到该用户');
      return;
    }
    await groupApi.adminSetUserQuota(uid, { max_groups: overrideMax.value, note: overrideNote.value });
    ElMessage.success('已保存');
    overrideUser.value = '';
    overrideNote.value = '';
    await loadQuota();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingOverride.value = false;
}

async function removeOverride(row: any) {
  try {
    await groupApi.adminSetUserQuota(row.user_id, { delete: true });
    ElMessage.success('已恢复默认');
    await loadQuota();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(loadAll);
</script>

<style scoped>
.ag { max-width: 1080px; margin: 0 auto; }
.ag-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.ag-sub { margin: 6px 0 14px; font-size: 0.82rem; color: var(--text-light, #64748b); }
.ag-card { margin-bottom: 14px; border-radius: 12px; }
.ag-card-head { display: flex; align-items: center; justify-content: space-between; font-weight: 600; font-size: 0.9rem; margin-bottom: 10px; gap: 10px; flex-wrap: wrap; }
.ag-row { display: flex; align-items: center; justify-content: space-between; padding: 8px 0; border-bottom: 1px dashed var(--border-color, #eee); }
.ag-name { font-size: 0.86rem; }
.ag-desc { font-size: 0.72rem; color: var(--text-light, #94a3b8); margin-top: 2px; }
.ag-roles { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 10px; margin-top: 12px; }
.ag-role { display: flex; align-items: center; gap: 8px; }
.ag-role-name { font-size: 0.82rem; min-width: 74px; }
.ag-role-hint { font-size: 0.68rem; color: var(--text-light, #94a3b8); }
.ag-add { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
.ag-filters { display: flex; gap: 8px; flex-wrap: wrap; }
.ag-gname { font-size: 0.86rem; font-weight: 600; }
.ag-gno { font-size: 0.72rem; color: var(--text-light, #94a3b8); }
.ag-pager { display: flex; justify-content: center; margin-top: 12px; }
</style>
