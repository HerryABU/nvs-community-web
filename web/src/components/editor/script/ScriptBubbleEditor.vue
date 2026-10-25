<template>
  <div class="sbe">
    <!-- 工具条 -->
    <div class="sbe-toolbar">
      <el-select
        v-model="pickCharacter"
        placeholder="选角色"
        size="small"
        style="width: 140px"
        filterable
        allow-create
        default-first-option
      >
        <el-option v-for="c in characters" :key="c.name" :label="c.name" :value="c.name" />
      </el-select>
      <el-button size="small" type="primary" @click="insertDialogue">插入气泡</el-button>
      <el-button size="small" @click="insertStage">场景提示</el-button>
      <el-button size="small" @click="insertNarration">旁白</el-button>
      <el-button size="small" @click="charManagerVisible = true">角色管理</el-button>
      <span class="muted small sbe-stats">
        {{ dialogueCount }} 气泡 · {{ characters.length }} 角色 · {{ charCount }} 字
      </span>
    </div>

    <div class="sbe-main">
      <!-- 角色栏 -->
      <aside class="sbe-rail">
        <div class="sbe-rail-title">角色</div>
        <button
          v-for="c in characters"
          :key="c.name"
          type="button"
          class="sbe-char"
          :class="[c.side === 'right' ? 'sbe-char-right' : 'sbe-char-left', { 'sbe-char-guest': !c.registered }]"
          :title="`插入「${c.name}」的气泡`"
          @click="quickInsert(c)"
        >
          <span class="sbe-char-avatar" :style="charAvatarStyle(c)">
            <template v-if="!c.avatar">{{ characterInitial(c.name) }}</template>
          </span>
          <span class="sbe-char-name">{{ c.name }}</span>
        </button>
        <div v-if="!characters.length" class="sbe-rail-empty">
          还没有角色<br />先「插入气泡」写一句，或去「角色管理」建档
        </div>
      </aside>

      <!-- 气泡列表 -->
      <div class="sbe-rows">
        <div v-if="!rows.length" class="sbe-empty">
          <p>这一章还没有气泡。</p>
          <p class="muted small">左侧选个角色点一下就能插入气泡；正文会写成 <code>#角色：台词</code>（左）/ <code>##角色：台词</code>（右）。</p>
        </div>

        <div v-for="(r, i) in rows" :key="r.id" class="sbe-row">
          <!-- 对话气泡 -->
          <div v-if="r.kind === 'dialogue'" class="sbe-bubble" :class="r.side === 'right' ? 'sbe-right' : 'sbe-left'">
            <div class="sbe-bubble-head">
              <span class="sbe-avatar" :style="avatarStyle(r.speaker)">
                <template v-if="!avatarText(r.speaker)">{{ characterInitial(r.speaker) }}</template>
              </span>
              <el-select
                v-model="r.speaker"
                size="small"
                class="sbe-speaker"
                filterable
                allow-create
                default-first-option
                @change="onSpeakerChange(r)"
              >
                <el-option v-for="c in characters" :key="c.name" :label="c.name" :value="c.name" />
              </el-select>
              <el-radio-group v-model="r.side" size="small" @change="emitChange">
                <el-radio-button value="left">左</el-radio-button>
                <el-radio-button value="right">右</el-radio-button>
              </el-radio-group>
              <span class="sbe-row-ops">
                <el-button size="small" text @click="moveRow(i, -1)">上移</el-button>
                <el-button size="small" text @click="moveRow(i, 1)">下移</el-button>
                <el-button size="small" text type="danger" @click="removeRow(i)">删除</el-button>
              </span>
            </div>
            <el-input
              v-model="r.text"
              type="textarea"
              :rows="2"
              placeholder="台词（可换行，换行会并进同一个气泡）"
              @input="emitChange"
            />
          </div>

          <!-- 场景提示 -->
          <div v-else-if="r.kind === 'stage'" class="sbe-block sbe-block-stage">
            <div class="sbe-block-head">
              <span class="sbe-block-tag">场景提示</span>
              <span class="sbe-row-ops">
                <el-button size="small" text @click="moveRow(i, -1)">上移</el-button>
                <el-button size="small" text @click="moveRow(i, 1)">下移</el-button>
                <el-button size="small" text type="danger" @click="removeRow(i)">删除</el-button>
              </span>
            </div>
            <el-input v-model="r.text" placeholder="如：清晨，街角咖啡店" @input="emitChange" />
          </div>

          <!-- 旁白 / 散文段 -->
          <div v-else-if="r.kind === 'narration' || r.kind === 'prose'" class="sbe-block sbe-block-prose">
            <div class="sbe-block-head">
              <span class="sbe-block-tag">{{ r.kind === 'prose' ? '散文段' : '旁白' }}</span>
              <span class="sbe-row-ops">
                <el-button size="small" text @click="moveRow(i, -1)">上移</el-button>
                <el-button size="small" text @click="moveRow(i, 1)">下移</el-button>
                <el-button size="small" text type="danger" @click="removeRow(i)">删除</el-button>
              </span>
            </div>
            <el-input
              v-model="r.text"
              type="textarea"
              :rows="2"
              :placeholder="r.kind === 'prose' ? '散文段落（按 Markdown 渲染）' : '旁白（居中显示）'"
              @input="emitChange"
            />
          </div>

          <!-- 图片气泡 -->
          <div v-else class="sbe-block sbe-block-image">
            <div class="sbe-block-head">
              <span class="sbe-block-tag">图片</span>
              <span class="sbe-row-ops">
                <el-button size="small" text @click="moveRow(i, -1)">上移</el-button>
                <el-button size="small" text @click="moveRow(i, 1)">下移</el-button>
                <el-button size="small" text type="danger" @click="removeRow(i)">删除</el-button>
              </span>
            </div>
            <div class="sbe-image-row">
              <el-input v-model="r.alt" placeholder="说明" style="width: 140px" @input="emitChange" />
              <el-input v-model="r.url" placeholder="图片地址" @input="emitChange" />
            </div>
            <img v-if="r.url" :src="r.url" class="sbe-image-preview" alt="" />
          </div>
        </div>
      </div>
    </div>

    <CharacterManager v-model="charManagerVisible" :novel-id="novelId" @changed="loadCharacters" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import CharacterManager from './CharacterManager.vue';
