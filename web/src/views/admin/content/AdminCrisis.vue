<template>
  <div class="admin-page">
    <AdminHeader
      title="心理危机干预"
      subtitle="独立模块：识别 → 留痕 → 汇总。AI 只做识别与报告，是否报警 / 是否发送关怀邮件必须由人工决定。"
      :icon="Sunny"
    />

    <div class="admin-page-body">
      <!-- 概览 -->
      <div class="stat-grid">
        <div class="stat-card stat-card--warn">
          <span class="stat-num">{{ stats.total }}</span>
          <span class="stat-label">高危观察期用户</span>
        </div>
        <div class="stat-card stat-card--danger">
          <span class="stat-num">{{ stats.minors }}</span>
          <span class="stat-label">其中未成年</span>
        </div>
        <div class="stat-card">
          <span class="stat-num">{{ stats.events }}</span>
          <span class="stat-label">累计危机事件</span>
        </div>
        <div class="stat-card">
          <span class="stat-num stat-num--sm">{{ stats.latest }}</span>
          <span class="stat-label">最晚观察到期</span>
        </div>
      </div>

      <!-- 观察期用户 -->
      <div class="admin-panel">
        <SectionHeader
          title="高危观察期用户"
          subtitle="命中自残/抑郁黑话或 AI 判危后自动进入观察期；期间指向这些用户的敌意评论会被提前折叠"
          :icon="'🫶'"
          gradient="blue"
        />
        <div class="cx-toolbar">
          <el-button size="small" :loading="loading" @click="loadUsers">刷新</el-button>
          <span class="muted small">观察期内的用户不会被打扰，仅在站内出现危机信号时留痕</span>
        </div>

        <StateBlock
          v-if="loading"
          state="loading"
          title="正在读取观察期用户"
          description="加载中…"
        />
        <StateBlock
          v-else-if="error"
          state="error"
          title="读取失败"
          :description="error"
          @retry="loadUsers"
        />
        <StateBlock
          v-else-if="users.length === 0"
          state="empty"
          title="当前没有处于高危观察期的用户"
          description="出现自残/抑郁黑话或 AI 判危时，用户会自动进入观察期并出现在这里"
        />
        <el-table v-else :data="users" size="small" stripe>
          <el-table-column prop="user_id" label="ID" width="70" />
          <el-table-column label="用户" min-width="160">
            <template #default="{ row }">
              {{ row.nickname || row.username }}
              <span class="muted small">@{{ row.username }}</span>
            </template>
          </el-table-column>
          <el-table-column label="未成年" width="80">
            <template #default="{ row }">
              <el-tag v-if="row.is_minor" size="small" type="warning" effect="plain">未成年</el-tag>
              <span v-else class="muted small">—</span>
            </template>
          </el-table-column>
          <el-table-column prop="event_count" label="危机事件" width="90" />
          <el-table-column label="观察期至" min-width="150">
            <template #default="{ row }">
              <span class="muted small">{{ row.observe_until || '—' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" :loading="reportLoading" @click="openReport(row)">
                查看报告
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 配置 -->
      <div class="admin-panel">
        <SectionHeader
          title="危机防护配置"
          subtitle="这里的取值立即生效；提示词决定 AI 如何判定危机信号，改完建议先观察一阵再加严"
          :icon="'⚙️'"
          gradient="amber"
        />
        <StateBlock v-if="cfgLoading" state="loading" title="正在读取配置" description="加载中…" />
        <el-form v-else-if="cfg" :model="cfg" label-width="130px" class="cx-form">
          <el-form-item label="启用危机防护">
            <el-switch v-model="cfg.minor_crisis_enabled" />
            <span class="form-hint">关闭后不再识别新危机信号，已进入观察期的用户不受影响</span>
          </el-form-item>
          <el-form-item label="危机干预热线">
            <el-input v-model="cfg.minor_crisis_hotline" type="textarea" :rows="3" placeholder="如：全国24小时心理援助热线 400-161-9995" />
            <span class="form-hint">命中危机时展示给用户，请填写真实可用的号码</span>
          </el-form-item>
          <el-form-item label="关怀话术">
            <el-input v-model="cfg.minor_crisis_comfort" type="textarea" :rows="5" placeholder="检测到危机时对用户说的安抚话术" />
          </el-form-item>
          <el-form-item label="判定提示词">
            <el-input v-model="cfg.minor_crisis_prompt" type="textarea" :rows="12" spellcheck="false" />
            <span class="form-hint">可在此补充黑话/规避写法，减少漏判；过度加严会增加误判</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="onSave">保存配置</el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>

    <!-- 干预报告 -->
    <el-drawer v-model="reportVisible" :title="reportTitle" size="660px">
      <div v-if="reportLoading" class="cx-report-loading">正在生成报告…</div>
      <pre v-else class="cx-report">{{ reportText }}</pre>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Sunny } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiSafetyApi, type CrisisUser, type SafetyConfig } from '@/api/ai/aiSafety';

