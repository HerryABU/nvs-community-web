<template>
  <div class="page-container">
    <div class="editor-header">
      <h1>{{ isEditing ? '编辑作品' : '新建作品' }}</h1>
      <div class="header-actions">
        <el-button @click="$router.back()">返回</el-button>
        <el-button type="primary" :loading="saving" @click="saveNovel">保存</el-button>
      </div>
    </div>

    <!-- 作品基本信息 -->
    <el-card class="editor-section">
      <template #header>作品信息</template>
      <el-form :model="form" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="16">
            <el-form-item label="标题" required>
              <el-input v-model="form.title" placeholder="请输入作品标题" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="分类">
              <el-select v-model="form.categories" multiple placeholder="可选择多个分类" style="width:100%" filterable allow-create>
                <el-option v-for="cat in categories" :key="cat" :label="cat" :value="cat" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="简介">
          <el-input v-model="form.summary" type="textarea" :rows="4" placeholder="作品简介" />
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="封面URL">
              <el-input v-model="form.cover_url" placeholder="封面图片链接（可选）">
                <template #append>
                  <el-button @click="openCoverGen">AI 生成</el-button>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="定价(元/章)">
              <el-input-number v-model="form.price_per_chapter" :min="0" :precision="2" :step="0.5" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio value="draft">草稿</el-radio>
            <el-radio value="published">发布</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="作品来源">
              <el-radio-group v-model="form.source_type">
                <el-radio value="original">原创</el-radio>
                <el-radio value="reprint">转载</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="创作方式">
              <el-radio-group v-model="form.creation_method">
                <el-radio value="human">人工</el-radio>
                <el-radio value="ai">AI</el-radio>
                <el-radio value="human_ai_assisted">人工借助AI</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="叙事形态">
          <el-radio-group v-model="form.mode">
            <el-radio value="prose">散文体</el-radio>
            <el-radio value="script">话本气泡体</el-radio>
            <el-radio value="comic">漫画体</el-radio>
            <el-radio value="image">图片模式</el-radio>
          </el-radio-group>
          <span style="margin-left:12px;font-size:0.8rem;color: var(--text-light, #999)">
            话本气泡体：正文按「角色：台词」逐行解析为对话气泡；漫画体/图片模式：每章以有序图片集呈现（漫画页或插画）
          </span>
        </el-form-item>
        <!-- #76 同人衍生 -->
        <el-divider content-position="left">同人衍生</el-divider>
        <el-form-item label="作品性质">
          <el-radio-group v-model="form.derivative_type">
            <el-radio value="">原创</el-radio>
            <el-radio value="site">站内同人</el-radio>
            <el-radio value="external">站外同人</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="引用原著" v-if="form.derivative_type === 'site'">
          <el-select
            v-model="form.derivative_origin_id"
            filterable
            remote
            :remote-method="searchOrigin"
            placeholder="搜索站内已发布原著"
            style="width:100%"
            clearable
          >
            <el-option v-for="o in originOptions" :key="o.id" :label="o.title" :value="o.id" />
          </el-select>
          <span v-if="originTitle" style="margin-left:8px;font-size:0.8rem;color: var(--text-light, #999)">
            已选：《{{ originTitle }}》
          </span>
        </el-form-item>
        <el-form-item label="来源标注" v-if="form.derivative_type === 'external'">
          <el-input
            v-model="form.derivative_external_source"
            placeholder="例如：原作《XXX》/ 作者 YYY / 来源平台 ZZ"
            maxlength="256"
            show-word-limit
          />
        </el-form-item>
        <!-- 隔离墙开关（仅编辑模式可见） -->
        <template v-if="isEditing">
          <el-divider content-position="left">隔离墙设置</el-divider>
          <el-form-item label="启用隔离">
            <el-switch v-model="form.wall_enabled" size="small" />
            <span style="margin-left:8px;font-size:0.8rem;color: var(--text-light, #999)">{{ form.wall_enabled !== false ? '已启用 - 读者进入需确认' : '已关闭 - 读者可直接阅读' }}</span>
          </el-form-item>
          <el-form-item label="警告语" v-if="form.wall_enabled !== false">
            <el-input v-model="form.wall_warning" type="textarea" :rows="3" placeholder="自定义读者进入时的隔离警告语（如：本作品涉及敏感题材，请确认您年满18周岁并愿意承担阅读风险）" maxlength="512" show-word-limit />
          </el-form-item>
        </template>
        <!-- #65 作品级阅读权限（可选等级由站长在后台配置） -->
          <el-form-item label="阅读权限">
            <ReadPermissionPicker v-model="form.read_permission" v-model:price="form.paid_price" :disabled="saving" />
          </el-form-item>
          <!-- 评论区开关：作者可关闭本作品评论区（含章评） -->
          <el-form-item label="评论区">
            <el-switch v-model="form.comments_enabled" size="small" />
            <span style="margin-left:8px;font-size:0.8rem;color: var(--text-light, #999)">{{ form.comments_enabled !== false ? '开启 - 读者可评论' : '已关闭 - 读者无法评论' }}</span>
          </el-form-item>
      </el-form>
    </el-card>

    <!-- 章节管理（仅编辑模式） -->
    <template v-if="isEditing && novelId">
      <el-card class="editor-section">
        <template #header>
          <div class="section-header">
            <span>章节管理（共 {{ chapters.length }} 章）</span>
            <div class="section-actions">
              <el-button type="success" size="small" @click="showImportDialog = true">
                <el-icon><Upload /></el-icon>导入文件
              </el-button>
              <el-button type="warning" size="small" @click="showBatchDialog = true">
                <el-icon><Upload /></el-icon>批量导入
              </el-button>
              <el-button type="primary" size="small" @click="goWriteFirst">
                <el-icon><Plus /></el-icon>新增章节
              </el-button>
            </div>
          </div>
        </template>
        <el-empty v-if="chapters.length === 0" description="暂无章节，点击「新增章节」或「导入文件」开始创作" />
        <div v-else class="chapter-list">
          <div v-for="ch in chapters" :key="ch.chapter_number" class="chapter-item">
            <span class="ch-num">第{{ ch.chapter_number }}章</span>
            <span class="ch-title">{{ ch.title }}</span>
            <span class="ch-words">{{ ch.word_count }}字</span>
            <div class="ch-actions">
              <el-button size="small" text @click="goWrite(ch.chapter_number)">编辑</el-button>
              <el-button size="small" text type="danger" @click="deleteChapter(ch)">删除</el-button>
            </div>
          </div>
        </div>
      </el-card>
    </template>

    <!-- 导入文件对话框 -->
    <el-dialog v-model="showImportDialog" title="导入文件" width="700px" destroy-on-close @closed="resetImport">
      <el-form label-width="80px" v-if="!importPreviewData">
        <el-form-item label="作品标题">
          <el-input v-model="importForm.title" :placeholder="isEditing ? '追加到当前作品（标题无需填写）' : '导入后作品标题（可选）'" />
          <div class="import-rule-hint" style="width:100%">
            <template v-if="isEditing">当前处于编辑模式：导入章节将<strong>追加到《{{ form.title }}》</strong>末尾，不会新建作品。</template>
            <template v-else>新建作品模式下将创建一部新作品。</template>
          </div>
        </el-form-item>
        <el-form-item label="选择文件">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            :on-change="onFileChange"
            :on-remove="onFileRemove"
            accept=".md,.txt,.html,.docx,.doc"
            drag
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
              将 MD / TXT / Word 文件拖到此处，或<em>点击上传</em>
            </div>
            <template #tip>
              <div class="el-upload__tip">
                支持 Word (.docx)、Markdown (.md)、纯文本 (.txt)。上传后可预览章节划分并调整分割规则。
              </div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>

      <!-- 预览阶段 -->
      <div v-if="importPreviewData" class="import-preview">
        <div class="import-preview-header">
          <span>共解析出 <strong>{{ importPreviewData.chapters?.length || 0 }}</strong> 章</span>
          <el-button size="small" text @click="resetImport">重新选择文件</el-button>
        </div>
        <el-form label-width="100px" size="small" style="margin-bottom:12px">
          <el-form-item label="自定义分割规则">
            <el-input v-model="importForm.splitRule" placeholder="默认自动识别。也可输入正则，如：(?m)^第[一二三四五六七八九十百千0-9]+章">
              <template #append>
                <el-button @click="doPreview" :loading="importPreviewing">重新解析</el-button>
              </template>
            </el-input>
            <div class="import-rule-hint">
              快捷规则：
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^第[一二三四五六七八九十百千\\d]+章'">第X章</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^#{1,3}\\s'"># / ## / ###</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^###\\s'">### 仅三级</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^##\\s'">## 仅二级</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^-{3,}$'">--- 分割线</el-button>
            </div>
          </el-form-item>
        </el-form>
        <div class="import-chapter-list">
          <div v-for="(ch, idx) in importChapters" :key="idx" class="import-chapter-item">
            <span class="import-ch-num">{{ idx + 1 }}</span>
            <el-input v-model="ch.title" size="small" style="flex:1" placeholder="章节标题" />
            <span class="import-ch-words">{{ ch.words }}字</span>
            <el-button size="small" text type="danger" @click="importChapters.splice(idx, 1)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-empty v-if="importChapters.length === 0" description="已删除所有章节" :image-size="40" />
        </div>
      </div>

      <template #footer>
        <el-button @click="showImportDialog = false">取消</el-button>
        <el-button v-if="!importPreviewData" type="primary" :loading="importPreviewing" @click="doPreview" :disabled="!importFile">
          预览章节
        </el-button>
        <el-button v-else type="primary" :loading="importing" @click="doImport">确认导入</el-button>
      </template>
    </el-dialog>

    <!-- 批量导入对话框 -->
    <el-dialog v-model="showBatchDialog" title="批量导入（Word / Markdown）" width="620px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="作品标题">
          <el-input v-model="batchForm.title" :placeholder="isEditing ? '追加到当前作品（标题无需填写）' : '合并为一部作品（可选）'" />
          <div class="import-rule-hint" style="width:100%">
            <template v-if="isEditing">当前处于编辑模式：导入章节将<strong>追加到《{{ form.title }}》</strong>末尾，不会新建作品。</template>
            <template v-else>新建作品模式下将创建一部新作品。</template>
          </div>
        </el-form-item>
        <el-form-item label="导入方式">
          <el-radio-group v-model="batchForm.mode">
            <el-radio-button value="file">一个文件一章</el-radio-button>
            <el-radio-button value="rule">自定义规则</el-radio-button>
            <el-radio-button value="auto">自动识别标题</el-radio-button>
          </el-radio-group>
          <div class="import-rule-hint" style="width:100%">
            <template v-if="batchForm.mode === 'file'">每个文件作为一章，章节标题取文件名（去扩展名），按文件名顺序编号</template>
            <template v-else-if="batchForm.mode === 'auto'">自动识别文件内的「第X章 / HTML标题 / Markdown 标题」拆分为多章</template>
          </div>
        </el-form-item>
        <el-form-item v-if="batchForm.mode === 'rule'" label="拆分规则">
          <el-input v-model="batchForm.splitRule" placeholder="正则表达式，如 (?m)^第[一二三四五六七八九十百千\d]+章" />
          <div class="import-rule-hint">
            快捷规则：
            <el-button size="small" text type="primary" @click="batchForm.splitRule='(?m)^第[一二三四五六七八九十百千\\d]+章'">第X章</el-button>
            <el-button size="small" text type="primary" @click="batchForm.splitRule='(?m)^#{1,3}\\s'"># / ## / ###</el-button>
            <el-button size="small" text type="primary" @click="batchForm.splitRule='(?m)^-{3,}$'">--- 分割线</el-button>
          </div>
        </el-form-item>
        <el-form-item label="标题前缀">
          <el-switch v-model="batchForm.titlePrefix" active-text="章节标题加「第N章 」前缀" inactive-text="使用原名" />
        </el-form-item>
        <el-form-item label="选择文件">
          <el-upload
            ref="batchUploadRef"
            :auto-upload="false"
            multiple
            :on-change="onBatchFileChange"
            accept=".md,.txt,.docx,.doc"
            drag
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">将多个 Word / Markdown 文件拖到此处</div>
            <template #tip>
              <div class="el-upload__tip">可一次选择多个 .docx / .md / .txt 文件，按所选方式合并为一部作品的章节</div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showBatchDialog = false">取消</el-button>
        <el-button type="primary" :loading="batchImporting" :disabled="batchFiles.length === 0" @click="doBatchImport">
          批量导入
        </el-button>
      </template>
    </el-dialog>

    <!-- AI 生成小说封面（复用 /api/ai/image 文生图） -->
    <el-dialog v-model="coverGenVisible" title="AI 生成小说封面" width="520px">
      <el-form label-width="70px">
        <el-form-item label="提示词">
          <el-input v-model="coverPrompt" type="textarea" :rows="4"
            placeholder="描述你想要的封面画面，可留空用作品信息自动生成" />
        </el-form-item>
        <el-form-item label="比例">
          <el-select v-model="coverRatio" style="width:160px">
            <el-option label="3:4（竖版封面）" value="3:4" />
            <el-option label="2:3" value="2:3" />
            <el-option label="1:1" value="1:1" />
          </el-select>
        </el-form-item>
        <el-form-item label="预览" v-if="coverPreview">
          <el-image :src="coverPreview" fit="cover" style="width:120px;height:160px;border-radius:6px" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="coverGenVisible = false">取消</el-button>
        <el-button type="primary" :loading="coverGenerating" @click="generateCover">生成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, chapterApi, type Novel, type Chapter } from '@/api/novel';
