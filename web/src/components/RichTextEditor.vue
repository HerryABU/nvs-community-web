<template>
  <div class="rich-text-editor" v-if="editor">
    <div class="rte-toolbar">
      <template v-for="(item, key) in toolbarItems" :key="key">
        <span v-if="item.type === 'separator'" class="rte-separator"></span>
        <button
          v-else
          :class="['rte-btn', { active: item.isActive?.() }]"
          @click="item.action"
          :title="item.title"
          type="button"
        >
          <span v-if="item.icon" v-html="item.icon"></span>
          <span v-else>{{ item.label }}</span>
        </button>
      </template>
    </div>
    <editor-content :editor="editor" class="rte-content" />
  </div>
</template>

<script setup lang="ts">
import { useEditor, EditorContent } from '@tiptap/vue-3';
import StarterKit from '@tiptap/starter-kit';
import Underline from '@tiptap/extension-underline';
import Link from '@tiptap/extension-link';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import { watch, onBeforeUnmount, ref } from 'vue';

const props = defineProps<{
  modelValue: string;
  placeholder?: string;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void;
}>();

const editor = useEditor({
  content: props.modelValue,
  extensions: [
    StarterKit.configure({
      heading: {
        levels: [1, 2, 3, 4],
      },
    }),
    Underline,
    Link.configure({
      openOnClick: false,
    }),
    Placeholder.configure({
      placeholder: props.placeholder || '开始写作...',
    }),
    TextAlign.configure({
      types: ['heading', 'paragraph'],
      alignments: ['left', 'center', 'right'],
    }),
  ],
  onUpdate: ({ editor }) => {
    emit('update:modelValue', editor.getHTML());
  },
  editorProps: {
    attributes: {
      class: 'prose prose-sm max-w-none focus:outline-none',
    },
  },
});

// 当外部值变化时更新编辑器内容
watch(
  () => props.modelValue,
  (newVal) => {
    if (editor.value && newVal !== editor.value.getHTML()) {
      editor.value.commands.setContent(newVal, false);
    }
  }
);

onBeforeUnmount(() => {
  editor.value?.destroy();
});

const toolbarItems = ref<Record<string, any>>({});

// 在编辑器准备好后设置工具栏
watch(
  () => editor.value,
  (ed) => {
    if (!ed) return;
    toolbarItems.value = {
      bold: {
        title: '粗体 (Ctrl+B)',
        icon: '<b>B</b>',
        action: () => ed.chain().focus().toggleBold().run(),
        isActive: () => ed.isActive('bold'),
      },
      italic: {
        title: '斜体 (Ctrl+I)',
        icon: '<i>I</i>',
        action: () => ed.chain().focus().toggleItalic().run(),
        isActive: () => ed.isActive('italic'),
      },
      underline: {
        title: '下划线 (Ctrl+U)',
        icon: '<u>U</u>',
        action: () => ed.chain().focus().toggleUnderline().run(),
        isActive: () => ed.isActive('underline'),
      },
      strike: {
        title: '删除线',
        icon: '<s>S</s>',
        action: () => ed.chain().focus().toggleStrike().run(),
        isActive: () => ed.isActive('strike'),
      },
      separator1: { type: 'separator' },
      h2: {
        title: '二级标题',
        label: 'H2',
        action: () => ed.chain().focus().toggleHeading({ level: 2 }).run(),
        isActive: () => ed.isActive('heading', { level: 2 }),
      },
      h3: {
        title: '三级标题',
        label: 'H3',
        action: () => ed.chain().focus().toggleHeading({ level: 3 }).run(),
        isActive: () => ed.isActive('heading', { level: 3 }),
      },
      separator2: { type: 'separator' },
      alignLeft: {
        title: '左对齐',
        icon: '⇤',
        action: () => ed.chain().focus().setTextAlign('left').run(),
        isActive: () => ed.isActive({ textAlign: 'left' }),
      },
      alignCenter: {
        title: '居中',
        icon: '⇔',
        action: () => ed.chain().focus().setTextAlign('center').run(),
        isActive: () => ed.isActive({ textAlign: 'center' }),
      },
      alignRight: {
        title: '右对齐',
        icon: '⇥',
        action: () => ed.chain().focus().setTextAlign('right').run(),
        isActive: () => ed.isActive({ textAlign: 'right' }),
      },
      separator2b: { type: 'separator' },
      bulletList: {
        title: '无序列表',
        icon: '•',
        action: () => ed.chain().focus().toggleBulletList().run(),
        isActive: () => ed.isActive('bulletList'),
      },
      orderedList: {
        title: '有序列表',
        icon: '1.',
        action: () => ed.chain().focus().toggleOrderedList().run(),
        isActive: () => ed.isActive('orderedList'),
      },
      blockquote: {
        title: '引用',
        icon: '❝',
        action: () => ed.chain().focus().toggleBlockquote().run(),
        isActive: () => ed.isActive('blockquote'),
      },
      codeBlock: {
        title: '代码块',
        icon: '&lt;/&gt;',
        action: () => ed.chain().focus().toggleCodeBlock().run(),
        isActive: () => ed.isActive('codeBlock'),
      },
      separator3: { type: 'separator' },
      horizontalRule: {
        title: '分割线',
        icon: '—',
        action: () => ed.chain().focus().setHorizontalRule().run(),
      },
      undo: {
        title: '撤销',
        icon: '↩',
        action: () => ed.chain().focus().undo().run(),
      },
      redo: {
        title: '重做',
        icon: '↪',
        action: () => ed.chain().focus().redo().run(),
      },
    };
  },
  { immediate: true }
);
</script>