// ---------- 观察期用户 ----------
const users = ref<CrisisUser[]>([]);
const loading = ref(false);
const error = ref('');

const stats = computed(() => {
  const list = users.value;
  const minors = list.filter((u) => u.is_minor).length;
  const events = list.reduce((s, u) => s + (u.event_count || 0), 0);
  // 「最晚观察到期」只用来给站长一个时间锚点，取字典序最大（ISO 时间可直接比较）
  const latest = list
    .map((u) => u.observe_until)
    .filter(Boolean)
    .sort()
    .pop();
  return { total: list.length, minors, events, latest: latest || '—' };
});

async function loadUsers() {
  loading.value = true;
  error.value = '';
  try {
    const res = await aiSafetyApi.crisisUsers();
    users.value = res.data?.data?.users || [];
  } catch (e: any) {
    // 失败必须落到可见 hint，不能静默
    error.value = e?.response?.data?.message || '读取观察期用户失败，请稍后重试';
  } finally {
    loading.value = false;
  }
}

// ---------- 干预报告 ----------
const reportVisible = ref(false);
const reportLoading = ref(false);
const reportText = ref('');
const reportTitle = ref('高危用户心理干预报告');

async function openReport(row: CrisisUser) {
  reportVisible.value = true;
  reportLoading.value = true;
  reportText.value = '';
  reportTitle.value = `心理干预报告 · ${row.nickname || row.username}（#${row.user_id}）`;
  try {
    const res = await aiSafetyApi.crisisReport(row.user_id);
    reportText.value = res.data?.data?.report || '（报告为空）';
  } catch (e: any) {
    reportText.value = `生成失败：${e?.response?.data?.message || '请稍后重试'}`;
  } finally {
    reportLoading.value = false;
  }
}

// ---------- 配置 ----------
// 注意：保存时提交完整 cfg 对象，避免只提交部分字段把其它安全配置清空。
const cfg = ref<SafetyConfig | null>(null);
const cfgLoading = ref(false);
const saving = ref(false);

async function loadConfig() {
  cfgLoading.value = true;
  try {
    const res = await aiSafetyApi.getConfig();
    cfg.value = res.data?.data?.config || null;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '读取配置失败');
  } finally {
    cfgLoading.value = false;
  }
}

async function onSave() {
  if (!cfg.value) return;
  saving.value = true;
  try {
    const res = await aiSafetyApi.saveConfig(cfg.value);
    if (res.data.code === 0) {
      if (res.data.data?.config) cfg.value = res.data.data.config;
      ElMessage.success('已保存，立即生效');
    } else {
      ElMessage.error(res.data.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  loadUsers();
  loadConfig();
});
</script>

<style scoped>
.cx-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.cx-form { max-width: 860px; }
.form-hint { margin-left: 10px; font-size: 0.76rem; color: var(--text-light, #999); }
.cx-report-loading { padding: 30px 0; text-align: center; color: var(--text-light, #888); }
.cx-report {
  margin: 0; padding: 14px 16px; white-space: pre-wrap; word-break: break-word;
  background: var(--bg-soft, #f7f8fa); border: 1px solid var(--border-color, #ebeef5);
  border-radius: 8px; font-size: 0.85rem; line-height: 1.7; color: var(--text-main, #1f2937);
}
.stat-num--sm { font-size: 1.05rem !important; }
</style>
