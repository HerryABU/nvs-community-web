<template>
  <div class="admin-shell">
    <div class="admin-container">
      <AdminHeader
        title="内容封禁管理"
        subtitle="小说 / 章节封禁（176号令）· 封禁记录原因、可解封、留证可追溯"
        :icon="Lock"
        color="var(--gradient-accent)"
      >
        <template #actions>
          <el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
        </template>
      </AdminHeader>

      <div class="admin-panel">
        <el-alert
          type="warning"
          :closable="false"
          show-icon
          style="margin-bottom: 16px"
          title="封禁说明"
          description="封禁（软删除）小说或章节会先归档完整快照到 softdel_ 表留证，再软删除原始记录，并在作者端展示封禁原因；封禁后该小说会从管理后台「字数排行榜」中移除。解封可恢复展示并清除封禁原因。"
        />

        <!-- 搜索栏 -->
        <div class="admin-toolbar">
          <el-input
            v-model="search"
            placeholder="按小说标题 / 作者搜索"
            clearable
            style="width: 320px"
            :prefix-icon="Search"
            @keyup.enter="load"
            @clear="load"
          />
          <el-button type="primary" :icon="Search" @click="load">搜索</el-button>
        </div>

        <!-- 小说列表 -->
        <el-table :data="novels" v-loading="loading" style="width: 100%">
          <el-table-column label="ID" width="70">
            <template #default="{ row }"><code>#{{ row.id }}</code></template>
          </el-table-column>
          <el-table-column label="标题" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="novel-title">{{ row.title }}</span>
            </template>
          </el-table-column>
          <el-table-column label="作者" width="120" show-overflow-tooltip>
            <template #default="{ row }">{{ row.author_name || `#${row.author_id}` }}</template>
          </el-table-column>
          <el-table-column label="字数" width="100" align="right">
            <template #default="{ row }">{{ (row.total_words || 0).toLocaleString() }}</template>
          </el-table-column>
          <el-table-column label="章节" width="80" align="right">
            <template #default="{ row }">{{ row.total_chapters || 0 }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.banned ? 'danger' : 'success'">
                {{ row.banned ? '已封禁' : '正常' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="封禁原因" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.banned" class="ban-reason">{{ row.ban_reason || '未填写' }}</span>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openChapters(row)">章节</el-button>
              <el-button v-if="!row.banned" size="small" text type="danger" @click="openBan('novel', row)">封禁</el-button>
              <el-button v-else size="small" text type="success" @click="doRestore('novel', row)">解封</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty description="暂无小说" /></template>
        </el-table>

        <!-- 分页 -->
        <div class="admin-pager">
          <el-pagination
            v-model:current-page="page"
            :page-size="pageSize"
            :total="total"
            layout="total, prev, pager, next"
            @current-change="load"
          />
        </div>
      </div>
    </div>

    <!-- 章节抽屉 -->
    <el-drawer v-model="showChapters" :title="`章节管理 · ${currentNovel?.title || ''}`" size="680px">
      <div v-loading="loadingChapters">
        <el-alert
          v-if="currentNovel?.banned"
          type="danger"
          :closable="false"
          show-icon
          style="margin-bottom: 12px"
          :title="`该小说已被封禁：${currentNovel?.ban_reason || '未填写原因'}`"
        />
        <el-table :data="chapters" size="small" style="width: 100%">
          <el-table-column label="章节号" width="80">
            <template #default="{ row }">第 {{ row.chapter_number }} 章</template>
          </el-table-column>
          <el-table-column label="标题" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">{{ row.title }}</template>
          </el-table-column>
          <el-table-column label="字数" width="90" align="right">
            <template #default="{ row }">{{ (row.word_count || 0).toLocaleString() }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.banned ? 'danger' : 'success'">{{ row.banned ? '已封禁' : '正常' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="封禁原因" min-width="140" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.banned" class="ban-reason">{{ row.ban_reason || '未填写' }}</span>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button v-if="!row.banned" size="small" text type="danger" @click="openBan('chapter', row)">封禁</el-button>
              <el-button v-else size="small" text type="success" @click="doRestore('chapter', row)">解封</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty description="暂无章节" /></template>
        </el-table>
      </div>
    </el-drawer>

    <!-- 封禁原因弹窗 -->
    <el-dialog v-model="showBan" title="封禁确认" width="480px">
      <el-form label-width="80px">
        <el-form-item label="目标">
          <span class="ban-target">
            <el-tag size="small" :type="banTarget?.banned ? 'danger' : 'success'">{{ typeLabel(banType) }}</el-tag>
            <strong style="margin-left: 8px">{{ banTargetLabel }}</strong>
          </span>
        </el-form-item>
        <el-form-item label="封禁原因" required>
          <el-input
            v-model="banReason"
            type="textarea"
            :rows="3"
            maxlength="500"
            show-word-limit
            placeholder="填写 176 号令封禁原因，作者端将看到此原因"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showBan = false">取消</el-button>
        <el-button type="danger" :loading="banning" @click="doBan">确认封禁</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { Lock, Refresh, Search } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';

const search = ref('');
const novels = ref<any[]>([]);
const loading = ref(false);
const page = ref(1);
const pageSize = 20;
const total = ref(0);

const showChapters = ref(false);
const chapters = ref<any[]>([]);
const loadingChapters = ref(false);
const currentNovel = ref<any>(null);

const showBan = ref(false);
const banType = ref<'novel' | 'chapter'>('novel');
const banTarget = ref<any>(null);
const banReason = ref('');
const banning = ref(false);

function typeLabel(t: string) {
  return t === 'novel' ? '小说' : '章节';
}

const banTargetLabel = computed(() => {
  if (!banTarget.value) return '';
  if (banType.value === 'novel') return `《${banTarget.value.title}》`;
  return `《${currentNovel.value?.title || ''}》第 ${banTarget.value.chapter_number} 章 ${banTarget.value.title}`;
});

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getAdminNovels({
      search: search.value || undefined,
      page: page.value,
      page_size: pageSize,
    });
    const data = res.data.data || {};
    novels.value = data.list || [];
    total.value = data.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  }
  loading.value = false;
}

