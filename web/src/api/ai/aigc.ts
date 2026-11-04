import api from '@/api';

/** 后台可配的标识设置（与后端的 models.AigcConfig 对应） */
export interface AigcConfig {
  enabled: boolean;
  label_text: string;
  position: 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left';
  font_size_pct: number;
  text_color: string;
  text_opacity: number;
  shadow_opacity: number;
  content_producer: string;
  reserved_include_model: boolean;
  waiver_enabled: boolean;
  waiver_agreement: string;
  waiver_confirm_text: string;
  prompt_attach_enabled: boolean;
  max_output_pixels: number;
}

/** 前台选项（只决定显示与否，不含后台数据） */
export interface AigcOptions {
  enabled: boolean;
  waiver_enabled: boolean;
  waiver_agreement: string;
  waiver_confirm_text: string;
  prompt_attach_enabled: boolean;
  label_text: string;
  max_output_pixels: number;
}

export interface AigcExportParams {
  asset_id?: number;
  task_id?: number;
  image_index?: number;
  crop_x?: number;
  crop_y?: number;
  crop_w?: number;
  crop_h?: number;
  target_w?: number;
  target_h?: number;
  format?: 'jpeg' | 'png';
  waive?: boolean;
  waiver_text?: string;
  include_prompt?: boolean;
  prompt?: string;
}

export interface AigcExportResult {
  blob: Blob;
  produceId: string;
  labeled: boolean;
  noteCode: string;
  filename: string;
}

/** 从后端返回的 JSON 错误里取出可读文案（导出接口正常返回二进制，出错才是 JSON） */
async function readBlobError(e: any): Promise<string> {
  const data = e?.response?.data;
  if (data instanceof Blob) {
    try {
      const txt = await data.text();
      const j = JSON.parse(txt);
      return j?.message || j?.msg || '导出失败';
    } catch {
      return '导出失败';
    }
  }
  return e?.response?.data?.message || e?.message || '导出失败';
}

export const aigcApi = {
  options: () => api.get<{ code: number; data: AigcOptions }>('/ai/aigc/options'),

  /**
   * 走出口管道导出：服务端合成 + 烧角标 + 写 AIGC 字段。
   * 前端只发参数、收二进制 —— 不碰像素、不写元数据。
   */
  async exportImage(p: AigcExportParams): Promise<AigcExportResult> {
    try {
      const res = await api.post('/ai/aigc/export', p, { responseType: 'blob', timeout: 120000 });
      const cd = String(res.headers['content-disposition'] || '');
      const m = cd.match(/filename="?([^";]+)"?/);
      return {
        blob: res.data as Blob,
        produceId: String(res.headers['x-aigc-produceid'] || ''),
        labeled: String(res.headers['x-aigc-labeled'] || '0') === '1',
        noteCode: String(res.headers['x-aigc-note'] || ''),
        filename: m ? m[1] : 'ai-image.jpg',
      };
    } catch (e: any) {
      throw new Error(await readBlobError(e));
    }
  },
};

export const aigcAdminApi = {
  getConfig: () => api.get<{ code: number; data: { config: AigcConfig } }>('/admin/aigc/config'),
  saveConfig: (c: AigcConfig) => api.put('/admin/aigc/config', c),
  exports: (page = 1, size = 20) =>
    api.get<{ code: number; data: { list: any[]; total: number } }>('/admin/aigc/exports', {
      params: { page, size },
    }),
  waivers: (page = 1, size = 20) =>
    api.get<{ code: number; data: { list: any[]; total: number } }>('/admin/aigc/waivers', {
      params: { page, size },
    }),
  verify: (produceId: string) =>
    api.get<{ code: number; data: any }>(`/admin/aigc/verify/${encodeURIComponent(produceId)}`),
  uploadFont: (fd: FormData) => api.postForm('/admin/aigc/font', fd),
};