import {
  parseScriptRows,
  serializeScriptRows,
  mergeCharacterViews,
  makeRow,
  colorForName,
  characterInitial,
} from './useScriptModel';
import type { ScriptCharacterView, ScriptRow } from './types';
import { characterApi, type NovelCharacter } from '@/api/novel/character';

const props = defineProps<{
  modelValue: string;
  mode: 'script' | 'mixed';
  novelId: number;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void;
}>();

const rows = ref<ScriptRow[]>([]);
const registered = ref<NovelCharacter[]>([]);
const charManagerVisible = ref(false);
const pickCharacter = ref('');

const characters = computed<ScriptCharacterView[]>(() => mergeCharacterViews(registered.value, rows.value));
const dialogueCount = computed(() => rows.value.filter((r) => r.kind === 'dialogue').length);
const charCount = computed(() =>
  rows.value.reduce((n, r) => n + (r.text || '').replace(/\s/g, '').length, 0),
);

function charAvatarStyle(c: ScriptCharacterView) {
  if (c.avatar) return { backgroundImage: `url(${c.avatar})` };
  return { background: c.color || colorForName(c.name) };
}
function avatarStyle(name: string) {
  const c = characters.value.find((x) => x.name === name);
  if (c?.avatar) return { backgroundImage: `url(${c.avatar})` };
  return { background: c?.color || colorForName(name) };
}
function avatarText(name: string) {
  const c = characters.value.find((x) => x.name === name);
  return c?.avatar ? '' : characterInitial(name);
}

// ---------- 内容同步 ----------

/** 把气泡行写回源文本（对外唯一出口） */
function emitChange() {
  emit('update:modelValue', serializeScriptRows(rows.value));
}

/** 外部内容变化（切章节 / 传统编辑器改动）→ 重新解析，不回写，避免无谓改写正文 */
watch(
  () => props.modelValue,
  (v) => {
    if (v === serializeScriptRows(rows.value)) return;
    rows.value = parseScriptRows(v, props.mode);
  },
);

/** 形态切换（script ⇄ mixed）→ 按新形态重新分组 */
watch(
  () => props.mode,
  (m) => {
    rows.value = parseScriptRows(serializeScriptRows(rows.value), m);
    emitChange();
  },
);

// ---------- 角色 ----------

async function loadCharacters() {
  if (!props.novelId) return;
  try {
    const res = await characterApi.list(props.novelId);
    if (res.data.code === 0) registered.value = res.data.data?.list || [];
  } catch {
    registered.value = [];
  }
}

// ---------- 行操作 ----------

function pushRow(row: ScriptRow) {
  rows.value.push(row);
  emitChange();
}

