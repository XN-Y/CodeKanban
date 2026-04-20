export type GitChangesLoadHandle = {
  token: number;
  controller: AbortController;
  signal: AbortSignal;
};

export function createGitChangesLoadController() {
  let currentToken = 0;
  let currentController: AbortController | null = null;

  function abortCurrent() {
    if (!currentController) {
      return;
    }
    currentController.abort();
    currentController = null;
  }

  return {
    begin(): GitChangesLoadHandle {
      currentToken += 1;
      abortCurrent();
      const controller = new AbortController();
      currentController = controller;
      return {
        token: currentToken,
        controller,
        signal: controller.signal,
      };
    },

    cancel() {
      currentToken += 1;
      abortCurrent();
    },

    isCurrent(handle: GitChangesLoadHandle) {
      return currentToken === handle.token && currentController === handle.controller;
    },

    release(handle: GitChangesLoadHandle) {
      if (currentController === handle.controller) {
        currentController = null;
      }
    },
  };
}
