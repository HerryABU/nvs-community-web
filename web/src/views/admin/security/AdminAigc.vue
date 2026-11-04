<template>
  <div class="admin-page">
    <AdminHeader
      title="AIGC 内容标识"
      subtitle="GB 45438—2025 · 显式角标 + 隐式元数据 · 标识义务由站长承担"
      :icon="Stamp"
    />
    <div class="admin-page-body admin-panel">
      <el-tabs v-model="tab">
        <!-- ================= 标识设置 ================= -->
        <el-tab-pane label="标识设置" name="config">
          <p class="hint-text">
            平台在**服务端**统一给 AI 图片加两层标识：先烧显式角标，再写隐式 AIGC 字段。
            原图落盘时不带任何标识，用户下载时才合成 —— 所以前端、外部脚本都绕不过去。
          </p>

          <el-form label-width="170px" class="aigc-form">
            <el-form-item label="启用内容标识">
              <el-switch v-model="cfg.enabled" />
              <span class="hint-text" style="margin-left: 10px">
                关闭后不再加角标、不写字段（仅限自建私有部署自担责任）
              </span>
            </el-form-item>

            <el-form-item label="生成合成服务提供者">
              <el-input v-model="cfg.content_producer" maxlength="120" placeholder="单位名称或 TC260 27 位编码" />
              <span class="hint-text">必填。写你自己（站长主体），<b>不得</b>填上游 API 厂商</span>
            </el-form-item>

            <el-divider content-position="left">显式标识（用户能看见的角标）</el-divider>

            <el-form-item label="角标文案">
              <el-input v-model="cfg.label_text" maxlength="24" style="width: 220px" />
              <span class="hint-text">须同时含「AI/人工智能」与「生成/合成」要素</span>
            </el-form-item>

            <el-form-item label="字号（占最短边）">
              <el-slider
                v-model="fontPct"
                :min="5"
                :max="15"
                :step="1"
                :show-tooltip="false"
                style="width: 260px"
              />
              <span class="aigc-val">{{ fontPct }}%</span>
              <span class="hint-text">国标要求 ≥5%，低于 5% 会被自动抬到 5%</span>
            </el-form-item>

            <el-form-item label="位置">
              <el-radio-group v-model="cfg.position" size="small">
                <el-radio-button value="bottom-right">右下</el-radio-button>
                <el-radio-button value="bottom-left">左下</el-radio-button>
                <el-radio-button value="top-right">右上</el-radio-button>
                <el-radio-button value="top-left">左上</el-radio-button>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="文字颜色 / 不透明度">
              <el-color-picker v-model="cfg.text_color" />
              <el-slider v-model="textOpacityPct" :min="50" :max="100" :show-tooltip="false" style="width: 180px; margin-left: 16px" />
              <span class="aigc-val">{{ textOpacityPct }}%</span>
            </el-form-item>

            <el-form-item label="描边强度">
              <el-slider v-model="shadowPct" :min="0" :max="80" :show-tooltip="false" style="width: 220px" />
              <span class="aigc-val">{{ shadowPct }}%</span>
              <span class="hint-text">浅色图上白字会看不见，靠这层深色描边兜底</span>
            </el-form-item>

            <el-divider content-position="left">隐式标识（写进文件元数据）</el-divider>

            <el-form-item label="模型名随文件传播">
              <el-switch v-model="cfg.reserved_include_model" />
              <span class="hint-text">
                默认<b>只进本站数据库</b>（用 ProduceID 回查）。开启后写进 ReservedCode1 的
                <code>h=…;m=…</code> 复合段 —— 属扩展用法，传播平台不保证解析
              </span>
            </el-form-item>

            <el-form-item label="允许申请不含角标版">
              <el-switch v-model="cfg.waiver_enabled" />
              <span class="hint-text">豁免只免角标，<b>隐式字段仍然写入</b>；申请记录单独留痕</span>
            </el-form-item>

            <template v-if="cfg.waiver_enabled">
              <el-form-item label="豁免协议条款">
                <el-input v-model="cfg.waiver_agreement" type="textarea" :rows="3" maxlength="600" show-word-limit />
              </el-form-item>
              <el-form-item label="要求手输的确认句">
                <el-input v-model="cfg.waiver_confirm_text" maxlength="120" />
                <span class="hint-text">用户须逐字输入这句才能拿到无角标版本（用于证明其知情）</span>
              </el-form-item>
            </template>

            <el-form-item label="允许附带提示词">
              <el-switch v-model="cfg.prompt_attach_enabled" />
              <span class="hint-text">提示词写独立命名空间，不碰法定七字段</span>
            </el-form-item>

            <el-form-item label="单次导出像素上限">
              <el-input-number v-model="cfg.max_output_pixels" :min="1000000" :max="120000000" :step="1000000" />
              <span class="hint-text">防一次请求把内存打爆（默认 4000 万像素）</span>
            </el-form-item>

            <el-form-item>
              <el-button type="primary" :loading="saving" @click="saveConfig">保存设置</el-button>
            </el-form-item>
          </el-form>

          <el-divider content-position="left">角标字体</el-divider>
          <div class="aigc-font">
            <p class="hint-text">
              内置 Noto Sans SC 子集（SIL OFL 1.1，约 12 KB，只含角标常用字），随二进制打包，不依赖系统字体。
              若文案用到子集外的字（会画成空白），可上传自己的 .ttf 覆盖。
            </p>
            <div class="aigc-font-row">
              <input ref="fontInput" type="file" accept=".ttf" style="display: none" @change="uploadFont" />
              <el-button size="small" @click="fontInput?.click()">上传 .ttf 字体</el-button>
              <span class="hint-text">上限 20 MB；解析失败会自动回退内置字体</span>
            </div>
          </div>
        </el-tab-pane>

        <!-- ================= 导出记录 ================= -->
        <el-tab-pane label="导出记录" name="exports">
          <p class="hint-text">
            每次下载 = 一条记录。<code>ProduceID</code> 是文件里的内容编号，模型名只在这里的数据库列里
            ——国标没有「模型」字段，不许多写。
          </p>
          <div class="aigc-toolbar">
            <el-button size="small" :loading="loadingExports" @click="loadExports">刷新</el-button>
            <span class="hint-text">共 {{ exportTotal }} 条</span>
          </div>
          <el-table :data="exports" size="small" stripe>
            <el-table-column prop="produce_id" label="ProduceID" min-width="230" show-overflow-tooltip />
            <el-table-column prop="model_name" label="模型" width="150" show-overflow-tooltip />
            <el-table-column label="尺寸" width="110">
              <template #default="{ row }">{{ row.width }}×{{ row.height }}</template>
            </el-table-column>
            <el-table-column label="角标" width="130">
              <template #default="{ row }">
                <el-tag v-if="row.labeled" size="small" type="success">
                  {{ row.label_text }} · {{ row.label_font_size }}px
                </el-tag>
                <el-tag v-else size="small" type="warning">已豁免</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="提示词" width="90">
              <template #default="{ row }">
                <el-tag v-if="row.prompt_attached" size="small" type="info">附带</el-tag>
                <span v-else class="muted">—</span>
              </template>
            </el-table-column>
            <el-table-column label="时间" width="160">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button link size="small" type="primary" @click="verify(row.produce_id)">核验</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination
            v-if="exportTotal > 20"
            layout="prev, pager, next"
            :total="exportTotal"
            :page-size="20"
            :current-page="exportPage"
            @current-change="(p: number) => { exportPage = p; loadExports(); }"
          />
        </el-tab-pane>

        <!-- ================= 豁免日志 ================= -->
        <el-tab-pane label="豁免日志" name="waivers">
          <p class="hint-text">
            独立表 <code>aigc_waiver_logs</code>，与论坛隔离墙记录<b>只可按用户/时间关联，字段不混用</b>。
            留存 ≥180 天，含用户手输原文与 SHA-256。
          </p>
          <div class="aigc-toolbar">
            <el-button size="small" :loading="loadingWaivers" @click="loadWaivers">刷新</el-button>
            <span class="hint-text">共 {{ waiverTotal }} 条</span>
          </div>
          <el-table :data="waivers" size="small" stripe>
            <el-table-column label="时间" width="160">
              <template #default="{ row }">{{ fmtTime(row.request_time || row.created_at) }}</template>
            </el-table-column>
            <el-table-column prop="user_name" label="用户" width="120">
              <template #default="{ row }">{{ row.user_name || ('#' + row.user_id) }}</template>
            </el-table-column>
            <el-table-column prop="produce_id" label="ProduceID" min-width="220" show-overflow-tooltip />
            <el-table-column label="结果" width="100">
              <template #default="{ row }">
                <el-tag v-if="row.response_result === 'granted'" size="small" type="success">已豁免</el-tag>
                <el-tag v-else size="small" type="danger">{{ row.response_result || '拒绝' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="deny_reason" label="拒绝原因" width="150" show-overflow-tooltip />
            <el-table-column label="手输原文" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">{{ row.input_text }}</template>
            </el-table-column>
          </el-table>
          <el-pagination
            v-if="waiverTotal > 20"
            layout="prev, pager, next"
            :total="waiverTotal"
            :page-size="20"
            :current-page="waiverPage"
            @current-change="(p: number) => { waiverPage = p; loadWaivers(); }"
          />
        </el-tab-pane>
      </el-tabs>
    </div>

    <el-dialog v-model="verifyOpen" title="核验结果" width="620px">
      <pre class="aigc-json">{{ verifyJson }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Stamp } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { aigcAdminApi, type AigcConfig } from '@/api/ai/aigc';

const tab = ref('config');
const saving = ref(false);

const cfg = reactive<AigcConfig>({
  enabled: true,
  label_text: 'AI生成',
  position: 'bottom-right',
  font_size_pct: 0.05,
  text_color: '#FFFFFF',
  text_opacity: 0.85,
  shadow_opacity: 0.3,
  content_producer: '',
  reserved_include_model: false,
  waiver_enabled: false,
  waiver_agreement: '',
  waiver_confirm_text: '',
  prompt_attach_enabled: false,
  max_output_pixels: 40000000,
});

// 百分比 → 0~1 的双向绑定（人在后台更习惯看 5% 而不是 0.05）
const fontPct = ref(5);
const textOpacityPct = ref(85);
const shadowPct = ref(30);

const fontInput = ref<HTMLInputElement>();

async function loadConfig() {
  try {
    const res = await aigcAdminApi.getConfig();
    const c = res.data?.data?.config;
    if (c) {
      Object.assign(cfg, c);
      fontPct.value = Math.round((c.font_size_pct || 0.05) * 100);
      textOpacityPct.value = Math.round((c.text_opacity ?? 0.85) * 100);
      shadowPct.value = Math.round((c.shadow_opacity ?? 0.3) * 100);
    }
  } catch {
    ElMessage.error('读取配置失败');
  }
}

async function saveConfig() {
  if (cfg.enabled && !cfg.content_producer.trim()) {
    ElMessage.warning('请填写「生成合成服务提供者」——这是国标要求的主体信息');
    tab.value = 'config';
    return;
  }
  saving.value = true;
  try {
    await aigcAdminApi.saveConfig({
      ...cfg,
      font_size_pct: fontPct.value / 100,
      text_opacity: textOpacityPct.value / 100,
      shadow_opacity: shadowPct.value / 100,
    });
    ElMessage.success('已保存，立即生效');
    await loadConfig();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function uploadFont(e: Event) {
  const input = e.target as HTMLInputElement;
  const f = input.files?.[0];
  input.value = '';
  if (!f) return;
  const fd = new FormData();
  fd.append('file', f);
  try {
    await aigcAdminApi.uploadFont(fd);
    ElMessage.success('字体已更新，新图立即使用新字体');
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || '上传失败');
  }
}

// ── 导出记录 ──
const exports = ref<any[]>([]);
const exportTotal = ref(0);
const exportPage = ref(1);
const loadingExports = ref(false);

async function loadExports() {
  loadingExports.value = true;
  try {
    const res = await aigcAdminApi.exports(exportPage.value, 20);
    exports.value = res.data?.data?.list || [];
    exportTotal.value = res.data?.data?.total || 0;
  } catch {
    ElMessage.error('读取导出记录失败');
  } finally {
    loadingExports.value = false;
  }
}

// ── 豁免日志 ──
const waivers = ref<any[]>([]);
const waiverTotal = ref(0);
const waiverPage = ref(1);
const loadingWaivers = ref(false);

async function loadWaivers() {
  loadingWaivers.value = true;
  try {
    const res = await aigcAdminApi.waivers(waiverPage.value, 20);
    waivers.value = res.data?.data?.list || [];
    waiverTotal.value = res.data?.data?.total || 0;
  } catch {
    ElMessage.error('读取豁免日志失败');
  } finally {
    loadingWaivers.value = false;
  }
}

// ── 核验 ──
const verifyOpen = ref(false);
const verifyJson = ref('');

async function verify(pid: string) {
  try {
    const res = await aigcAdminApi.verify(pid);
    verifyJson.value = JSON.stringify(res.data?.data || {}, null, 2);
    verifyOpen.value = true;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '查不到这条记录');
  }
}

function fmtTime(s: string) {
  if (!s) return '';
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  return d.toLocaleString('zh-CN', { hour12: false });
}

onMounted(async () => {
  await loadConfig();
  await loadExports();
  await loadWaivers();
});
</script>

<style scoped>
.admin-page-body { max-width: 1040px; }
.aigc-form { max-width: 880px; }
.aigc-val {
  min-width: 46px;
  margin-left: 10px;
  font-size: 0.78rem;
  color: var(--el-color-primary, #409eff);
  font-variant-numeric: tabular-nums;
}
.aigc-font-row { display: flex; align-items: center; gap: 10px; margin-top: 8px; }
.aigc-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.aigc-json {
  margin: 0;
  padding: 12px;
  border-radius: 8px;
  background: var(--bg-page, #f6f8fa);
  font-size: 0.76rem;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}
.hint-text code {
  padding: 0 4px;
  border-radius: 3px;
  background: var(--bg-page, #eef2f7);
  font-size: 0.72rem;
}
</style>