function insertDialogue() {
  const name = pickCharacter.value || characters.value[0]?.name || '';
  if (!name) {
    ElMessage.warning('先在左边选一个角色，或去「角色管理」建一个');
    return;
  }
  const c = characters.value.find((x) => x.name === name);
  const row = makeRow('dialogue', { speaker: name, side: c?.side || 'left' });
  rows.value.push(row);
  emitChange();
}

function quickInsert(c: ScriptCharacterView) {
  rows.value.push(makeRow('dialogue', { speaker: c.name, side: c.side }));
  emitChange();
}

function insertStage() {
  pushRow(makeRow('stage'));
}

function insertNarration() {
  pushRow(makeRow(props.mode === 'mixed' ? 'prose' : 'narration'));
}

function removeRow(i: number) {
  rows.value.splice(i, 1);
  emitChange();
}

function moveRow(i: number, dir: -1 | 1) {
  const j = i + dir;
  if (j < 0 || j >= rows.value.length) return;
  const arr = rows.value;
  const tmp = arr[i];
  arr[i] = arr[j];
  arr[j] = tmp;
  emitChange();
}

/** 换角色后，若新角色没登记则用占位色；左右沿用当前气泡 */
function onSpeakerChange(r: ScriptRow) {
  emitChange();
}

onMounted(() => {
  rows.value = parseScriptRows(props.modelValue, props.mode);
  loadCharacters();
});
</script>

<style scoped>
.sbe { display: flex; flex-direction: column; height: 100%; min-height: 420px; }
.sbe-toolbar {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
  padding: 8px 0 10px; border-bottom: 1px solid var(--border-color, #ebeef5);
}
.sbe-stats { margin-left: auto; }
.sbe-main { display: flex; gap: 12px; flex: 1; min-height: 0; padding-top: 10px; }

/* 角色栏 */
.sbe-rail {
  width: 132px; flex-shrink: 0; overflow-y: auto;
  border: 1px solid var(--border-color, #ebeef5); border-radius: 10px; padding: 8px 6px;
  background: var(--bg-card, #fff);
}
.sbe-rail-title { font-size: 0.78rem; color: var(--text-light, #999); margin-bottom: 6px; padding-left: 4px; }
.sbe-rail-empty { font-size: 0.72rem; color: var(--text-light, #aaa); line-height: 1.6; padding: 8px 4px; }
.sbe-char {
  display: flex; align-items: center; gap: 6px; width: 100%;
  border: 1px solid transparent; border-radius: 20px; background: transparent;
  padding: 4px 6px; margin-bottom: 4px; cursor: pointer; text-align: left;
}
.sbe-char:hover { background: var(--el-fill-color-light, #f4f4f5); }
.sbe-char-guest { opacity: 0.75; }
.sbe-char-avatar {
  width: 26px; height: 26px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 0.78rem; font-weight: 600;
  background-size: cover; background-position: center;
}
.sbe-char-name { font-size: 0.8rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* 气泡列表 */
.sbe-rows { flex: 1; min-width: 0; overflow-y: auto; padding-right: 4px; }
.sbe-empty {
  border: 1px dashed var(--border-color, #ddd); border-radius: 10px;
  padding: 28px 16px; text-align: center; color: var(--text-light, #999);
}
.sbe-empty code { background: var(--el-fill-color-light, #f4f4f5); padding: 1px 4px; border-radius: 3px; }
.sbe-row { margin-bottom: 10px; }

.sbe-bubble { border: 1px solid var(--border-color, #ebeef5); border-radius: 12px; padding: 10px 12px; background: var(--bg-card, #fff); }
.sbe-bubble.sbe-right { border-color: var(--el-color-warning-light-5, #fdf6ec); }
.sbe-bubble-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.sbe-avatar {
  width: 28px; height: 28px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 0.8rem; font-weight: 600;
  background-size: cover; background-position: center;
}
.sbe-speaker { width: 120px; }
.sbe-row-ops { margin-left: auto; display: flex; align-items: center; }

.sbe-block { border: 1px dashed var(--border-color, #e0e0e0); border-radius: 10px; padding: 8px 12px; background: var(--bg-card, #fff); }
.sbe-block-head { display: flex; align-items: center; margin-bottom: 6px; }
.sbe-block-tag {
  font-size: 0.72rem; color: var(--text-light, #999);
  background: var(--el-fill-color-light, #f4f4f5); border-radius: 4px; padding: 1px 7px;
}
.sbe-block-stage { border-style: solid; }
.sbe-block-image .sbe-image-row { display: flex; gap: 8px; margin-bottom: 8px; }
.sbe-image-preview { max-width: 100%; max-height: 180px; border-radius: 8px; display: block; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; }
</style>
