<template>
  <div class="admin-page">
    <AdminHeader
      title="阅读权限"
      subtitle="小说 / 博客统一的作品级权限 · 可用等级、会员门槛与付费解锁由站长决定"
      :icon="Lock"
    />
    <div class="admin-page-body">
      <div class="admin-panel" v-loading="loading">
        <SectionHeader title="可用等级" subtitle="作者只能从勾选的等级里选择；未勾选的等级不会出现在作者端" :icon="'🔒'" gradient="blue" />

        <el-alert type="info" :closable="false" show-icon class="rp-tip">
          <template #title>停用等级后的既有内容如何处理</template>
          <p>
            历史内容若使用了被停用的等级，判定时会<b>保守降级为「登录可见」</b> ——
            既不会意外全站公开，也不会把内容彻底锁死。作者下次保存时需改选其它等级。
          </p>
          <p>
            「付费解锁」是否开放，取决于这里是否勾选 <b>paid</b>；勾选后作者可设定售价，
            但售价不得低于下方「付费最低价」。购买记录的扣费通道由支付模块提供。
          </p>
        </el-alert>

        <el-form label-width="170px" class="rp-form">
          <el-form-item label="开放给作者的等级">
            <el-checkbox-group v-model="form.enabled">
              <el-checkbox
                v-for="key in allLevels"
                :key="key"
                :value="key"
                :disabled="key === 'public'"
              >
                {{ defaultLabels[key] || key }}
                <span class="rp-key">{{ key }}</span>
              </el-checkbox>
            </el-checkbox-group>
            <div class="rp-hint">「公开」为兜底等级，恒定开放、不可取消。</div>
          </el-form-item>

          <el-form-item label="VIP 最低等级">
            <el-input-number v-model="form.vip_min_level" :min="0" :max="99" :step="1" controls-position="right" />
            <span class="rp-inline-hint">选择「VIP 可见」的作品，要求读者会员等级 ≥ 此值</span>
          </el-form-item>

          <el-form-item label="VIP 最高等级">
            <el-input-number v-model="form.vip_max_level" :min="1" :max="99" :step="1" controls-position="right" />
            <span class="rp-inline-hint">#66 卡密可发行的最高等级（自动不低于最低等级）</span>
          </el-form-item>

          <el-form-item label="付费最低价（代币）">
            <el-input-number v-model="form.paid_min_price" :min="0" :max="1000000" :step="1" controls-position="right" />
            <span class="rp-inline-hint">仅在勾选 paid 后生效，防止作者把付费内容标成 0</span>
          </el-form-item>

          <el-form-item label="等级文案（可选）">
            <div class="rp-labels">
              <div v-for="key in form.enabled" :key="key" class="rp-label-row">
                <span class="rp-label-key">{{ defaultLabels[key] || key }}</span>
                <el-input
                  v-model="form.labels[key]"
                  :placeholder="defaultLabels[key] || key"
                  maxlength="20"
                  clearable
                  style="width: 220px"
                />
                <span class="rp-hint">留空使用默认文案</span>
              </div>
              <div v-if="form.enabled.length === 0" class="rp-hint">请先勾选等级</div>
            </div>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="saving" @click="save">保存配置（立即生效）</el-button>
            <el-button :disabled="saving" @click="reload">重置为已保存</el-button>
          </el-form-item>
        </el-form>

        <SectionHeader title="配置预览" subtitle="作者端下拉实际看到的内容" :icon="'👀'" gradient="teal" />
        <div class="rp-preview">
          <el-tag v-for="l in previewLevels" :key="l.key" effect="plain" class="rp-tag">{{ l.label }}</el-tag>
          <span v-if="previewLevels.length === 0" class="rp-hint">未开放任何等级</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Lock } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import { readPermApi, type ReadPermConfig, type ReadPermLevel } from '@/api/readperm';

const loading = ref(false);
const saving = ref(false);
const allLevels = ref<string[]>([]);
const defaultLabels = ref<Record<string, string>>({});

const form = reactive<{
  enabled: string[];
  labels: Record<string, string>;
  vip_min_level: number;
  vip_max_level: number;
  paid_min_price: number;
}>({
  enabled: ['public'],
  labels: {},
  vip_min_level: 1,
  vip_max_level: 3,
  paid_min_price: 1,
});

const previewLevels = computed<ReadPermLevel[]>(() =>
  form.enabled.map((key) => ({
    key,
    label: form.labels[key]?.trim() || defaultLabels.value[key] || key,
  })),
);

async function reload() {
  loading.value = true;
  try {
    const res = await readPermApi.adminGet();
    const d = res.data?.data;
    if (!d) return;
    allLevels.value = d.all_levels || [];
    defaultLabels.value = d.default_labels || {};
    const cfg = d.config;
    form.enabled = (cfg.enabled_levels || ['public']).slice();
    form.labels = { ...(cfg.labels || {}) };
    form.vip_min_level = cfg.vip_min_level ?? 1;
    form.vip_max_level = cfg.vip_max_level ?? 3;
    form.paid_min_price = cfg.paid_min_price ?? 1;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '读取阅读权限配置失败');
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    // 只保留当前启用等级的自定义文案，避免停用后残留脏数据
    const labels: Record<string, string> = {};
    for (const key of form.enabled) {
      const v = (form.labels[key] || '').trim();
      if (v) labels[key] = v;
    }
    const payload: ReadPermConfig = {
      enabled_levels: form.enabled,
      labels,
      vip_min_level: form.vip_min_level,
      vip_max_level: form.vip_max_level,
      paid_min_price: form.paid_min_price,
    };
    await readPermApi.adminSave(payload);
    ElMessage.success('已保存，立即生效');
    await reload();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

onMounted(reload);
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: var(--admin-content-max); margin: 0 auto; }
.admin-panel { background: var(--bg-card, #fff); border-radius: var(--radius-md, 10px); padding: 20px; box-shadow: var(--shadow-xs, 0 1px 3px rgba(0,0,0,.06)); }
.rp-tip { margin-bottom: 18px; }
.rp-tip p { margin: 6px 0 0; line-height: 1.7; font-size: 0.86rem; }
.rp-form { margin-top: 4px; }
.rp-key { margin-left: 6px; font-size: 0.76rem; color: var(--text-light, #999); }
.rp-hint { font-size: 0.8rem; color: var(--text-light, #999); line-height: 1.6; }
.rp-inline-hint { margin-left: 12px; font-size: 0.8rem; color: var(--text-light, #999); }
.rp-labels { display: flex; flex-direction: column; gap: 8px; }
.rp-label-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.rp-label-key { min-width: 88px; font-size: 0.86rem; }
.rp-preview { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 12px; }
.rp-tag { font-size: 0.82rem; }
</style>
