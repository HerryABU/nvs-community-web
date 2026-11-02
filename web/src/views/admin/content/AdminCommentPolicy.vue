<template>
  <div class="admin-page">
    <AdminHeader
      title="评论区策略"
      subtitle="楼加密的连带范围：只锁楼正文，还是连楼内回复一起藏"
      :icon="ChatLineRound"
    />
    <div class="admin-page-body">
      <div class="admin-panel" v-loading="loading">
        <SectionHeader title="楼加密" subtitle="加密的是「我这一条说了什么」，还是「大家聊了什么」" :icon="'🔒'" gradient="amber" />

        <el-alert type="info" :closable="false" show-icon class="policy-warn">
          <template #title>两种语义的区别</template>
          <div class="policy-explain">
            <p>
              <b>只锁楼正文（推荐）</b>：未解锁者看到 🔒 占位，但该楼下的
              <b>楼内回复（评论区）照常可见可读</b>。加密的是楼主自己的发言，别人对他的讨论不受影响 ——
              否则会出现「回复者本人说的话，作者自己却看不见」的荒唐局面。
            </p>
            <p>
              <b>连带隐藏评论区</b>：未解锁时，整块对话（含全部楼内回复）对外完全不可见。
              适合把楼当作「一次私密定向发言」的场景。
            </p>
          </div>
        </el-alert>

        <el-form label-width="180px" class="policy-form">
          <el-form-item label="楼加密连带隐藏楼内回复">
            <el-switch v-model="form.lock_hides_replies" :disabled="saving" />
            <span class="form-tip">
              关闭 = 评论区不加密（楼内回复照常可见）；开启 = 整块对话对外不可见
            </span>
          </el-form-item>

          <el-form-item label="允许对未解锁的楼回复">
            <el-switch v-model="form.locked_floor_can_reply" :disabled="saving" />
            <span class="form-tip">
              仅在「评论区不加密」时生效。开启后未解锁用户也能回复未解锁的楼
              （此时楼内回复本就公开，不存在试探加密内容的通道）；关闭则该楼只读
            </span>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="saving" @click="save">保存配置（立即生效）</el-button>
            <el-button @click="reload" :disabled="saving">重置为已保存</el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { ChatLineRound } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import api from '@/api';

export interface CommentPolicy {
  lock_hides_replies: boolean;
  locked_floor_can_reply: boolean;
}

const loading = ref(false);
const saving = ref(false);
const form = reactive<CommentPolicy>({
  lock_hides_replies: false,
  locked_floor_can_reply: false,
});

async function reload() {
  loading.value = true;
  try {
    const r = await api.get<{ config: CommentPolicy }>('/admin/comment-policy');
    const cfg = r.data?.data?.config;
    if (cfg) {
      form.lock_hides_replies = !!cfg.lock_hides_replies;
      form.locked_floor_can_reply = !!cfg.locked_floor_can_reply;
    }
  } catch {
    ElMessage.error('读取评论区策略失败');
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    await api.put('/admin/comment-policy', {
      lock_hides_replies: form.lock_hides_replies,
      locked_floor_can_reply: form.locked_floor_can_reply,
    });
    ElMessage.success('已保存，立即生效');
    await reload();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

onMounted(reload);
</script>

<style scoped>
.policy-warn {
  margin-bottom: 16px;
}
.policy-explain p {
  margin: 4px 0;
  line-height: 1.7;
}
.policy-form {
  max-width: 720px;
}
.form-tip {
  margin-left: 12px;
  font-size: 12px;
  color: var(--text-light);
  line-height: 1.5;
}
</style>
