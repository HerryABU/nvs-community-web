<template>
  <div class="compose-page">
    <div class="compose-head">
      <h2>发布内容</h2>
      <el-radio-group v-model="kind" size="small">
        <el-radio-button value="video">短视频</el-radio-button>
        <el-radio-button value="micro">微文（可贴图）</el-radio-button>
        <el-radio-button value="music">音乐</el-radio-button>
      </el-radio-group>
    </div>

    <el-card class="compose-card" shadow="never">
      <el-form label-width="86px">
        <!-- 短视频 -->
        <template v-if="kind === 'video'">
          <el-form-item label="视频">
            <div class="cp-upload">
              <input ref="videoInput" type="file" accept="video/mp4,video/webm" hidden @change="onVideoPick" />
              <div v-if="videoUrl" class="cp-preview">
                <video :src="media(videoUrl)" :poster="media(coverUrl)" controls style="max-width: 340px; max-height: 260px" />
                <el-button size="small" text type="danger" @click="videoUrl = ''; videoFile = null">移除</el-button>
              </div>
              <el-button v-else size="small" :loading="uploading" @click="videoInput?.click()">选择视频（MP4/WebM，≤50MB）</el-button>
              <div class="cp-hint">支持 Ctrl+V 粘贴封面图；未选则用默认占位图。</div>
            </div>
          </el-form-item>
          <el-form-item label="封面（可选）">
            <div class="cp-upload">
              <input ref="coverInput" type="file" accept="image/*" hidden @change="onCoverPick" />
              <img v-if="coverUrl" :src="media(coverUrl)" class="cp-cover" alt="封面" />
              <el-button size="small" @click="coverInput?.click()">{{ coverUrl ? '更换封面' : '上传封面（可选）' }}</el-button>
            </div>
          </el-form-item>
        </template>

        <!-- 音乐（wav/mp3/m4a/ogg/mp4，≤50MB） -->
        <template v-else-if="kind === 'music'">
          <el-form-item label="音频">
            <div class="cp-upload">
              <input ref="audioInput" type="file" accept="audio/*,.wav,.mp3,.m4a,.ogg" hidden @change="onAudioPick" />
              <audio v-if="audioUrl" :src="media(audioUrl)" controls class="cp-audio" />
              <el-button v-else size="small" :loading="uploading" @click="audioInput?.click()">选择音频（≤50MB）</el-button>
            </div>
          </el-form-item>
          <el-form-item label="封面（可选）">
            <div class="cp-upload">
              <input ref="musicCoverInput" type="file" accept="image/*" hidden @change="onMusicCoverPick" />
              <img v-if="coverUrl" :src="media(coverUrl)" class="cp-cover" alt="封面" />
              <el-button size="small" @click="musicCoverInput?.click()">{{ coverUrl ? '更换封面' : '上传封面（可选）' }}</el-button>
              <div class="cp-hint">支持 Ctrl+V 粘贴封面图；不上传也能发，卡片会用音符占位。</div>
            </div>
          </el-form-item>
        </template>

        <!-- 微文多图 -->
        <template v-else>
          <el-form-item label="配图">
            <div class="cp-upload">
              <input ref="imageInput" type="file" accept="image/*" multiple hidden @change="onImagesPick" />
              <div class="cp-grid">
                <div v-for="(u, i) in images" :key="i" class="cp-thumb">
                  <img :src="media(u)" :alt="`图${i + 1}`" />
                  <button @click="images.splice(i, 1)">×</button>
                </div>
                <button v-if="images.length < maxImages" class="cp-add" @click="imageInput?.click()">＋</button>
              </div>
              <div class="cp-hint">支持 Ctrl+V 粘贴图片。最多 {{ maxImages }} 张（后台可调），单张 ≤10MB。</div>
            </div>
          </el-form-item>
        </template>

        <el-form-item label="标题"><el-input v-model="title" maxlength="128" show-word-limit placeholder="可留空" /></el-form-item>
        <el-form-item :label="kind === 'video' || kind === 'music' ? '描述' : '正文'">
          <div class="cp-editor">
            <div class="cp-editor-bar">
              <el-radio-group v-model="mdMode" size="small">
                <el-radio-button value="edit">编辑</el-radio-button>
                <el-radio-button value="preview">Markdown 预览</el-radio-button>
              </el-radio-group>
              <span class="cp-hint">支持 **粗体**、# 标题、- 列表、&gt; 引用、```代码```、[链接](url)</span>
            </div>
            <el-input
              v-if="mdMode === 'edit'"
              v-model="content"
              type="textarea"
              :rows="7"
              maxlength="5000"
              show-word-limit
              :placeholder="kind === 'micro' ? '写下你的想法…（支持 Markdown）' : '说说这个作品…（支持 Markdown）'"
            />
            <!-- 复用全站 Cherry 渲染器（已 DOMPurify），只做预览不引入编辑器 UI -->
            <div v-else class="cp-md-preview cherry-markdown cherry-previewer" v-html="previewHtml" />
          </div>
        </el-form-item>
        <el-form-item label="标签"><el-input v-model="tags" placeholder="逗号分隔，如：日常,手绘" /></el-form-item>
      </el-form>
      <div class="cp-footer">
        <el-button @click="$router.back()">取消</el-button>
        <el-button type="primary" :loading="publishing" @click="publish">发布</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi } from '@/api/feed';
