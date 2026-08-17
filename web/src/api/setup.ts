import api from './index';

// 建站向导接口（一次性安装）
export const setupApi = {
  // 查询安装状态（公开，不泄露密钥）
  getStatus() {
    return api.get('/setup/status');
  },
  // 环境检查（目录权限 / 端口）
  envCheck() {
    return api.get('/setup/env-check');
  },
  // 测试数据库连通性（Ping）
  dbTest(database: { driver: string; host?: string; port?: string; user?: string; password?: string; name?: string; path?: string }) {
    return api.post('/setup/db-test', { database });
  },
  // 执行安装（需一次性安装密钥）
  install(data: {
    setup_key: string;
    database: { driver: string; host?: string; port?: string; user?: string; password?: string; name?: string; path?: string };
    site_name: string;
    admin: { username: string; email: string; password: string };
  }) {
    return api.post('/setup/install', data);
  },
};