<style scoped>
.rich-text-editor {
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  overflow: hidden;
  background: #fff;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.rte-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  padding: 6px 8px;
  border-bottom: 1px solid #e8e8e8;
  background: #fafafa;
  flex-shrink: 0;
}

.rte-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 30px;
  height: 30px;
  padding: 0 6px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
  font-size: 0.85rem;
  color: #555;
  transition: all 0.15s;
}

.rte-btn:hover {
  background: #e8e8e8;
  border-color: #ccc;
}

.rte-btn.active {
  background: #d0e0ff;
  border-color: #91b9ff;
  color: #1a56db;
}

.rte-separator {
  display: inline-block;
  width: 1px;
  height: 24px;
  margin: 0 6px;
  background: #ddd;
  vertical-align: middle;
}

.rte-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  min-height: 200px;
}

.rte-content :deep(.ProseMirror) {
  outline: none;
  min-height: 300px;
  font-family: Georgia, 'Noto Serif SC', serif;
  font-size: 1.05rem;
  line-height: 1.8;
  color: #333;
}

.rte-content :deep(.ProseMirror p) {
  margin-bottom: 0.8em;
  text-indent: 2em;
}

.rte-content :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  float: left;
  color: #adb5bd;
  pointer-events: none;
  height: 0;
  text-indent: 2em;
}

.rte-content :deep(.ProseMirror h1),
.rte-content :deep(.ProseMirror h2),
.rte-content :deep(.ProseMirror h3),
.rte-content :deep(.ProseMirror h4) {
  margin: 1em 0 0.6em;
  text-indent: 0;
  font-weight: 700;
  line-height: 1.4;
}

.rte-content :deep(.ProseMirror h2) {
  font-size: 1.4rem;
  border-bottom: 1px solid #eee;
  padding-bottom: 6px;
}

.rte-content :deep(.ProseMirror h3) {
  font-size: 1.2rem;
}

.rte-content :deep(.ProseMirror blockquote) {
  border-left: 4px solid #e67e22;
  padding: 8px 16px;
  margin: 12px 0;
  background: #fef9f3;
  text-indent: 0;
}

.rte-content :deep(.ProseMirror pre) {
  background: #f5f5f5;
  padding: 16px;
  border-radius: 6px;
  overflow-x: auto;
  margin: 12px 0;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 0.9rem;
  text-indent: 0;
}

.rte-content :deep(.ProseMirror code) {
  background: #f0f0f0;
  padding: 2px 6px;
  border-radius: 3px;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 0.9em;
}

.rte-content :deep(.ProseMirror hr) {
  border: none;
  border-top: 2px solid #e0e0e0;
  margin: 24px 0;
}

.rte-content :deep(.ProseMirror ul),
.rte-content :deep(.ProseMirror ol) {
  padding-left: 1.5em;
  margin: 0.5em 0;
}

.rte-content :deep(.ProseMirror li) {
  margin-bottom: 0.3em;
}
</style>
