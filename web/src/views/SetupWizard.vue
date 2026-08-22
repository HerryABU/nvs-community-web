<template>
  <div class="setup-page">
    <div class="setup-card">
      <div class="setup-head">
        <div class="logo">NVS</div>
        <h1>建站向导</h1>
        <p class="sub">首次运行 · 仅需配置一次，完成后向导将永久关闭</p>
      </div>

      <el-steps :active="active" align-center class="steps">
        <el-step title="环境检查" description="权限 / 端口" />
        <el-step title="验证身份" description="安装密钥" />
        <el-step title="数据库" description="连接配置" />
        <el-step title="站点与管理员" description="基础信息" />
      </el-steps>

      <div class="step-body">
        <!-- 步骤 0：环境检查 -->
        <div v-if="active === 0" class="env-panel">
          <div v-if="envLoading" class="env-loading">正在检测环境…</div>
          <el-descriptions v-else-if="env" :column="2" border size="small">
            <el-descriptions-item label="操作系统">{{ env.os }} / {{ env.arch }}</el-descriptions-item>
            <el-descriptions-item label="服务端口">{{ env.server_port }}</el-descriptions-item>
            <el-descriptions-item label="数据目录可写">
              <el-tag :type="env.data_writable ? 'success' : 'danger'" size="small">{{ env.data_writable ? '是' : '否' }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="上传目录可写">
              <el-tag :type="env.upload_writable ? 'success' : 'danger'" size="small">{{ env.upload_writable ? '是' : '否' }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <el-alert type="info" :closable="false" show-icon style="margin-top:12px">
            <template #title>系统启动前请确保数据目录与上传目录可写，端口未被占用。</template>
          </el-alert>
        </div>

        <!-- 步骤 1：安装密钥 -->
        <el-form v-if="active === 1" label-position="top" @submit.prevent>
          <div class="key-tip">
            <el-alert type="warning" :closable="false" show-icon>
              <template #title>
                为严防死守，请从<b>服务器控制台启动日志</b>中获取「安装密钥」。
                该密钥仅显示一次，任何他人无法进入此向导。
              </template>
            </el-alert>
          </div>
          <el-form-item label="安装密钥" required>
            <el-input v-model="form.setup_key" placeholder="粘贴服务器控制台打印的安装密钥" show-password size="large" />
          </el-form-item>
        </el-form>

        <!-- 步骤 2：数据库配置 -->
        <el-form v-if="active === 2" label-position="top" @submit.prevent>
          <el-form-item label="数据库类型" required>
            <el-radio-group v-model="form.database.driver">
              <el-radio-button value="sqlite">SQLite（本地文件，零配置）</el-radio-button>
              <el-radio-button value="mysql">MySQL（服务端数据库）</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <template v-if="form.database.driver === 'mysql'">
            <div class="grid-2">
              <el-form-item label="主机" required>
                <el-input v-model="form.database.host" placeholder="127.0.0.1" />
              </el-form-item>
              <el-form-item label="端口" required>
                <el-input v-model="form.database.port" placeholder="3306" />
              </el-form-item>
            </div>
            <el-form-item label="用户名" required>
              <el-input v-model="form.database.user" placeholder="nvs_user" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="form.database.password" type="password" show-password placeholder="数据库密码" />
            </el-form-item>
            <el-form-item label="数据库名" required>
              <el-input v-model="form.database.name" placeholder="nvs" />
            </el-form-item>
            <div class="hint">需确保该 MySQL 账号已存在且有权建库/建表。</div>
          </template>
          <template v-else>
            <div class="sqlite-box">
              <p>将使用本地文件 <code>data/nvs.db</code> 存储全部数据，无需额外安装任何服务。</p>
              <p class="hint">适合单机部署与个人站点；后续可在后台「数据库管理」中迁移到 MySQL。</p>
            </div>
          </template>

          <el-form-item>
            <el-button :loading="testingDb" @click="testDb">测试连接</el-button>
            <span v-if="dbTestResult" :class="dbTestResult.ok ? 'test-ok' : 'test-bad'">
              {{ dbTestResult.ok ? '连接成功' : ('连接失败：' + dbTestResult.error) }}
            </span>
          </el-form-item>
        </el-form>

        <!-- 步骤 3：站点与管理员 -->
        <el-form v-if="active === 3" label-position="top" @submit.prevent>
          <el-form-item label="站点名称" required>
            <el-input v-model="form.site_name" placeholder="例如：星海文学" maxlength="64" />
          </el-form-item>
          <el-divider content-position="left">管理员账号（用于登录后台）</el-divider>
          <el-form-item label="用户名" required>
            <el-input v-model="form.admin.username" placeholder="admin" maxlength="64" />
          </el-form-item>
          <el-form-item label="邮箱" required>
            <el-input v-model="form.admin.email" placeholder="admin@example.com" maxlength="128" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="form.admin.password" type="password" show-password placeholder="至少 8 位，含字母和数字" />
            <div class="hint">强制高强度：至少 8 位，且同时包含字母与数字。</div>
          </el-form-item>
        </el-form>
      </div>

      <div class="step-actions">
        <el-button v-if="active > 0" @click="active--">上一步</el-button>
        <el-button v-if="active < 3" type="primary" @click="next">下一步</el-button>
        <el-button v-if="active === 3" type="primary" :loading="installing" @click="doInstall">完成安装</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { setupApi } from '@/api/setup';

const router = useRouter();
const active = ref(0);
const installing = ref(false);
const envLoading = ref(true);
const env = ref<any>(null);
const testingDb = ref(false);
const dbTestResult = ref<any>(null);

const form = reactive({
  setup_key: '',
  database: { driver: 'sqlite', host: '127.0.0.1', port: '3306', user: '', password: '', name: '', path: '' },
  site_name: '',
  admin: { username: 'admin', email: '', password: '' },
});

onMounted(async () => {
  // 已安装则跳转登录
  try {
    const res = await setupApi.getStatus();
    if (res.data?.data?.installed) {
      router.replace('/login');
      return;
    }
  } catch { /* 忽略，继续显示向导 */ }
  loadEnvCheck();
});

async function loadEnvCheck() {
  envLoading.value = true;
  try {
    const res = await setupApi.envCheck();
    env.value = res.data?.data;
  } catch { env.value = null; } finally {
    envLoading.value = false;
  }
}

async function testDb() {
  const db = form.database;
  if (db.driver === 'mysql' && (!db.host || !db.port || !db.user || !db.name)) {
    ElMessage.warning('请先完整填写 MySQL 连接信息');
    return;
  }
  testingDb.value = true;
  dbTestResult.value = null;
  try {
    const res = await setupApi.dbTest({
      driver: db.driver,
      host: db.driver === 'mysql' ? db.host : '',
      port: db.driver === 'mysql' ? db.port : '',
      user: db.driver === 'mysql' ? db.user : '',
      password: db.driver === 'mysql' ? db.password : '',
      name: db.driver === 'mysql' ? db.name : '',
      path: '',
    });
    dbTestResult.value = res.data?.data;
  } catch (e: any) {
    dbTestResult.value = { ok: false, error: e?.response?.data?.message || '请求失败' };
  } finally {
    testingDb.value = false;
  }
}

function isStrong(pw: string) {
  return pw.length >= 8 && /[a-zA-Z]/.test(pw) && /[0-9]/.test(pw);
}

function next() {
  if (active.value === 1) {
    if (!form.setup_key.trim()) { ElMessage.warning('请输入安装密钥'); return; }
    active.value = 2;
  } else if (active.value === 2) {
    if (form.database.driver === 'mysql' && (!form.database.host || !form.database.port || !form.database.user || !form.database.name)) {
      ElMessage.warning('请完整填写 MySQL 连接信息');
      return;
    }
    active.value = 3;
  } else {
    active.value++;
  }
}

async function doInstall() {
  if (!form.site_name.trim()) { ElMessage.warning('请填写站点名称'); return; }
  if (!form.admin.username.trim() || !form.admin.email.trim() || !form.admin.password) {
    ElMessage.warning('请完整填写管理员信息');
    return;
  }
  if (!isStrong(form.admin.password)) {
    ElMessage.warning('管理员密码强度不足：至少 8 位，且需同时包含字母和数字');
    return;
  }
  installing.value = true;
  try {
    const payload = {
      setup_key: form.setup_key,
      database: {
        driver: form.database.driver,
        host: form.database.driver === 'mysql' ? form.database.host : '',
        port: form.database.driver === 'mysql' ? form.database.port : '',
        user: form.database.driver === 'mysql' ? form.database.user : '',
        password: form.database.driver === 'mysql' ? form.database.password : '',
        name: form.database.driver === 'mysql' ? form.database.name : '',
        path: '',
      },
      site_name: form.site_name,
      admin: { username: form.admin.username, email: form.admin.email, password: form.admin.password },
    };
    await setupApi.install(payload);
    ElMessage.success('安装成功，请使用管理员账号登录');
    setTimeout(() => router.replace('/login'), 800);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '安装失败，请检查配置');
  } finally {
    installing.value = false;
  }
}
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 16px;
  background: var(--gradient-primary);
  box-sizing: border-box;
}
.setup-card {
  width: 100%;
  max-width: 600px;
  background: #fff;
  border-radius: 16px;
  padding: 36px 40px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.18);
}
.setup-head { text-align: center; margin-bottom: 20px; }
.logo {
  display: inline-block;
  background: var(--gradient-primary);
  color: #fff;
  font-weight: 800;
  font-size: 1.1rem;
  padding: 6px 14px;
  border-radius: 8px;
  letter-spacing: 1px;
}
.setup-head h1 { margin: 14px 0 6px; font-size: 1.6rem; color: #1e293b; }
.sub { color: #64748b; font-size: 0.9rem; }
.steps { margin: 8px 0 24px; }
.step-body { min-height: 280px; }
.env-panel { padding: 4px 0; }
.env-loading { color: #94a3b8; text-align: center; padding: 40px 0; }
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 0 16px; }
.hint { color: #94a3b8; font-size: 0.82rem; }
.sqlite-box { background: #f8fafc; border: 1px dashed #cbd5e1; border-radius: 10px; padding: 16px; }
.sqlite-box p { margin: 0 0 6px; color: #334155; font-size: 0.9rem; }
.sqlite-box code { background: #eef2ff; color: #4f46e5; padding: 1px 6px; border-radius: 4px; }
.key-tip { margin-bottom: 16px; }
.test-ok { color: #10b981; font-size: 0.9rem; }
.test-bad { color: #ef4444; font-size: 0.9rem; }
.step-actions { display: flex; justify-content: flex-end; gap: 12px; margin-top: 8px; }

/* ====== 移动端建站向导适配 ====== */
@media (max-width: 640px) {
  .grid-2 {
    grid-template-columns: 1fr;
  }
  .step-actions {
    flex-wrap: wrap;
  }
  .step-actions .el-button {
    flex: 1;
  }
}
</style>
