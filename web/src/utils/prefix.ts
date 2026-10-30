// 站点路径前缀解析（智能定向，严禁硬编码）：
// 站点可运行在三种挂载形态下，本模块统一向全站输出当前生效的前缀：
//
// 1) 无前缀（默认）：NVS 直接挂在域名根，访问形如 :8080/login —— 前缀为 ''；
// 2) 后端配置 URL_PREFIX（如 .env 设 URL_PREFIX=/go-nvs）：后端在 index.html
//    注入 window.__NVS_PREFIX__='/go-nvs'（权威来源，名字任意配置）；
// 3) 自建反向代理只改写 <base href>（如代理把 HTML 中 <base href="/"> 改写为
//    <base href="/go-nvs/">、转发时剥掉前缀）：后端感知不到前缀、__NVS_PREFIX__
//    为空，此时从 document.baseURI（受 <base> 影响）推导出浏览器实际挂载前缀。
//
// 三种形态自动兼容，无需改动业务代码；前缀名字（/go-nvs 仅为例）可任意。
export function getAppPrefix(): string {
  // 权威来源：后端注入（URL_PREFIX 已配置）
  const injected: unknown = (window as any).__NVS_PREFIX__;
  if (typeof injected === 'string' && injected) {
    return injected;
  }
  // 兜底来源：自建反向代理改写的 <base href> → 取其 href 推导前缀。
  // 关键：必须读 <base> 元素本身，不能读 document.baseURI —— 后者等于「当前页面 URL」，
  // 含路由路径（如 /comment-network），深链接到子路由时会把前缀错算成路由路径，
  // 导致所有 /api 请求前缀错位、整站 API 失效。
  // 如 base href="/go-nvs/" → pathname "/go-nvs" → 前缀 "/go-nvs"；
  // 无 <base> 标签（根挂载且未注入前缀）→ 前缀 ""（而非路由路径）。
  try {
    const baseEl = document.querySelector('base');
    if (baseEl && baseEl.href) {
      const p = new URL(baseEl.href).pathname.replace(/\/+$/, '');
      return p && p !== '/' ? p : '';
    }
  } catch {
    /* 忽略解析异常 */
  }
  return '';
}

// 模块加载即解析一次（脚本位于 base 标签之后执行，document.baseURI 已就绪）
export const appPrefix: string = getAppPrefix();
