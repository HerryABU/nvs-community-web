<template>
  <div class="decorate-page">
    <!-- ============ 区块一：我的主页装扮（仅本人可见，原始 HTML） ============ -->
    <div class="dec-card">
      <div class="dec-card-head">
        <h2>🎨 我的主页装扮</h2>
        <el-tag size="small" type="info">仅你本人登录可见</el-tag>
      </div>
      <el-alert type="info" :closable="false" show-icon class="dec-hint">
        <template #title>自 XSS 提示</template>
        此处代码<b>仅在你自己的登录态下渲染</b>，支持完整 HTML / CSS / JavaScript（等同 alist 个人自定义）。
        脚本不会对其他访客生效，请仅书写你本人信任的内容。
      </el-alert>

      <el-input
        v-model="myRaw"
        type="textarea"
        :rows="12"
        class="dec-editor"
        placeholder="在此粘贴 / 编写自定义 HTML 片段，例如：&#10;<div style=&quot;padding:16px;background:linear-gradient(135deg,#667eea,#764ba2);color:#fff;border-radius:12px&quot;>&#10;  &lt;h2&gt;欢迎来到我的主页&lt;/h2&gt;&#10;&lt;/div&gt;"
        @keydown="onMyKeydown"
      />
      <div class="dec-foot">
        <span class="dec-tip">快捷键：Ctrl/⌘ + B 粗体 · I 斜体 · U 下划线 · S 保存（拦截浏览器另存为）</span>
        <el-button type="primary" :loading="savingMy" @click="saveMy">保存我的装扮</el-button>
      </div>

      <el-collapse v-model="myPreviewOpen" class="dec-preview-collapse">
        <el-collapse-item name="preview">
          <template #title><span class="dec-preview-title">实时预览（仅你本人看到的效果）</span></template>
          <div class="dec-preview" v-html="myRaw || '<span style=\'color:#999\'>（空）</span>'" />
        </el-collapse-item>
      </el-collapse>
    </div>

    <!-- ============ 区块二：作者公开主页（消毒 + 站长审核） ============ -->
    <div class="dec-card">
      <div class="dec-card-head">
        <h2>🌐 作者公开主页</h2>
        <el-tag size="small" :type="statusTag(review.author.review_status)">{{ statusLabel(review.author.review_status) }}</el-tag>
      </div>
      <el-alert type="warning" :closable="false" show-icon class="dec-hint">
        <template #title>对外可见 · 需消毒与审核</template>
        他人访问你的作者主页时会看到这段内容。提交后服务端会<b>自动消毒</b>（剥离脚本 / 危险 CSS），
        并进入<b>站长 / 管理员审核</b>队列。若预检发现需清理的内容，原文会一并送站长复核。
      </el-alert>

      <!-- 审核状态条 -->
      <div v-if="review.author.review_status" class="dec-status-bar">
        <span class="dec-status-label">当前状态：</span>
        <el-tag size="small" :type="statusTag(review.author.review_status)">{{ statusLabel(review.author.review_status) }}</el-tag>
        <span v-if="review.author.had_issues" class="dec-status-issue">⚠ 预检发现需清理项</span>
        <span v-if="review.author.review_note" class="dec-status-note">站长意见：{{ review.author.review_note }}</span>
      </div>

      <el-input
        v-model="authorRaw"
        type="textarea"
        :rows="12"
        class="dec-editor"
        placeholder="在此编写对外展示的作者主页片段（将自动消毒后送审）"
        @keydown="onAuthorKeydown"
      />
      <div class="dec-foot">
        <span class="dec-tip">Ctrl/⌘ + S 提交审核</span>
        <el-button type="primary" :loading="submittingAuthor" @click="submitAuthor">提交审核</el-button>
      </div>

      <!-- 提交结果：预检反馈 -->
      <el-alert
        v-if="submitResult"
        :type="submitResult.had_issues ? 'warning' : 'success'"
        :closable="false"
        show-icon
        class="dec-submit-result"
      >
        <template #title>
          {{ submitResult.had_issues ? '已消毒并送审，预检发现需清理项' : '已消毒并送审' }}
        </template>
        <div v-if="submitResult.removed.length">被移除 / 调整的项：</div>
        <ul v-if="submitResult.removed.length" class="dec-removed-list">
          <li v-for="(r, i) in submitResult.removed" :key="i">{{ r }}</li>
        </ul>
        <div v-if="submitResult.had_issues">原文已送站长复核，通过后将展示消毒后的内容。</div>
        <div v-else>消毒后内容将进入站长审核队列，通过即对外可见。</div>
      </el-alert>

      <el-collapse v-model="authorPreviewOpen" class="dec-preview-collapse">
        <el-collapse-item name="preview">
          <template #title><span class="dec-preview-title">消毒后预览（站长审核通过后将展示此内容）</span></template>
          <div class="dec-preview" v-html="authorSafePreview || '<span style=\'color:#999\'>（空）</span>'" />
        </el-collapse-item>
      </el-collapse>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { homeCustomizeApi, type AuthorSubmitResult } from '@/api/homeCustomize';
