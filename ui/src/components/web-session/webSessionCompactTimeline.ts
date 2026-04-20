import type { WebSessionBlock } from '@/stores/webSession';

interface CompactTimelineGroupItem {
  toolId: string;
  kind: string;
  title: string;
  summary: string;
  command: string;
  input?: unknown;
  output?: string;
  status: 'running' | 'done' | 'error';
  timestamp?: string;
  startedAt?: string;
  completedAt?: string;
}

const FILE_CHANGE_KIND = 'file_change';
const SYNTHETIC_FILE_CHANGE_GROUP_PREFIX = 'timeline-file-change:';
const PROJECTED_FILE_CHANGE_KEY_PREFIX = 'compact-file-change:';

export function projectWebSessionCompactTimelineBlocks(blocks: WebSessionBlock[]): WebSessionBlock[] {
  if (blocks.length === 0) {
    return blocks;
  }

  const projected: WebSessionBlock[] = [];
  for (let index = 0; index < blocks.length; ) {
    const block = blocks[index];
    if (!isFileChangeBlock(block)) {
      projected.push(block);
      index += 1;
      continue;
    }

    const explicitGroupId = getCommandGroupId(block);
    const group = [block];
    let nextIndex = index + 1;

    while (nextIndex < blocks.length) {
      const candidate = blocks[nextIndex];
      if (!isFileChangeBlock(candidate)) {
        break;
      }

      const candidateGroupId = getCommandGroupId(candidate);
      if (explicitGroupId) {
        if (candidateGroupId !== explicitGroupId) {
          break;
        }
      } else if (candidateGroupId) {
        break;
      }

      group.push(candidate);
      nextIndex += 1;
    }

    if (group.length === 1) {
      projected.push(block);
      index = nextIndex;
      continue;
    }

    const groupId = explicitGroupId || buildSyntheticGroupId(group[0], projected.length);
    projected.push(buildGroupedFileChangeBlock(group, groupId));
    index = nextIndex;
  }

  return projected;
}

function isFileChangeBlock(block: WebSessionBlock): boolean {
  return block.kind === 'tool' && block.tool?.kind === FILE_CHANGE_KIND;
}

function getCommandGroupId(block: WebSessionBlock): string {
  return String(block.tool?.commandGroup?.id || '').trim();
}

function buildSyntheticGroupId(block: WebSessionBlock, fallbackIndex: number): string {
  const anchor = String(block.id || block.key || '').trim() || `index-${fallbackIndex}`;
  return `${SYNTHETIC_FILE_CHANGE_GROUP_PREFIX}${anchor}`;
}

function buildGroupedFileChangeBlock(group: WebSessionBlock[], groupId: string): WebSessionBlock {
  const first = group[0];
  const last = group[group.length - 1];
  const projected = cloneBlock(last);
  const mergedGroupItems: CompactTimelineGroupItem[] = [];

  let count = group.length;
  let firstSeq: number | undefined;
  let lastSeq: number | undefined;

  for (const block of group) {
    const commandGroup = block.tool?.commandGroup;
    if (commandGroup?.count) {
      count = Math.max(count, Math.trunc(commandGroup.count));
    }
    if (Number.isFinite(commandGroup?.firstSeq)) {
      firstSeq =
        firstSeq == null
          ? Number(commandGroup?.firstSeq)
          : Math.min(firstSeq, Number(commandGroup?.firstSeq));
    }
    if (Number.isFinite(commandGroup?.lastSeq)) {
      lastSeq =
        lastSeq == null
          ? Number(commandGroup?.lastSeq)
          : Math.max(lastSeq, Number(commandGroup?.lastSeq));
    }
    mergeGroupItemsInto(mergedGroupItems, getBlockGroupItems(block));
  }

  count = Math.max(count, mergedGroupItems.length);

  if (!projected.tool) {
    return projected;
  }

  const nextCommandGroup = {
    ...(projected.tool.commandGroup ?? {
      id: groupId,
      count,
    }),
    id: groupId,
    count,
    latestToolId: String(projected.tool.id || last.tool?.id || '').trim() || undefined,
    compacted: true,
    ...(firstSeq != null ? { firstSeq } : {}),
    ...(lastSeq != null ? { lastSeq } : {}),
  };

  projected.key = `${PROJECTED_FILE_CHANGE_KEY_PREFIX}${groupId}`;
  projected.payload = {
    ...(projected.payload ?? {}),
    groupItems: mergedGroupItems,
  };
  projected.tool = {
    ...projected.tool,
    meta: {
      ...(projected.tool.meta ?? {}),
      commandGroup: nextCommandGroup,
    },
    commandGroup: nextCommandGroup,
  };

  // Keep the original ordering anchor from the first block in the merged run.
  projected.orderIndex = first.orderIndex;

  return projected;
}

