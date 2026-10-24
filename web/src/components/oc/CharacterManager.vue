<template>
  <div class="ocm-wrap" v-loading="loading">
    <div class="ocm-bar">
      <el-radio-group v-model="scope" size="small" @change="load">
        <el-radio-button value="mine">我的角色</el-radio-button>
        <el-radio-button value="public">角色库</el-radio-button>
      </el-radio-group>
      <el-input v-model="keyword" placeholder="搜索角色名" size="small" style="width: 180px" clearable @change="load" />
      <el-button type="primary" size="small" @click="openEditor()">新建角色卡</el-button>
    </div>

    <div class="ocm-grid">
      <div v-for="c in list" :key="c.id" class="ocm-card">
        <div class="ocm-head">
          <el-avatar :size="42" :src="c.avatar_url || ''">{{ (c.name || '?')[0] }}</el-avatar>
          <div class="ocm-title">
            <div class="ocm-name">{{ c.name }}</div>
            <div class="ocm-sub">
              <el-tag v-if="c.visibility === 'public'" size="small" :type="c.review === 'approved' ? 'success' : 'warning'">
                {{ c.review === 'approved' ? '已公开' : '待审核' }}
              </el-tag>
              <span v-else class="ocm-tip">私有</span>
            </div>
          </div>
        </div>
        <div class="ocm-persona">{{ c.persona || '（未填人设）' }}</div>
        <div class="ocm-state">
          <span>情绪 {{ c.mood }}</span><span>好感 {{ c.affinity }}</span><span>精力 {{ c.energy }}</span>
        </div>
        <div class="ocm-actions">
          <el-button size="small" text type="primary" @click="openEditor(c)">编辑</el-button>
          <el-button v-if="canBind" size="small" text @click="bindAgent(c)">加入本群 AI</el-button>
          <el-popconfirm title="确认删除该角色卡？（保留记录以备追溯）" @confirm="remove(c)">
            <template #reference><el-button size="small" text type="danger">删除</el-button></template>
          </el-popconfirm>
        </div>
      </div>
      <el-empty v-if="!loading && list.length === 0" description="还没有角色卡" :image-size="60" />
    </div>

    <!-- 编辑角色卡 -->
    <el-dialog v-model="editorVisible" :title="form.id ? '编辑角色卡' : '新建角色卡'" width="760px" top="6vh">
      <el-form label-width="96px" size="default">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="角色名"><el-input v-model="form.name" maxlength="40" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="头像 URL"><el-input v-model="form.avatar_url" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="人设">
          <el-input v-model="form.persona" type="textarea" :rows="3" placeholder="身份/性格/背景（常驻注入）" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="外貌"><el-input v-model="form.appearance" type="textarea" :rows="2" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="说话风格"><el-input v-model="form.speech" type="textarea" :rows="2" placeholder="口吻/口头禅/句长" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="场景">
          <el-input v-model="form.scenario" type="textarea" :rows="2" placeholder="当前场景与冲突（scenario）" />
        </el-form-item>
        <el-form-item label="开场白">
          <el-input v-model="form.first_message" type="textarea" :rows="2" placeholder="first_mes：角色说的第一句" />
        </el-form-item>
        <el-form-item label="对话示例">
          <el-input v-model="form.mes_example" type="textarea" :rows="3" placeholder="mes_example：示范说话风格（最强的风格锚点）" />
        </el-form-item>
        <el-form-item label="系统提示">
          <el-input v-model="form.system_prompt" type="textarea" :rows="2" placeholder="system_prompt（可选）" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="自带世界书">
              <el-select v-model="form.book_id" clearable placeholder="无">
                <el-option v-for="b in books" :key="b.id" :label="b.title" :value="b.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="可见性">
              <el-radio-group v-model="form.visibility">
                <el-radio value="private">私有</el-radio>
                <el-radio value="public">公开（需审核）</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="标签">
              <el-input v-model="form.tags" placeholder="逗号分隔" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="表情立绘">
          <el-input v-model="form.emote_map" placeholder="情绪:图片URL, 笑:/a.png, 哭:/b.png" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editorVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue';
import { ElMessage } from 'element-plus';
import { ocApi, type OcCharacter, type OcWorldBook } from '@/api/oc';

const props = defineProps<{ groupId: number; canBind: boolean }>();
const emit = defineEmits<{ bound: [] }>();

const loading = ref(false);
const saving = ref(false);
const scope = ref<'mine' | 'public'>('mine');
const keyword = ref('');
const list = ref<OcCharacter[]>([]);
const books = ref<OcWorldBook[]>([]);

const editorVisible = ref(false);
const form = reactive<Partial<OcCharacter>>({
  name: '', avatar_url: '', persona: '', appearance: '', speech: '',
  scenario: '', first_message: '', mes_example: '', system_prompt: '',
  book_id: 0, tags: '', emote_map: '', visibility: 'private',
});

async function load() {
  loading.value = true;
  try {
    const res = await ocApi.listCharacters({ scope: scope.value, keyword: keyword.value });
    list.value = res.data.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function loadBooks() {
  try {
    const res = await ocApi.listBooks({ scope: 'mine' });
    books.value = res.data.data?.list || [];
  } catch { /* 世界书不可用不影响角色编辑 */ }
}

function openEditor(c?: OcCharacter) {
  Object.assign(form, c
    ? { ...c }
    : {
        id: undefined, name: '', avatar_url: '', persona: '', appearance: '', speech: '',
        scenario: '', first_message: '', mes_example: '', system_prompt: '',
        book_id: 0, tags: '', emote_map: '', visibility: 'private',
      });
  editorVisible.value = true;
}

async function save() {
  if (!form.name?.trim()) {
    ElMessage.warning('请填写角色名');
    return;
  }
  saving.value = true;
  try {
    if (form.id) await ocApi.updateCharacter(form.id, form);
    else await ocApi.createCharacter(form);
    ElMessage.success('已保存');
    editorVisible.value = false;
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function remove(c: OcCharacter) {
  try {
    await ocApi.deleteCharacter(c.id);
    ElMessage.success('已删除');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function bindAgent(c: OcCharacter) {
  if (!props.groupId) return;
  try {
    await ocApi.addAgent(props.groupId, { character_id: c.id });
    ElMessage.success(`已把「${c.name}」加入本群 AI 角色`);
    emit('bound');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  }
}

onMounted(() => { load(); loadBooks(); });
</script>

<style scoped>
.ocm-bar {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 12px;
}
.ocm-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 12px;
}
.ocm-card {
  border: 1px solid var(--border-color); border-radius: 10px; padding: 12px; background: var(--bg-card);
  display: flex; flex-direction: column; gap: 8px;
}
.ocm-head { display: flex; gap: 10px; align-items: center; }
.ocm-name { font-weight: 600; }
.ocm-sub { margin-top: 2px; }
.ocm-tip { font-size: 0.75rem; color: var(--text-muted); }
.ocm-persona {
  font-size: 0.82rem; color: var(--text-muted); line-height: 1.6;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}
.ocm-state { display: flex; gap: 10px; font-size: 0.75rem; color: var(--text-muted); }
.ocm-actions { display: flex; gap: 4px; flex-wrap: wrap; border-top: 1px solid var(--border-color); padding-top: 6px; }
</style>
