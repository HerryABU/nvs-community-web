<template>
  <div class="admin-page">
    <AdminHeader title="合规中心" subtitle="等保识别 · 检查应对工具箱 · 制度文档 · 176号令" :icon="Medal" />
    <div class="admin-page-body">
      <!-- 自查报告 -->
      <div class="admin-panel">
        <div class="admin-section-title">📋 合规自查报告</div>
        <el-button type="primary" plain size="small" :loading="reportLoading" @click="loadReport" style="margin-bottom: 10px">
          生成/刷新自查报告
        </el-button>
        <el-table v-if="report" :data="report.checklist" border stripe size="small">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <el-table-column prop="item" label="检查材料" min-width="180" />
          <el-table-column label="就绪状态" width="100">
            <template #empty>
              <StateBlock
                state="empty"
                skeleton="table"
                :rows="4"
                compact
                title="暂无数据"
                description="换个筛选条件试试，或等有数据后再来看。"
              />
            </template>
            <template #default="{ row }">
              <el-tag :type="row.ready ? 'success' : 'warning'" size="small">{{ row.ready ? '已就绪' : '待补充' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="detail" label="说明" min-width="260" show-overflow-tooltip />
        </el-table>
        <EmptyState v-else icon="📋" text="尚未生成自查报告" description="点击上方按钮生成 / 刷新合规自查报告" />
      </div>

      <!-- 等保定级 -->
      <div class="admin-panel">
        <div class="admin-section-title">🛡️ 等保/关基识别指引（176号令 第七条第四/五项）</div>
        <el-form label-width="120px">
          <el-form-item label="定级自测">
            <el-button @click="showQuiz = !showQuiz">{{ showQuiz ? '收起问卷' : '开始自测问卷' }}</el-button>
          </el-form-item>
        </el-form>
        <div v-if="showQuiz" class="quiz-box">
          <el-checkbox-group v-model="quizAnswers">
            <el-checkbox v-for="q in quizQuestions" :key="q" :label="q" style="display:flex; margin-bottom:8px" />
          </el-checkbox-group>
          <el-button type="primary" plain size="small" @click="calcGrade">计算建议等保等级</el-button>
          <div v-if="quizResult" class="quiz-result">
            <el-alert :title="quizResult" type="info" :closable="false" show-icon />
          </div>
          <span class="hint-text" style="display:block;margin-top:6px">提示：如网站服务政府/事业单位或属重要数据运营者，建议咨询当地网安部门确认是否需关基评估（关键信息基础设施保护）。</span>
        </div>
        <el-form label-width="120px" style="margin-top: 12px">
          <el-form-item label="当前定级">
            <el-select v-model="gradeForm.grade" style="width: 180px">
              <el-option label="未定级" value="未定级" />
              <el-option label="等保一级" value="1" />
              <el-option label="等保二级" value="2" />
              <el-option label="等保三级" value="3" />
            </el-select>
          </el-form-item>
          <el-form-item label="等保备注">
            <el-input v-model="gradeForm.note" type="textarea" :rows="2" placeholder="如：已完成二级定级备案，备案号 XX" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="gradeSaving" @click="saveGrade">保存定级记录</el-button>
            <span class="hint-text">备案指引：联系属地公安网安部门或通过当地等保测评机构办理定级与备案</span>
          </el-form-item>
        </el-form>
      </div>

      <!-- 检查应对工具箱 -->
      <div class="admin-panel">
        <div class="admin-section-title">📦 检查应对工具箱（176号令 5.2 检查材料清单）</div>
        <el-form label-width="130px">
          <el-form-item label="检查材料包">
            <el-button type="primary" :loading="exporting.bundle" @click="exportBundle">一键导出材料包 ZIP</el-button>
            <span class="hint-text">
              报告 + 算法说明 + 等保定级 + 漏洞整改 + 用户规模 + 蜜罐摘要 + 日志清单，附 SHA256 清单；导出动作写入审计
            </span>
          </el-form-item>
          <el-form-item label="访问日志导出">
            <div style="display:flex; gap:8px; align-items:center">
              <el-date-picker v-model="logRange" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" style="width: 280px" />
              <el-button type="primary" plain :loading="exporting.logs" @click="exportLogs">导出 ZIP</el-button>
            </div>
            <span class="hint-text">按时间范围导出 log/requests 下的访问日志（JSON 格式），供留存/检查</span>
          </el-form-item>
          <el-form-item label="用户信息导出">
            <el-button type="primary" plain :loading="exporting.users" @click="exportUsers">导出用户清单 CSV</el-button>
            <span class="hint-text">用户注册信息（ID/用户名/邮箱/注册时间/IP）清单，配合检查</span>
          </el-form-item>
          <el-form-item label="漏洞报告导出">
            <el-button type="primary" plain @click="$router.push('/admin/vulnerabilities')">前往漏洞管理</el-button>
          </el-form-item>
          <el-form-item label="制度文档包">
            <el-button type="primary" plain @click="openDocs">查看制度模板（docs/compliance）</el-button>
            <span class="hint-text">三份制度模板（网络安全管理制度 / 内容审核制度 / 应急响应预案）见项目 docs/compliance 目录；可按需修改后使用并在此记录版本</span>
          </el-form-item>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Medal } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { appPrefix } from '@/utils/prefix';
import { ElMessage } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';

const report = ref<any>(null);
const reportLoading = ref(false);
const showQuiz = ref(false);
const quizAnswers = ref<string[]>([]);
const quizResult = ref('');
const gradeForm = reactive({ grade: '未定级', note: '' });
const gradeSaving = ref(false);
const logRange = ref<[string, string]>(['', '']);
const exporting = reactive({ logs: false, users: false, bundle: false });

const quizQuestions = [
  '网站是否存储 50 万条以上个人信息？',
  '网站是否提供付费服务（涉及交易）？',
  '网站是否服务政府/事业单位？',
  '网站日均访问量是否超过 1 万？',
  '网站是否存储重要数据（如用户实名信息、内容审核记录）？',
  '网站是否涉及新闻/出版/教育等公共服务？',
];

async function loadReport() {
  reportLoading.value = true;
  try {
    const res = await adminApi.getComplianceReport();
    report.value = res.data.data || null;
  } catch { ElMessage.error('生成报告失败'); }
  finally { reportLoading.value = false; }
}

function calcGrade() {
  const n = quizAnswers.value.length;
  if (n <= 2) quizResult.value = '建议等保一级：当前规模较小，满足基本安全义务即可。';
  else if (n <= 4) quizResult.value = '建议等保二级：涉及个人信息/交易/公共服务，建议开展二级等保定级备案。';
  else quizResult.value = '建议等保三级，并评估是否属于关键信息基础设施（关基），建议咨询当地网安部门。';
}

async function loadGrade() {
  try {
    const res = await adminApi.getSecurityGrade();
    const d = res.data.data || {};
    gradeForm.grade = d.grade || '未定级';
    gradeForm.note = d.note || '';
  } catch { /* ignore */ }
}
async function saveGrade() {
  gradeSaving.value = true;
  try {
    await adminApi.updateSecurityGrade({ grade: gradeForm.grade, note: gradeForm.note });
    ElMessage.success('定级记录已保存');
  } catch { ElMessage.error('保存失败'); }
  finally { gradeSaving.value = false; }
}

function downloadBlob(res: any, filename: string) {
  const blob = new Blob([res.data], { type: res.data?.type || 'application/octet-stream' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

async function exportLogs() {
  if (!logRange.value?.[0] || !logRange.value?.[1]) { ElMessage.warning('请先选择时间范围'); return; }
  exporting.logs = true;
  try {
    const res = await adminApi.exportComplianceLogs(logRange.value[0], logRange.value[1]);
    downloadBlob(res, `access-logs_${logRange.value[0]}_${logRange.value[1]}.zip`);
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '导出失败'); }
  finally { exporting.logs = false; }
}
async function exportBundle() {
  exporting.bundle = true;
  try {
    const res = await adminApi.exportComplianceBundle();
    downloadBlob(res, `compliance-bundle_${new Date().toISOString().slice(0, 10)}.zip`);
    ElMessage.success('材料包已导出（含 SHA256 清单）');
  } catch { ElMessage.error('导出失败'); }
  finally { exporting.bundle = false; }
}

async function exportUsers() {
  exporting.users = true;
  try {
    const res = await adminApi.exportComplianceUsers();
    downloadBlob(res, `users_${new Date().toISOString().slice(0, 10)}.csv`);
  } catch { ElMessage.error('导出失败'); }
  finally { exporting.users = false; }
}
function openDocs() {
  window.open(appPrefix + '/docs', '_blank');
}

onMounted(() => { loadReport(); loadGrade(); });
</script>

<style scoped>
.admin-page { --admin-content-max: 1000px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1000px; }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin-left: 8px; line-height: 1.6; }
.quiz-box { background: var(--bg-card, #f9fafb); border: 1px dashed var(--border-color); border-radius: 8px; padding: 14px; margin-bottom: 12px; }
.quiz-result { margin-top: 10px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .quiz-box { background: #1e293b; }
</style>
