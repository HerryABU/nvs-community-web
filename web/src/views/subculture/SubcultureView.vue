<template>
  <div class="page-container subc">
    <div class="subc-head">
      <h1 class="subc-title">亚文化创作区</h1>
      <el-tag size="small" type="info">独立创作空间 · 兼容 Furry</el-tag>
      <span class="subc-tip">
        这里与「群组」完全独立：你可以管理自己的 OC 角色卡、把多张角色卡打包成「OC 组（阵容）」，
        并用 AI 助手快速生成 / 补全角色。OC 组可被任意群组的「召唤 OC 组」调用。
      </span>
    </div>

    <el-tabs v-model="tab" class="subc-tabs">
      <!-- ============ 角色库 ============ -->
      <el-tab-pane label="OC 角色库" name="chars">
        <div class="subc-filter">
          <span class="subc-filter-label">按分类筛选：</span>
          <el-radio-group v-model="kindFilter" size="small">
            <el-radio-button value="all">全部</el-radio-button>
            <el-radio-button value="native">原生 OC</el-radio-button>
            <el-radio-button value="furry">Furry 兽人</el-radio-button>
          </el-radio-group>
        </div>
        <CharacterManager :group-id="0" :can-bind="false" :filter-kind="kindFilter" />
      </el-tab-pane>

      <!-- ============ 我的 OC 组 ============ -->
      <el-tab-pane label="我的 OC 组" name="groups">
        <div class="subc-bar">
          <el-radio-group v-model="groupScope" size="small" @change="loadGroups">
            <el-radio-button value="mine">我的阵容</el-radio-button>
            <el-radio-button value="public">公开库</el-radio-button>
          </el-radio-group>
          <el-input v-model="groupKeyword" placeholder="搜索 OC 组" size="small" style="width: 200px" clearable @change="loadGroups" />
          <el-button v-if="groupScope === 'mine'" type="primary" size="small" @click="openGroupEditor()">新建 OC 组</el-button>
        </div>

        <div v-loading="groupLoading" class="subc-glist">
          <div v-for="g in groups" :key="g.id" class="subc-gcard">
            <div class="subc-ghead">
              <el-avatar :size="40" :src="g.cover_url || ''">{{ (g.name || '?')[0] }}</el-avatar>
              <div class="subc-gtitle">
                <div class="subc-gname">{{ g.name }}</div>
                <div class="subc-gsub">
                  <el-tag v-if="g.visibility === 'public'" size="small" :type="g.review === 'approved' ? 'success' : 'warning'">
                    {{ g.review === 'approved' ? '已公开' : '待审核' }}
                  </el-tag>
                  <span v-else class="subc-tip2">私有</span>
                  <span class="subc-tip2">成员 {{ g.member_count }}</span>
                </div>
              </div>
            </div>
            <div class="subc-gsummary">{{ g.summary || '（无简介）' }}</div>
            <div class="subc-gactions">
              <el-button size="small" text type="primary" @click="openMemberManager(g)">管理成员</el-button>
              <template v-if="groupScope === 'mine'">
                <el-button size="small" text @click="openGroupEditor(g)">编辑</el-button>
                <el-popconfirm title="确认删除该 OC 组？（保留记录以备追溯）" @confirm="removeGroup(g)">
                  <template #reference><el-button size="small" text type="danger">删除</el-button></template>
                </el-popconfirm>
              </template>
            </div>
          </div>
          <el-empty v-if="!groupLoading && groups.length === 0" :description="groupScope === 'mine' ? '还没有 OC 组，点「新建 OC 组」把角色打包成阵容' : '公开库暂无内容'" :image-size="60" />
        </div>
      </el-tab-pane>

      <!-- ============ AI 创作助手 ============ -->
      <el-tab-pane label="AI 创作助手" name="ai">
        <div class="subc-ai">
          <el-card class="subc-ai-card" shadow="never">
            <template #header><span class="subc-ai-h">✨ 从灵感生成 OC 角色卡</span></template>
            <el-form label-width="96px" size="default">
              <el-form-item label="核心概念" required>
                <el-input v-model="gen.concept" type="textarea" :rows="2" placeholder="例如：一只社恐的赛博狼，在末世捡破烂维生" />
              </el-form-item>
              <el-form-item label="风格基调">
                <el-input v-model="gen.style_directive" placeholder="如：偏轻松 / 暗黑 / 赛博朋克（可空）" />
              </el-form-item>
              <el-form-item label="OC 分类">
                <el-radio-group v-model="gen.oc_kind">
                  <el-radio value="native">原生 OC</el-radio>
                  <el-radio value="furry">Furry 兽人</el-radio>
                </el-radio-group>
              </el-form-item>
              <template v-if="gen.oc_kind === 'furry'">
                <el-row :gutter="12">
                  <el-col :span="8"><el-form-item label="物种"><el-input v-model="gen.species" placeholder="狼/猫/龙…" /></el-form-item></el-col>
                  <el-col :span="8"><el-form-item label="拟人类型">
                    <el-select v-model="gen.anthro_type" clearable placeholder="可选" style="width:100%">
                      <el-option label="完全兽（feral）" value="feral" />
                      <el-option label="半兽人（anthro）" value="anthro" />
                      <el-option label="完全拟人（full）" value="full" />
                    </el-select>
                  </el-form-item></el-col>
                  <el-col :span="8"><el-form-item label="动物特征"><el-input v-model="gen.species_traits" placeholder="耳朵/尾巴/毛色…" /></el-form-item></el-col>
                </el-row>
              </template>
              <el-form-item label="补充要求">
                <el-input v-model="gen.extra_requirements" type="textarea" :rows="2" placeholder="性别、年龄、背景约束等（可空）" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="genLoading" @click="doGenerate">生成角色卡</el-button>
                <el-button v-if="genResult" @click="goEdit(genResult.id)">去编辑</el-button>
              </el-form-item>
            </el-form>
            <el-alert v-if="genResult" type="success" :closable="false" :title="`已生成「${genResult.name}」`" />
          </el-card>

          <el-card class="subc-ai-card" shadow="never">
            <template #header><span class="subc-ai-h">🔧 补全 / 润色已有角色卡</span></template>
            <el-form label-width="96px" size="default">
              <el-form-item label="选择角色" required>
                <el-select v-model="fill.charId" filterable placeholder="选择我的角色卡" style="width:100%">
                  <el-option v-for="c in myChars" :key="c.id" :label="c.name" :value="c.id" />
                </el-select>
              </el-form-item>
              <el-form-item label="重点字段">
                <el-input v-model="fill.focus" placeholder="如：补全外貌与说话风格（可空）" />
              </el-form-item>
              <el-form-item label="OC 分类">
                <el-radio-group v-model="fill.oc_kind">
                  <el-radio value="">不变</el-radio>
                  <el-radio value="native">原生 OC</el-radio>
                  <el-radio value="furry">Furry 兽人</el-radio>
                </el-radio-group>
              </el-form-item>
              <template v-if="fill.oc_kind === 'furry'">
                <el-row :gutter="12">
                  <el-col :span="8"><el-form-item label="物种"><el-input v-model="fill.species" placeholder="狼/猫/龙…" /></el-form-item></el-col>
                  <el-col :span="8"><el-form-item label="拟人类型">
                    <el-select v-model="fill.anthro_type" clearable placeholder="可选" style="width:100%">
                      <el-option label="完全兽（feral）" value="feral" />
                      <el-option label="半兽人（anthro）" value="anthro" />
                      <el-option label="完全拟人（full）" value="full" />
                    </el-select>
                  </el-form-item></el-col>
                  <el-col :span="8"><el-form-item label="动物特征"><el-input v-model="fill.species_traits" placeholder="耳朵/尾巴/毛色…" /></el-form-item></el-col>
                </el-row>
              </template>
              <el-form-item label="补充要求">
                <el-input v-model="fill.extra_requirements" type="textarea" :rows="2" placeholder="（可空）" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="fillLoading" @click="doFill">补全角色卡</el-button>
              </el-form-item>
            </el-form>
            <el-alert v-if="fillResult" type="success" :closable="false" :title="`已补全「${fillResult.name}」`" />
          </el-card>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 新建 / 编辑 OC 组 -->
    <el-dialog v-model="groupEditorVisible" :title="groupForm.id ? '编辑 OC 组' : '新建 OC 组'" width="520px">
      <el-form label-width="80px" size="default">
        <el-form-item label="名称"><el-input v-model="groupForm.name" maxlength="64" /></el-form-item>
        <el-form-item label="简介"><el-input v-model="groupForm.summary" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="封面 URL"><el-input v-model="groupForm.cover_url" placeholder="封面图链接（可空）" /></el-form-item>
        <el-form-item label="标签"><el-input v-model="groupForm.tags" placeholder="逗号分隔（可空）" /></el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="groupForm.visibility">
            <el-radio value="private">私有</el-radio>
            <el-radio value="public">公开（需审核）</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupEditorVisible = false">取消</el-button>
        <el-button type="primary" :loading="groupSaving" @click="saveGroup">保存</el-button>
      </template>
    </el-dialog>

    <!-- 成员管理 -->
    <el-dialog v-model="memberVisible" :title="`管理成员 · ${activeGroup?.name || ''}`" width="680px" top="5vh">
      <div class="subc-mbar">
        <el-select v-model="addCharId" filterable placeholder="选择要加入的角色卡" style="flex:1" @change="onAddChar">
          <el-option v-for="c in myChars" :key="c.id" :label="c.name + (c.oc_kind === 'furry' ? ' 🐾' : '')" :value="c.id" />
        </el-select>
        <span class="subc-tip2">共 {{ members.length }} 名成员</span>
      </div>
      <el-table :data="members" size="small" empty-text="还没有成员，从上方加入角色卡">
        <el-table-column label="角色" min-width="200">
          <template #default="{ row }">
            <div class="subc-mrow">
              <el-avatar :size="30" :src="row.character?.avatar_url || ''">{{ (row.character?.name || '?')[0] }}</el-avatar>
              <div>
                <div>{{ row.character?.name || '（角色已失效）' }}
                  <el-tag v-if="row.character?.oc_kind === 'furry'" size="small" type="warning">🐾 Furry</el-tag>
                </div>
                <div class="subc-tip2">{{ row.character?.persona?.slice(0, 30) || '' }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" align="right">
          <template #default="{ row, $index }">
            <el-button size="small" text :disabled="$index === 0" @click="moveMember($index, -1)">上移</el-button>
            <el-button size="small" text :disabled="$index === members.length - 1" @click="moveMember($index, 1)">下移</el-button>
            <el-button size="small" text type="danger" @click="removeMember(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import CharacterManager from '@/components/oc/CharacterManager.vue';
import { ocApi, ocGroupApi, ocAiApi, type OcCharacter, type OcGroup, type OcGroupMember } from '@/api/oc';

const router = useRouter();
const tab = ref('chars');

// ===== 角色库 furry 筛选 =====
const kindFilter = ref<'all' | 'native' | 'furry'>('all');

// ===== 我的 OC 组 =====
const groupScope = ref<'mine' | 'public'>('mine');
const groupKeyword = ref('');
const groups = ref<OcGroup[]>([]);
const groupLoading = ref(false);

async function loadGroups() {
  groupLoading.value = true;
  try {
    const res = await ocGroupApi.list({ scope: groupScope.value, keyword: groupKeyword.value });
    groups.value = res.data.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    groupLoading.value = false;
  }
}

const groupEditorVisible = ref(false);
const groupSaving = ref(false);
const groupForm = reactive<Partial<OcGroup>>({ name: '', summary: '', cover_url: '', tags: '', visibility: 'private' });

function openGroupEditor(g?: OcGroup) {
  Object.assign(groupForm, g
    ? { ...g }
    : { id: undefined, name: '', summary: '', cover_url: '', tags: '', visibility: 'private' });
  groupEditorVisible.value = true;
}

async function saveGroup() {
  if (!groupForm.name?.trim()) {
    ElMessage.warning('请填写 OC 组名称');
    return;
  }
  groupSaving.value = true;
  try {
    if (groupForm.id) await ocGroupApi.update(groupForm.id, groupForm);
    else await ocGroupApi.create(groupForm);
    ElMessage.success('已保存');
    groupEditorVisible.value = false;
    await loadGroups();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    groupSaving.value = false;
  }
}

async function removeGroup(g: OcGroup) {
  try {
    await ocGroupApi.remove(g.id);
    ElMessage.success('已删除');
    await loadGroups();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

// ===== 成员管理 =====
const memberVisible = ref(false);
const activeGroup = ref<OcGroup | null>(null);
const members = ref<OcGroupMember[]>([]);
const addCharId = ref<number | undefined>(undefined);
const myChars = ref<OcCharacter[]>([]);

async function ensureMyChars() {
  if (myChars.value.length) return;
  try {
    const res = await ocApi.listCharacters({ scope: 'mine', page_size: 50 });
    myChars.value = res.data.data?.list || [];
  } catch { /* 静默 */ }
}

async function openMemberManager(g: OcGroup) {
  activeGroup.value = g;
  await ensureMyChars();
  await loadMembers(g.id);
  memberVisible.value = true;
}

async function loadMembers(gid: number) {
  try {
    const res = await ocGroupApi.listMembers(gid);
    members.value = res.data.data?.members || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载成员失败');
  }
}

async function onAddChar(cid: number) {
  if (!activeGroup.value || !cid) return;
  try {
    await ocGroupApi.addMember(activeGroup.value.id, cid);
    ElMessage.success('已加入 OC 组');
    await loadMembers(activeGroup.value.id);
    addCharId.value = undefined;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  }
}

async function removeMember(row: OcGroupMember) {
  if (!activeGroup.value) return;
  try {
    await ocGroupApi.removeMember(activeGroup.value.id, row.character_id);
    ElMessage.success('已移出');
    await loadMembers(activeGroup.value.id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '移除失败');
  }
}

async function moveMember(index: number, dir: number) {
  if (!activeGroup.value) return;
  const target = index + dir;
  if (target < 0 || target >= members.value.length) return;
  const arr = members.value.map((m) => m.character_id);
  [arr[index], arr[target]] = [arr[target], arr[index]];
  try {
    await ocGroupApi.reorder(activeGroup.value.id, arr);
    await loadMembers(activeGroup.value.id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '排序失败');
  }
}

// ===== AI 创作助手 =====
const genLoading = ref(false);
const genResult = ref<OcCharacter | null>(null);
const gen = reactive({
  concept: '', style_directive: '', oc_kind: 'native',
  species: '', species_traits: '', anthro_type: '', extra_requirements: '',
});

async function doGenerate() {
  if (!gen.concept.trim()) {
    ElMessage.warning('请填写核心概念/灵感');
    return;
  }
  genLoading.value = true;
  genResult.value = null;
  try {
    const res = await ocAiApi.generate({ ...gen });
    genResult.value = res.data.data;
    ElMessage.success('AI 已生成角色卡');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  } finally {
    genLoading.value = false;
  }
}

const fillLoading = ref(false);
const fillResult = ref<OcCharacter | null>(null);
const fill = reactive({
  charId: undefined as number | undefined, focus: '', oc_kind: '',
  species: '', species_traits: '', anthro_type: '', extra_requirements: '',
});

async function doFill() {
  if (!fill.charId) {
    ElMessage.warning('请选择要补全的角色卡');
    return;
  }
  fillLoading.value = true;
  fillResult.value = null;
  try {
    const res = await ocAiApi.fill(fill.charId, {
      focus: fill.focus, extra_requirements: fill.extra_requirements,
      oc_kind: fill.oc_kind, species: fill.species, species_traits: fill.species_traits, anthro_type: fill.anthro_type,
    });
    fillResult.value = res.data.data;
    ElMessage.success('AI 已补全角色卡');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '补全失败');
  } finally {
    fillLoading.value = false;
  }
}

function goEdit(_id?: number) {
  router.push('/oc');
}

onMounted(() => {
  loadGroups();
});
</script>

<style scoped>
.subc { max-width: 1120px; margin: 0 auto; padding: 20px 16px 48px; }
.subc-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 16px; }
.subc-title { font-size: 1.5rem; margin: 0; }
.subc-tip { font-size: 0.8rem; color: var(--text-muted); flex-basis: 100%; margin-top: 4px; line-height: 1.6; }
.subc-filter { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.subc-filter-label { font-size: 0.85rem; color: var(--text-muted); }
.subc-bar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
.subc-glist { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px; }
.subc-gcard { border: 1px solid var(--border-color); border-radius: 10px; padding: 12px; background: var(--bg-card); display: flex; flex-direction: column; gap: 8px; }
.subc-ghead { display: flex; gap: 10px; align-items: center; }
.subc-gname { font-weight: 600; }
.subc-gsub { margin-top: 2px; display: flex; gap: 8px; align-items: center; }
.subc-tip2 { font-size: 0.75rem; color: var(--text-muted); }
.subc-gsummary { font-size: 0.82rem; color: var(--text-muted); line-height: 1.6; min-height: 36px; }
.subc-gactions { display: flex; gap: 4px; flex-wrap: wrap; border-top: 1px solid var(--border-color); padding-top: 6px; }
.subc-ai { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.subc-ai-h { font-weight: 600; }
.subc-mbar { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.subc-mrow { display: flex; gap: 8px; align-items: center; }
@media (max-width: 860px) { .subc-ai { grid-template-columns: 1fr; } }
</style>
