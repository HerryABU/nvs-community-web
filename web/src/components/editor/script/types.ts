// ==================== 话本气泡体：编辑器数据模型 ====================
// 解析/序列化规则与服务端 server/scriptmode/script.go 保持一致，
// 两边必须同步改，否则编辑器里看到的和渲染出来的不一样。

/** 气泡左右 */
export type ScriptSide = 'left' | 'right';

/** 行类型：对话 / 场景提示 / 旁白（话本体）/ 散文段（混排）/ 图片 */
export type ScriptRowKind = 'dialogue' | 'stage' | 'narration' | 'prose' | 'image';

/** 解析出的一行（气泡级，与渲染节点一一对应） */
export interface ScriptRow {
  /** 稳定 key（渲染用） */
  id: string;
  kind: ScriptRowKind;
  /** 角色名（kind=dialogue） */
  speaker: string;
  /** 台词 / 提示文本 / 旁白文本；对话多段用 \n 分隔 */
  text: string;
  /** 左右（kind=dialogue） */
  side: ScriptSide;
  /** 图片说明（kind=image） */
  alt?: string;
  /** 图片地址（kind=image） */
  url?: string;
  /** 原始行（未改动时原样输出，避免无谓改写正文） */
  raw: string;
  /** 本行之前原文是否有空行（序列化时保留段落间隔） */
  blankBefore: boolean;
}

/** 角色（编辑器内视图：合并了作品角色表与正文里出现的角色） */
export interface ScriptCharacterView {
  name: string;
  avatar: string;
  color: string;
  side: ScriptSide;
  /** 是否已在作品角色表里登记 */
  registered: boolean;
  intro: string;
}