import { publicApi } from '@/api/admin';
import { authApi } from '@/api/auth';
import api from '@/api/index';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Upload, Plus, UploadFilled, Delete } from '@element-plus/icons-vue';
import ReadPermissionPicker from '@/components/readperm/ReadPermissionPicker.vue';

const route = useRoute();
const router = useRouter();

const categories = ref<string[]>([]);

const novelId = ref<number | null>(route.params.id ? Number(route.params.id) : null);
const isEditing = !!novelId.value;

const saving = ref(false);
const chapters = ref<Chapter[]>([]);

const form = reactive<Partial<Novel>>({
  wall_enabled: true,
  wall_warning: '',
  comments_enabled: true, // 评论区开关：默认开启，作者可关闭本作品评论区
  title: '',
  category: '其他',
  categories: [] as string[],
  summary: '',
  cover_url: '',
  price_per_chapter: 0,
  status: 'draft',
  source_type: 'original',
  creation_method: 'human',
  mode: 'prose', // prose 散文体 / script 话本气泡体
  read_permission: 'public', // #65 作品级阅读权限
  paid_price: 0,             // #65 paid 等级售价（代币）
  // #76 同人衍生
  derivative_type: '' as any,          // ""原创 / site站内同人 / external站外同人
  derivative_origin_id: 0,
  derivative_origin_title: '',
  derivative_external_source: '',
});

