<template>
  <div class="decorate-page">
    <!-- ============ 区块一：资料卡（他人看到的主页头部） ============ -->
    <div class="dec-card">
      <div class="dec-card-head">
        <h2>🪪 资料卡</h2>
        <div class="dec-head-right">
          <el-tag size="small" type="success">所有人可见</el-tag>
          <el-button size="small" text @click="openPublicPage">看看别人怎么看到我 →</el-button>
        </div>
      </div>

      <div class="prof-layout">
        <div class="prof-avatar-col">
          <el-avatar :size="88" :src="form.avatar_url || ''">
            {{ (form.nickname || authStore.user?.username || 'A')[0] }}
          </el-avatar>
          <el-input
            v-model="form.avatar_url"
            size="small"
            placeholder="/uploads/xxx.png 或 https://…"
            class="prof-avatar-input"
          />
          <div class="prof-avatar-tip">站内上传路径或图片直链</div>
        </div>

        <div class="prof-fields">
          <el-form label-width="72px" label-position="left">
            <el-form-item label="昵称">
              <el-input v-model="form.nickname" maxlength="32" show-word-limit placeholder="别人看到的名字" />
            </el-form-item>
            <el-form-item label="一句话签名">
              <el-input v-model="form.signature" maxlength="128" show-word-limit placeholder="如：写故事的人，偶尔写代码" />
            </el-form-item>
            <el-form-item label="简介">
              <el-input
                v-model="form.bio"
                type="textarea"
                :rows="3"
                maxlength="500"
                show-word-limit
                placeholder="介绍一下自己：创作方向、更新节奏、想对读者说的话…"
              />
            </el-form-item>
          </el-form>
          <div class="dec-foot">
            <span class="dec-tip">资料卡会显示在 <code>/author/{{ authStore.user?.id }}</code> 主页顶部</span>
            <el-button type="primary" :loading="savingProfile" @click="saveProfile">保存资料</el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- ============ 区块二：我的主页装扮（仅本人可见，原始 HTML） ============ -->
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
        :rows="10"
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

    <!-- ============ 区块三：作者公开主页（消毒 + 站长审核） ============ -->
    <div class="dec-card">
      <div class="dec-card-head">
        <h2>🌐 作者公开主页</h2>
        <el-tag size="small" :type="statusTag(review.author.review_status)">{{ statusLabel(review.author.review_status) }}</el-tag>
      </div>
      <el-alert type="warning" :closable="false" show-icon class="dec-hint">
        <template #title>对外可见 · 需消毒与审核</template>
        他人访问你的作者主页时会看到这段内容（作为主页<b>封面</b>展示）。提交后服务端会<b>自动消毒</b>（剥离脚本 / 危险 CSS），
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
        :rows="10"
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
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { authApi } from '@/api/auth';
import { homeCustomizeApi, type AuthorSubmitResult } from '@/api/homeCustomize';
import { editorKeydownHandler, HTML_TOKENS } from '@/utils/editorShortcuts';
import { ElMessage } from 'element-plus';

const router = useRouter();
const authStore = useAuthStore();
const authorId = ref(0);

// ── 资料卡（昵称 / 签名 / 简介 / 头像）──
const form = reactive({ nickname: '', signature: '', bio: '', avatar_url: '' });
const savingProfile = ref(false);

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

function openPublicPage() {
  if (!authorId.value) return;
  router.push(`/author/${authorId.value}`);
}

async function load() {
  authorId.value = authStore.user?.id ?? 0;
  // 资料卡先用登录态里的值铺底（避免接口慢时空白）
  form.nickname = authStore.user?.nickname || authStore.user?.username || '';
  form.signature = authStore.user?.signature || '';
  form.bio = authStore.user?.bio || '';
  form.avatar_url = authStore.user?.avatar_url || '';

  try {
    const res = await homeCustomizeApi.getMyHomeCustomize();
    if (res.data.code === 0) {
      const d = res.data.data;
      myRaw.value = d.my_raw_html || '';
      Object.assign(review.author, d.author || {});
      authorRaw.value = d.author?.raw_html || '';
      authorSafePreview.value = d.author?.safe_html || '';
    }
  } catch { /* 装扮接口不可用不阻断资料编辑 */ }
}

