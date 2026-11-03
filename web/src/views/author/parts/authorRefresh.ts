import { ref } from 'vue';

/**
 * 作者工作台各独立视图之间的「数据刷新信号」。
 *
 * 背景：工作台被拆成多个独立视图（?tab= 切换）后，视图之间不再共享状态。
 * 例如「回收站」里恢复了一篇小说，「我的作品」视图必须跟着刷新；
 * 又如从编辑器返回工作台，各视图都应重新拉取。
 *
 * 用一个模块级计数器做广播：谁改了数据就 bump 一次，各视图 watch 到变化后自行重载。
 */
export const authorDataVersion = ref(0);

export function bumpAuthorData() {
  authorDataVersion.value++;
}