// AI 生成小说封面（复用 /api/ai/image 文生图，不需要新建端点）
const coverGenVisible = ref(false);
const coverPrompt = ref('');
const coverRatio = ref('3:4');
const coverGenerating = ref(false);
const coverPreview = ref('');
function openCoverGen() {
  coverPrompt.value = `小说《${form.title || ''}》的封面：主题${form.summary || ''}。风格：${form.creation_method === 'ai' ? 'AI 质感' : '写实插画'}`;
  coverPreview.value = '';
  coverGenVisible.value = true;
}
async function generateCover() {
  coverGenerating.value = true;
  try {
    const prompt = coverPrompt.value.trim() || `小说《${form.title}》封面`;
    const res = await api.post<{ code: number; data: { images?: string[] } }>('/ai/image', {
      prompt,
      ratio: coverRatio.value,
      n: 1,
    });
    if (res.data.code === 0 && res.data.data.images && res.data.data.images.length) {
      const url = res.data.data.images[0];
      form.cover_url = url;
      coverPreview.value = url;
      ElMessage.success('已生成，点击「保存」即可使用');
    } else {
      ElMessage.error('生成失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  } finally {
    coverGenerating.value = false;
  }
}

// #76 同人衍生：搜索站内原著
const originOptions = ref<{ id: number; title: string }[]>([]);
const originTitle = computed(() =>
  originOptions.value.find((o) => o.id === form.derivative_origin_id)?.title || form.derivative_origin_title || '',
);
async function searchOrigin(q: string) {
  if (!q) {
    originOptions.value = [];
    return;
  }
  try {
    const res = await api.get<{ code: number; data: { data: Novel[] } }>('/novels', {
      params: { search: q, page: 1, page_size: 10 },
    });
    originOptions.value = (res.data.data?.data || []).map((n) => ({ id: n.id, title: n.title }));
  } catch {
    originOptions.value = [];
  }
}

// 导入相关
const showImportDialog = ref(false);
const importing = ref(false);
const importPreviewing = ref(false);
const uploadRef = ref();
const importFile = ref<File | null>(null);
const importPreviewData = ref<any>(null);
const importChapters = ref<{ title: string; words: number }[]>([]);
const importForm = reactive({ title: '', splitRule: '' });

// 批量导入
const showBatchDialog = ref(false);
const batchImporting = ref(false);
const batchUploadRef = ref();
const batchFiles = ref<File[]>([]);
const batchForm = reactive({ title: '', mode: 'file', splitRule: '', titlePrefix: false });

function onBatchFileChange(file: any) {
  if (file.raw) batchFiles.value.push(file.raw);
}

async function doBatchImport() {
  if (batchFiles.value.length === 0) { ElMessage.warning('请选择文件'); return; }
  batchImporting.value = true;
  try {
    const res = await novelApi.importBatch(
      batchFiles.value,
      batchForm.title.trim() || undefined,
      undefined,
      batchForm.mode,
      batchForm.mode === 'rule' ? batchForm.splitRule.trim() : undefined,
      batchForm.titlePrefix,
      novelId.value || undefined, // 编辑模式：追加到当前小说，不再新建书本
    );
    const data = res.data.data;
    ElMessage.success(`批量导入成功！${data.files_count} 个文件，共 ${data.chapters_count} 章${data.is_append ? '（已追加到当前作品）' : ''}`);
    showBatchDialog.value = false;
    batchFiles.value = [];
    batchForm.title = '';
    batchForm.mode = 'file';
    batchForm.splitRule = '';
    batchForm.titlePrefix = false;
    batchUploadRef.value?.clearFiles();
    // 追加到当前小说则刷新章节列表；新建作品则跳转到编辑页
    if (novelId.value && data.novel_id === novelId.value) {
      const cr = await chapterApi.getChapters(novelId.value);
      chapters.value = cr.data.data || [];
    } else if (!novelId.value && data.novel_id) {
      router.push(`/author/editor/${data.novel_id}`);
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '批量导入失败');
  } finally {
    batchImporting.value = false;
  }
}

const sensitiveZones = ref<string[]>([]);
const showWallToggle = computed(() => {
  const cats = form.categories?.length ? form.categories : (form.category ? [form.category] : []);
  return cats.some((c: string) => {
    if (sensitiveZones.value.includes(c)) return true;
    return sensitiveZones.value.some(z => c.includes(z) || (z.includes(c) && c.length >= 2));
  });
});

async function loadSensitiveZones() {
  try {
    const res = await authApi.get('/site-info');
    const d = res.data?.data;
    if (d?.wall_enabled && Array.isArray(d.wall_zones)) { sensitiveZones.value = d.wall_zones; }
  } catch {}
}

async function loadNovel() {
  if (!novelId.value) return;
  try {
    const [novelRes, chaptersRes] = await Promise.all([
      novelApi.getNovel(novelId.value),
      chapterApi.getChapters(novelId.value),
    ]);
    const novel = novelRes.data.data;
    Object.assign(form, {
      title: novel.title,
      category: novel.category,
      categories: novel.categories || (novel.category ? [novel.category] : []),
      summary: novel.summary,
      cover_url: novel.cover_url,
      price_per_chapter: novel.price_per_chapter,
      status: novel.status,
      source_type: novel.source_type || 'original',
      creation_method: novel.creation_method || 'human',
      // 叙事形态：prose 散文体 / script 话本气泡体
      mode: novel.mode || 'prose',
      wall_enabled: novel.wall_enabled !== false,
      wall_warning: novel.wall_warning || '',
      comments_enabled: novel.comments_enabled !== false,
      read_permission: novel.read_permission || 'public',
      paid_price: novel.paid_price || 0,
      // #76 同人衍生回填
      derivative_type: novel.derivative_type || '',
      derivative_origin_id: novel.derivative_origin_id || 0,
      derivative_origin_title: novel.derivative_origin_title || '',
      derivative_external_source: novel.derivative_external_source || '',
    });
    // 编辑态回填时，把已选原著放进候选，保证标题可显示
    if (novel.derivative_origin_id) {
      originOptions.value = [{ id: novel.derivative_origin_id, title: novel.derivative_origin_title || '' }];
    }
    chapters.value = chaptersRes.data.data || [];
  } catch (e) {
    console.error(e);
  }
}

async function saveNovel() {
  if (!form.title) {
    ElMessage.warning('请输入作品标题');
    return;
  }
  saving.value = true;
  try {
    if (isEditing && novelId.value) {
      await novelApi.updateNovel(novelId.value, form);
      ElMessage.success('作品已更新');
    } else {
      const res = await novelApi.createNovel(form);
      ElMessage.success('作品已创建');
      router.replace(`/author/editor/${res.data.data.id}`);
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

function goWrite(chapterNum: number) {
  router.push(`/author/editor/${novelId.value}/chapter/${chapterNum}`);
}

function goWriteFirst() {
  const nextNum = chapters.value.length > 0
    ? Math.max(...chapters.value.map(c => c.chapter_number)) + 1
    : 1;
  router.push(`/author/editor/${novelId.value}/chapter/${nextNum}`);
}

async function deleteChapter(ch: Chapter) {
  try {
    await ElMessageBox.confirm(`确认删除「第${ch.chapter_number}章 ${ch.title}」？`, '删除确认', {
      confirmButtonText: '确认删除',
      type: 'warning',
    });
    await chapterApi.deleteChapter(novelId.value!, ch.chapter_number);
    chapters.value = chapters.value.filter(c => c.chapter_number !== ch.chapter_number);
    ElMessage.success('已删除');
  } catch { /* 取消 */ }
}

// 导入文件
function onFileChange(file: any) {
  importFile.value = file.raw;
  importPreviewData.value = null;
  importChapters.value = [];
}

function onFileRemove() {
  importFile.value = null;
  importPreviewData.value = null;
}

function resetImport() {
  importFile.value = null;
  importPreviewData.value = null;
  importChapters.value = [];
  importForm.splitRule = '';
  uploadRef.value?.clearFiles();
}

async function doPreview() {
  if (!importFile.value) {
    ElMessage.warning('请选择文件');
    return;
  }
  importPreviewing.value = true;
  try {
    const res = await novelApi.importPreview(importFile.value, importForm.splitRule || undefined);
    if (res.data.code === 0) {
      importPreviewData.value = res.data.data;
      importChapters.value = (res.data.data.chapters || []).map((ch: any) => ({
        title: ch.title,
        words: ch.words,
      }));
      ElMessage.success(`解析完成，共 ${res.data.data.total} 章`);
    } else {
      ElMessage.error(res.data.message || '解析失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '预览失败');
  } finally {
    importPreviewing.value = false;
  }
}

async function doImport() {
  if (!importFile.value) return;
  importing.value = true;
  try {
    // 编辑模式：追加到已有小说；新建模式：创建新小说
    const targetNovelId = novelId.value || undefined;
    const res = await novelApi.importNovel(
      importFile.value,
      importForm.title || importFile.value.name.replace(/\.[^.]+$/, ''),
      (form.categories && form.categories.length > 0) ? form.categories[0] : '其他',
      targetNovelId,
      importForm.splitRule || undefined,
    );
    const data = res.data.data;
    ElMessage.success(`导入成功！共 ${data.chapters_count} 章`);

    if (!novelId.value && data.novel_id) {
      router.replace(`/author/editor/${data.novel_id}`);
      return;
    }

    if (novelId.value) {
      const chaptersRes = await chapterApi.getChapters(novelId.value);
      chapters.value = chaptersRes.data.data || [];
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '导入失败');
  } finally {
    importing.value = false;
    showImportDialog.value = false;
    resetImport();
  }
}

async function loadCategories() {
  try {
    const res = await publicApi.getCategories();
    if (res.data.code === 0 && Array.isArray(res.data.data)) {
      categories.value = res.data.data;
    }
  } catch (e) { /* ignore */ }
}

onMounted(() => {
  loadCategories();
  loadSensitiveZones();
  if (isEditing) loadNovel();
});
</script>

<style scoped>
.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.editor-header h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
}

.header-actions {
  display: flex;
  gap: 12px;
}

.editor-section {
  margin-bottom: 24px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-actions {
  display: flex;
  gap: 8px;
}

.chapter-item {
  display: flex;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid var(--border-color, #eee);
}

.chapter-item:last-child {
  border-bottom: none;
}

.ch-num {
  color: var(--text-light);
  min-width: 80px;
}

.ch-title {
  flex: 1;
  font-weight: 500;
}

.ch-words {
  color: var(--text-light);
  font-size: 0.85rem;
  margin-right: 16px;
}

/* 导入预览 */
.import-preview {
  max-height: 60vh;
  overflow-y: auto;
}

.import-preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color, #eee);
}

.import-rule-hint {
  margin-top: 4px;
  font-size: 0.8rem;
  color: var(--text-light, #999);
}

.import-chapter-list {
  max-height: 300px;
  overflow-y: auto;
}

.import-chapter-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid var(--border-color, #eee);
}

.import-ch-num {
  font-weight: 600;
  min-width: 28px;
  color: var(--primary-color);
  text-align: center;
}

.import-ch-words {
  font-size: 0.8rem;
  color: var(--text-light, #999);  white-space: nowrap;
}

/* ====== 移动端作品编辑器适配 ====== */
@media (max-width: 768px) {
  .editor-header {
    flex-wrap: wrap;
    gap: 10px;
  }
  .editor-header h1 {
    font-size: 1.25rem;
  }
  .header-actions {
    width: 100%;
    justify-content: flex-end;
  }
  /* 表单栅格单列 */
  .editor-section .el-row > .el-col {
    width: 100% !important;
    flex: 0 0 100% !important;
    margin-bottom: 6px;
  }
  .section-header {
    flex-wrap: wrap;
    gap: 10px;
  }
  .section-actions {
    flex-wrap: wrap;
    width: 100%;
  }
  .section-actions .el-button {
    flex: 1;
    margin-left: 0 !important;
  }
  /* 章节行：压缩为可换行布局 */
  .chapter-item {
    flex-wrap: wrap;
    padding: 10px 0;
  }
  .ch-num {
    min-width: 0;
    font-size: 0.85rem;
  }
  .ch-words {
    margin-right: auto;
  }
  .ch-title {
    order: 3;
    width: 100%;
    flex: none;
    padding-left: 0;
  }
  /* 导入预览列表横滚 */
  .import-chapter-list {
    max-height: 260px;
  }
}
</style>