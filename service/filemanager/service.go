package filemanager

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"code-kanban/model"
	"code-kanban/utils"
)

const (
	defaultUploadChunkSize  = 4 * 1024 * 1024
	defaultUploadSessionTTL = 24 * time.Hour
	defaultArchiveTTL       = 30 * time.Minute
	defaultTextPreviewBytes = 256 * 1024
)

var (
	errScopeNotFound    = errors.New("file scope not found")
	errArchiveNotFound  = errors.New("archive not found")
	errUploadNotFound   = errors.New("upload session not found")
	errOffsetMismatch   = errors.New("upload offset mismatch")
	errTargetExists     = errors.New("target already exists")
	errProtectedPath    = errors.New("path is protected")
	errUnsupportedEntry = errors.New("unsupported file entry")
)

type Config struct {
	DataDir          string
	UploadChunkSize  int64
	UploadSessionTTL time.Duration
	ArchiveTTL       time.Duration
	TextPreviewBytes int64
}

type Service struct {
	logger           *zap.Logger
	uploadsDir       string
	archivesDir      string
	uploadChunkSize  int64
	uploadSessionTTL time.Duration
	archiveTTL       time.Duration
	textPreviewBytes int64
	lockMap          sync.Map
}

type archiveSource struct {
	rel  string
	path string
	info os.FileInfo
}

