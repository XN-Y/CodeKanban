export type FileManagerScopeKind = 'project' | 'worktree';
export type FileManagerEntryKind = 'file' | 'directory' | 'symlink';
export type FileManagerPreviewKind =
  | 'image'
  | 'text'
  | 'markdown'
  | 'pdf'
  | 'audio'
  | 'video'
  | 'binary';

export interface FileManagerScope {
  id: string;
  kind: FileManagerScopeKind;
  label: string;
  rootPath: string;
  worktreeId?: string;
}

export interface FileManagerBreadcrumb {
  name: string;
  path: string;
}

export interface FileManagerEntry {
  name: string;
  path: string;
  kind: FileManagerEntryKind;
  size: number;
  modifiedAt: string;
  mime?: string;
  extension?: string;
  previewKind: FileManagerPreviewKind;
  hidden: boolean;
}

export interface FileManagerListResult {
  scope: FileManagerScope;
  currentPath: string;
  parentPath?: string;
  breadcrumbs: FileManagerBreadcrumb[];
  entries: FileManagerEntry[];
}

export interface FileManagerPreviewResult {
  entry: FileManagerEntry;
  previewKind: FileManagerPreviewKind;
  textContent?: string;
  truncated: boolean;
  inlineUrl: string;
  downloadUrl: string;
}

export interface FileManagerArchiveJob {
  archiveId: string;
  fileName: string;
  size: number;
  createdAt: string;
  expiresAt: string;
  downloadUrl: string;
}

export interface FileManagerUploadSession {
  uploadId: string;
  projectId: string;
  scopeId: string;
  directoryPath: string;
  targetPath: string;
  fileName: string;
  size: number;
  offset: number;
  chunkSize: number;
  createdAt: string;
  updatedAt: string;
  expiresAt: string;
}

export interface FileManagerBulkFailure {
  path: string;
  name: string;
  message: string;
}

export interface FileManagerBulkResult {
  succeeded: Array<{
    path: string;
    name: string;
  }>;
  failed: FileManagerBulkFailure[];
}

export type FileTransferDirection = 'upload' | 'download';
export type FileTransferStatus =
  | 'queued'
  | 'running'
  | 'paused'
  | 'completed'
  | 'failed'
  | 'canceled';

export interface FileTransferTask {
  id: string;
  projectId: string;
  scopeId: string;
  directoryPath: string;
  direction: FileTransferDirection;
  name: string;
  status: FileTransferStatus;
  loaded: number;
  total?: number;
  progress: number | null;
  speed: number;
  error?: string;
  createdAt: number;
  updatedAt: number;
}
