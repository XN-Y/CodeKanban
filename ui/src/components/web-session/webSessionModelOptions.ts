export type WebSessionAgentOption = 'claude' | 'codex';

export type WebSessionModelOption = {
  label: string;
  value: string;
};

export const CUSTOM_MODEL_VALUE = '__custom_model__';

export const CLAUDE_MODEL_OPTIONS: WebSessionModelOption[] = [
  { label: 'Opus', value: 'opus' },
  { label: 'Sonnet', value: 'sonnet' },
  { label: 'Haiku', value: 'haiku' },
];

export const CODEX_MODEL_OPTIONS: WebSessionModelOption[] = [
  { label: 'GPT-5.3 Codex', value: 'gpt-5.3-codex' },
  { label: 'GPT-5.3 Codex Spark', value: 'gpt-5.3-codex-spark' },
  { label: 'GPT-5.4', value: 'gpt-5.4' },
  { label: 'GPT-5.5', value: 'gpt-5.5' },
  { label: 'GPT-5.4 mini', value: 'gpt-5.4-mini' },
  { label: 'GPT-5.4 nano', value: 'gpt-5.4-nano' },
  { label: 'GPT-5.4 Pro', value: 'gpt-5.4-pro' },
  { label: 'GPT-5.5 Pro', value: 'gpt-5.5-pro' },
];

export function defaultModelForAgent(agent: WebSessionAgentOption) {
  return agent === 'claude' ? 'opus' : 'gpt-5.5';
}
