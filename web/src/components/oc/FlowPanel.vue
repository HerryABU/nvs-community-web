<template>
  <div class="ofp-wrap" v-loading="loading">
    <div class="ofp-bar">
      <el-select v-model="flowId" placeholder="选择对话流" style="width: 220px" @change="loadTurns">
        <el-option v-for="f in flows" :key="f.id" :label="`${f.title || '未命名'}（${f.turn_count}/${f.max_turns}）`" :value="f.id" />
      </el-select>
      <el-button size="small" type="primary" @click="createFlow">新建对话流</el-button>
      <template v-if="current">
        <el-button size="small" :loading="stepping" :disabled="current.status !== 'running'" @click="step">推进一轮</el-button>
        <el-button v-if="current.status === 'running'" size="small" @click="setStatus('paused')">暂停</el-button>
        <el-button v-else-if="current.status === 'paused'" size="small" @click="setStatus('running')">继续</el-button>
        <el-button size="small" type="danger" plain @click="setStatus('stopped')">结束</el-button>
        <el-button size="small" @click="auditWindow">审核窗口</el-button>
        <el-button size="small" type="success" @click="publishDlg = true">发布为话本小说</el-button>
      </template>
      <el-tag v-if="current" size="small" :type="statusType(current.status)">{{ statusLabel(current.status) }}</el-tag>
      <span v-if="current?.stop_reason" class="ofp-tip">{{ current.stop_reason }}</span>
    </div>

    <!-- 轮次气泡流 -->
    <div class="ofp-turns">
      <div v-for="t in turns" :key="t.id" class="ofp-turn" :class="{ mine: !t.is_ai }">
        <div class="ofp-meta">
          <span class="ofp-name">{{ t.character_name || (t.is_ai ? 'AI 角色' : '我') }}</span>
          <el-tag v-if="t.is_ai" size="small" type="info">AI · {{ t.model_name || '模型' }}</el-tag>
          <el-tag v-if="t.emotion" size="small" type="warning">[{{ t.emotion }}]</el-tag>
          <el-tag v-if="t.review !== 'approved'" size="small" type="danger">{{ t.review === 'pending' ? '待复核' : '已拦截' }}</el-tag>
          <span class="ofp-seq">#{{ t.seq }}</span>
        </div>
        <div class="ofp-text">{{ t.content }}</div>
      </div>
      <el-empty v-if="!loading && turns.length === 0" description="还没有发言，先「推进一轮」" :image-size="60" />
    </div>

    <!-- 真人以角色身份发言 -->
    <div v-if="current && current.status === 'running'" class="ofp-input">
      <el-select v-model="humanCharId" placeholder="选择我的角色" size="small" style="width: 180px" clearable>
        <el-option v-for="c in myChars" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-input v-model="humanText" placeholder="以角色身份说一句…" size="small" @keyup.enter="sendHuman" />
      <el-button size="small" type="primary" :loading="sending" @click="sendHuman">发言</el-button>
    </div>

    <!-- 发布 -->
    <el-dialog v-model="publishDlg" title="发布为话本小说" width="520px">
      <el-alert type="info" :closable="false" class="ofp-alert"
        title="一键导入 + 提交审核（不是绕过审核）：内容会过违禁词，AI 参与自动打标，并记录来源对话流。" />
      <el-form label-width="104px" size="default">
        <el-form-item label="目标作品">
          <el-input v-model.number="pubForm.novel_id" placeholder="留 0 = 新建作品" />
        </el-form-item>
        <el-form-item v-if="!pubForm.novel_id" label="作品标题">
          <el-input v-model="pubForm.title" placeholder="如：雨夜对话" />
        </el-form-item>
        <el-form-item label="正文形态">
          <el-radio-group v-model="pubForm.style">
            <el-radio value="dialogue">对话体（话本气泡）</el-radio>
            <el-radio value="narrative">叙述体</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="轮次范围">
          <el-input-number v-model="pubForm.from_seq" :min="0" /> -
          <el-input-number v-model="pubForm.to_seq" :min="0" />
          <span class="ofp-tip" style="margin-left:8px">0 = 全部</span>
        </el-form-item>
        <el-form-item label="生成设定集"><el-switch v-model="pubForm.include_settings" /></el-form-item>
        <el-form-item label="AI 润色">
          <el-switch v-model="pubForm.polish" />
          <span class="ofp-tip" style="margin-left:8px">消耗代币，受预算约束</span>
        </el-form-item>
        <el-form-item label="立即发布"><el-switch v-model="pubForm.publish_now" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="publishDlg = false">取消</el-button>
        <el-button type="primary" :loading="publishing" @click="doPublish">发布</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ocApi, type OcFlow, type OcTurn, type OcCharacter } from '@/api/oc';

const props = defineProps<{ groupId: number }>();
const router = useRouter();

const loading = ref(false);
const stepping = ref(false);
const sending = ref(false);
const publishing = ref(false);

const flows = ref<OcFlow[]>([]);
const flowId = ref<number>(0);
const current = ref<OcFlow | null>(null);
const turns = ref<OcTurn[]>([]);
const myChars = ref<OcCharacter[]>([]);
const humanCharId = ref<number>(0);
const humanText = ref('');

