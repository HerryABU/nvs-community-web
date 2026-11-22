import { onBeforeUnmount, onMounted, ref } from 'vue';

/** 窄屏判断：与 AdminLayout / ChapterEditor 的 768px 断点保持一致。 */
export function useIsMobile(breakpoint = 768) {
  const isMobile = ref(typeof window !== 'undefined' && window.innerWidth < breakpoint);
  const onResize = () => {
    isMobile.value = window.innerWidth < breakpoint;
  };
  onMounted(() => window.addEventListener('resize', onResize, { passive: true }));
  onBeforeUnmount(() => window.removeEventListener('resize', onResize));
  return isMobile;
}
