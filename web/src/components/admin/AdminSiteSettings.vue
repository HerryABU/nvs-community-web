<template>
  <el-collapse-item title="站点设置" name="site">
    <div class="panel-body">
      <el-form :model="configForm" label-width="120px" @submit.prevent="saveConfig">
        <el-form-item label="站点名称">
          <el-input v-model="configForm.site_name" placeholder="星海文学" maxlength="64" />
        </el-form-item>
        <el-form-item label="VIP 付费">
          <el-switch v-model="vipEnabled" active-text="开启" inactive-text="关闭" />
          <span class="hint-text">关闭后，作者无法申请 VIP 付费功能</span>
        </el-form-item>
        <el-divider />
        <h3 style="margin:0 0 12px 0;font-size:1rem;">邮件验证设置</h3>
        <el-form-item label="邮箱验证">
          <el-switch v-model="emailVerifyEnabled" active-text="开启" inactive-text="关闭" />
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
          <el-switch v-model="captchaEnabled" active-text="开启" inactive-text="关闭" />
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
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';

const configForm = reactive<Record<string, string>>({
  site_name: '', vip_enabled: '', email_verify: '', captcha_enabled: '',
  smtp_host: '', smtp_port: '', smtp_user: '', smtp_password: '', smtp_from: '',
});
const vipEnabled = ref(false);
const emailVerifyEnabled = ref(false);
const captchaEnabled = ref(false);
const savingConfig = ref(false);

// 分类管理
const editableCategories = ref<string[]>([]);
const editingCatIdx = ref(-1);
const editCatName = ref('');
const savingCategories = ref(false);
const catEditRef = ref<any>(null);

async function loadConfig() {
  try {
    const res = await adminApi.getConfig();
    const data = res.data.data || res.data || {};
    configForm.site_name = data.site_name || '';
    vipEnabled.value = data.vip_enabled === 'true';
    emailVerifyEnabled.value = data.email_verify === 'true';
    captchaEnabled.value = data.captcha_enabled === 'true';
    configForm.smtp_host = data.smtp_host || '';
    configForm.smtp_port = data.smtp_port || '587';
    configForm.smtp_user = data.smtp_user || '';
    configForm.smtp_password = data.smtp_password || '';
    configForm.smtp_from = data.smtp_from || '';
    editableCategories.value = (data.categories || '').split(',').filter(Boolean);
  } catch { /* ignore */ }
}

async function saveConfig() {
  savingConfig.value = true;
  try {
    await adminApi.updateConfig({
      site_name: configForm.site_name,
      vip_enabled: vipEnabled.value ? 'true' : 'false',
      email_verify: emailVerifyEnabled.value ? 'true' : 'false',
      captcha_enabled: captchaEnabled.value ? 'true' : 'false',
      smtp_host: configForm.smtp_host,
      smtp_port: configForm.smtp_port,
      smtp_user: configForm.smtp_user,
      smtp_password: configForm.smtp_password,
      smtp_from: configForm.smtp_from,
    });
    ElMessage.success('配置已保存');
  } catch { ElMessage.error('保存失败'); }
  finally { savingConfig.value = false; }
}

function addCategory() { editableCategories.value.push('新分类'); resetCategories(); }
function removeCategory(idx: number) { editableCategories.value.splice(idx, 1); }

function startEditCat(idx: number, name: string) {
  editingCatIdx.value = idx;
  editCatName.value = name;
}

function saveCatName(idx: number) {
  if (editCatName.value.trim()) editableCategories.value[idx] = editCatName.value.trim();
  editingCatIdx.value = -1;
}

function cancelEditCat() { editingCatIdx.value = -1; }

async function saveCategories() {
  savingCategories.value = true;
  try {
    configForm.categories = editableCategories.value.join(',');
    await adminApi.updateConfig({ categories: configForm.categories });
    ElMessage.success('分类已保存');
  } catch { ElMessage.error('保存失败'); }
  finally { savingCategories.value = false; }
}

function resetCategories() { savingCategories.value = false; }

onMounted(() => { loadConfig(); });
</script>