const publishDlg = ref(false);
const pubForm = reactive({
  novel_id: 0, title: '', style: 'dialogue', from_seq: 0, to_seq: 0,
  include_settings: true, polish: false, publish_now: false,
});

function statusLabel(s: string) {
  return s === 'running' ? '进行中' : s === 'paused' ? '已暂停' : '已结束';
}
function statusType(s: string): 'success' | 'warning' | 'info' {
  return s === 'running' ? 'success' : s === 'paused' ? 'warning' : 'info';
}

async function loadFlows() {
  loading.value = true;
  try {
    const res = await ocApi.listFlows(props.groupId);
    flows.value = res.data.data?.list || [];
    if (!flowId.value && flows.value.length > 0) {
      flowId.value = flows.value[0].id;
      await loadTurns();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载对话流失败');
  } finally {
    loading.value = false;
  }
}

async function loadTurns() {
  current.value = flows.value.find((f) => f.id === flowId.value) || null;
  if (!flowId.value) return;
  loading.value = true;
  try {
    const res = await ocApi.listTurns(flowId.value, 0, 200);
    turns.value = res.data.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载轮次失败');
  } finally {
    loading.value = false;
  }
}

async function createFlow() {
  try {
    const { value } = await ElMessageBox.prompt('对话流标题', '新建对话流', { inputValue: '新的剧情线' });
    const res: any = await ocApi.createFlow(props.groupId, { title: value, schedule: 'round_robin' });
    ElMessage.success('已创建');
    await loadFlows();
    flowId.value = res?.data?.data?.id || flowId.value;
    await loadTurns();
  } catch { /* 取消 */ }
}

async function step() {
  stepping.value = true;
  try {
    const res = await ocApi.step(flowId.value);
    const d = res.data.data;
    if (d?.stopped) {
      ElMessage.warning(d.reason || '对话流已停止');
    } else {
      ElMessage.success('已生成一轮');
    }
    await loadFlows();
    await loadTurns();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '推进失败');
  } finally {
    stepping.value = false;
  }
}

async function setStatus(status: string) {
  try {
    await ocApi.setStatus(flowId.value, status);
    ElMessage.success('已更新');
    await loadFlows();
    await loadTurns();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function auditWindow() {
  try {
    const res: any = await ocApi.audit(flowId.value, 0);
    const w = res?.data?.data?.window;
    ElMessage[w?.result === 'hit' ? 'warning' : 'success'](
      w ? `窗口 v${w.from_seq}~v${w.to_seq}：${w.result === 'hit' ? '命中，已标记待复核' : '通过'}` : '已审核',
    );
    await loadTurns();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '审核失败');
  }
}

async function sendHuman() {
  const text = humanText.value.trim();
  if (!text) return;
  sending.value = true;
  try {
    await ocApi.humanTurn(flowId.value, { character_id: humanCharId.value || 0, content: text });
    humanText.value = '';
    await loadTurns();
    await loadFlows();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发言失败');
  } finally {
    sending.value = false;
  }
}

async function doPublish() {
  publishing.value = true;
  try {
    const res: any = await ocApi.publish(flowId.value, { ...pubForm });
    const d = res?.data?.data;
    ElMessage.success(`已发布：${d?.chapter_count || 0} 章 / ${d?.words || 0} 字`);
    publishDlg.value = false;
    if (d?.novel_id) router.push(`/novel/${d.novel_id}`);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发布失败');
  } finally {
    publishing.value = false;
  }
}

onMounted(async () => {
  await loadFlows();
  try {
    const res = await ocApi.listCharacters({ scope: 'mine' });
    myChars.value = res.data.data?.list || [];
  } catch { /* 无角色卡时仅提示 */ }
});
</script>

<style scoped>
.ofp-bar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
.ofp-tip { font-size: 0.75rem; color: var(--text-muted); }
.ofp-turns {
  display: flex; flex-direction: column; gap: 10px; max-height: 46vh; overflow-y: auto;
  padding: 10px; border: 1px solid var(--border-color); border-radius: 10px; background: var(--bg-soft);
}
.ofp-turn { max-width: 82%; }
.ofp-turn.mine { margin-left: auto; text-align: right; }
.ofp-meta { display: flex; align-items: center; gap: 6px; font-size: 0.75rem; color: var(--text-muted); margin-bottom: 3px; }
.ofp-turn.mine .ofp-meta { justify-content: flex-end; }
.ofp-name { font-weight: 600; color: var(--text-color); }
.ofp-seq { opacity: 0.6; }
.ofp-text {
  display: inline-block; text-align: left; padding: 8px 12px; border-radius: 12px;
  background: var(--bg-card); border: 1px solid var(--border-color); line-height: 1.8; font-size: 0.92rem;
  white-space: pre-wrap; word-break: break-word;
}
.ofp-turn.mine .ofp-text { background: var(--el-color-primary-light-9); }
.ofp-input { display: flex; gap: 8px; margin-top: 10px; }
.ofp-alert { margin-bottom: 12px; }
</style>
