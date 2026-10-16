import api from './index';

export interface WeChatAuthUrlResp {
  // 微信开放平台 PC 扫码授权 URL（前端据此渲染二维码或直接跳转）
  url: string;
  // 一次性防 CSRF state（服务端存储，TTL 5 分钟，回调时一次性消费）
  state: string;
  // state 用途：login=扫码登录/自动注册；bind=已登录用户绑定微信
  purpose: 'login' | 'bind';
}

export interface WeChatBindStatusResp {
  // 当前账号是否已绑定微信
  bound: boolean;
  // 账号昵称（微信注册用户的昵称取自微信昵称）
  nickname: string;
  // 账号头像
  avatar: string;
}

export const wechatApi = {
  // 获取微信授权 URL（含一次性 state）。
  // purpose: 'login'（默认，登录页二维码用）| 'bind'（账号绑定页用）。
  getAuthUrl(purpose: 'login' | 'bind' = 'login') {
    return api.get<{ code: number; message: string; data: WeChatAuthUrlResp }>(
      '/auth/wechat/url',
      { params: { purpose } }
    );
  },

  // 查询当前账号的微信绑定状态（需登录）
  getBindStatus() {
    return api.get<{ code: number; message: string; data: WeChatBindStatusResp }>(
      '/auth/wechat/bind-status'
    );
  },

  // 绑定微信（需登录）。
  //
  // 【两段式绑定流程说明】
  // 微信授权码 code 只能用一次、且与 state 配对，因此绑定分两段：
  //   第一段（拿授权）：前端调用 getAuthUrl('bind') 得到 { url, state }，
  //     把 state 存入 sessionStorage，再 window.location.href = url 跳转微信授权页；
  //     为防止刷新/重进丢失，建议同时记录到 sessionStorage 的 key 形如 'nvs:wechat:bind:state'。
  //   第二段（完成绑定）：用户在手机微信点确认后，微信把浏览器 302 到后端
  //     /api/auth/wechat/callback?code&state；后端识别 purpose=bind 且请求携带登录态时，
  //     直接把微信写入当前账号，并 302 回前端 /user/wechat-bind?wechat=bound。
  //     因此常规绑定无需前端调用本接口。
  //   本接口（POST /auth/wechat/bind）面向自行实现弹窗/iframe 授权的前端场景：
  //     从授权回调地址自行取回 code+state 后调用，后端校验 state 并完成绑定。
  bindWeChat(code: string, state: string) {
    return api.post<{ code: number; message: string }>('/auth/wechat/bind', { code, state });
  },

  // 解绑微信（需登录；清空 openid/unionid，之后可重新绑定）
  unbindWeChat() {
    return api.post<{ code: number; message: string }>('/auth/wechat/unbind');
  },
};