type archiveMeta struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	FileName  string    `json:"fileName"`
	FilePath  string    `json:"filePath"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type uploadMeta struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"projectId"`
	ScopeID    string    `json:"scopeId"`
	ScopeRoot  string    `json:"scopeRoot"`
	Directory  string    `json:"directory"`
	TargetPath string    `json:"targetPath"`
	FileName   string    `json:"fileName"`
	Size       int64     `json:"size"`
	Offset     int64     `json:"offset"`
	ChunkSize  int64     `json:"chunkSize"`
	PartPath   string    `json:"partPath"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

func NewService(cfg Config, logger *zap.Logger) (*Service, error) {
	if strings.TrimSpace(cfg.DataDir) == "" {
		cfg.DataDir = utils.GetDataDir()
	}
	if cfg.UploadChunkSize <= 0 {
		cfg.UploadChunkSize = defaultUploadChunkSize
	}
	if cfg.UploadSessionTTL <= 0 {
		cfg.UploadSessionTTL = defaultUploadSessionTTL
	}
	if cfg.ArchiveTTL <= 0 {
		cfg.ArchiveTTL = defaultArchiveTTL
	}
	if cfg.TextPreviewBytes <= 0 {
		cfg.TextPreviewBytes = defaultTextPreviewBytes
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	rootDir := filepath.Join(cfg.DataDir, "file-manager")
	uploadsDir := filepath.Join(rootDir, "uploads")
	archivesDir := filepath.Join(rootDir, "archives")
	for _, dir := range []string{rootDir, uploadsDir, archivesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	return &Service{
		logger:           logger.Named("file-manager"),
		uploadsDir:       uploadsDir,
		archivesDir:      archivesDir,
		uploadChunkSize:  cfg.UploadChunkSize,
		uploadSessionTTL: cfg.UploadSessionTTL,
		archiveTTL:       cfg.ArchiveTTL,
		textPreviewBytes: cfg.TextPreviewBytes,
	}, nil
}

func (s *Service) StartBackground(ctx context.Context) {
	if s == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		s.cleanup(time.Now())

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				s.cleanup(now)
			}
		}
	}()
}

func (s *Service) ListScopes(ctx context.Context, projectID string) ([]Scope, error) {
	project, worktrees, err := s.loadProjectScopes(ctx, projectID)
	if err != nil {
		return nil, err
	}

	projectRoot := filepath.Clean(project.Path)
	includeProjectScope := true
	for _, worktree := range worktrees {
		if worktree.IsMain && filepath.Clean(worktree.Path) == projectRoot {
			includeProjectScope = false
			break
		}
	}

	scopes := make([]Scope, 0, len(worktrees)+1)
	if includeProjectScope {
		scopes = append(scopes, Scope{
			ID:       projectScopeID(project.Id),
			Kind:     ScopeKindProject,
			Label:    project.Name,
			RootPath: projectRoot,
		})
	}

	for _, worktree := range worktrees {
		label := strings.TrimSpace(worktree.BranchName)
		if label == "" {
			label = filepath.Base(worktree.Path)
		}
		scopes = append(scopes, Scope{
			ID:         worktreeScopeID(worktree.Id),
			Kind:       ScopeKindWorktree,
			Label:      label,
			RootPath:   filepath.Clean(worktree.Path),
			WorktreeID: worktree.Id,
		})
	}

	return scopes, nil
}

func (s *Service) List(ctx context.Context, projectID, scopeID, currentPath string) (*ListResult, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}

	normalizedPath, absPath, info, err := s.resolveExisting(scope, currentPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target path is not a directory")
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	items := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" || name == ".git" {
			continue
		}

		entryPath := filepath.Join(absPath, name)
		relativePath := joinRelativePath(normalizedPath, name)
		lstat, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}

		item := Entry{
			Name:       name,
			Path:       toSlashPath(relativePath),
			ModifiedAt: lstat.ModTime(),
			Extension:  strings.ToLower(filepath.Ext(name)),
			Hidden:     strings.HasPrefix(name, "."),
		}

		switch {
		case lstat.Mode()&os.ModeSymlink != 0:
			item.Kind = EntryKindSymlink
			item.PreviewKind = PreviewKindBinary
		case lstat.IsDir():
			item.Kind = EntryKindDirectory
			item.PreviewKind = PreviewKindBinary
		default:
			item.Kind = EntryKindFile
			item.Size = lstat.Size()
			item.Mime = detectMimeFromName(name)
			item.PreviewKind = detectPreviewKind(name, item.Mime)
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind == EntryKindDirectory
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	return &ListResult{
		Scope:       scope,
		CurrentPath: toSlashPath(normalizedPath),
		ParentPath:  toSlashPath(parentRelativePath(normalizedPath)),
		Breadcrumbs: buildBreadcrumbs(normalizedPath),
		Entries:     items,
	}, nil
}

func (s *Service) Preview(ctx context.Context, projectID, scopeID, path string) (*PreviewResult, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}

	normalizedPath, absPath, info, err := s.resolveExisting(scope, path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("directories cannot be previewed")
	}

	entry, err := s.buildFileEntry(normalizedPath, info)
	if err != nil {
		return nil, err
	}

	result := &PreviewResult{
		Entry:       entry,
		PreviewKind: entry.PreviewKind,
	}

	if entry.PreviewKind == PreviewKindImage ||
		entry.PreviewKind == PreviewKindPDF ||
		entry.PreviewKind == PreviewKindAudio ||
		entry.PreviewKind == PreviewKindVideo {
		return result, nil
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	limit := s.textPreviewBytes
	if limit <= 0 {
		limit = defaultTextPreviewBytes
	}

	buffer := bytes.NewBuffer(nil)
	written, err := io.Copy(buffer, io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if written == 0 {
		return result, nil
	}

	raw := buffer.Bytes()
	if int64(len(raw)) > limit {
		result.Truncated = true
		raw = raw[:limit]
	}
	if !utf8.Valid(raw) {
		if entry.PreviewKind == PreviewKindText || entry.PreviewKind == PreviewKindMarkdown {
			result.PreviewKind = PreviewKindBinary
			result.Entry.PreviewKind = PreviewKindBinary
		}
		return result, nil
	}

	if entry.PreviewKind != PreviewKindText && entry.PreviewKind != PreviewKindMarkdown {
		result.PreviewKind = PreviewKindText
		result.Entry.PreviewKind = PreviewKindText
	}
	result.TextContent = string(raw)
	return result, nil
}

func (s *Service) ResolveFile(ctx context.Context, projectID, scopeID, path string) (Scope, string, os.FileInfo, string, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return Scope{}, "", nil, "", err
	}
	normalizedPath, absPath, info, err := s.resolveExisting(scope, path)
	if err != nil {
		return Scope{}, "", nil, "", err
	}
	if info.IsDir() {
		return Scope{}, "", nil, "", fmt.Errorf("target path is a directory")
	}
	return scope, absPath, info, normalizedPath, nil
}

func (s *Service) CreateDirectory(ctx context.Context, projectID, scopeID, parentPath, name string) (*Entry, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}
	cleanName, err := sanitizeEntryName(name)
	if err != nil {
		return nil, err
	}

	normalizedParent, _, info, err := s.resolveExisting(scope, parentPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("parent path is not a directory")
	}

	targetRel := joinRelativePath(normalizedParent, cleanName)
	targetAbs, err := s.resolveCreatePath(scope, targetRel)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(targetAbs); err == nil {
		return nil, errTargetExists
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Mkdir(targetAbs, 0o755); err != nil {
		return nil, err
	}

	stat, err := os.Stat(targetAbs)
	if err != nil {
		return nil, err
	}
	entry := Entry{
		Name:        cleanName,
		Path:        toSlashPath(targetRel),
		Kind:        EntryKindDirectory,
		ModifiedAt:  stat.ModTime(),
		PreviewKind: PreviewKindBinary,
		Hidden:      strings.HasPrefix(cleanName, "."),
	}
	return &entry, nil
}

func (s *Service) Rename(ctx context.Context, projectID, scopeID, path, newName string) (*Entry, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}
	cleanName, err := sanitizeEntryName(newName)
	if err != nil {
		return nil, err
	}

	normalizedPath, absPath, _, err := s.resolveExisting(scope, path)
	if err != nil {
		return nil, err
	}
	if normalizedPath == "" {
		return nil, fmt.Errorf("scope root cannot be renamed")
	}

	parentRel := parentRelativePath(normalizedPath)
	targetRel := joinRelativePath(parentRel, cleanName)
	targetAbs, err := s.resolveCreatePath(scope, targetRel)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(targetAbs); err == nil {
		return nil, errTargetExists
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Rename(absPath, targetAbs); err != nil {
		return nil, err
	}

	stat, err := os.Lstat(targetAbs)
	if err != nil {
		return nil, err
	}

	if stat.Mode()&os.ModeSymlink != 0 {
		return nil, errUnsupportedEntry
	}
	if stat.IsDir() {
		return &Entry{
			Name:        cleanName,
			Path:        toSlashPath(targetRel),
			Kind:        EntryKindDirectory,
			ModifiedAt:  stat.ModTime(),
			PreviewKind: PreviewKindBinary,
			Hidden:      strings.HasPrefix(cleanName, "."),
		}, nil
	}
	entry, err := s.buildFileEntry(targetRel, stat)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *Service) Copy(ctx context.Context, projectID, scopeID string, sourcePaths []string, destinationPath string) (*BulkResult, error) {
	return s.bulkTransfer(ctx, projectID, scopeID, sourcePaths, destinationPath, false)
}

func (s *Service) Move(ctx context.Context, projectID, scopeID string, sourcePaths []string, destinationPath string) (*BulkResult, error) {
	return s.bulkTransfer(ctx, projectID, scopeID, sourcePaths, destinationPath, true)
}

func (s *Service) Delete(ctx context.Context, projectID, scopeID string, paths []string) (*BulkResult, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}
	result := &BulkResult{}
	for _, rawPath := range paths {
		normalizedPath, absPath, info, err := s.resolveExisting(scope, rawPath)
		name := filepath.Base(normalizedPath)
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedPath),
				Name:    name,
				Message: err.Error(),
			})
			continue
		}
		if normalizedPath == "" {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    "",
				Name:    scope.Label,
				Message: "scope root cannot be deleted",
			})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedPath),
				Name:    info.Name(),
				Message: errUnsupportedEntry.Error(),
			})
			continue
		}
		if err := os.RemoveAll(absPath); err != nil {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedPath),
				Name:    info.Name(),
				Message: err.Error(),
			})
			continue
		}
		result.Succeeded = append(result.Succeeded, FileRef{
			Path: toSlashPath(normalizedPath),
			Name: info.Name(),
		})
	}
	return result, nil
}

func (s *Service) CreateArchive(ctx context.Context, projectID, scopeID string, paths []string, fileName string) (*ArchiveJob, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}

	sources := make([]archiveSource, 0, len(paths))
	for _, rawPath := range paths {
		normalizedPath, absPath, info, err := s.resolveExisting(scope, rawPath)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errUnsupportedEntry
		}
		sources = append(sources, archiveSource{
			rel:  normalizedPath,
			path: absPath,
			info: info,
		})
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}

	archiveID := utils.NewID()
	baseName := strings.TrimSpace(fileName)
	if baseName == "" {
		baseName = defaultArchiveName(sources)
	}
	if !strings.HasSuffix(strings.ToLower(baseName), ".zip") {
		baseName += ".zip"
	}
	baseName = filepath.Base(strings.ReplaceAll(baseName, "\\", "/"))
	if baseName == "" || baseName == "." {
		baseName = fmt.Sprintf("download-%s.zip", time.Now().Format("20060102-150405"))
	}

	archivePath := filepath.Join(s.archivesDir, archiveID+".zip")
	output, err := os.Create(archivePath)
	if err != nil {
		return nil, err
	}

	zipWriter := zip.NewWriter(output)
	for _, source := range sources {
		rootName := filepath.Base(source.path)
		if err := writeZipEntry(zipWriter, source.path, rootName); err != nil {
			_ = zipWriter.Close()
			_ = output.Close()
			_ = os.Remove(archivePath)
			return nil, err
		}
	}
	if err := zipWriter.Close(); err != nil {
		_ = output.Close()
		_ = os.Remove(archivePath)
		return nil, err
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(archivePath)
		return nil, err
	}

	stat, err := os.Stat(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		return nil, err
	}

	now := time.Now()
	meta := archiveMeta{
		ID:        archiveID,
		ProjectID: projectID,
		FileName:  baseName,
		FilePath:  archivePath,
		Size:      stat.Size(),
		CreatedAt: now,
		ExpiresAt: now.Add(s.archiveTTL),
	}
	if err := s.writeJSONFile(s.archiveMetaPath(archiveID), meta); err != nil {
		_ = os.Remove(archivePath)
		return nil, err
	}

	return &ArchiveJob{
		ID:        archiveID,
		FileName:  meta.FileName,
		Size:      meta.Size,
		CreatedAt: meta.CreatedAt,
		ExpiresAt: meta.ExpiresAt,
	}, nil
}

func (s *Service) GetArchive(projectID, archiveID string) (*ArchiveJob, string, error) {
	meta, err := s.loadArchiveMeta(archiveID)
	if err != nil {
		return nil, "", err
	}
	if meta.ProjectID != projectID {
		return nil, "", errArchiveNotFound
	}
	if time.Now().After(meta.ExpiresAt) {
		s.deleteArchiveMeta(meta)
		return nil, "", errArchiveNotFound
	}
	if _, err := os.Stat(meta.FilePath); err != nil {
		return nil, "", errArchiveNotFound
	}
	return &ArchiveJob{
		ID:        meta.ID,
		FileName:  meta.FileName,
		Size:      meta.Size,
		CreatedAt: meta.CreatedAt,
		ExpiresAt: meta.ExpiresAt,
	}, meta.FilePath, nil
}

func (s *Service) CreateUploadSession(ctx context.Context, projectID, scopeID, directoryPath, fileName string, size int64) (*UploadSession, error) {
	if size <= 0 {
		return nil, fmt.Errorf("file size must be greater than zero")
	}
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}
	cleanName, err := sanitizeEntryName(fileName)
	if err != nil {
		return nil, err
	}
	normalizedDir, _, info, err := s.resolveExisting(scope, directoryPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target directory is not a folder")
	}
	targetRel := joinRelativePath(normalizedDir, cleanName)
	targetAbs, err := s.resolveCreatePath(scope, targetRel)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(targetAbs); err == nil {
		return nil, errTargetExists
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	now := time.Now()
	uploadID := utils.NewID()
	partPath := filepath.Join(s.uploadsDir, uploadID+".part")
	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	if err := partFile.Close(); err != nil {
		return nil, err
	}

	meta := uploadMeta{
		ID:         uploadID,
		ProjectID:  projectID,
		ScopeID:    scope.ID,
		ScopeRoot:  scope.RootPath,
		Directory:  toSlashPath(normalizedDir),
		TargetPath: toSlashPath(targetRel),
		FileName:   cleanName,
		Size:       size,
		Offset:     0,
		ChunkSize:  s.uploadChunkSize,
		PartPath:   partPath,
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  now.Add(s.uploadSessionTTL),
	}
	if err := s.writeJSONFile(s.uploadMetaPath(uploadID), meta); err != nil {
		_ = os.Remove(partPath)
		return nil, err
	}
	return uploadSessionFromMeta(meta), nil
}

func (s *Service) GetUploadSession(projectID, uploadID string) (*UploadSession, error) {
	meta, err := s.loadUploadMeta(uploadID)
	if err != nil {
		return nil, err
	}
	if meta.ProjectID != projectID {
		return nil, errUploadNotFound
	}
	if time.Now().After(meta.ExpiresAt) {
		s.deleteUploadMeta(meta)
		return nil, errUploadNotFound
	}
	return uploadSessionFromMeta(meta), nil
}

func (s *Service) AppendUploadChunk(projectID, uploadID string, expectedOffset int64, contentLength int64, reader io.Reader) (*UploadSession, error) {
	var session *UploadSession
	err := s.withLock(uploadID, func() error {
		meta, err := s.loadUploadMeta(uploadID)
		if err != nil {
			return err
		}
		if meta.ProjectID != projectID {
			return errUploadNotFound
		}
		if time.Now().After(meta.ExpiresAt) {
			s.deleteUploadMeta(meta)
			return errUploadNotFound
		}
		if meta.Offset != expectedOffset {
			return errOffsetMismatch
		}
		if contentLength <= 0 {
			return fmt.Errorf("chunk body is required")
		}
		if contentLength > meta.ChunkSize {
			return fmt.Errorf("chunk exceeds upload chunk size")
		}
		remaining := meta.Size - meta.Offset
		if remaining <= 0 {
			return fmt.Errorf("upload already reached target size")
		}
		if contentLength > remaining {
			return fmt.Errorf("chunk exceeds remaining file size")
		}

		partFile, err := os.OpenFile(meta.PartPath, os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer partFile.Close()

		if _, err := partFile.Seek(meta.Offset, io.SeekStart); err != nil {
			return err
		}

		written, err := io.Copy(partFile, io.LimitReader(reader, contentLength))
		if err != nil {
			return err
		}
		if written != contentLength {
			return fmt.Errorf("incomplete chunk write")
		}

		meta.Offset += written
		meta.UpdatedAt = time.Now()
		meta.ExpiresAt = meta.UpdatedAt.Add(s.uploadSessionTTL)
		if err := s.writeJSONFile(s.uploadMetaPath(uploadID), meta); err != nil {
			return err
		}

		session = uploadSessionFromMeta(meta)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) CompleteUpload(ctx context.Context, projectID, uploadID string) (*Entry, error) {
	var entry *Entry
	err := s.withLock(uploadID, func() error {
		meta, err := s.loadUploadMeta(uploadID)
		if err != nil {
			return err
		}
		if meta.ProjectID != projectID {
			return errUploadNotFound
		}
		if meta.Offset != meta.Size {
			return fmt.Errorf("upload is incomplete")
		}

		scope, err := s.scopeByID(ctx, projectID, meta.ScopeID)
		if err != nil {
			return err
		}
		targetAbs, err := s.resolveCreatePath(scope, meta.TargetPath)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(targetAbs); err == nil {
			return errTargetExists
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(meta.PartPath, targetAbs); err != nil {
			return err
		}
		_ = os.Remove(s.uploadMetaPath(uploadID))

		stat, err := os.Stat(targetAbs)
		if err != nil {
			return err
		}
		built, err := s.buildFileEntry(meta.TargetPath, stat)
		if err != nil {
			return err
		}
		entry = &built
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *Service) CancelUpload(projectID, uploadID string) error {
	return s.withLock(uploadID, func() error {
		meta, err := s.loadUploadMeta(uploadID)
		if err != nil {
			return err
		}
		if meta.ProjectID != projectID {
			return errUploadNotFound
		}
		s.deleteUploadMeta(meta)
		return nil
	})
}

func (s *Service) loadProjectScopes(ctx context.Context, projectID string) (*model.Project, []*model.Worktree, error) {
	q, err := model.ResolveQueries(nil)
	if err != nil {
		return nil, nil, err
	}
	project, err := q.ProjectGetByID(ctx, strings.TrimSpace(projectID))
	if err != nil {
		return nil, nil, err
	}
	worktrees, err := q.WorktreeListByProject(ctx, project.Id)
	if err != nil {
		return nil, nil, err
	}
	return project, worktrees, nil
}

func (s *Service) scopeByID(ctx context.Context, projectID, scopeID string) (Scope, error) {
	scopes, err := s.ListScopes(ctx, projectID)
	if err != nil {
		return Scope{}, err
	}
	normalizedID := strings.TrimSpace(scopeID)
	if normalizedID == "" {
		return scopes[0], nil
	}
	for _, scope := range scopes {
		if scope.ID == normalizedID {
			return scope, nil
		}
	}
	return Scope{}, errScopeNotFound
}

func (s *Service) resolveExisting(scope Scope, path string) (string, string, os.FileInfo, error) {
	normalized, absPath, err := resolveAbsolutePath(scope.RootPath, path)
	if err != nil {
		return "", "", nil, err
	}
	if err := ensureProtectedPath(normalized); err != nil {
		return "", "", nil, err
	}
	rootReal, err := evalOrAbs(scope.RootPath)
	if err != nil {
		return "", "", nil, err
	}
	targetReal, err := evalOrAbs(absPath)
	if err != nil {
		return "", "", nil, err
	}
	if !isWithinRoot(rootReal, targetReal) {
		return "", "", nil, fmt.Errorf("path escapes the selected scope")
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		return "", "", nil, err
	}
	return normalized, absPath, info, nil
}

func (s *Service) resolveCreatePath(scope Scope, path string) (string, error) {
	normalized, absPath, err := resolveAbsolutePath(scope.RootPath, path)
	if err != nil {
		return "", err
	}
	if err := ensureProtectedPath(normalized); err != nil {
		return "", err
	}
	rootReal, err := evalOrAbs(scope.RootPath)
	if err != nil {
		return "", err
	}
	parent := absPath
	for {
		parent = filepath.Dir(parent)
		if parent == "" {
			return "", fmt.Errorf("failed to resolve path parent")
		}
		if _, err := os.Lstat(parent); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	parentReal, err := evalOrAbs(parent)
	if err != nil {
		return "", err
	}
	if !isWithinRoot(rootReal, parentReal) {
		return "", fmt.Errorf("path escapes the selected scope")
	}
	return absPath, nil
}

func (s *Service) buildFileEntry(relativePath string, info os.FileInfo) (Entry, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return Entry{}, errUnsupportedEntry
	}
	entry := Entry{
		Name:        info.Name(),
		Path:        toSlashPath(relativePath),
		ModifiedAt:  info.ModTime(),
		Hidden:      strings.HasPrefix(info.Name(), "."),
		PreviewKind: PreviewKindBinary,
	}
	if info.IsDir() {
		entry.Kind = EntryKindDirectory
		return entry, nil
	}
	entry.Kind = EntryKindFile
	entry.Size = info.Size()
	entry.Extension = strings.ToLower(filepath.Ext(info.Name()))
	entry.Mime = detectMimeFromName(info.Name())
	entry.PreviewKind = detectPreviewKind(info.Name(), entry.Mime)
	return entry, nil
}

func (s *Service) bulkTransfer(ctx context.Context, projectID, scopeID string, sourcePaths []string, destinationPath string, move bool) (*BulkResult, error) {
	scope, err := s.scopeByID(ctx, projectID, scopeID)
	if err != nil {
		return nil, err
	}

	normalizedDest, _, destInfo, err := s.resolveExisting(scope, destinationPath)
	if err != nil {
		return nil, err
	}
	if !destInfo.IsDir() {
		return nil, fmt.Errorf("destination path is not a directory")
	}

	result := &BulkResult{}
	for _, rawSource := range sourcePaths {
		normalizedSource, sourceAbs, sourceInfo, err := s.resolveExisting(scope, rawSource)
		name := filepath.Base(normalizedSource)
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedSource),
				Name:    name,
				Message: err.Error(),
			})
			continue
		}
		if normalizedSource == "" {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    "",
				Name:    scope.Label,
				Message: "scope root cannot be moved or copied",
			})
			continue
		}
		if sourceInfo.Mode()&os.ModeSymlink != 0 {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedSource),
				Name:    sourceInfo.Name(),
				Message: errUnsupportedEntry.Error(),
			})
			continue
		}

		targetRel := joinRelativePath(normalizedDest, filepath.Base(sourceAbs))
		targetAbs, err := s.resolveCreatePath(scope, targetRel)
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedSource),
				Name:    sourceInfo.Name(),
				Message: err.Error(),
			})
			continue
		}
		if _, err := os.Lstat(targetAbs); err == nil {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedSource),
				Name:    sourceInfo.Name(),
				Message: errTargetExists.Error(),
			})
			continue
		} else if !os.IsNotExist(err) {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedSource),
				Name:    sourceInfo.Name(),
				Message: err.Error(),
			})
			continue
		}
		if sourceInfo.IsDir() && isWithinRoot(sourceAbs, targetAbs) {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedSource),
				Name:    sourceInfo.Name(),
				Message: "destination cannot be inside the source directory",
			})
			continue
		}

		if move {
			err = movePath(sourceAbs, targetAbs, sourceInfo)
		} else {
			err = copyPath(sourceAbs, targetAbs, sourceInfo)
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{
				Path:    toSlashPath(normalizedSource),
				Name:    sourceInfo.Name(),
				Message: err.Error(),
			})
			continue
		}
		result.Succeeded = append(result.Succeeded, FileRef{
			Path: toSlashPath(targetRel),
			Name: filepath.Base(targetRel),
		})
	}
	return result, nil
}

func (s *Service) cleanup(now time.Time) {
	s.cleanupUploadMetas(now)
	s.cleanupArchiveMetas(now)
}

func (s *Service) cleanupUploadMetas(now time.Time) {
	pattern := filepath.Join(s.uploadsDir, "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, path := range paths {
		var meta uploadMeta
		if err := s.readJSONFile(path, &meta); err != nil {
			continue
		}
		if !now.After(meta.ExpiresAt) {
			continue
		}
		s.deleteUploadMeta(meta)
	}
}

func (s *Service) cleanupArchiveMetas(now time.Time) {
	pattern := filepath.Join(s.archivesDir, "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, path := range paths {
		var meta archiveMeta
		if err := s.readJSONFile(path, &meta); err != nil {
			continue
		}
		if !now.After(meta.ExpiresAt) {
			continue
		}
		s.deleteArchiveMeta(meta)
	}
}

func (s *Service) uploadMetaPath(id string) string {
	return filepath.Join(s.uploadsDir, id+".json")
}

func (s *Service) archiveMetaPath(id string) string {
	return filepath.Join(s.archivesDir, id+".json")
}

func (s *Service) loadUploadMeta(id string) (uploadMeta, error) {
	var meta uploadMeta
	if err := s.readJSONFile(s.uploadMetaPath(id), &meta); err != nil {
		if os.IsNotExist(err) {
			return uploadMeta{}, errUploadNotFound
		}
		return uploadMeta{}, err
	}
	return meta, nil
}

func (s *Service) loadArchiveMeta(id string) (archiveMeta, error) {
	var meta archiveMeta
	if err := s.readJSONFile(s.archiveMetaPath(id), &meta); err != nil {
		if os.IsNotExist(err) {
			return archiveMeta{}, errArchiveNotFound
		}
		return archiveMeta{}, err
	}
	return meta, nil
}

func (s *Service) deleteUploadMeta(meta uploadMeta) {
	_ = os.Remove(meta.PartPath)
	_ = os.Remove(s.uploadMetaPath(meta.ID))
}

func (s *Service) deleteArchiveMeta(meta archiveMeta) {
	_ = os.Remove(meta.FilePath)
	_ = os.Remove(s.archiveMetaPath(meta.ID))
}

func (s *Service) withLock(id string, fn func() error) error {
	lockAny, _ := s.lockMap.LoadOrStore(id, &sync.Mutex{})
	lock := lockAny.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

func (s *Service) readJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func (s *Service) writeJSONFile(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func uploadSessionFromMeta(meta uploadMeta) *UploadSession {
	return &UploadSession{
		ID:         meta.ID,
		ProjectID:  meta.ProjectID,
		ScopeID:    meta.ScopeID,
		Directory:  meta.Directory,
		TargetPath: meta.TargetPath,
		FileName:   meta.FileName,
		Size:       meta.Size,
		Offset:     meta.Offset,
		ChunkSize:  meta.ChunkSize,
		CreatedAt:  meta.CreatedAt,
		UpdatedAt:  meta.UpdatedAt,
		ExpiresAt:  meta.ExpiresAt,
	}
}

func projectScopeID(projectID string) string {
	return "project:" + strings.TrimSpace(projectID)
}

func worktreeScopeID(worktreeID string) string {
	return "worktree:" + strings.TrimSpace(worktreeID)
}

func resolveAbsolutePath(root, relative string) (string, string, error) {
	normalized := normalizeRelativePath(relative)
	target := root
	if normalized != "" {
		target = filepath.Join(root, filepath.FromSlash(normalized))
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	if !isWithinRoot(absRoot, absTarget) {
		return "", "", fmt.Errorf("path escapes the selected scope")
	}
	return normalized, absTarget, nil
}

func normalizeRelativePath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	cleaned := pathpkg.Clean(trimmed)
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func joinRelativePath(base, name string) string {
	if strings.TrimSpace(base) == "" {
		return normalizeRelativePath(name)
	}
	if strings.TrimSpace(name) == "" {
		return normalizeRelativePath(base)
	}
	return normalizeRelativePath(base + "/" + name)
}

func parentRelativePath(value string) string {
	normalized := normalizeRelativePath(value)
	if normalized == "" {
		return ""
	}
	parent := filepath.Dir(filepath.FromSlash(normalized))
	if parent == "." {
		return ""
	}
	return toSlashPath(parent)
}

func buildBreadcrumbs(value string) []Breadcrumb {
	breadcrumbs := []Breadcrumb{{Name: "/", Path: ""}}
	normalized := normalizeRelativePath(value)
	if normalized == "" {
		return breadcrumbs
	}
	parts := strings.Split(normalized, "/")
	current := ""
	for _, part := range parts {
		current = joinRelativePath(current, part)
		breadcrumbs = append(breadcrumbs, Breadcrumb{
			Name: part,
			Path: toSlashPath(current),
		})
	}
	return breadcrumbs
}

func toSlashPath(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(value))), "./")
}

func ensureProtectedPath(relative string) error {
	normalized := normalizeRelativePath(relative)
	if normalized == "" {
		return nil
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".git" {
			return errProtectedPath
		}
	}
	return nil
}

func evalOrAbs(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Abs(path)
}

func isWithinRoot(root, target string) bool {
	rootClean := filepath.Clean(root)
	targetClean := filepath.Clean(target)
	rel, err := filepath.Rel(rootClean, targetClean)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func sanitizeEntryName(name string) (string, error) {
	baseName := filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if baseName == "" || baseName == "." || baseName == "/" {
		return "", fmt.Errorf("file name is required")
	}
	if baseName == ".git" {
		return "", errProtectedPath
	}
	if strings.Contains(baseName, string(filepath.Separator)) || strings.Contains(baseName, "/") {
		return "", fmt.Errorf("file name must not contain path separators")
	}
	return baseName, nil
}

func detectMimeFromName(name string) string {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		return ""
	}
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		return parsed
	}
	return contentType
}

func detectPreviewKind(name, mimeType string) PreviewKind {
	normalizedMime := strings.ToLower(strings.TrimSpace(mimeType))
	extension := strings.ToLower(filepath.Ext(name))

	switch {
	case strings.HasPrefix(normalizedMime, "image/"):
		return PreviewKindImage
	case normalizedMime == "application/pdf":
		return PreviewKindPDF
	case strings.HasPrefix(normalizedMime, "audio/"):
		return PreviewKindAudio
	case strings.HasPrefix(normalizedMime, "video/"):
		return PreviewKindVideo
	case normalizedMime == "text/markdown" || isMarkdownExtension(extension):
		return PreviewKindMarkdown
	case strings.HasPrefix(normalizedMime, "text/") || isTextExtension(extension):
		return PreviewKindText
	default:
		return PreviewKindBinary
	}
}

func isMarkdownExtension(extension string) bool {
	switch extension {
	case ".md", ".markdown", ".mdown":
		return true
	default:
		return false
	}
}

func isTextExtension(extension string) bool {
	switch extension {
	case ".txt", ".log", ".json", ".yaml", ".yml", ".toml", ".ini", ".env", ".go", ".js", ".ts", ".tsx", ".jsx", ".vue", ".css", ".scss", ".html", ".xml", ".sql", ".sh", ".bash", ".zsh", ".py", ".java", ".c", ".cc", ".cpp", ".h", ".hpp", ".rs", ".dockerfile", ".makefile":
		return true
	default:
		return false
	}
}

func movePath(sourceAbs, targetAbs string, info os.FileInfo) error {
	if err := os.Rename(sourceAbs, targetAbs); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	if err := copyPath(sourceAbs, targetAbs, info); err != nil {
		return err
	}
	return os.RemoveAll(sourceAbs)
}

func copyPath(sourceAbs, targetAbs string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return errUnsupportedEntry
	}
	if info.IsDir() {
		return copyDirectory(sourceAbs, targetAbs)
	}
	return copyFile(sourceAbs, targetAbs, info.Mode())
}

func copyDirectory(sourceAbs, targetAbs string) error {
	return filepath.WalkDir(sourceAbs, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errUnsupportedEntry
		}
		relative, err := filepath.Rel(sourceAbs, current)
		if err != nil {
			return err
		}
		target := targetAbs
		if relative != "." {
			target = filepath.Join(targetAbs, relative)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(current, target, info.Mode())
	})
}

func copyFile(sourceAbs, targetAbs string, mode os.FileMode) error {
	input, err := os.Open(sourceAbs)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return nil
}

func defaultArchiveName(items []archiveSource) string {
	if len(items) == 1 {
		return filepath.Base(items[0].path) + ".zip"
	}
	return fmt.Sprintf("download-%s.zip", time.Now().Format("20060102-150405"))
}

func ErrScopeNotFound() error {
	return errScopeNotFound
}

func ErrArchiveNotFound() error {
	return errArchiveNotFound
}

func ErrUploadNotFound() error {
	return errUploadNotFound
}

func ErrOffsetMismatch() error {
	return errOffsetMismatch
}

func ErrTargetExists() error {
	return errTargetExists
}

func ErrProtectedPath() error {
	return errProtectedPath
}

func ErrUnsupportedEntry() error {
	return errUnsupportedEntry
}

func writeZipEntry(zipWriter *zip.Writer, sourcePath, archiveName string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errUnsupportedEntry
	}
	if info.IsDir() {
		return filepath.Walk(sourcePath, func(current string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			lstat, err := os.Lstat(current)
			if err != nil {
				return err
			}
			if lstat.Mode()&os.ModeSymlink != 0 {
				return errUnsupportedEntry
			}
			relative, err := filepath.Rel(sourcePath, current)
			if err != nil {
				return err
			}
			targetName := archiveName
			if relative != "." {
				targetName = filepath.Join(archiveName, relative)
			}
			targetName = filepath.ToSlash(targetName)
			if lstat.IsDir() {
				if targetName != "" && !strings.HasSuffix(targetName, "/") {
					targetName += "/"
				}
				header, err := zip.FileInfoHeader(lstat)
				if err != nil {
					return err
				}
				header.Name = targetName
				_, err = zipWriter.CreateHeader(header)
				return err
			}
			return addFileToZip(zipWriter, current, targetName, lstat)
		})
	}
	return addFileToZip(zipWriter, sourcePath, filepath.ToSlash(archiveName), info)
}

func addFileToZip(zipWriter *zip.Writer, path, archiveName string, info os.FileInfo) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archiveName
	header.Method = zip.Deflate
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}