function cloneBlock(block: WebSessionBlock): WebSessionBlock {
  return {
    ...block,
    attachments: block.attachments.map(attachment => ({ ...attachment })),
    tool: block.tool
      ? {
          ...block.tool,
          meta: block.tool.meta ? { ...block.tool.meta } : undefined,
          commandGroup: block.tool.commandGroup ? { ...block.tool.commandGroup } : undefined,
        }
      : undefined,
    payload: block.payload ? { ...block.payload } : undefined,
  };
}

function getBlockGroupItems(block: WebSessionBlock): CompactTimelineGroupItem[] {
  const payloadItems = normalizeGroupItems(block.payload?.groupItems);
  if (payloadItems.length > 0) {
    return payloadItems;
  }

  if (!block.tool) {
    return [];
  }

  const summary = resolveFileChangeSummary(block);
  const completedAt =
    block.tool.status === 'running' ? undefined : toISOString(block.observedAt ?? block.timestamp);

  return [
    {
      toolId: String(block.tool.id || '').trim(),
      kind: String(block.tool.kind || '').trim(),
      title: String(block.tool.name || '').trim(),
      summary,
      command: summary,
      input: block.tool.input,
      output: typeof block.tool.output === 'string' ? block.tool.output : undefined,
      status: normalizeStatus(block.tool.status),
      timestamp: toISOString(block.timestamp),
      startedAt: toISOString(block.tool.startedAt ?? block.timestamp),
      completedAt,
    },
  ];
}

function normalizeGroupItems(raw: unknown): CompactTimelineGroupItem[] {
  if (!Array.isArray(raw)) {
    return [];
  }

  const items: CompactTimelineGroupItem[] = [];
  for (const entry of raw) {
    const record = asRecord(entry);
    if (!record) {
      continue;
    }

    items.push({
      toolId: stringValue(record.toolId),
      kind: stringValue(record.kind),
      title: stringValue(record.title),
      summary: stringValue(record.summary),
      command: stringValue(record.command),
      input: record.input,
      output: optionalString(record.output),
      status: normalizeStatus(record.status),
      timestamp: toISOString(record.timestamp),
      startedAt: toISOString(record.startedAt),
      completedAt: toISOString(record.completedAt),
    });
  }

  return items;
}

function mergeGroupItemsInto(
  target: CompactTimelineGroupItem[],
  nextItems: CompactTimelineGroupItem[]
): void {
  for (const nextItem of nextItems) {
    const toolId = nextItem.toolId.trim();
    if (!toolId) {
      target.push(nextItem);
      continue;
    }

    const existingIndex = target.findIndex(item => item.toolId.trim() === toolId);
    if (existingIndex < 0) {
      target.push(nextItem);
      continue;
    }

    target.splice(existingIndex, 1, {
      ...target[existingIndex],
      ...nextItem,
    });
  }
}

function resolveFileChangeSummary(block: WebSessionBlock): string {
  if (!block.tool) {
    return '';
  }

  const directPath = resolveFileChangePath(block.tool.input);
  if (directPath) {
    return directPath;
  }

  return stringValue(asRecord(block.tool.meta)?.subtitle);
}

function resolveFileChangePath(raw: unknown): string {
  const record = asRecord(raw);
  if (!record) {
    return '';
  }

  const directPath = firstNonEmpty(
    stringValue(record.path),
    stringValue(record.file_path),
    stringValue(record.new_path),
    stringValue(record.old_path),
    stringValue(record.newPath),
    stringValue(record.oldPath),
    stringValue(record.move_path),
    stringValue(record.movePath)
  );
  if (directPath) {
    return directPath;
  }

  const changes = Array.isArray(record.changes) ? record.changes : [];
  for (const change of changes) {
    const changePath = resolveFileChangePath(change);
    if (changePath) {
      return changePath;
    }
  }

  return '';
}

function normalizeStatus(value: unknown): 'running' | 'done' | 'error' {
  switch (String(value || '').trim()) {
    case 'done':
    case 'completed':
      return 'done';
    case 'error':
      return 'error';
    default:
      return 'running';
  }
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined;
  }
  return value as Record<string, unknown>;
}

function optionalString(value: unknown): string | undefined {
  const normalized = stringValue(value);
  return normalized || undefined;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function firstNonEmpty(...values: string[]): string {
  for (const value of values) {
    if (value) {
      return value;
    }
  }
  return '';
}

function toISOString(value: unknown): string | undefined {
  if (typeof value === 'string') {
    const normalized = value.trim();
    return normalized || undefined;
  }

  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return undefined;
  }

  try {
    return new Date(value).toISOString();
  } catch {
    return undefined;
  }
}
