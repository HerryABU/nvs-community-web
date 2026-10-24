import { ref } from 'vue';
import api from '@/api';

/**
 * AI 生成内容标识（176 号令合规展示）
 * 后端侧表 ai_content_marks，公开接口只回模型名，不含任何用户内容。
 * 用法：const { label, load } = useAiMark(); await load('blog', id)
 */
export function useAiMark() {
  const label = ref(''); // 空串表示无标记
  const loading = ref(false);

  async function load(type: 'blog' | 'novel_chapter' | 'forum_post' | 'comment', id: number | string) {
    const nid = Number(id);
    if (!nid) return;
    loading.value = true;
    try {
      const res = await api.get<{ code: number; data: { marks: Record<string, string> } }>('/ai/marks', {
        params: { items: `${type}:${nid}` },
      });
      if (res.data.code === 0) {
        const model = res.data.data.marks?.[`${type}:${nid}`];
        label.value = model ? `AI 辅助创作${model ? ` · ${model}` : ''}` : '';
      }
    } catch {
      /* 标识查询失败不应影响正文展示 */
    }
    loading.value = false;
  }

  return { label, loading, load };
}