import { editorKeydownHandler, HTML_TOKENS } from '@/utils/editorShortcuts';
import { ElMessage } from 'element-plus';

const authStore = useAuthStore();
const authorId = ref(0);

const myRaw = ref('');
const authorRaw = ref('');
const savingMy = ref(false);
const submittingAuthor = ref(false);

const submitResult = ref<AuthorSubmitResult | null>(null);
const authorSafePreview = ref('');

const myPreviewOpen = ref(['preview']);
const authorPreviewOpen = ref(['preview']);

// 作者公开主页提交 / 审核状态
const review = reactive<{ author: { raw_html: string; safe_html: string; public_html: string; review_status: string; had_issues: boolean; review_note: string } }>({
  author: { raw_html: '', safe_html: '', public_html: '', review_status: '', had_issues: false, review_note: '' },
});

function statusTag(s: string) {
  return (s === 'approved' ? 'success' : s === 'rejected' ? 'danger' : s === 'pending' ? 'warning' : 'info') as any;
}
function statusLabel(s: string) {
  return (s === 'approved' ? '已通过' : s === 'rejected' ? '已驳回' : s === 'pending' ? '待审核' : '未提交') as string;
}

async function load() {
  authorId.value = authStore.user?.id ?? 0;
  try {
    const res = await homeCustomizeApi.getMyHomeCustomize();
    if (res.data.code === 0) {
      const d = res.data.data;
      myRaw.value = d.my_raw_html || '';
      Object.assign(review.author, d.author || {});
      authorRaw.value = d.author?.raw_html || '';
      authorSafePreview.value = d.author?.safe_html || '';
    }
  } catch { /* ignore */ }
}

async function saveMy() {
  savingMy.value = true;
  try {
    const res = await homeCustomizeApi.updateMyHomeCustomize(myRaw.value);
    if (res.data.code === 0) {
      ElMessage.success('我的主页装扮已保存');
    } else {
      ElMessage.error(res.data.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingMy.value = false;
}

async function submitAuthor() {
  if (!authorId.value) return;
  submittingAuthor.value = true;
  submitResult.value = null;
  try {
    const res = await homeCustomizeApi.updateAuthorHomeCustomize(authorId.value, authorRaw.value);
    if (res.data.code === 0) {
      const r = res.data.data;
      submitResult.value = r;
      authorSafePreview.value = r.safe_html;
      review.author.review_status = 'pending';
      review.author.had_issues = r.had_issues;
      ElMessage.success(r.had_issues ? '已消毒并送站长复核' : '已提交，等待站长审核');
    } else {
      ElMessage.error(res.data.message || '提交失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  }
  submittingAuthor.value = false;
}

function onMyKeydown(e: KeyboardEvent) {
  editorKeydownHandler(e, { tokens: HTML_TOKENS, onSave: saveMy });
}
function onAuthorKeydown(e: KeyboardEvent) {
  editorKeydownHandler(e, { tokens: HTML_TOKENS, onSave: submitAuthor });
}

onMounted(load);
</script>

<style scoped>
.decorate-page { display: flex; flex-direction: column; gap: 20px; }
.dec-card {
  background: #fff;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.06);
}
[data-theme="dark"] .dec-card { background: #1e293b; box-shadow: 0 1px 6px rgba(0, 0, 0, 0.25); }
.dec-card-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.dec-card-head h2 { font-size: 1.15rem; margin: 0; color: var(--primary-color); }
.dec-hint { margin-bottom: 14px; }
.dec-status-bar {
  display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
  background: #f8fafc; border-radius: 8px; padding: 8px 12px; margin-bottom: 12px; font-size: 0.85rem;
}
[data-theme="dark"] .dec-status-bar { background: #0f172a; }
.dec-status-label { color: var(--text-light); }
.dec-status-issue { color: var(--el-color-warning); }
.dec-status-note { color: var(--text-muted); }
.dec-editor { font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 0.85rem; }
.dec-foot { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 10px; flex-wrap: wrap; }
.dec-tip { font-size: 0.78rem; color: var(--text-muted); }
.dec-submit-result { margin-top: 12px; }
.dec-removed-list { margin: 6px 0 0 18px; padding: 0; font-size: 0.82rem; }
.dec-removed-list li { margin: 2px 0; }
.dec-preview-collapse { margin-top: 12px; }
.dec-preview-title { font-size: 0.85rem; color: var(--text-light); }
.dec-preview {
  border: 1px dashed #cbd5e1; border-radius: 8px; padding: 16px; min-height: 48px;
  background: #fff; font-size: 0.9rem; overflow: auto;
}
[data-theme="dark"] .dec-preview { background: #0f172a; border-color: #334155; }
</style>