async function openChapters(novel: any) {
  currentNovel.value = novel;
  showChapters.value = true;
  loadingChapters.value = true;
  try {
    const res = await adminApi.getAdminChapters(novel.id);
    chapters.value = res.data.data || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载章节失败');
  }
  loadingChapters.value = false;
}

function openBan(type: 'novel' | 'chapter', row: any) {
  banType.value = type;
  banTarget.value = row;
  banReason.value = '';
  showBan.value = true;
}

async function doBan() {
  if (!banReason.value.trim()) {
    ElMessage.warning('请填写封禁原因（作者端将展示此原因）');
    return;
  }
  banning.value = true;
  try {
    await adminApi.moderateContent({
      type: banType.value,
      id: banTarget.value.id,
      reason: banReason.value.trim(),
      action: 'ban',
    });
    ElMessage.success('已封禁并留证');
    showBan.value = false;
    banReason.value = '';
    if (banType.value === 'novel') {
      await load();
    } else {
      // 刷新章节
      currentNovel.value.banned = false; // 小说本身可能未封禁
      await openChapters(currentNovel.value);
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '封禁失败');
  }
  banning.value = false;
}

async function doRestore(type: 'novel' | 'chapter', row: any) {
  const label = type === 'novel' ? `小说《${row.title}》` : `章节「${row.title}」`;
  try {
    await ElMessageBox.confirm(
      `确定要解封${label}吗？解封后将恢复展示并清除封禁原因。`,
      '解封确认',
      { type: 'warning', confirmButtonText: '解封', cancelButtonText: '取消' },
    );
  } catch {
    return;
  }
  try {
    await adminApi.restoreContent({ type, id: row.id });
    ElMessage.success('已解封');
    if (type === 'novel') {
      await load();
    } else {
      await openChapters(currentNovel.value);
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '解封失败');
  }
}

onMounted(load);
</script>

<style scoped>
code {
  background: #eef2ff;
  color: #4f46e5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.85rem;
}
.novel-title {
  font-weight: 600;
  color: var(--text-color, #333);
}
.ban-reason {
  color: #dc2626;
  font-size: 0.85rem;
}
.muted {
  color: var(--text-light, #999);
}
.admin-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}
.admin-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.ban-target {
  display: flex;
  align-items: center;
}
[data-theme="dark"] code {
  background: #1e293b;
  color: #93c5fd;
}
[data-theme="dark"] .novel-title {
  color: #e2e8f0;
}
</style>