async function saveProfile() {
  const nick = form.nickname.trim();
  if (!nick) { ElMessage.warning('昵称不能为空'); return; }
  savingProfile.value = true;
  try {
    // 注意：头像传空串 = 清除头像（后端接受空串清空）
    const res = await authApi.updateProfile({
      nickname: nick,
      signature: form.signature.trim(),
      bio: form.bio.trim(),
      avatar_url: form.avatar_url.trim(),
    });
    if (res.data.code === 0) {
      ElMessage.success('资料已保存');
      // 同步登录态，导航栏头像/昵称立即更新
      await authStore.fetchUser(true);
    } else {
      ElMessage.error((res.data as any).message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingProfile.value = false;
  }
}

async function saveMy() {
  savingMy.value = true;
  try {
    const res = await homeCustomizeApi.updateMyHomeCustomize(myRaw.value);
    if (res.data.code === 0) {
      ElMessage.success('我的主页装扮已保存');
    } else {
      ElMessage.error((res.data as any).message || '保存失败');
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
      ElMessage.error((res.data as any).message || '提交失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  }
  submittingAuthor.value = false;
}

// el-input 的 keydown 事件类型是 Event | KeyboardEvent，这里收窄后交快捷键工具处理
function onMyKeydown(e: Event | KeyboardEvent) {
  if (e instanceof KeyboardEvent) editorKeydownHandler(e, { tokens: HTML_TOKENS, onSave: saveMy });
}
function onAuthorKeydown(e: Event | KeyboardEvent) {
  if (e instanceof KeyboardEvent) editorKeydownHandler(e, { tokens: HTML_TOKENS, onSave: submitAuthor });
}

onMounted(load);
</script>

<style scoped>
.decorate-page { display: flex; flex-direction: column; gap: 20px; }
.dec-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 20px;
}
.dec-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.dec-card-head h2 {
  font-size: 1.05rem;
  margin: 0;
  color: var(--text-color);
}
.dec-head-right { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

/* 资料卡布局：头像列 + 字段列 */
.prof-layout {
  display: flex;
  gap: 22px;
  align-items: flex-start;
}
.prof-avatar-col {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
.prof-avatar-input { margin-top: 4px; }
.prof-avatar-tip {
  font-size: 0.72rem;
  color: var(--text-muted, #94a3b8);
  text-align: center;
  line-height: 1.5;
}
.prof-fields { flex: 1; min-width: 0; }

.dec-hint { margin-bottom: 14px; }
.dec-status-bar {
  display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
  background: var(--bg-soft, #f8fafc); border-radius: 8px; padding: 8px 12px; margin-bottom: 12px; font-size: 0.85rem;
}
.dec-status-label { color: var(--text-light); }
.dec-status-issue { color: var(--el-color-warning); }
.dec-status-note { color: var(--text-muted); }
.dec-editor { font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 0.85rem; }
.dec-foot { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 10px; flex-wrap: wrap; }
.dec-tip { font-size: 0.78rem; color: var(--text-muted); }
.dec-tip code { background: var(--bg-soft, #f1f5f9); padding: 1px 4px; border-radius: 3px; }
.dec-submit-result { margin-top: 12px; }
.dec-removed-list { margin: 6px 0 0 18px; padding: 0; font-size: 0.82rem; }
.dec-removed-list li { margin: 2px 0; }
.dec-preview-collapse { margin-top: 12px; }
.dec-preview-title { font-size: 0.85rem; color: var(--text-light); }
.dec-preview {
  border: 1px dashed var(--border-color, #cbd5e1);
  border-radius: 8px;
  padding: 16px;
  min-height: 48px;
  background: var(--bg-body, #fff);
  font-size: 0.9rem;
  overflow: auto;
}

@media (max-width: 640px) {
  .prof-layout { flex-direction: column; }
  .prof-avatar-col { width: 100%; }
  .prof-fields { width: 100%; }
}
</style>
