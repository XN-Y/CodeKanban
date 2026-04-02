import type { EditorPreference } from '@/stores/settings';

export const DEFAULT_EDITOR: EditorPreference = 'vscode';

export const EDITOR_OPTIONS: Array<{ label: string; value: EditorPreference }> = [
  { label: 'VSCode', value: 'vscode' },
  { label: 'Cursor', value: 'cursor' },
  { label: 'Trae', value: 'trae' },
  { label: 'Zed', value: 'zed' },
  { label: '自定义命令', value: 'custom' },
];

export const EDITOR_LABEL_MAP: Record<EditorPreference, string> = EDITOR_OPTIONS.reduce(
  (acc, option) => {
    acc[option.value] = option.label;
    return acc;
  },
  {} as Record<EditorPreference, string>
);

const EDITOR_VALUE_SET = new Set<EditorPreference>(EDITOR_OPTIONS.map(option => option.value));

export function isEditorPreference(value: unknown): value is EditorPreference {
  if (typeof value !== 'string') {
    return false;
  }
  return EDITOR_VALUE_SET.has(value as EditorPreference);
}
