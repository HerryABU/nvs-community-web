<template>
  <div class="admin-page">
    <AdminHeader title="实名记录" subtitle="已完成实名认证的用户（姓名解密仅在本页返回，身份证仅脱敏展示）" :icon="Postcard" />
    <div class="admin-page-body admin-panel">

      <div class="stat-cards" v-loading="loading">
        <div class="stat-card">
          <div class="stat-value">{{ total }}</div>
          <div class="stat-label">实名用户总数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ minorCount }}</div>
          <div class="stat-label">未成年人（受保护）</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ adultCount }}</div>
          <div class="stat-label">成年用户</div>
        </div>
      </div>

      <div class="filter-bar">
        <el-input
          v-model="keyword"
          placeholder="用户名 / 昵称 / 身份证脱敏串"
          size="small"
          style="width: 260px"
          clearable
          :prefix-icon="Search"
          @keyup.enter="reload"
        />
        <el-button size="small" type="primary" :icon="Search" @click="reload">查询</el-button>
        <el-button size="small" :icon="RefreshLeft" @click="resetFilters">重置</el-button>
      </div>

      <p class="hint-text">
        合规说明：姓名以 AES-256-GCM 加密存储（REAL_NAME_ENC_KEY），身份证号仅存哈希与脱敏串；
        国家实名核验为代理模式，服务器不存储姓名与身份证明文。
      </p>

      <el-table :data="rows" v-loading="loading" stripe border style="width:100%">
        <el-table-column label="用户" min-width="170">
          <template #default="{ row }">
            <span class="cell-user">{{ row.nickname || row.username || '—' }}</span>
            <span class="cell-uid">@{{ row.username }} #{{ row.user_id }}</span>
          </template>
        </el-table-column>
        <el-table-column label="姓名" prop="real_name" width="110" />
        <el-table-column label="身份证号" prop="id_card_masked" width="170" />
        <el-table-column label="年龄" width="80">
          <template #default="{ row }">
            <el-tag :type="row.minor_protected ? 'warning' : 'success'" size="small">
              {{ row.age || '—' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="角色" prop="role" width="100" />
        <el-table-column label="手机号" prop="phone" width="130" />
        <el-table-column label="实名时间" prop="real_name_at" width="170" />
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="onPage"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { Postcard, Search, RefreshLeft } from '@element-plus/icons-vue';

interface RealNameRow {
  user_id: number; username: string; nickname: string; role: string; phone: string;
  real_name: string; id_card_masked: string; minor_protected: boolean; age: number;
  real_name_at: string | null;
}

const keyword = ref('');
const loading = ref(false);
const rows = ref<RealNameRow[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

const minorCount = computed(() => rows.value.filter((r) => r.minor_protected).length);
const adultCount = computed(() => rows.value.length - minorCount.value);

async function reload() {
  loading.value = true;
  try {
    const res = await adminApi.listRealName({ page: page.value, page_size: pageSize.value, keyword: keyword.value.trim() });
    rows.value = res.data?.list || [];
    total.value = res.data?.total || 0;
  } catch {
    rows.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function resetFilters() {
  keyword.value = '';
  page.value = 1;
  reload();
}

function onPage(p: number) {
  page.value = p;
  reload();
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
.cell-user { display: block; font-weight: 500; }
.cell-uid { display: block; font-size: 12px; color: var(--el-text-color-secondary); }
.pager { margin-top: 14px; text-align: right; }
</style>
