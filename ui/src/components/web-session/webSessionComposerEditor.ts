export interface WebSessionComposerSelection {
  start: number;
  end: number;
}

export interface WebSessionComposerEditorExposed {
  focus: () => void;
  getSelectionRange: () => WebSessionComposerSelection;
  setSelectionRange: (start: number, end?: number) => void;
}
