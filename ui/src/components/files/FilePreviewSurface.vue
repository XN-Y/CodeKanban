<template>
  <div class="file-preview-shell" :class="{ 'is-mobile': mobile }">
    <div v-if="showHeader" class="file-preview-header">
      <div class="file-preview-heading">
        <n-button
          v-if="showBackButton"
          text
          size="small"
          class="file-preview-back"
          @click="emit('close')"
        >
          {{ backLabel }}
        </n-button>
        <div v-if="headerTitle" class="file-preview-title">{{ headerTitle }}</div>
        <div v-if="headerMeta" class="file-preview-meta">{{ headerMeta }}</div>
      </div>
      <div v-if="previewResult" class="file-preview-actions">
        <n-button tertiary size="small" @click="emit('download')">
          {{ downloadLabel }}
        </n-button>
      </div>
    </div>

    <div v-if="previewLoading" class="file-preview-empty">
      <n-spin size="small" />
    </div>
    <div v-else-if="previewError" class="file-preview-empty">
      <n-alert type="error" :show-icon="false">{{ previewError }}</n-alert>
    </div>
    <template v-else-if="previewResult">
      <div class="file-preview-content">
        <img
          v-if="previewResult.previewKind === 'image'"
          :src="previewResult.inlineUrl"
          :alt="previewResult.entry.name"
          class="file-preview-image"
          @click="emit('image-preview')"
        />
        <div
          v-else-if="previewResult.previewKind === 'markdown'"
          class="file-preview-markdown chat-markdown"
          v-html="renderedMarkdown"
        ></div>
        <pre v-else-if="previewResult.previewKind === 'text'" class="file-preview-text">{{
          previewResult.textContent
        }}</pre>
        <iframe
          v-else-if="previewResult.previewKind === 'pdf'"
          :src="previewResult.inlineUrl"
          class="file-preview-frame"
          :title="previewResult.entry.name"
        ></iframe>
        <audio
          v-else-if="previewResult.previewKind === 'audio'"
          :src="previewResult.inlineUrl"
          controls
          class="file-preview-media"
        ></audio>
        <video
          v-else-if="previewResult.previewKind === 'video'"
          :src="previewResult.inlineUrl"
          controls
          class="file-preview-media"
        ></video>
        <pre
          v-else-if="previewResult.previewKind === 'binary' && previewFallbackText"
          class="file-preview-text"
          >{{ previewFallbackText }}</pre
        >
        <div v-else class="file-preview-binary">
          {{ binaryPreviewHint }}
        </div>
      </div>

      <div v-if="previewResult.truncated" class="file-preview-truncated">
        {{ previewTruncatedLabel }}
      </div>
    </template>
    <div v-else class="file-preview-empty">
      {{ emptyLabel }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { FileManagerPreviewResult } from '@/types/fileManager';

const props = withDefaults(
  defineProps<{
    previewResult: FileManagerPreviewResult | null;
    previewLoading: boolean;
    previewError: string;
    previewFallbackText: string;
    renderedMarkdown: string;
    previewMeta: string;
    emptyLabel: string;
    binaryPreviewHint: string;
    previewTruncatedLabel: string;
    downloadLabel: string;
    backLabel?: string;
    fallbackTitle?: string;
    mobile?: boolean;
    showBackButton?: boolean;
  }>(),
  {
    backLabel: '',
    fallbackTitle: '',
    mobile: false,
    showBackButton: false,
  }
);

const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'download'): void;
  (e: 'image-preview'): void;
}>();

const showHeader = computed(() => props.showBackButton || Boolean(props.previewResult));
const headerTitle = computed(() => props.previewResult?.entry.name || props.fallbackTitle);
const headerMeta = computed(() => (props.previewResult ? props.previewMeta : ''));
</script>

<style scoped>
.file-preview-shell {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

.file-preview-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 16px;
  border-bottom: 1px solid rgba(24, 35, 51, 0.08);
  background: rgba(255, 255, 255, 0.86);
}

.file-preview-shell.is-mobile .file-preview-header {
  position: sticky;
  top: 0;
  z-index: 1;
  padding-top: calc(14px + env(safe-area-inset-top, 0px));
}

.file-preview-heading {
  flex: 1;
  min-width: 0;
}

.file-preview-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.file-preview-back {
  margin: -6px 0 8px -10px;
}

.file-preview-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 15px;
  font-weight: 700;
}

.file-preview-meta {
  margin-top: 4px;
  color: rgba(34, 46, 67, 0.62);
  font-size: 12px;
}

.file-preview-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  -webkit-overflow-scrolling: touch;
  padding: 16px;
  scrollbar-width: thin;
  scrollbar-color: rgba(37, 90, 143, 0.38) transparent;
}

.file-preview-content::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.file-preview-content::-webkit-scrollbar-track {
  background: transparent;
}

.file-preview-content::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(37, 90, 143, 0.28);
}

.file-preview-content::-webkit-scrollbar-thumb:hover {
  background: rgba(37, 90, 143, 0.42);
}

.file-preview-image,
.file-preview-frame,
.file-preview-media {
  width: 100%;
  border-radius: 14px;
  background: #f4f7fb;
}

.file-preview-image {
  display: block;
  object-fit: contain;
  cursor: zoom-in;
}

.file-preview-frame {
  min-height: 420px;
  border: none;
}

.file-preview-shell.is-mobile .file-preview-frame {
  min-height: 62vh;
}

.file-preview-text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
}

.file-preview-markdown {
  font-size: 14px;
}

.file-preview-binary,
.file-preview-truncated {
  color: rgba(34, 46, 67, 0.7);
  font-size: 13px;
}

.file-preview-truncated {
  padding: 0 16px 16px;
}

.file-preview-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 220px;
  padding: 16px;
  text-align: center;
}

@media (max-width: 820px) {
  .file-preview-header {
    padding: 14px 16px;
  }

  .file-preview-title {
    font-size: 14px;
  }

  .file-preview-content {
    padding: 14px 16px calc(20px + env(safe-area-inset-bottom, 0px));
  }
}
</style>