import { uploadApi } from '@/api/social';
import { authApi } from '@/api/auth';
import { getAppPrefix } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';
import { usePasteImage } from '@/composables/usePasteImage';

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const prefix = getAppPrefix() || '';
function media(u?: string) {
  if (!u) return '';
  if (/^https?:\/\//i.test(u)) return u;
  return prefix + (u.startsWith('/') ? u : '/' + u);
}

const initialKind = String(route.query.kind || '');
const kind = ref(['video', 'micro', 'music'].includes(initialKind) ? initialKind : 'micro');
const title = ref('');
const content = ref('');
const tags = ref('');
const videoUrl = ref('');
const audioUrl = ref('');
const videoFile = ref<File | null>(null);
const coverUrl = ref('');
const images = ref<string[]>([]);
const maxImages = ref(20);
const uploading = ref(false);
const publishing = ref(false);

const videoInput = ref<HTMLInputElement | null>(null);
const audioInput = ref<HTMLInputElement | null>(null);
const coverInput = ref<HTMLInputElement | null>(null);
const musicCoverInput = ref<HTMLInputElement | null>(null);
const imageInput = ref<HTMLInputElement | null>(null);

// 轻量 Markdown：只借全站渲染器做预览，不引入 Cherry 编辑器 UI
const mdMode = ref<'edit' | 'preview'>('edit');
const previewHtml = ref('');
watch([content, mdMode], async () => {
  if (mdMode.value !== 'preview') return;
  const { renderMarkdown } = await import('@/markdown/renderer');
  previewHtml.value = renderMarkdown(content.value);
}, { immediate: true });

function pickUrl(res: any): string {
  const d = res?.data?.data ?? res?.data;
  return typeof d === 'string' ? d : (d?.url || '');
}

async function onVideoPick(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0];
  if (!f) return;
  if (f.size > 50 * 1024 * 1024) { ElMessage.error('视频不能超过 50MB'); return; }
  uploading.value = true;
  try {
    const res = await uploadApi.upload(f);
    videoUrl.value = pickUrl(res);
    videoFile.value = f;
    if (!videoUrl.value) ElMessage.error('上传失败：未返回地址');
  } catch (err: any) { ElMessage.error(err?.response?.data?.message || '视频上传失败'); }
  finally { uploading.value = false; }
}

async function onAudioPick(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0];
  if (!f) return;
  if (f.size > 50 * 1024 * 1024) { ElMessage.error('音频不能超过 50MB'); return; }
  uploading.value = true;
  try {
    const res = await uploadApi.upload(f);
    audioUrl.value = pickUrl(res);
    if (!audioUrl.value) ElMessage.error('上传失败：未返回地址');
  } catch (err: any) { ElMessage.error(err?.response?.data?.message || '音频上传失败'); }
  finally { uploading.value = false; }
}

async function uploadCoverFile(f: File) {
  uploading.value = true;
  try {
    const fd = new FormData();
    fd.append('image', f);
    const res = await authApi.uploadImage(fd);
    coverUrl.value = pickUrl(res);
  } catch (err: any) { ElMessage.error(err?.response?.data?.message || '封面上传失败'); }
  finally { uploading.value = false; }
}

async function onCoverPick(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0];
  if (f) await uploadCoverFile(f);
}

async function onMusicCoverPick(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0];
  if (f) await uploadCoverFile(f);
}

async function uploadMicroImage(f: File) {
  if (images.value.length >= maxImages.value) return;
  if (f.size > 10 * 1024 * 1024) { ElMessage.warning(`${f.name || '粘贴的图片'}超过 10MB，已跳过`); return; }
  uploading.value = true;
  try {
    const fd = new FormData();
    fd.append('image', f);
    const res = await authApi.uploadImage(fd);
    const url = pickUrl(res);
    if (url) images.value.push(url);
  } catch (err: any) { ElMessage.error(err?.response?.data?.message || '图片上传失败'); }
  finally { uploading.value = false; }
}

