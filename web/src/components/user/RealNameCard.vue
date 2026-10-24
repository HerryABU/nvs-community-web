<template>
  <el-card class="settings-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <el-icon><Postcard /></el-icon>
        <span>实名认证</span>
        <el-tag v-if="status?.real_name_verified" type="success" size="small" style="margin-left: 8px">已认证</el-tag>
        <el-tag v-else type="info" size="small" style="margin-left: 8px">未认证</el-tag>
      </div>
    </template>

    <!-- 已认证：展示脱敏信息 -->
    <el-descriptions v-if="status?.real_name_verified" :column="1" border size="small">
      <el-descriptions-item label="身份证号">
        {{ status.id_card_masked || '—' }}
      </el-descriptions-item>
      <el-descriptions-item label="出生日期">
        {{ status.birth_date ? status.birth_date.slice(0, 10) : '—' }}
      </el-descriptions-item>
      <el-descriptions-item label="未成年人保护">
        <el-tag :type="status.minor_protected ? 'warning' : 'success'" size="small">
          {{ status.minor_protected ? '已纳入青少年保护' : '成年用户' }}
        </el-tag>
      </el-descriptions-item>
    </el-descriptions>
    <p v-if="status?.real_name_verified" class="rn-tip">
      姓名与身份证号已加密存储（AES-256-GCM），仅可用于合规取证核验。
    </p>

    <!-- 未认证：申请表单 -->
    <el-form v-else :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
      <el-form-item label="真实姓名">
        <el-input v-model="form.real_name" placeholder="请输入真实姓名" maxlength="32" show-word-limit />
      </el-form-item>
      <el-form-item label="身份证号">
        <el-input v-model="form.id_card" placeholder="请输入 18 位居民身份证号码" maxlength="18" />
      </el-form-item>
      <el-form-item v-if="nationalAvailable">
        <el-checkbox v-model="advancedMode">
          高级模式（仅验证年龄）：不保存姓名与身份证号，只确认是否已成年
        </el-checkbox>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="submit">提交实名认证</el-button>
        <el-button v-if="nationalAvailable" :loading="checking" style="margin-left: 8px" @click="nationalCheck">
          国家实名核验
        </el-button>
      </el-form-item>
      <p class="rn-tip">
        用于未成年人保护与内容合规（公安部令第176号）。身份证号仅保存哈希与脱敏形式，绝不明文存储。
      </p>
    </el-form>

    <!-- 出生日期自报/修改（未实名用户；已实名以证件为准，F21.1） -->
    <div v-if="!status?.real_name_verified" class="rn-birth">
      <span class="rn-birth__label">出生日期（未成年保护）</span>
      <div class="rn-birth__row">
        <el-date-picker
          v-model="birthDate"
          type="date"
          value-format="YYYY-MM-DD"
          placeholder="选择出生日期"
          format="YYYY年MM月DD日"
          :clearable="false"
          style="flex: 1"
        />
        <el-button :loading="savingBirth" :disabled="!birthDate" @click="saveBirthDate">保存</el-button>
      </div>
      <p class="rn-tip">仅用于推算年龄段，不收集其他个人信息。</p>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { authApi } from '@/api/auth';
import { useAppStore } from '@/stores/app';
import { ElMessage } from 'element-plus';
import { Postcard } from '@element-plus/icons-vue';

const appStore = useAppStore();
const status = ref<{ real_name_verified: boolean; id_card_masked: string; birth_date: string | null; minor_protected: boolean } | null>(null);
const saving = ref(false);
const checking = ref(false);
const nationalAvailable = ref(false);

const form = reactive({ real_name: '', id_card: '' });
// 高级模式：仅验证年龄，服务器不存储姓名/身份证（F21.1 未成年检测）
const advancedMode = ref(false);
// 出生日期自报（未实名用户）
const birthDate = ref('');
const savingBirth = ref(false);

async function saveBirthDate() {
  if (!birthDate.value) return;
  savingBirth.value = true;
  try {
    const res = await authApi.setBirthDate(birthDate.value);
    if (res.data.code === 0) {
      ElMessage.success(res.data.data.minor_protected ? '已保存，未成年人保护已生效' : '已保存');
      await load();
    } else {
      ElMessage.error(res.data.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '保存失败');
  } finally {
    savingBirth.value = false;
  }
}

const rules = {
  real_name: [
    { required: true, message: '请输入真实姓名', trigger: 'blur' },
    { min: 2, max: 32, message: '姓名长度应为 2-32 个字符', trigger: 'blur' },
  ],
  id_card: [
    { required: true, message: '请输入身份证号', trigger: 'blur' },
    { pattern: /^\d{17}[\dXx]$/, message: '身份证号应为 18 位（末位可为 X）', trigger: 'blur' },
  ],
};

async function load() {
  try {
    const res = await authApi.getRealNameStatus();
    status.value = res.data.data;
    if (res.data.data.birth_date) birthDate.value = res.data.data.birth_date.slice(0, 10);
    nationalAvailable.value = !!appStore.siteInfo?.national_realname_enabled;
  } catch {
    /* 忽略：保持未认证态 */
  }
}

async function submit() {
  saving.value = true;
  try {
    const res = await authApi.applyRealName(form.real_name, form.id_card.trim().toUpperCase());
    if (res.data.code === 0) {
      ElMessage.success(res.data.message || '实名认证成功');
      await load();
    } else {
      ElMessage.error(res.data.message || '提交失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '提交失败，请检查身份证号');
  } finally {
    saving.value = false;
  }
}

// 国家实名核验：姓名+身份证仅随请求转发；普通模式通过后完成实名，
// 高级模式只验龄——服务器不存储姓名与身份证号（F21.1）
async function nationalCheck() {
  if (!form.real_name || !form.id_card) {
    ElMessage.warning('请先填写姓名与身份证号');
    return;
  }
  checking.value = true;
  try {
    const res = await authApi.nationalRealNameCheck(form.real_name, form.id_card.trim().toUpperCase(), advancedMode.value);
    if (res.data.code === 0 && res.data.data.matched) {
      ElMessage.success(advancedMode.value
        ? (res.data.data.minor_protected ? '核验通过：已识别为未成年人，保护已生效' : '核验通过：已成年')
        : '国家实名核验通过，已完成实名');
      await load();
    } else {
      ElMessage.error(res.data.data?.message || '核验未通过，请检查姓名与身份证号');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '核验服务暂时不可用');
  } finally {
    checking.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.rn-tip {
  margin: 10px 0 0;
  font-size: 0.82rem;
  color: var(--text-light);
  line-height: 1.6;
}

.rn-birth {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px dashed var(--border-color, #dcdfe6);
}
.rn-birth__label {
  display: block;
  margin-bottom: 8px;
  font-size: 0.85rem;
  color: var(--text-light);
}
.rn-birth__row {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
