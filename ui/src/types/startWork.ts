export type StartWorkAction = 'terminal' | 'claude' | 'codex';

export type StartWorkAgent = Exclude<StartWorkAction, 'terminal'>;