async function onImagesPick(e: Event) {
  const files = Array.from((e.target as HTMLInputElement).files || []);
  for (const f of files) await uploadMicroImage(f);
  if (imageInput.value) imageInput.value.value = '';
}

// Ctrl+V 粘贴图片：微文进配图列表，短视频进封面
usePasteImage({
  enabled: computed(() => kind.value === 'micro'),
  multiple: true, maxSizeMB: 10,
  onImage: (f) => uploadMicroImage(f),
  onError: (m) => ElMessage.warning(m),
});
usePasteImage({
  enabled: computed(() => kind.value === 'video' || kind.value === 'music'),
  maxSizeMB: 10,
  onImage: (f) => uploadCoverFile(f),
  onError: (m) => ElMessage.warning(m),
});

async function publish() {
  if (!auth.isLoggedIn) { ElMessage.warning('请先登录'); router.push('/login?redirect=/feed/compose'); return; }
  if (kind.value === 'video' && !videoUrl.value) { ElMessage.warning('请先上传视频'); return; }
  if (kind.value === 'music' && !audioUrl.value) { ElMessage.warning('请先上传音乐'); return; }
  if (kind.value === 'micro' && !content.value.trim() && images.value.length === 0) {
    ElMessage.warning('写点什么或上传一张图'); return;
  }
  publishing.value = true;
  try {
    const res = await feedApi.create({
      kind: kind.value,
      title: title.value,
      content: content.value,
      video_url: videoUrl.value,
      audio_url: audioUrl.value,
      cover_url: coverUrl.value,
      images: images.value,
      tags: tags.value,
    });
    const id = res.data?.data?.id;
    ElMessage.success('发布成功');
    router.push(`/discover?id=${id}`);
  } catch (err: any) { ElMessage.error(err?.response?.data?.message || '发布失败'); }
  finally { publishing.value = false; }
}

onMounted(async () => {
  if (!auth.isLoggedIn) router.push('/login?redirect=/feed/compose');
  try {
    const r = await feedApi.config();
    maxImages.value = r.data?.data?.max_micro_images || 20;
  } catch { maxImages.value = 20; }
});
</script>

<style scoped>
.compose-page { max-width: 740px; margin: 0 auto; padding: 24px 16px 50px; }
.compose-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.compose-head h2 { margin: 0; font-size: 1.25rem; }
.compose-card { border-radius: 10px; }
.cp-upload { display: flex; flex-direction: column; gap: 8px; align-items: flex-start; width: 100%; }
.cp-hint { font-size: 0.78rem; color: var(--text-muted, #6b7280); }
.cp-preview { display: flex; flex-direction: column; gap: 6px; }
/* 音频：必须给确定宽度。.cp-upload 若是 flex 项（宽度 auto），width:100% 会解析成 0，
   播放条在页面上等于不存在（实测 rect 宽 0，能出声但看不到也点不到）。 */
.cp-audio { width: 100%; max-width: 420px; min-width: 240px; }
.cp-cover { width: 160px; height: 100px; object-fit: cover; border-radius: 6px; }
.cp-grid { display: grid; grid-template-columns: repeat(4, 92px); gap: 8px; }
.cp-thumb { position: relative; width: 92px; height: 92px; }
.cp-thumb img { width: 100%; height: 100%; object-fit: cover; border-radius: 6px; }
.cp-thumb button { position: absolute; top: -6px; right: -6px; width: 20px; height: 20px; border-radius: 50%; border: none; background: #f56c6c; color: #fff; cursor: pointer; line-height: 18px; }
.cp-add { width: 92px; height: 92px; border: 1px dashed var(--border-color, #d1d5db); border-radius: 6px; background: transparent; color: var(--text-muted, #6b7280); font-size: 1.5rem; cursor: pointer; }
.cp-editor { width: 100%; }
.cp-editor-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.cp-md-preview { min-height: 170px; padding: 12px 14px; border: 1px solid var(--border-color); border-radius: 6px; background: var(--bg-soft, #f8fafc); line-height: 1.7; overflow-wrap: anywhere; }
.cp-footer { display: flex; justify-content: flex-end; gap: 8px; padding-top: 12px; border-top: 1px solid var(--border-color); }
@media (max-width: 768px) {
  .compose-page { padding: 14px 10px 30px; }
  .cp-grid { grid-template-columns: repeat(3, 76px); }
  .cp-thumb, .cp-add { width: 76px; height: 76px; }
}
</style>
