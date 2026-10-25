<template>
  <el-drawer
    :model-value="modelValue"
    title="角色管理"
    size="min(520px, 94vw)"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
  >
    <div class="cm-tip">
      角色是<b>作品级</b>的：头像、主题色、默认左右在所有章节通用。
      气泡编辑器插入气泡时可以直接挑角色，不用每次手打名字。
    </div>

    <div class="cm-list" v-loading="loading">
      <div v-if="!loading && !list.length" class="cm-empty">
        还没有角色。点下面「新增角色」建一个，气泡编辑器里就能直接用了。
      </div>
      <div v-for="c in list" :key="c.id" class="cm-item">
        <div
          class="cm-avatar"
          :style="c.avatar ? { backgroundImage: `url(${c.avatar})` } : { background: c.color || colorForName(c.name) }"
        >
          <span v-if="!c.avatar">{{ characterInitial(c.name) }}</span>
        </div>
        <div class="cm-info">
          <div class="cm-name">
            {{ c.name }}
            <el-tag size="small" :type="c.side === 'right' ? 'warning' : 'primary'">
              {{ c.side === 'right' ? '默认右侧' : '默认左侧' }}
            </el-tag>
          </div>
          <div class="cm-intro">{{ c.intro || '（无简介）' }}</div>
        </div>
        <div class="cm-ops">
          <el-button size="small" text type="primary" @click="startEdit(c)">编辑</el-button>
          <el-button size="small" text type="danger" @click="remove(c)">删除</el-button>
        </div>
      </div>
    </div>

    <!-- 新增 / 编辑表单 -->
    <div class="cm-form" v-if="formOpen">
      <div class="cm-form-title">{{ editing ? '编辑角色' : '新增角色' }}</div>
      <el-form label-width="80px" size="small">
        <el-form-item label="角色名" required>
          <el-input v-model="form.name" placeholder="气泡里写作「角色名：台词」" maxlength="32" />
        </el-form-item>
        <el-form-item label="默认左右">
          <el-radio-group v-model="form.side">
            <el-radio-button value="left">左侧 #</el-radio-button>
            <el-radio-button value="right">右侧 ##</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="主题色">
          <el-color-picker v-model="form.color" />
          <span class="muted small">头像底色与气泡描边</span>
        </el-form-item>
        <el-form-item label="头像">
          <div class="cm-avatar-row">
            <div
              class="cm-avatar cm-avatar-sm"
              :style="form.avatar ? { backgroundImage: `url(${form.avatar})` } : { background: form.color || colorForName(form.name) }"
            >
              <span v-if="!form.avatar">{{ characterInitial(form.name) }}</span>
            </div>
            <el-input v-model="form.avatar" placeholder="图片地址（可空）" class="cm-avatar-input" />
            <el-upload :show-file-list="false" :http-request="doUpload" accept="image/*">
              <el-button size="small">上传</el-button>
            </el-upload>
          </div>
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.intro" type="textarea" :rows="2" maxlength="256" placeholder="选填" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
          <el-button @click="formOpen = false">取消</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-button v-else type="primary" plain style="width:100%" @click="startCreate">新增角色</el-button>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { characterApi, type NovelCharacter } from '@/api/novel/character';
import { uploadApi } from '@/api/social';
import { characterInitial, colorForName } from './useScriptModel';

const props = defineProps<{
  modelValue: boolean;
  novelId: number;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void;
  (e: 'changed'): void;
}>();

const list = ref<NovelCharacter[]>([]);
const loading = ref(false);
const saving = ref(false);
const formOpen = ref(false);
const editing = ref<NovelCharacter | null>(null);
const form = reactive({ name: '', avatar: '', color: '', side: 'left', intro: '' });

async function load() {
  if (!props.novelId) return;
  loading.value = true;
  try {
    const res = await characterApi.list(props.novelId);
    if (res.data.code === 0) list.value = res.data.data?.list || [];
  } catch {
    list.value = [];
  } finally {
    loading.value = false;
  }
}

function startCreate() {
  editing.value = null;
  Object.assign(form, { name: '', avatar: '', color: '', side: 'left', intro: '' });
  formOpen.value = true;
}

function startEdit(c: NovelCharacter) {
  editing.value = c;
  Object.assign(form, {
    name: c.name,
    avatar: c.avatar || '',
    color: c.color || '',
    side: c.side === 'right' ? 'right' : 'left',
    intro: c.intro || '',
  });
  formOpen.value = true;
}

async function doUpload(opts: any) {
  try {
    const res = await uploadApi.upload(opts.file);
    if (res.data.code === 0) {
      form.avatar = res.data.data.url;
      ElMessage.success('头像已上传');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '上传失败');
  }
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('角色名不能为空');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      name: form.name.trim(),
      avatar: form.avatar.trim(),
      color: form.color || colorForName(form.name),
      side: form.side,
      intro: form.intro.trim(),
    };
    const res = editing.value
      ? await characterApi.update(props.novelId, editing.value.id, payload)
      : await characterApi.create(props.novelId, payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      formOpen.value = false;
      await load();
      emit('changed');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function remove(c: NovelCharacter) {
  try {
    await ElMessageBox.confirm(
      `删除角色「${c.name}」？已写入正文的台词不会受影响，但该角色将不再是「已登记」角色。`,
      '删除确认',
      { type: 'warning' },
    );
  } catch { return; }
  try {
    const res = await characterApi.remove(props.novelId, c.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await load();
      emit('changed');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

watch(() => props.modelValue, (v) => { if (v) load(); });
onMounted(() => { if (props.modelValue) load(); });
</script>

<style scoped>
.cm-tip {
  font-size: 0.78rem; color: var(--text-light, #777); line-height: 1.6;
  background: var(--el-fill-color-light, #f8f9fa); border-radius: 8px; padding: 8px 12px; margin-bottom: 12px;
}
.cm-list { display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; min-height: 40px; }
.cm-empty { font-size: 0.8rem; color: var(--text-light, #999); padding: 12px; text-align: center; border: 1px dashed var(--border-color, #ddd); border-radius: 8px; }
.cm-item { display: flex; align-items: center; gap: 10px; border: 1px solid var(--border-color, #ebeef5); border-radius: 10px; padding: 8px 10px; }
.cm-avatar {
  width: 36px; height: 36px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-weight: 600; font-size: 0.95rem;
  background-size: cover; background-position: center;
}
.cm-avatar-sm { width: 30px; height: 30px; font-size: 0.85rem; }
.cm-info { flex: 1; min-width: 0; }
.cm-name { font-weight: 600; font-size: 0.88rem; display: flex; align-items: center; gap: 6px; }
.cm-intro { font-size: 0.75rem; color: var(--text-light, #999); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cm-ops { flex-shrink: 0; }
.cm-form { border: 1px solid var(--border-color, #ebeef5); border-radius: 10px; padding: 12px 12px 0; }
.cm-form-title { font-weight: 600; margin-bottom: 8px; }
.cm-avatar-row { display: flex; align-items: center; gap: 8px; width: 100%; }
.cm-avatar-input { flex: 1; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; margin-left: 6px; }
</style>
