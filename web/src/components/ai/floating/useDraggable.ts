import { ref, type Ref } from 'vue';

/**
 * 通用拖拽合成：把某个「拖拽手柄」上的指针位移，映射到目标浮层的 left/top。
 *
 * 用法：
 *   const { pos, onHandleDown } = useDraggable(initialLeft, initialRightOrTop)
 *   <div class="layer" :style="{ left: pos.x + 'px', top: pos.y + 'px' }">
 *     <div class="layer__handle" @pointerdown="onHandleDown">…</div>
 *   </div>
 *
 * 设计要点：
 * - 用 pointer 事件 + setPointerCapture，拖拽过程中指针移出手柄也不丢。
 * - 仅相对位移，不依赖 initial 时的布局，避免抖动。
 * - 边界收敛在 [0, viewport]，保证浮层始终可见可抓。
 */
export function useDraggable(initialX = 120, initialY = 120) {
  const x = ref(initialX);
  const y = ref(initialY);
  const dragging = ref(false);

  let startX = 0;
  let startY = 0;
  let originX = 0;
  let originY = 0;
  let activePointer: number | null = null;

  function clamp(v: number, min: number, max: number) {
    return Math.max(min, Math.min(max, v));
  }

  function onMove(e: PointerEvent) {
    if (!dragging.value || e.pointerId !== activePointer) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    const maxX = window.innerWidth - 80;
    const maxY = window.innerHeight - 48;
    x.value = clamp(originX + dx, 0, Math.max(0, maxX));
    y.value = clamp(originY + dy, 0, Math.max(0, maxY));
  }

  function onUp(e: PointerEvent) {
    if (e.pointerId !== activePointer) return;
    dragging.value = false;
    activePointer = null;
    window.removeEventListener('pointermove', onMove);
    window.removeEventListener('pointerup', onUp);
  }

  function onHandleDown(e: PointerEvent) {
    // 只认主键（左键 / 触控），避免右键菜单干扰
    if (e.button !== 0) return;
    e.preventDefault();
    dragging.value = true;
    activePointer = e.pointerId;
    startX = e.clientX;
    startY = e.clientY;
    originX = x.value;
    originY = y.value;
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  }

  function setPos(nx: number, ny: number) {
    x.value = nx;
    y.value = ny;
  }

  return { x, y, dragging, onHandleDown, setPos };
}

export type DraggablePos = { x: Ref<number>; y: Ref<number> };
