export type WebSessionSnapshotLoadHandle = {
  token: number;
  controller: AbortController;
  signal: AbortSignal;
};

export function createWebSessionSnapshotLoadController() {
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
    begin(): WebSessionSnapshotLoadHandle {
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

    isCurrent(handle: WebSessionSnapshotLoadHandle) {
      return currentToken === handle.token && currentController === handle.controller;
    },

    release(handle: WebSessionSnapshotLoadHandle) {
      if (currentController === handle.controller) {
        currentController = null;
      }
    },
  };
}
