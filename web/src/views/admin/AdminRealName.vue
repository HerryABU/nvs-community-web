<template>
  <div class="admin-page">
    <AdminHeader title="实名与未成年保护" subtitle="核验提供商 · 实名策略 · 未成年模式 · 实名记录" :icon="Postcard" />
    <div class="admin-page-body">

      <!-- 统计概览 -->
      <div class="admin-panel">
        <SectionHeader title="实名概览" subtitle="实名与未成年统计" :icon="'📊'" gradient="primary" />
        <div class="admin-stat-grid" v-loading="loading">
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-primary)">👥</div>
            <div>
              <div class="admin-stat-value">{{ total }}</div>
              <div class="admin-stat-label">实名用户总数</div>
            </div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-amber)">🔒</div>
            <div>
              <div class="admin-stat-value">{{ minorTotal }}</div>
              <div class="admin-stat-label">未成年人（受保护）</div>
            </div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-success)">✅</div>
            <div>
              <div class="admin-stat-value">{{ Math.max(total - minorTotal, 0) }}</div>
              <div class="admin-stat-label">成年用户</div>
            </div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-blue)">🧭</div>
            <div>
              <div class="admin-stat-value">{{ providerLabel }}</div>
              <div class="admin-stat-label">当前核验提供商</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 核验提供商配置 -->
      <div class="admin-panel">
        <SectionHeader title="核验提供商" subtitle="国家授权服务商 / 腾讯云 / 阿里云 / 华为云 / 网易易盾" :icon="'🛡️'" gradient="blue" />
        <p class="hint-text">
          姓名与身份证号仅随核验请求转发至所选提供商，服务器不存储明文、不记日志。
          各提供商按调用计费，未配置时用户端隐藏实名入口。
        </p>
        <RealNameProviderForm />
      </div>

      <!-- 实名与未成年保护策略 -->
      <div class="admin-panel">
        <SectionHeader title="保护策略" subtitle="发布实名强制 · 未成年保护模式" :icon="'⚖️'" gradient="danger" />
        <el-form label-width="140px" class="policy-form" @submit.prevent="savePolicy">
          <el-form-item label="发布需实名">
            <el-switch v-model="realnameRequired" active-text="开启" inactive-text="关闭" />
            <span class="hint-text">开启后，用户发布作品/章节/评论前须先完成实名认证（站长与警方豁免）</span>
          </el-form-item>
          <el-form-item label="未成年保护模式">
            <el-select v-model="minorMode" style="width: 300px">
              <el-option label="关闭" value="off" />
              <el-option label="软模式（提醒 + 时长提示）" value="soft" />
              <el-option label="严格（未实名限制发布）" value="strict" />
            </el-select>
            <span class="hint-text">
              软模式：未填写出生日期的用户将收到填写提醒；严格模式：未成年人禁止发布作品/章节（评论等社区互动不受限）。
              两种模式都会向未提供出生日期的用户询问年龄。
            </span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingPolicy" @click="savePolicy">保存策略</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 实名记录 -->
      <div class="admin-panel">
        <SectionHeader title="实名记录" subtitle="姓名解密仅本页返回，身份证仅脱敏展示" :icon="'🪪'" gradient="blue" />
        <div class="admin-toolbar">
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
          <el-checkbox v-model="minorOnly" size="small" style="margin-left: 12px" @change="reload">仅看未成年人</el-checkbox>
          <el-button size="small" :icon="RefreshLeft" @click="resetFilters">重置</el-button>
        </div>

        <p class="hint-text">
          合规说明：姓名以 AES-256-GCM 加密存储（REAL_NAME_ENC_KEY），身份证号仅存哈希与脱敏串；
          实名核验为代理模式，服务器不存储姓名与身份证明文。
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
          <template #empty><EmptyState icon="📭" text="暂无实名记录" /></template>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import RealNameProviderForm from '@/components/admin/RealNameProviderForm.vue';
import { Postcard, Search, RefreshLeft } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';

interface RealNameRow {
  user_id: number; username: string; nickname: string; role: string; phone: string;
  real_name: string; id_card_masked: string; minor_protected: boolean; age: number;
  real_name_at: string | null;
}

const keyword = ref('');
const minorOnly = ref(false);
const loading = ref(false);
const rows = ref<RealNameRow[]>([]);
const total = ref(0);
const minorTotal = ref(0);
const page = ref(1);
const pageSize = ref(20);

// ===== 保护策略 =====
const realnameRequired = ref(false);
const minorMode = ref<'off' | 'soft' | 'strict'>('off');
const savingPolicy = ref(false);

// ===== 提供商（展示用） =====
const providerLabel = ref('未配置');

const providerNames: Record<string, string> = {
  national: '国家/授权服务商',
  tencent: '腾讯云',
  alibaba: '阿里云',
  huawei: '华为云',
  netease: '网易易盾',
};

async function reload() {
  loading.value = true;
  try {
    const res = await adminApi.listRealName({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value.trim(),
      minor_only: minorOnly.value,
    });
    rows.value = res.data?.list || [];
    total.value = res.data?.total || 0;
    minorTotal.value = res.data?.minor_total || 0;
  } catch {
    rows.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function resetFilters() {
  keyword.value = '';
  minorOnly.value = false;
  page.value = 1;
  reload();
}

function onPage(p: number) {
  page.value = p;
  reload();
}

async function loadPolicy() {
  try {
    const [cfgRes, providerRes] = await Promise.all([
      adminApi.getConfig(),
      adminApi.getRealNameProvider(),
    ]);
    const data = cfgRes.data.data || cfgRes.data || {};
    realnameRequired.value = data.realname_required === true || data.realname_required === 'true';
    minorMode.value = (data.minor_mode as any) || 'off';
    const p = providerRes.data.data?.config?.provider || '';
    providerLabel.value = providerNames[p] || '未配置';
  } catch { /* ignore */ }
}

async function savePolicy() {
  savingPolicy.value = true;
  try {
    await adminApi.updateConfig({
      realname_required: realnameRequired.value ? 'true' : 'false',
      minor_mode: minorMode.value,
    });
    ElMessage.success('保护策略已保存');
  } catch { ElMessage.error('保存失败'); }
  finally { savingPolicy.value = false; }
}

onMounted(() => {
  reload();
  loadPolicy();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin: 0 0 12px; }
.policy-form { max-width: 640px; }
.cell-user { display: block; font-weight: 500; }
.cell-uid { display: block; font-size: 12px; color: var(--el-text-color-secondary); }
.pager { margin-top: 14px; text-align: right; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
