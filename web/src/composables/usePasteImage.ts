import { onMounted, onScopeDispose, unref, type Ref } from 'vue';

// ==================== 剪贴板图片上传（Ctrl+V） ====================
// 只识别 clipboardData 里的 image/*，不影响输入框里的普通文字粘贴；
// 拿到 File 后交给调用方走各自的上传接口，composable 本身不依赖任何 API。

export interface PasteImageOptions {
  /** 是否启用（可用 ref/computed 动态控制，例如弹窗打开期间） */
  enabled?: boolean | Ref<boolean>;
  /** 是否允许一次粘贴多张 */
  multiple?: boolean;
  /** 单张大小上限（MB） */
  maxSizeMB?: number;
  /** 监听目标，缺省 window */
  target?: HTMLElement | null | Ref<HTMLElement | null>;
  /** 收到图片文件；返回 Promise 时自动串行等待（非 multiple 场景） */
  onImage: (file: File) => void | Promise<void>;
  onError?: (message: string) => void;
}

export function usePasteImage(options: PasteImageOptions) {
  let listeningTarget: HTMLElement | Window | null = null;

  function onPaste(e: ClipboardEvent) {
    if (!unref(options.enabled ?? true)) return;
    const items = Array.from(e.clipboardData?.items || []);
    const imageItems = items.filter((it) => it.type.startsWith('image/'));
    if (!imageItems.length) return; // 普通文字粘贴不拦截

    let handled = false;
    for (const item of imageItems) {
      const file = item.getAsFile();
      if (!file) continue;
      const max = options.maxSizeMB;
      if (max && file.size > max * 1024 * 1024) {
        options.onError?.(`粘贴的图片超过 ${max}MB，已跳过`);
        continue;
      }
      handled = true;
      void options.onImage(file);
      if (!options.multiple) break;
    }
    if (handled) e.preventDefault();
  }

  onMounted(() => {
    listeningTarget = (unref(options.target) as HTMLElement | null) || window;
    listeningTarget.addEventListener('paste', onPaste as EventListener);
  });
  onScopeDispose(() => {
    listeningTarget?.removeEventListener('paste', onPaste as EventListener);
    listeningTarget = null;
  });
}
