<template>
  <div class="directory-picker">
    <!-- 当前路径显示和手动输入 -->
    <div class="path-input-row">
      <n-input
        v-model:value="inputPath"
        :placeholder="t('common.enterPath')"
        size="small"
        @keyup.enter="navigateToPath"
      >
        <template #prefix>
          <n-icon><FolderOutline /></n-icon>
        </template>
      </n-input>
      <n-button size="small" :disabled="!parentPath" @click="loadDirectory(parentPath)" style="padding: 0 8px;">
        <n-icon><ArrowUpOutline /></n-icon>
      </n-button>
      <n-button size="small" @click="navigateToPath" :disabled="!inputPath">
        {{ t('common.go') }}
      </n-button>
    </div>

    <!-- 搜索条 -->
    <n-input
      v-if="directories.length > 0 || searchQuery"
      v-model:value="searchQuery"
      size="small"
      :placeholder="t('common.searchDirectory')"
      clearable
    >
      <template #prefix>
        <n-icon><SearchOutline /></n-icon>
      </template>
    </n-input>

    <!-- 目录列表 -->
    <n-spin :show="loading">
      <div class="directory-list">
        <!-- 返回上级 -->
        <div
          v-if="parentPath && !searchQuery"
          class="directory-item parent-item"
          @click="loadDirectory(parentPath)"
        >
          <n-icon :size="18"><ArrowUpOutline /></n-icon>
          <span class="dir-name">..</span>
        </div>

        <!-- 目录列表 -->
        <div
          v-for="dir in filteredDirectories"
          :key="dir.path"
          class="directory-item"
          :class="{ selected: selectedPath === dir.path }"
          @click="selectDirectory(dir)"
          @dblclick="loadDirectory(dir.path)"
        >
          <n-icon :size="18" class="folder-icon">
            <FolderOutline />
          </n-icon>
          <span class="dir-name">{{ dir.name }}</span>
          <n-icon :size="14" class="chevron-icon">
            <ChevronForwardOutline />
          </n-icon>
        </div>

        <!-- 搜索无结果 -->
        <div v-if="!loading && searchQuery && filteredDirectories.length === 0" class="empty-state">
          {{ t('common.noMatchingDirectories') }}
        </div>

        <!-- 空状态 -->
        <div v-if="!loading && directories.length === 0 && currentPath && !searchQuery" class="empty-state">
          {{ t('common.noSubdirectories') }}
        </div>

        <!-- 未加载状态 -->
        <div v-if="!loading && !currentPath" class="empty-state">
          {{ t('common.enterPathToStart') }}
        </div>
      </div>
    </n-spin>

    <!-- 选中路径显示 -->
    <div v-if="selectedPath" class="selected-path-row">
      <span class="label">{{ t('common.selectedPath') }}:</span>
      <code class="selected-path">{{ selectedPath }}</code>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useMessage } from 'naive-ui';
import { FolderOutline, ArrowUpOutline, ChevronForwardOutline, SearchOutline } from '@vicons/ionicons5';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';

interface DirEntry {
  name: string;
  path: string;
}

interface ListDirsResponse {
  dirs: DirEntry[];
  parentPath: string;
  currentPath: string;
}

const props = defineProps<{
  initialPath?: string;
}>();

const emit = defineEmits<{
  (e: 'select', path: string): void;
}>();

const { t } = useLocale();
const message = useMessage();

const loading = ref(false);
const directories = ref<DirEntry[]>([]);
const currentPath = ref('');
const parentPath = ref('');
const selectedPath = ref('');
const inputPath = ref('');
const searchQuery = ref('');

// 过滤后的目录列表
const filteredDirectories = computed(() => {
  if (!searchQuery.value.trim()) return directories.value;
  const query = searchQuery.value.toLowerCase();
  return directories.value.filter(dir => dir.name.toLowerCase().includes(query));
});

async function loadDirectory(path: string) {
  console.log('[DirectoryPicker] loadDirectory called:', path);
  if (!path) return;

  loading.value = true;
  try {
    const response = await http
      .Get<{ item?: ListDirsResponse }>('/fs/list-dirs', {
        params: { path },
        cacheFor: 0,
      })
      .send();

    console.log('[DirectoryPicker] response:', response);
    if (response?.item) {
      directories.value = response.item.dirs || [];
      currentPath.value = response.item.currentPath;
      parentPath.value = response.item.parentPath || '';
      inputPath.value = response.item.currentPath;
      searchQuery.value = ''; // 清空搜索

      // 进入目录时自动选中该目录
      selectedPath.value = response.item.currentPath;
      console.log('[DirectoryPicker] selectedPath updated to:', selectedPath.value);
      emit('select', response.item.currentPath);
    }
  } catch (error) {
    console.error('Failed to load directory:', error);
    message.error(t('common.loadFailed'));
  } finally {
    loading.value = false;
  }
}

function selectDirectory(dir: DirEntry) {
  console.log('[DirectoryPicker] selectDirectory (single click):', dir.path);
  selectedPath.value = dir.path;
  emit('select', dir.path);
}

function navigateToPath() {
  if (inputPath.value) {
    loadDirectory(inputPath.value.trim());
  }
}

// 监听 initialPath 变化
watch(() => props.initialPath, (newPath) => {
  if (newPath) {
    loadDirectory(newPath);
  }
}, { immediate: true });

// 暴露方法供外部调用
defineExpose({
  getSelectedPath: () => selectedPath.value,
  loadDirectory,
});
</script>

<style scoped>
.directory-picker {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.path-input-row {
  display: flex;
  gap: 8px;
}

.path-input-row .n-input {
  flex: 1;
}

.breadcrumb-row {
  padding: 4px 0;
  overflow-x: auto;
}

.breadcrumb-row :deep(.n-breadcrumb-item) {
  cursor: pointer;
}

.breadcrumb-row :deep(.n-breadcrumb-item:hover) {
  color: var(--n-text-color-hover);
}

.directory-list {
  min-height: 200px;
  max-height: 300px;
  overflow-y: auto;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  background: var(--n-color-embedded);
}

.directory-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  transition: background-color 0.2s;
  border-bottom: 1px solid var(--n-divider-color);
}

.directory-item:last-child {
  border-bottom: none;
}

.directory-item:hover {
  background: var(--n-color-hover);
}

.directory-item.selected {
  background: var(--n-color-hover);
  box-shadow: inset 3px 0 0 var(--n-primary-color);
}

.directory-item.parent-item {
  color: var(--n-text-color-3);
}

.folder-icon {
  color: var(--n-primary-color);
  flex-shrink: 0;
}

.dir-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.chevron-icon {
  color: var(--n-text-color-3);
  flex-shrink: 0;
}

.empty-state {
  padding: 40px 20px;
  text-align: center;
  color: var(--n-text-color-3);
  font-size: 13px;
}

.selected-path-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  background: var(--n-color-embedded);
  border-radius: 6px;
  font-size: 12px;
}

.selected-path-row .label {
  color: var(--n-text-color-3);
  flex-shrink: 0;
}

.selected-path {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: monospace;
  color: var(--n-primary-color);
}
</style>
