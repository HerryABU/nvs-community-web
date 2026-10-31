<template>
  <div class="admin-page">
    <AdminHeader title="算法推荐管理" subtitle="算法透明度 · 用户选择权 · 176号令第七条第九项" :icon="DataAnalysis" />
    <div class="admin-page-body admin-panel">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="合规说明"
        description="《公安机关网络空间安全监督检查办法》第七条第九项要求依法落实算法推荐管理。请在此配置平台推荐排序规则，并填写面向用户的算法说明（将在首页「📊 推荐说明」中公开展示）。"
        style="margin-bottom: 16px"
      />
      <el-form label-width="150px" @submit.prevent>
        <el-form-item label="首页推荐排序规则">
          <el-select v-model="form.sort_rule" style="width: 260px">
            <el-option label="最近更新（updated_at）" value="updated_at" />
            <el-option label="最近创建（created_at）" value="created_at" />
            <el-option label="章节数优先（total_chapters）" value="total_chapters" />
          </el-select>
        </el-form-item>
        <el-form-item label="排行榜计算周期">
          <el-select v-model="form.rank_period" style="width: 260px">
            <el-option label="全部时间" value="all" />
            <el-option label="近 7 天" value="7d" />
            <el-option label="近 30 天" value="30d" />
            <el-option label="近 90 天" value="90d" />
          </el-select>
        </el-form-item>
        <el-form-item label="个性化推荐">
          <el-switch v-model="form.personalized" active-text="开启" inactive-text="关闭" />
          <span class="hint-text">默认关闭。NVS 目前不做基于阅读历史的个性化推荐，此开关为未来扩展预留；开启后用户仍可在个人设置中关闭（算法选择权）</span>
        </el-form-item>
        <el-form-item label="算法说明（面向用户）">
          <el-input v-model="form.description" type="textarea" :rows="10" placeholder="请用白话文说明平台的作品展示与排序逻辑，普通用户应能理解" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存配置</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { DataAnalysis } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const form = reactive({ sort_rule: 'updated_at', rank_period: 'all', personalized: false, description: '' });
const saving = ref(false);

async function load() {
  try {
    const res = await adminApi.getAlgorithms();
    const d = res.data.data || {};
    form.sort_rule = d.sort_rule || 'updated_at';
    form.rank_period = d.rank_period || 'all';
    form.personalized = !!d.personalized;
    form.description = d.description || '';
  } catch { /* ignore */ }
}

async function save() {
  saving.value = true;
  try {
    await adminApi.updateAlgorithms(form);
    ElMessage.success('算法配置已保存，首页立即生效');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.admin-page { --admin-content-max: 860px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 860px; }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin-left: 8px; line-height: 1.6; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
