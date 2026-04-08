<template>
  <div class="web-session-panel" :style="webSessionStyleVars">
    <WebSessionCompletionNotifier />
    <WebSessionApprovalNotifier />

    <div class="panel-main">
      <div class="panel-body">
        <div class="panel-content">
          <div class="panel-header">
            <div v-if="isMobile && sessions.length > 0" class="mobile-tab-selector">
              <button
                type="button"
                class="mobile-nav-btn"
                :disabled="!hasPrevSession"
                @click="goToPrevSession"
              >
                <n-icon size="18">
                  <ChevronBackOutline />
                </n-icon>
              </button>
              <n-dropdown
                trigger="manual"
                placement="bottom-start"
                :show="showMobileTabSelector"
                :options="mobileTabOptions"
                @select="handleMobileTabSelect"
                @clickoutside="showMobileTabSelector = false"
              >
                <button
                  type="button"
                  class="mobile-tab-trigger"
                  @click="showMobileTabSelector = !showMobileTabSelector"
                >
                  <span class="mobile-tab-title">{{ activeSessionTitle }}</span>
                  <n-icon class="mobile-tab-arrow" :class="{ 'is-open': showMobileTabSelector }">
                    <ChevronDownOutline />
                  </n-icon>
                </button>
              </n-dropdown>
              <button
                type="button"
                class="mobile-nav-btn"
                :disabled="!hasNextSession"
                @click="goToNextSession"
              >
                <n-icon size="18">
                  <ChevronForwardOutline />
                </n-icon>
              </button>
            </div>

            <div v-else-if="sessions.length > 0" ref="tabsContainerRef" class="tabs-container">
              <n-tabs
                :value="activeSessionId"
                type="card"
                closable
                size="small"
                :theme-overrides="tabsThemeOverrides"
                @update:value="handleSessionSelect"
                @close="handleArchiveSession"
              >
                <n-tab-pane
                  v-for="session in sessions"
                  :key="session.id"
                  :name="session.id"
                  display-directive="show:lazy"
                  :tab-props="createTabProps(session)"
                >
                  <template #tab>
                    <span class="tab-label" :title="session.title">
                      <span
                        v-if="shouldShowSessionStatusDot(session)"
                        class="status-dot"
                        :class="getSessionStatusDotClass(session)"
                      ></span>
                      <span class="tab-title" :style="tabTitleStyle">{{ session.title }}</span>
                      <span
                        class="ai-status-pill"
                        :class="[
                          `state-${getSessionPillStateClass(session)}`,
                          getSessionPillSizeClass(),
                        ]"
                        :title="getSessionStatusTooltip(session)"
                      >
                        <span
                          class="ai-status-icon"
                          v-html="getSessionAssistantIcon(session)"
                        ></span>
                        <span class="ai-status-text">{{ getSessionStatusLabel(session) }}</span>
                        <span class="ai-status-emoji">{{ getSessionStatusEmoji(session) }}</span>
                      </span>
                    </span>
                  </template>
                </n-tab-pane>
              </n-tabs>
              <div class="active-tab-indicator" :style="activeTabIndicatorStyle"></div>
            </div>

            <div v-else class="empty-tabs-label">{{ emptyStateTitle }}</div>

            <n-dropdown
              trigger="manual"
              placement="bottom-start"
              :show="!!contextMenuSession"
              :options="contextMenuOptions"
              :x="contextMenuX"
              :y="contextMenuY"
              @select="handleContextMenuSelect"
              @clickoutside="contextMenuSession = null"
            />

            <div class="header-actions">
              <n-button
                secondary
                size="small"
                class="new-session-button"
                :title="t('webSession.newSession')"
                :aria-label="t('webSession.newSession')"
                @click="handleStartDraftSession()"
              >
                <template #icon>
                  <n-icon><AddOutline /></n-icon>
                </template>
              </n-button>
            </div>
          </div>

          <div v-if="currentSession" class="timeline-shell">
            <div ref="timelineScrollRef" class="timeline-scroll" @scroll="handleTimelineScroll">
              <div ref="timelineListRef" class="timeline-list">
                <div v-if="historyMeta.loading" class="history-loading">
                  {{
                    currentRealSession?.syncState === 'syncing'
                      ? t('webSession.syncLoading')
                      : t('common.loading')
                  }}
                </div>

                <div
                  v-if="showTimelineSyncLoading"
                  class="timeline-loading-shell"
                  aria-hidden="true"
                >
                  <div
                    v-for="row in timelineLoadingRows"
                    :key="row"
                    class="timeline-loading-card"
                  ></div>
                </div>

                <div v-else-if="visibleBlocks.length === 0" class="timeline-intro">
                  <span class="timeline-intro-badge">
                    {{ currentSession.agent === 'codex' ? 'Codex' : 'Claude' }}
                  </span>
                  <div class="timeline-intro-title">{{ t('webSession.readyTitle') }}</div>
                  <div class="timeline-intro-text">{{ t('webSession.readyDescription') }}</div>
                </div>

                <div
                  v-for="item in visibleBlocks"
                  :key="item.key"
                  class="timeline-item"
                  :class="`kind-${item.kind}`"
                >
                  <div v-if="!shouldHideTimelineMeta(item)" class="item-meta">
                    <span class="item-role">{{ timelineRoleLabel(item) }}</span>
                    <span class="item-time">{{ formatTime(item.timestamp) }}</span>
                  </div>

                  <div
                    v-if="item.kind === 'tool' && item.tool && isPlanTool(item.tool)"
                    class="timeline-tool-shell"
                  >
                    <div class="tool-card timeline-tool-card is-plan-tool is-static-plan-tool">
                      <div class="tool-body plan-tool-body">
                        <div class="plan-tool-header">
                          <span class="plan-tool-badge">{{ t('webSession.planCardBadge') }}</span>
                          <span class="plan-tool-caption">{{
                            t('webSession.planCardCaption')
                          }}</span>
                        </div>
                        <div
                          v-if="item.tool.output"
                          class="plan-tool-content chat-markdown"
                          v-html="renderMarkdown(item.tool.output)"
                        ></div>
                        <div v-if="showPlanActions(item.tool.id)" class="plan-tool-actions">
                          <div class="plan-tool-action-row">
                            <n-button
                              size="small"
                              type="primary"
                              class="plan-tool-action-primary"
                              @click="handlePlanCardImplement"
                            >
                              {{ t('webSession.planActionImplement') }}
                            </n-button>
                            <n-button
                              size="small"
                              secondary
                              class="plan-tool-action-secondary"
                              @click="handlePlanCardCancel"
                            >
                              {{ t('webSession.planActionCancel') }}
                            </n-button>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div v-else-if="item.kind === 'tool' && item.tool" class="timeline-tool-shell">
                    <div
                      v-if="isCompactTool(item.tool)"
                      class="tool-card timeline-tool-card command-tool-card"
                      :class="`state-${item.tool.status}`"
                    >
                      <button
                        type="button"
                        class="command-tool-button"
                        @click="openCommandExecutionDetail(item)"
                      >
                        <span class="command-tool-copy">
                          <span class="command-tool-topline">
                            <span class="command-tool-label">{{
                              compactToolLabel(item.tool)
                            }}</span>
                            <span
                              v-if="getCompactToolCount(item.tool) > 1"
                              class="command-tool-count"
                            >
                              x{{ getCompactToolCount(item.tool) }}
                            </span>
                            <span class="command-tool-time">{{ formatTime(item.timestamp) }}</span>
                          </span>
                          <span
                            class="command-tool-command"
                            :title="getCompactToolSummary(item.tool)"
                          >
                            {{
                              getCompactToolSummary(item.tool) ||
                              t('webSession.compactToolNoSummary')
                            }}
                          </span>
                        </span>
                        <span class="tool-state-badge" :class="`state-${item.tool.status}`">
                          <span class="tool-state-dot"></span>
                          {{ toolStateLabel(item.tool) }}
                        </span>
                      </button>
                    </div>

                    <div v-else class="tool-card timeline-tool-card">
                      <button
                        type="button"
                        class="tool-header"
                        @click="toggleToolExpanded(item.tool.id)"
                      >
                        <span class="tool-header-main">
                          <span class="tool-header-leading">
                            <span class="tool-kind">{{ toolKindLabel(item.tool) }}</span>
                            <span class="tool-name">{{ item.tool.name }}</span>
                          </span>
                          <span class="tool-state-badge" :class="`state-${item.tool.status}`">
                            <span class="tool-state-dot"></span>
                            {{ toolStateLabel(item.tool) }}
                          </span>
                        </span>
                        <span v-if="formatToolPreview(item.tool)" class="tool-preview">{{
                          formatToolPreview(item.tool)
                        }}</span>
                      </button>
                      <div v-if="isToolExpanded(item.tool.id)" class="tool-body">
                        <div v-if="item.tool.input" class="tool-section">
                          <div class="tool-section-label">{{ t('webSession.toolInput') }}</div>
                          <pre class="tool-code">{{ stringifyValue(item.tool.input) }}</pre>
                        </div>
                        <div v-if="item.tool.output" class="tool-section">
                          <div class="tool-section-label">{{ t('webSession.toolOutput') }}</div>
                          <pre class="tool-code">{{ item.tool.output }}</pre>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div
                    v-else-if="item.kind === 'system' && item.detail"
                    class="timeline-history-card-shell"
                  >
                    <div
                      class="approval-card history-interaction-card"
                      :class="historyInteractionCardClass(item)"
                    >
                      <div class="approval-card-header">
                        <span class="approval-badge" :class="historyInteractionBadgeClass(item)">
                          {{ historyInteractionTitle(item) }}
                        </span>
                        <span class="approval-time">{{ formatTime(item.timestamp) }}</span>
                      </div>

                      <div
                        v-if="historyInteractionPrompt(item)"
                        class="approval-prompt history-interaction-prompt"
                      >
                        {{ historyInteractionPrompt(item) }}
                      </div>

                      <div
                        v-if="item.detail.questions?.length"
                        class="history-question-list user-input-card"
                      >
                        <div
                          v-for="question in item.detail.questions"
                          :key="`${item.id}:${question.id}`"
                          class="user-input-question history-question-card"
                        >
                          <div class="user-input-question-header">
                            {{ historyQuestionTitle(question) }}
                          </div>
                          <div
                            v-if="
                              question.header &&
                              question.question &&
                              question.header !== question.question
                            "
                            class="user-input-question-copy"
                          >
                            {{ question.question }}
                          </div>
                          <div v-if="question.options.length > 0" class="history-option-list">
                            <div
                              v-for="option in question.options"
                              :key="`${question.id}:${option.label}`"
                              class="history-option-row"
                            >
                              <div class="history-option-label">{{ option.label }}</div>
                              <div v-if="option.description" class="history-option-description">
                                {{ option.description }}
                              </div>
                            </div>
                          </div>
                          <div
                            v-if="question.isOther || question.options.length === 0"
                            class="history-question-note"
                          >
                            {{
                              question.isSecret
                                ? t('webSession.historySecretInput')
                                : t('webSession.historyFreeformInput')
                            }}
                          </div>
                        </div>
                      </div>

                      <div
                        v-if="item.detail.answers?.length"
                        class="history-answer-list user-input-card"
                      >
                        <div
                          v-for="answer in item.detail.answers"
                          :key="`${item.id}:${answer.id}`"
                          class="user-input-question history-answer-card"
                        >
                          <div class="user-input-question-header">{{ answer.label }}</div>
                          <div class="history-answer-values">
                            <span
                              v-for="value in formatHistoryAnswerValues(answer)"
                              :key="`${answer.id}:${value}`"
                              class="history-answer-chip"
                            >
                              {{ value }}
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div
                    v-else
                    class="item-bubble"
                    :class="item.level ? `level-${item.level}` : undefined"
                  >
                    <div
                      v-if="item.text"
                      class="item-text chat-markdown"
                      v-html="renderMarkdown(item.text)"
                    ></div>
                    <div v-if="item.attachments.length > 0" class="attachment-row">
                      <span
                        v-for="attachment in item.attachments"
                        :key="attachment.id"
                        class="attachment-pill"
                      >
                        <n-popover
                          v-if="canPreviewAttachment(attachment)"
                          trigger="hover"
                          placement="top-start"
                          :delay="120"
                        >
                          <template #trigger>
                            <button
                              type="button"
                              class="attachment-preview-trigger"
                              :title="attachment.name"
                              @click="openAttachmentPreview(attachment)"
                            >
                              <span class="attachment-preview-trigger-text">{{
                                attachment.name
                              }}</span>
                            </button>
                          </template>
                          <div class="attachment-hover-preview">
                            <img
                              :src="getAttachmentPreviewUrl(attachment.id)"
                              :alt="attachment.name"
                              class="attachment-hover-image"
                              loading="lazy"
                            />
                          </div>
                        </n-popover>
                        <button
                          v-else
                          type="button"
                          class="attachment-preview-trigger is-static"
                          :title="attachment.name"
                        >
                          <span class="attachment-preview-trigger-text">{{ attachment.name }}</span>
                        </button>
                      </span>
                    </div>
                  </div>
                </div>

                <div v-if="showRuntimeStrip" class="runtime-strip">
                  <button
                    type="button"
                    class="live-card"
                    :class="[
                      `phase-${liveState.phase}`,
                      {
                        'show-jump-hint': showJumpToBottom,
                      },
                    ]"
                    :title="t('webSession.jumpToBottom')"
                    @click="handleLiveCardClick"
                  >
                    <div class="live-card-main">
                      <span class="live-orb"></span>
                      <div class="live-copy">
                        <div class="live-title">{{ liveStateLabel }}</div>
                        <div class="live-detail" :class="{ 'is-placeholder': !liveStateDetail }">
                          {{ liveStateSecondaryText }}
                        </div>
                      </div>
                    </div>
                    <div class="live-meta">
                      <span v-if="liveStateWorking" class="live-activity" aria-hidden="true">
                        <span class="live-activity-bar"></span>
                        <span class="live-activity-bar"></span>
                        <span class="live-activity-bar"></span>
                      </span>
                      <span v-if="showJumpToBottom" class="live-jump-hint">
                        {{ t('webSession.jumpToBottom') }}
                      </span>
                      <span class="live-time" :title="getLiveTimeTooltip(liveState)">{{
                        getLiveTimeText(liveState)
                      }}</span>
                    </div>
                  </button>

                  <div
                    v-if="pendingApproval"
                    class="approval-card"
                    :class="{ 'is-stale': pendingApproval.stale }"
                  >
                    <div class="approval-card-header">
                      <span class="approval-badge">{{ t('webSession.approvalTitle') }}</span>
                      <span class="approval-time">{{
                        formatTime(pendingApproval.requestedAt)
                      }}</span>
                    </div>
                    <div class="approval-prompt">
                      {{ pendingApproval.prompt || t('webSession.approvalPromptFallback') }}
                    </div>
                    <div v-if="pendingApproval.stale" class="approval-note">
                      {{ pendingApproval.recoveryMessage || t('webSession.recoveredRuntimeHint') }}
                    </div>
                    <div class="approval-actions">
                      <n-button
                        size="small"
                        type="primary"
                        :disabled="pendingApproval.stale"
                        @click="handleApproval('approve')"
                      >
                        {{ t('webSession.approvalApprove') }}
                      </n-button>
                      <n-button
                        size="small"
                        secondary
                        :disabled="pendingApproval.stale"
                        @click="handleApproval('reject')"
                      >
                        {{ t('webSession.approvalReject') }}
                      </n-button>
                      <n-button
                        size="small"
                        tertiary
                        :disabled="pendingApproval.stale"
                        @click="handleAbortCurrent"
                      >
                        {{ t('webSession.stop') }}
                      </n-button>
                    </div>
                  </div>

                  <div
                    v-else-if="pendingUserInput && !inlinePlanChoice"
                    class="approval-card user-input-card"
                    :class="{ 'is-stale': pendingUserInput.stale }"
                  >
                    <div class="approval-card-header">
                      <span class="approval-badge">{{ t('webSession.userInputTitle') }}</span>
                      <span class="approval-time">{{
                        formatTime(pendingUserInput.requestedAt)
                      }}</span>
                    </div>
                    <div class="approval-prompt">
                      {{ pendingUserInput.prompt || t('webSession.userInputPromptFallback') }}
                    </div>
                    <div v-if="pendingUserInput.stale" class="approval-note">
                      {{ pendingUserInput.recoveryMessage || t('webSession.recoveredRuntimeHint') }}
                    </div>
                    <div
                      v-for="question in pendingUserInput.questions"
                      :key="question.id"
                      class="user-input-question"
                    >
                      <div class="user-input-question-header">
                        {{ question.header || question.question }}
                      </div>
                      <div
                        v-if="
                          question.header &&
                          question.question &&
                          question.header !== question.question
                        "
                        class="user-input-question-copy"
                      >
                        {{ question.question }}
                      </div>
                      <n-checkbox-group
                        v-if="question.options.length > 0"
                        v-model:value="userInputSelections[question.id]"
                        :disabled="pendingUserInput.stale"
                        class="user-input-options"
                      >
                        <div
                          v-for="option in question.options"
                          :key="`${question.id}:${option.label}`"
                          class="user-input-option"
                        >
                          <n-checkbox :value="option.label">
                            <span class="user-input-option-label">{{ option.label }}</span>
                          </n-checkbox>
                          <div v-if="option.description" class="user-input-option-description">
                            {{ option.description }}
                          </div>
                        </div>
                      </n-checkbox-group>
                      <n-input
                        v-if="question.isOther || question.options.length === 0"
                        v-model:value="userInputDrafts[question.id]"
                        :type="question.isSecret ? 'password' : 'text'"
                        size="small"
                        :disabled="pendingUserInput.stale"
                        :show-password-on="question.isSecret ? 'mousedown' : undefined"
                        :placeholder="userInputPlaceholder(question)"
                        @keydown="handleUserInputEnter"
                      />
                    </div>
                    <div class="approval-actions">
                      <n-button
                        size="small"
                        type="primary"
                        :disabled="pendingUserInput.stale"
                        @click="handleUserInputSubmit"
                      >
                        {{ t('webSession.userInputSubmit') }}
                      </n-button>
                      <n-button
                        size="small"
                        tertiary
                        :disabled="pendingUserInput.stale"
                        @click="handleAbortCurrent"
                      >
                        {{ t('webSession.stop') }}
                      </n-button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="!currentSession" class="empty-state">
            <n-empty :description="emptyStateDescription" />
          </div>

          <div class="composer">
            <input
              ref="fileInputRef"
              type="file"
              accept="image/*"
              multiple
              class="hidden-file-input"
              @change="handleFileChange"
            />

            <div
              class="composer-shell"
              :class="{
                'is-drag-over': isComposerDragOver,
              }"
              @paste.capture="handleComposerPaste"
              @dragenter="handleComposerDragEnter"
              @dragover="handleComposerDragOver"
              @dragleave="handleComposerDragLeave"
              @drop="handleComposerDrop"
            >
              <div class="composer-config">
                <div class="composer-config-row">
                  <n-select
                    v-model:value="selectedAgent"
                    class="composer-select agent-select"
                    size="small"
                    :options="agentOptions"
                    :disabled="Boolean(currentSession?.nativeSessionId)"
                  />
                  <n-select
                    v-model:value="selectedModel"
                    class="composer-select model-select"
                    size="small"
                    :options="modelOptions"
                  />
                  <n-select
                    v-if="selectedAgent === 'codex'"
                    v-model:value="selectedReasoningEffort"
                    class="composer-select reasoning-select"
                    size="small"
                    :options="reasoningEffortOptions"
                  />
                  <div class="composer-mode-row">
                    <n-button-group class="composer-mode-switch">
                      <n-button
                        size="small"
                        :type="selectedWorkflowMode === 'default' ? 'primary' : 'default'"
                        @click="setWorkflowMode('default')"
                      >
                        {{ t('webSession.workflowDefault') }}
                      </n-button>
                      <n-button
                        size="small"
                        :type="selectedWorkflowMode === 'plan' ? 'primary' : 'default'"
                        @click="setWorkflowMode('plan')"
                      >
                        {{ t('webSession.workflowPlan') }}
                      </n-button>
                    </n-button-group>
                    <n-select
                      v-model:value="selectedPermissionLevel"
                      class="composer-select permission-select"
                      size="small"
                      :options="permissionLevelOptions"
                    />
                  </div>
                  <div v-if="currentSession" class="composer-path" :title="currentSession.cwd">
                    {{ currentSession.cwd }}
                  </div>
                </div>
              </div>

              <div v-if="draftAttachments.length > 0" class="draft-attachments">
                <span
                  v-for="(attachment, index) in draftAttachments"
                  :key="attachment.id"
                  class="draft-attachment-pill"
                >
                  <n-popover
                    v-if="canPreviewAttachment(attachment)"
                    trigger="hover"
                    placement="top-start"
                    :delay="120"
                  >
                    <template #trigger>
                      <button
                        type="button"
                        class="attachment-preview-trigger"
                        :title="draftAttachmentDisplayName(attachment, index)"
                        @click="openDraftAttachmentPreview(attachment, index)"
                      >
                        <span class="attachment-preview-trigger-text">{{
                          draftAttachmentDisplayName(attachment, index)
                        }}</span>
                      </button>
                    </template>
                    <div class="attachment-hover-preview">
                      <img
                        :src="getAttachmentPreviewUrl(attachment.id)"
                        :alt="attachment.name"
                        class="attachment-hover-image"
                        loading="lazy"
                      />
                    </div>
                  </n-popover>
                  <button
                    v-else
                    type="button"
                    class="attachment-preview-trigger is-static"
                    :title="draftAttachmentDisplayName(attachment, index)"
                  >
                    <span class="attachment-preview-trigger-text">{{
                      draftAttachmentDisplayName(attachment, index)
                    }}</span>
                  </button>
                  <button
                    type="button"
                    class="draft-attachment-remove"
                    @click="removeAttachment(attachment.id)"
                  >
                    ×
                  </button>
                </span>
              </div>

              <div v-if="pendingInputs.length > 0" class="pending-inputs">
                <div v-for="item in pendingInputs" :key="item.id" class="pending-input-item">
                  <span class="pending-input-badge" :class="`mode-${item.mode}`">
                    {{ pendingModeLabel(item.mode) }}
                  </span>
                  <span class="pending-input-preview">{{ pendingInputPreview(item) }}</span>
                  <button
                    type="button"
                    class="pending-input-remove"
                    @click="handleRemovePendingInput(item.id)"
                  >
                    ×
                  </button>
                </div>
              </div>

              <n-input
                ref="composerInputRef"
                v-model:value="composerText"
                type="textarea"
                class="composer-input"
                :autosize="{ minRows: 2, maxRows: 7 }"
                :placeholder="composerPlaceholder"
                @keydown.enter.exact="handleComposerEnter"
              />

              <div class="composer-footer">
                <div class="composer-footer-left">
                  <button type="button" class="composer-icon-btn" @click="openFilePicker">
                    <n-icon size="14"><ImageOutline /></n-icon>
                  </button>
                  <span class="composer-hint">{{ composerHint }}</span>
                </div>

                <div class="composer-footer-right">
                  <n-button
                    v-if="currentSession?.status === 'running'"
                    secondary
                    type="warning"
                    class="composer-stop-btn"
                    @click="handleAbortCurrent"
                  >
                    {{ t('webSession.stop') }}
                  </n-button>
                  <template v-if="canStageDuringRun">
                    <n-button secondary class="composer-queue-btn" @click="handlePreinput('queue')">
                      {{ t('webSession.preinputQueue') }}
                    </n-button>
                    <n-button
                      type="primary"
                      class="composer-send-btn"
                      @click="handlePreinput('redirect')"
                    >
                      {{ t('webSession.preinputRedirect') }}
                    </n-button>
                  </template>
                  <n-button
                    v-else
                    type="primary"
                    class="composer-send-btn"
                    :disabled="!canSend"
                    @click="handleSubmit"
                  >
                    {{ t('webSession.send') }}
                  </n-button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="showCrossProjectSidebar" ref="sidebarRootRef" class="session-sidebar-shell">
          <div
            class="terminal-resizer"
            :class="{ 'is-dragging': isSidebarResizing }"
            @mousedown="startSidebarResize"
          >
            <div class="resizer-handle"></div>
          </div>

          <aside class="session-sidebar" :style="{ width: effectiveSidebarWidthPx + 'px' }">
            <div class="session-sidebar-header">
              <div class="session-sidebar-title-wrap">
                <div class="session-sidebar-title">{{ t('webSession.allSessions') }}</div>
                <div class="session-sidebar-subtitle">
                  {{ t('webSession.crossProjectSessions') }}
                </div>
              </div>
              <span class="session-sidebar-count">{{
                crossProjectSessions.length + archivedSidebarMeta.total
              }}</span>
            </div>

            <div
              v-if="crossProjectSessions.length === 0 && crossProjectArchivedSessions.length === 0"
              class="session-sidebar-empty"
            >
              {{ t('webSession.emptyTitle') }}
            </div>

            <div v-else class="session-sidebar-list">
              <div class="session-sidebar-section">
                <div class="session-sidebar-section-header">
                  <span>{{ t('webSession.currentSessions') }}</span>
                  <span class="session-sidebar-section-count">{{
                    crossProjectSessions.length
                  }}</span>
                </div>
                <div v-if="crossProjectSessions.length === 0" class="session-sidebar-section-empty">
                  {{ t('webSession.currentSessionsEmpty') }}
                </div>
                <button
                  v-for="item in crossProjectSessions"
                  :key="`current:${item.projectId}:${item.session.id}`"
                  type="button"
                  class="session-sidebar-item"
                  :class="[
                    'session-sidebar-row',
                    ...getSidebarSessionClasses(item),
                    { 'is-active': item.isCurrent },
                  ]"
                  :style="{ '--session-sidebar-accent': getSidebarSessionAccentColor(item) }"
                  :title="`${item.projectName} · ${item.session.title}${getSidebarSessionSubtitle(item) ? ` · ${getSidebarSessionSubtitle(item)}` : ''}`"
                  @click="handleSidebarSessionSelect(item)"
                >
                  <div class="session-sidebar-main">
                    <div class="session-sidebar-title-line">
                      <span
                        class="session-sidebar-agent-icon"
                        v-html="getSessionAssistantIcon(item.session)"
                      ></span>
                      <span class="session-sidebar-item-title">{{ item.session.title }}</span>
                      <span
                        v-if="getSidebarSessionSubtitle(item)"
                        class="session-sidebar-state-text"
                      >
                        · {{ getSidebarSessionSubtitle(item) }}
                      </span>
                    </div>
                  </div>

                  <div class="session-sidebar-actions">
                    <span
                      v-if="item.projectIndex"
                      class="project-index-badge session-project-badge"
                      :class="{ 'is-single-project': isSingleSidebarProject }"
                      :style="{ '--badge-color': item.projectIndex.color }"
                    >
                      {{ item.projectIndex.index }}
                    </span>
                    <span
                      class="session-current-indicator"
                      :class="{ 'is-hidden': !item.isCurrent }"
                      :title="t('terminal.currentActiveSession')"
                    >
                      <svg
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2.5"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      >
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                        <circle cx="12" cy="12" r="3"></circle>
                      </svg>
                    </span>
                  </div>
                </button>
              </div>

              <div class="session-sidebar-section">
                <div class="session-sidebar-section-header">
                  <span>{{ t('webSession.archivedSessions') }}</span>
                  <span class="session-sidebar-section-count">{{ archivedSidebarMeta.total }}</span>
                </div>
                <div
                  v-if="crossProjectArchivedSessions.length === 0 && !archivedSidebarMeta.loading"
                  class="session-sidebar-section-empty"
                >
                  {{ t('webSession.archivedSessionsEmpty') }}
                </div>
                <div
                  v-if="archivedSidebarMeta.loading && crossProjectArchivedSessions.length === 0"
                  class="session-sidebar-section-empty"
                >
                  {{ t('common.loading') }}
                </div>
                <button
                  v-for="item in crossProjectArchivedSessions"
                  :key="`archived:${item.projectId}:${item.session.id}`"
                  type="button"
                  class="session-sidebar-item"
                  :class="[
                    'session-sidebar-row',
                    'is-archived',
                    ...getSidebarSessionClasses(item),
                    { 'is-active': item.isCurrent },
                  ]"
                  :style="{ '--session-sidebar-accent': getSidebarSessionAccentColor(item) }"
                  :title="`${item.projectName} · ${item.session.title}${getSidebarSessionSubtitle(item) ? ` · ${getSidebarSessionSubtitle(item)}` : ''}`"
                  @click="handleArchivedSidebarSessionSelect(item)"
                >
                  <div class="session-sidebar-main">
                    <div class="session-sidebar-title-line">
                      <span
                        class="session-sidebar-agent-icon"
                        v-html="getSessionAssistantIcon(item.session)"
                      ></span>
                      <span class="session-sidebar-item-title">{{ item.session.title }}</span>
                      <span
                        v-if="getSidebarSessionSubtitle(item)"
                        class="session-sidebar-state-text"
                      >
                        · {{ getSidebarSessionSubtitle(item) }}
                      </span>
                    </div>
                  </div>

                  <div class="session-sidebar-actions">
                    <span
                      v-if="item.projectIndex"
                      class="project-index-badge session-project-badge"
                      :class="{ 'is-single-project': isSingleSidebarProject }"
                      :style="{ '--badge-color': item.projectIndex.color }"
                    >
                      {{ item.projectIndex.index }}
                    </span>
                    <span class="session-archived-pill">{{ t('webSession.archivedBadge') }}</span>
                  </div>
                </button>
                <button
                  v-if="archivedSidebarMeta.hasMore"
                  type="button"
                  class="session-sidebar-load-more"
                  :disabled="archivedSidebarMeta.loading"
                  @click="handleLoadMoreArchived"
                >
                  {{
                    archivedSidebarMeta.loading
                      ? t('common.loading')
                      : t('webSession.loadMoreArchived')
                  }}
                </button>
              </div>
            </div>
          </aside>
        </div>
      </div>
    </div>

    <n-modal
      :show="showAttachmentPreview"
      preset="card"
      class="attachment-preview-modal"
      :title="activeAttachmentPreview?.name"
      :bordered="false"
      :segmented="{ content: false, footer: false }"
      :mask-closable="true"
      closable
      style="width: min(92vw, 960px)"
      @update:show="handleAttachmentPreviewVisibilityChange"
    >
      <div v-if="activeAttachmentPreview" class="attachment-preview-modal-body">
        <img
          :src="activeAttachmentPreview.url"
          :alt="activeAttachmentPreview.name"
          class="attachment-preview-modal-image"
        />
      </div>
    </n-modal>

    <n-modal
      :show="showCommandExecutionDetail"
      preset="card"
      class="command-execution-detail-modal"
      :title="commandExecutionDetailTitle"
      :bordered="false"
      :segmented="{ content: false, footer: false }"
      :mask-closable="true"
      closable
      style="width: min(92vw, 960px)"
      @update:show="handleCommandExecutionDetailVisibilityChange"
    >
      <div v-if="activeCommandExecutionDetail" class="command-execution-detail-summary">
        {{
          t('webSession.compactToolDetailCount', {
            kind: compactToolLabel(activeCommandExecutionDetail),
            count: activeCommandExecutionDetail.count,
          })
        }}
      </div>
      <div v-if="loadingCommandExecutionDetail" class="command-execution-detail-loading">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="commandExecutionDetailItems.length > 0" class="command-execution-detail-list">
        <details
          v-for="(detailItem, index) in commandExecutionDetailItems"
          :key="`${detailItem.toolId}:${detailItem.timestamp}`"
          class="command-execution-detail-item"
          :open="index === 0"
        >
          <summary class="command-execution-detail-item-summary">
            <span class="command-execution-detail-item-label">
              {{
                index === 0
                  ? t('webSession.compactToolLatest')
                  : `#${commandExecutionDetailItems.length - index}`
              }}
            </span>
            <span class="command-execution-detail-item-command">
              {{ detailItem.summary || detailItem.command || t('webSession.compactToolNoSummary') }}
            </span>
            <span class="tool-state-badge" :class="`state-${detailItem.status}`">
              <span class="tool-state-dot"></span>
              {{ toolStateLabel(detailItem) }}
            </span>
            <span class="command-execution-detail-item-time">
              {{ formatCommandExecutionDetailTime(detailItem) }}
            </span>
          </summary>

          <div class="command-execution-detail-item-body">
            <div class="tool-section">
              <div class="tool-section-label">{{ t('webSession.compactToolSummary') }}</div>
              <pre class="tool-code">{{
                detailItem.summary || detailItem.command || t('webSession.compactToolNoSummary')
              }}</pre>
            </div>
            <div v-if="showCommandExecutionInput(detailItem)" class="tool-section">
              <div class="tool-section-label">{{ t('webSession.toolInput') }}</div>
              <pre class="tool-code">{{ stringifyValue(detailItem.input) }}</pre>
            </div>
            <div v-if="detailItem.output" class="tool-section">
              <div class="tool-section-label">{{ t('webSession.toolOutput') }}</div>
              <pre class="tool-code">{{ detailItem.output }}</pre>
            </div>
          </div>
        </details>
      </div>
      <div v-else class="command-execution-detail-empty">
        {{ t('webSession.compactToolEmpty') }}
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import {
  type CSSProperties,
  computed,
  h,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  shallowRef,
  watch,
  type HTMLAttributes,
} from 'vue';
import { useRouter } from 'vue-router';
import { useDebounceFn, useResizeObserver, useStorage } from '@vueuse/core';
import { storeToRefs } from 'pinia';
import { NInput, useDialog, useMessage, type DropdownOption } from 'naive-ui';
import {
  AddOutline,
  ChevronBackOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  ImageOutline,
} from '@vicons/ionicons5';
import Sortable, { type SortableEvent } from 'sortablejs';
import { getPresetById } from '@/constants/themes';
import { useLocale } from '@/composables/useLocale';
import { useResponsive } from '@/composables/useResponsive';
import { useProjectStore } from '@/stores/project';
import { useSettingsStore } from '@/stores/settings';
import {
  useWebSessionStore,
  type WebSessionBlock,
  type WebSessionHistoryAnswerEntry,
  type WebSessionLiveState,
  type WebSessionPendingInput,
  type WebSessionUserInputOption,
  type WebSessionUserInputQuestion,
} from '@/stores/webSession';
import type { WebSessionSummary } from '@/types/models';
import {
  calculateCardTabIndicatorStyle,
  hiddenCardTabIndicatorStyle,
} from '@/utils/cardTabIndicator';
import { getAssistantIconByType } from '@/utils/assistantIcon';
import { isDarkHex } from '@/utils/color';
import { renderMarkdown } from '@/utils/markdown';
import {
  buildComposerTextWithImagePlaceholders,
  resolveImageAttachmentDisplayName,
  stripManagedComposerImagePlaceholders,
} from '@/utils/webSessionImages';
import { urlBase } from '@/api';
import { http } from '@/api/http';
import WebSessionApprovalNotifier from '@/components/web-session/WebSessionApprovalNotifier.vue';
import WebSessionCompletionNotifier from '@/components/web-session/WebSessionCompletionNotifier.vue';

const MAX_TAB_TITLE_WIDTH = 160;
const TAB_LABEL_EXTRA_SPACE = 40;
const TABS_CONTAINER_STATIC_OFFSET = 220;
const TABS_CONTAINER_MIN_OFFSET = 140;
const SHARED_WIDTH_HIDE_THRESHOLD = 860;
const SIDEBAR_STATUS_TEXT_THRESHOLD = 280;
const MIN_SESSION_SIDEBAR_WIDTH = 200;
const MAX_SESSION_SIDEBAR_WIDTH = 400;
const DEFAULT_SESSION_SIDEBAR_WIDTH = 240;
const MIN_SESSION_MAIN_WIDTH = 420;
const WEB_SESSION_CATCH_UP_SETTLE_MS = 180;
const DRAFT_SESSION_STORAGE_KEY = 'workspace-web-session-draft-tabs';
const ACTIVE_DRAFT_SESSION_STORAGE_KEY = 'workspace-web-session-active-draft';
const LIVE_TIME_TICK_MS = 1000;
const PROJECT_INDEX_COLORS = [
  '#10b981',
  '#3b82f6',
  '#f59e0b',
  '#8b5cf6',
  '#ec4899',
  '#14b8a6',
  '#ef4444',
  '#6366f1',
];

const props = withDefaults(
  defineProps<{
    projectId: string;
    showSidebar?: boolean;
    isActive?: boolean;
  }>(),
  {
    showSidebar: true,
    isActive: true,
  }
);

const liveStateClockMs = ref(Date.now());
let liveStateClockTimer: number | null = null;

type DraftSessionTab = WebSessionSummary & {
  isDraft: true;
};

type ArchivedPreviewSessionTab = WebSessionSummary & {
  isArchivedPreview: true;
};

type SessionTab =
  | (WebSessionSummary & { isDraft?: false; isArchivedPreview?: false })
  | DraftSessionTab
  | ArchivedPreviewSessionTab;

type InlinePlanChoiceOption = {
  label: string;
  isExecute: boolean;
};

type InlinePlanChoice = {
  questionId: string;
  prompt: string;
  options: InlinePlanChoiceOption[];
};

type CommandExecutionDetailItem = {
  toolId: string;
  kind: string;
  title: string;
  summary: string;
  command: string;
  input?: unknown;
  output?: string;
  status: 'running' | 'done' | 'error';
  timestamp: string;
  startedAt?: string;
  completedAt?: string;
};

type CommandExecutionDetail = {
  groupId: string;
  kind: string;
  title: string;
  summary: string;
  count: number;
  firstSeq: number;
  lastSeq: number;
  status: 'running' | 'done' | 'error';
  latestToolId?: string;
  items: CommandExecutionDetailItem[];
};

function isDraftSession(session: SessionTab | null | undefined): session is DraftSessionTab {
  return Boolean(session && 'isDraft' in session && session.isDraft);
}

function isArchivedPreviewSession(
  session: SessionTab | null | undefined
): session is ArchivedPreviewSessionTab {
  return Boolean(session && 'isArchivedPreview' in session && session.isArchivedPreview);
}

const webSessionStore = useWebSessionStore();
const projectStore = useProjectStore();
const settingsStore = useSettingsStore();
const router = useRouter();
const dialog = useDialog();
const message = useMessage();
const { t } = useLocale();
const { isMobile } = useResponsive();
const { activeTheme, currentPresetId, confirmBeforeTerminalClose, showWebSessionReasoning } =
  storeToRefs(settingsStore);
const persistedDraftSessionsByProject = useStorage<Record<string, DraftSessionTab[]>>(
  DRAFT_SESSION_STORAGE_KEY,
  {}
);
const persistedActiveDraftSessionIdByProject = useStorage<Record<string, string>>(
  ACTIVE_DRAFT_SESSION_STORAGE_KEY,
  {}
);

const tabsContainerRef = ref<HTMLElement | null>(null);
const timelineScrollRef = ref<HTMLDivElement | null>(null);
const timelineListRef = ref<HTMLDivElement | null>(null);
const fileInputRef = ref<HTMLInputElement | null>(null);
const composerInputRef = ref<InstanceType<typeof NInput> | null>(null);
const sidebarRootRef = ref<HTMLElement | null>(null);
const autoFollowBottom = ref(true);
const showJumpToBottom = ref(false);
const expandedTools = ref<Record<string, boolean>>({});
const showMobileTabSelector = ref(false);
const contextMenuSession = ref<SessionTab | null>(null);
const contextMenuX = ref(0);
const contextMenuY = ref(0);
const activeTabIndicatorStyle = ref(hiddenCardTabIndicatorStyle());
const tabsContainerWidth = ref(0);
const tabTitleMaxWidth = ref(MAX_TAB_TITLE_WIDTH);
const isComposerDragOver = ref(false);
const showAttachmentPreview = ref(false);
const activeAttachmentPreview = ref<{
  id: string;
  name: string;
  url: string;
} | null>(null);
const showCommandExecutionDetail = ref(false);
const loadingCommandExecutionDetail = ref(false);
const activeCommandExecutionDetail = ref<CommandExecutionDetail | null>(null);
const activeCommandExecutionGroupId = ref('');
const dismissedPlanActions = ref<Record<string, boolean>>({});
const userInputSelections = ref<Record<string, string[]>>({});
const userInputDrafts = ref<Record<string, string>>({});
const viewedEventSeqBySession = ref<Record<string, number>>({});
const webSessionCatchUpActive = ref(false);
const frozenBlocks = ref<WebSessionBlock[] | null>(null);
const pendingHistoryAnchor = ref<{
  sessionId: string;
  previousHeight: number;
  previousTop: number;
} | null>(null);
const tabDragSortable = shallowRef<Sortable | null>(null);
let composerDragDepth = 0;
let webSessionCatchUpTimer: number | null = null;
let webSessionCatchUpToken = 0;
const loadedSidebarProjectIds = new Set<string>();
const sidebarContainerWidth = ref(0);
const isSidebarResizing = ref(false);
const sidebarWidthPx = useStorage<number>(
  'workspace-web-session-sidebar-width',
  DEFAULT_SESSION_SIDEBAR_WIDTH
);
let sidebarResizeObserver: ResizeObserver | null = null;

const IMAGE_ATTACHMENT_NAME_PATTERN = /\.(png|jpe?g|gif|webp|bmp|svg|tiff?)$/i;

const draftAgent = ref<'claude' | 'codex'>('codex');
const draftModel = ref('gpt-5.4');
const draftReasoningEffort = ref<'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh'>('xhigh');
const draftWorkflowMode = ref<'default' | 'plan'>('default');
const draftPermissionLevel = ref<'default' | 'elevated' | 'yolo'>('elevated');
const draftSessions = ref<DraftSessionTab[]>([]);
const activeDraftSessionId = ref('');
const archivedPreviewSession = ref<ArchivedPreviewSessionTab | null>(null);

const realSessions = computed<SessionTab[]>(() =>
  webSessionStore.getSessions(props.projectId).map(session => ({
    ...session,
    isDraft: false as const,
  }))
);
const sessions = computed<SessionTab[]>(() => [
  ...realSessions.value,
  ...(archivedPreviewSession.value ? [archivedPreviewSession.value] : []),
  ...draftSessions.value,
]);
const currentSession = computed<SessionTab | null>(() => {
  if (activeDraftSessionId.value) {
    return draftSessions.value.find(session => session.id === activeDraftSessionId.value) ?? null;
  }
  if (archivedPreviewSession.value) {
    return archivedPreviewSession.value;
  }
  const activeRealId = webSessionStore.getActiveSessionId(props.projectId);
  return realSessions.value.find(session => session.id === activeRealId) ?? null;
});
const currentRealSession = computed<WebSessionSummary | null>(() => {
  const session = currentSession.value;
  return session && !isDraftSession(session) ? session : null;
});
const currentDraftSessionId = computed(() => currentSession.value?.id ?? '');
const composerRawText = computed(
  () => webSessionStore.getDraft(props.projectId, currentDraftSessionId.value).text
);
const composerText = computed({
  get: () => {
    const draft = webSessionStore.getDraft(props.projectId, currentDraftSessionId.value);
    const rawText =
      draft.attachments.length > 0 ? stripManagedComposerImagePlaceholders(draft.text) : draft.text;
    return buildComposerTextWithImagePlaceholders(rawText, draft.attachments.length);
  },
  set: value => {
    const sessionId = currentDraftSessionId.value;
    if (!sessionId) {
      return;
    }
    const attachmentCount = webSessionStore.getDraft(props.projectId, sessionId).attachments.length;
    webSessionStore.setDraftText(
      props.projectId,
      sessionId,
      attachmentCount > 0 ? stripManagedComposerImagePlaceholders(value) : value
    );
  },
});
const liveBlocks = computed(() =>
  currentRealSession.value ? webSessionStore.getBlocks(currentRealSession.value.id) : []
);
const blocks = computed(() =>
  webSessionCatchUpActive.value ? (frozenBlocks.value ?? []) : liveBlocks.value
);

function cloneBlockForFreeze(block: WebSessionBlock): WebSessionBlock {
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
    detail: block.detail
      ? {
          ...block.detail,
          questions: block.detail.questions?.map(question => ({
            ...question,
            options: question.options.map(option => ({ ...option })),
          })),
          answers: block.detail.answers?.map(answer => ({
            ...answer,
            values: [...answer.values],
          })),
        }
      : undefined,
  };
}

function snapshotBlocksForFreeze() {
  frozenBlocks.value = liveBlocks.value.map(cloneBlockForFreeze);
}

function clearWebSessionCatchUpTimer() {
  if (webSessionCatchUpTimer != null) {
    window.clearTimeout(webSessionCatchUpTimer);
    webSessionCatchUpTimer = null;
  }
}

function stopWebSessionCatchUp(reason: string) {
  clearWebSessionCatchUpTimer();
  if (!webSessionCatchUpActive.value && !frozenBlocks.value) {
    return;
  }
  webSessionCatchUpActive.value = false;
  frozenBlocks.value = null;
  webSessionCatchUpToken += 1;
  console.debug('[Web Session Catch-Up] settled', {
    sessionId: currentRealSession.value?.id,
    reason,
  });
}

function isDocumentVisible() {
  return typeof document === 'undefined' || document.visibilityState === 'visible';
}

function beginWebSessionCatchUp(reason: string) {
  if (!currentRealSession.value) {
    return;
  }
  if (!webSessionCatchUpActive.value) {
    snapshotBlocksForFreeze();
    webSessionCatchUpActive.value = true;
  }
  clearWebSessionCatchUpTimer();
  console.debug('[Web Session Catch-Up] start', {
    sessionId: currentRealSession.value.id,
    reason,
  });
}

async function refreshWebSessionCatchUp(reason: string) {
  const sessionId = currentRealSession.value?.id;
  if (!sessionId) {
    stopWebSessionCatchUp(`${reason}-no-session`);
    return;
  }

  beginWebSessionCatchUp(reason);
  const token = ++webSessionCatchUpToken;

  try {
    if (props.projectId) {
      await webSessionStore.loadSessions(props.projectId, true);
    }
    await webSessionStore.refreshSessionSnapshot(sessionId);
  } catch (error) {
    console.warn('[Web Session Catch-Up] Failed to refresh session snapshot', {
      sessionId,
      reason,
      error,
    });
  }

  if (token !== webSessionCatchUpToken) {
    return;
  }

  clearWebSessionCatchUpTimer();
  webSessionCatchUpTimer = window.setTimeout(() => {
    if (token !== webSessionCatchUpToken) {
      return;
    }
    stopWebSessionCatchUp(reason);
    nextTick(() => {
      const container = timelineScrollRef.value;
      if (!container) {
        return;
      }
      if (autoFollowBottom.value) {
        syncScrollToBottom();
      } else {
        updateBottomState(container);
      }
      markSessionViewed(sessionId);
    });
  }, WEB_SESSION_CATCH_UP_SETTLE_MS);
}

function handleWebSessionDocumentVisibilityChange() {
  if (!isDocumentVisible()) {
    beginWebSessionCatchUp('document-hidden');
    return;
  }
  void refreshWebSessionCatchUp('document-visible');
}

function handleWebSessionWindowFocus() {
  if (!isDocumentVisible()) {
    return;
  }
  void refreshWebSessionCatchUp('window-focus');
}

function handleWebSessionWindowPageShow() {
  if (!isDocumentVisible()) {
    return;
  }
  void refreshWebSessionCatchUp('window-pageshow');
}

function isReasoningBlock(block: WebSessionBlock) {
  if (!block.tool) {
    return false;
  }
  return (
    normalizeToolKindValue(block.tool.kind) === 'reasoning' ||
    normalizeToolKindValue(String(block.tool.meta?.kind ?? '')) === 'reasoning'
  );
}

function hasReasoningContent(block: WebSessionBlock) {
  if (!isReasoningBlock(block)) {
    return false;
  }
  return Boolean(block.tool?.output?.trim());
}
function normalizeChoiceText(value: string) {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, ' ');
}
function isPlanTool(tool?: {
  name: string;
  kind?: string;
  meta?: Record<string, unknown> | undefined;
}) {
  if (!tool) {
    return false;
  }
  const meta = tool.meta ?? {};
  const candidates: string[] = [
    tool.name,
    tool.kind ?? '',
    typeof meta.kind === 'string' ? meta.kind : '',
    typeof meta.title === 'string' ? meta.title : '',
  ];
  return candidates.some(value => normalizeChoiceText(value) === 'plan');
}
function isExecutePlanOption(option: WebSessionUserInputOption) {
  const text = normalizeChoiceText(`${option.label} ${option.description}`);
  const mentionsPlan = /计划|plan/.test(text);
  const mentionsExecute = /开始|执行|实现|实施|继续|start|execute|implement|proceed/.test(text);
  const mentionsCancel = /取消|暂不|稍后|later|cancel|dismiss|hold/.test(text);
  return mentionsExecute && (mentionsPlan || !mentionsCancel);
}
function isCancelPlanOption(option: WebSessionUserInputOption) {
  const text = normalizeChoiceText(`${option.label} ${option.description}`);
  return /取消|暂不|稍后|稍后再说|later|cancel|dismiss|hold|keep planning|stay in plan/.test(text);
}
function isPlanChoiceQuestion(question?: WebSessionUserInputQuestion) {
  if (!question || question.options.length !== 2) {
    return false;
  }
  const hasExecute = question.options.some(isExecutePlanOption);
  const hasCancel = question.options.some(isCancelPlanOption);
  return hasExecute && hasCancel;
}
function isPlanChoiceRequestBlock(block: WebSessionBlock) {
  return (
    block.kind === 'system' &&
    block.detail?.type === 'user_input_request' &&
    isPlanChoiceQuestion(block.detail.questions?.[0])
  );
}

const currentRunStartIndex = computed(() => {
  for (let index = blocks.value.length - 1; index >= 0; index -= 1) {
    if (blocks.value[index].kind === 'user') {
      return index;
    }
  }
  return -1;
});

function shouldRenderToolBlockInTimeline(block: WebSessionBlock, index: number) {
  if (block.kind !== 'tool' || !block.tool) {
    return true;
  }
  if (isReasoningBlock(block)) {
    return hasReasoningContent(block);
  }
  const activeToolGroupId = liveState.value.tool?.groupId || '';
  const activeToolId = liveState.value.tool?.id || '';
  const blockGroupId = block.tool.commandGroup?.id || '';
  if (
    liveState.value.running &&
    block.tool.status === 'running' &&
    ((activeToolGroupId && blockGroupId === activeToolGroupId) ||
      (activeToolId && block.tool.id === activeToolId))
  ) {
    return false;
  }
  return true;
}

const visibleBlocks = computed(() =>
  blocks.value.filter((block, index) => {
    if (!showWebSessionReasoning.value && isReasoningBlock(block)) {
      return false;
    }
    if (isPlanChoiceRequestBlock(block)) {
      return false;
    }
    if (!shouldRenderToolBlockInTimeline(block, index)) {
      return false;
    }
    return true;
  })
);
const latestPlanToolId = computed(() => {
  for (let index = blocks.value.length - 1; index >= 0; index -= 1) {
    const block = blocks.value[index];
    if (block?.kind === 'tool' && block.tool && isPlanTool(block.tool)) {
      return block.tool.id;
    }
  }
  return '';
});
const hasUserMessageAfterLatestPlan = computed(() => {
  const planToolId = latestPlanToolId.value;
  if (!planToolId) {
    return false;
  }
  const planIndex = blocks.value.findIndex(
    block => block.kind === 'tool' && block.tool?.id === planToolId
  );
  if (planIndex < 0) {
    return false;
  }
  return blocks.value.slice(planIndex + 1).some(block => block.kind === 'user');
});
const liveState = computed(() =>
  currentRealSession.value
    ? webSessionStore.getLiveState(currentRealSession.value.id)
    : ({ phase: 'idle', running: false, updatedAt: Date.now() } as WebSessionLiveState)
);
const pendingApproval = computed(() =>
  currentRealSession.value ? webSessionStore.getPendingApproval(currentRealSession.value.id) : null
);
const pendingUserInput = computed(() =>
  currentRealSession.value ? webSessionStore.getPendingUserInput(currentRealSession.value.id) : null
);
const inlinePlanChoice = computed<InlinePlanChoice | null>(() => {
  const request = pendingUserInput.value;
  if (!request || request.stale || !latestPlanToolId.value) {
    return null;
  }
  const question = request.questions[0];
  if (request.questions.length !== 1 || !isPlanChoiceQuestion(question)) {
    return null;
  }
  return {
    questionId: question.id,
    prompt: request.prompt?.trim() || question.question?.trim() || question.header?.trim() || '',
    options: question.options.map(option => ({
      label: option.label,
      isExecute: isExecutePlanOption(option),
    })),
  };
});
const isPlanWaitingApprovalState = computed(
  () =>
    liveState.value.phase === 'waiting_approval' &&
    !pendingApproval.value &&
    Boolean(latestPlanToolId.value) &&
    !hasUserMessageAfterLatestPlan.value
);
const showRuntimeStrip = computed(() => {
  if (pendingApproval.value || pendingUserInput.value) {
    return true;
  }
  if (isPlanWaitingApprovalState.value) {
    return false;
  }
  if (liveState.value.phase === 'idle') {
    return false;
  }
  if (
    liveState.value.phase === 'done' &&
    latestPlanToolId.value &&
    !hasUserMessageAfterLatestPlan.value
  ) {
    return false;
  }
  return true;
});
const hasRecoveredRuntimeRequest = computed(() =>
  Boolean(pendingApproval.value?.stale || pendingUserInput.value?.stale)
);
const recoveredRuntimeHint = computed(
  () =>
    pendingApproval.value?.recoveryMessage ||
    pendingUserInput.value?.recoveryMessage ||
    t('webSession.recoveredRuntimeHint')
);
const historyMeta = computed(() =>
  currentRealSession.value
    ? webSessionStore.getHistoryMeta(currentRealSession.value.id)
    : { hasMore: false, beforeCursor: '', total: 0, loading: false }
);
const showTimelineSyncLoading = computed(
  () =>
    Boolean(currentRealSession.value) &&
    visibleBlocks.value.length === 0 &&
    (historyMeta.value.loading || currentRealSession.value?.syncState === 'syncing')
);
const timelineLoadingRows = [1, 2, 3, 4, 5];
const draftAttachments = computed(() =>
  webSessionStore.getDraftAttachments(props.projectId, currentDraftSessionId.value)
);
function draftAttachmentDisplayName(attachment: { name: string }, index: number) {
  return resolveImageAttachmentDisplayName(attachment.name, index + 1);
}
function openDraftAttachmentPreview(
  attachment: { id: string; name: string; mime?: string },
  index: number
) {
  openAttachmentPreview({
    ...attachment,
    name: draftAttachmentDisplayName(attachment, index),
  });
}
const pendingInputs = computed(() =>
  currentRealSession.value ? webSessionStore.getPendingInputs(currentRealSession.value.id) : []
);
const currentSessionLatestEventSeq = computed(() =>
  currentRealSession.value ? webSessionStore.getLatestEventSeq(currentRealSession.value.id) : 0
);
const isRunActive = computed(() => Boolean(currentSession.value?.status === 'running'));
const hasDraftContent = computed(
  () => composerRawText.value.trim().length > 0 || draftAttachments.value.length > 0
);
const canSend = computed(() => !isRunActive.value && hasDraftContent.value);
const canStageDuringRun = computed(() => isRunActive.value && hasDraftContent.value);
const composerPlaceholder = computed(() => t('webSession.inputPlaceholder'));
const composerHint = computed(() => {
  if (hasRecoveredRuntimeRequest.value) {
    return t('webSession.composerHintRecovered');
  }
  if (pendingApproval.value || liveState.value.phase === 'waiting_approval') {
    return t('webSession.composerHintApproval');
  }
  if (pendingUserInput.value) {
    return t('webSession.composerHintUserInput');
  }
  if (liveState.value.running) {
    return t('webSession.composerHintRunning');
  }
  return t('webSession.composerHintIdle');
});
const liveStateLabel = computed(() => {
  if (hasRecoveredRuntimeRequest.value) {
    return t('webSession.liveRecovered');
  }
  switch (liveState.value.phase) {
    case 'starting':
      return t('webSession.liveStarting');
    case 'thinking':
      return t('webSession.liveThinking');
    case 'tool':
      if (isCompactToolKind(liveState.value.tool?.kind)) {
        const count = Math.max(1, Number(liveState.value.tool?.count ?? 1) || 1);
        const label = compactToolLabel(liveState.value.tool);
        const toolLabel = count > 1 ? `${label} x${count}` : label;
        return t('webSession.liveTool', { tool: toolLabel });
      }
      return t('webSession.liveTool', { tool: liveState.value.tool?.name || 'Tool' });
    case 'waiting_approval':
      return t('webSession.liveWaitingApproval');
    case 'waiting_input':
      return t('webSession.liveWaitingInput');
    case 'done':
      return t('webSession.liveDone');
    case 'error':
      return t('webSession.liveError');
    default:
      return t('webSession.liveIdle');
  }
});
const liveStateDetail = computed(() => {
  if (hasRecoveredRuntimeRequest.value) {
    return recoveredRuntimeHint.value;
  }
  if (pendingApproval.value?.prompt) {
    return pendingApproval.value.prompt;
  }
  if (liveState.value.phase === 'waiting_approval') {
    return t('webSession.liveWaitingApprovalDetail');
  }
  if (pendingUserInput.value?.prompt) {
    return pendingUserInput.value.prompt;
  }
  if (liveState.value.phase === 'tool' && liveState.value.tool?.summary) {
    return liveState.value.tool.summary;
  }
  if (liveState.value.phase === 'tool' && liveState.value.tool?.kind) {
    return liveState.value.tool.kind;
  }
  if (liveState.value.phase === 'error' && liveState.value.errorMessage) {
    return liveState.value.errorMessage;
  }
  return '';
});
const liveStateSecondaryText = computed(() => {
  if (liveStateDetail.value) {
    return liveStateDetail.value;
  }
  switch (liveState.value.phase) {
    case 'starting':
      return t('webSession.liveStartingDetail');
    case 'thinking':
      return t('webSession.liveThinkingDetail');
    case 'tool':
      return compactToolLabel(liveState.value.tool);
    case 'waiting_approval':
      return t('webSession.liveWaitingApprovalDetail');
    case 'waiting_input':
      return t('webSession.liveWaitingInputDetail');
    case 'done':
      return t('webSession.liveDoneDetail');
    case 'error':
      return t('webSession.liveErrorDetail');
    default:
      return t('webSession.liveIdleDetail');
  }
});
const liveStateWorking = computed(() =>
  ['starting', 'thinking', 'tool'].includes(liveState.value.phase)
);
const activeSessionId = computed(() => currentSession.value?.id ?? '');
const emptyStateTitle = computed(() => t('webSession.draftTitle'));
const emptyStateDescription = computed(() => t('webSession.draftDescription'));
const activeSessionTitle = computed(() => currentSession.value?.title ?? emptyStateTitle.value);
const showCrossProjectSidebar = computed(() => !isMobile.value && props.showSidebar);
const currentSessionIndex = computed(() =>
  sessions.value.findIndex(session => session.id === activeSessionId.value)
);
const hasPrevSession = computed(() => currentSessionIndex.value > 0);
const hasNextSession = computed(
  () => currentSessionIndex.value >= 0 && currentSessionIndex.value < sessions.value.length - 1
);

watch(
  pendingUserInput,
  value => {
    if (!value) {
      userInputSelections.value = {};
      userInputDrafts.value = {};
      return;
    }
    const nextSelections: Record<string, string[]> = {};
    const nextDrafts: Record<string, string> = {};
    value.questions.forEach(question => {
      nextSelections[question.id] = [...(userInputSelections.value[question.id] ?? [])];
      nextDrafts[question.id] = userInputDrafts.value[question.id] ?? '';
    });
    userInputSelections.value = nextSelections;
    userInputDrafts.value = nextDrafts;
  },
  { immediate: true }
);

const mobileTabOptions = computed<DropdownOption[]>(() =>
  sessions.value.map(session => ({
    label: session.title,
    key: session.id,
  }))
);
const contextMenuOptions = computed<DropdownOption[]>(() => [
  {
    label: t('webSession.newSession'),
    key: 'new',
  },
  {
    label: t('common.edit'),
    key: 'rename',
    disabled: !contextMenuSession.value,
  },
  {
    label: t('webSession.syncFromTerminal'),
    key: 'sync',
    disabled:
      !contextMenuSession.value ||
      isDraftSession(contextMenuSession.value) ||
      isArchivedPreviewSession(contextMenuSession.value) ||
      contextMenuSession.value.agent !== 'codex' ||
      !contextMenuSession.value.nativeSessionId,
  },
  {
    label: t('webSession.archiveAction'),
    key: 'archive',
    disabled:
      !contextMenuSession.value ||
      isDraftSession(contextMenuSession.value) ||
      isArchivedPreviewSession(contextMenuSession.value),
  },
  {
    label: t('common.delete'),
    key: 'delete',
    disabled: !contextMenuSession.value,
  },
]);
const tabsThemeOverrides = computed(() => {
  const theme = activeTheme.value;
  const preset = getPresetById(currentPresetId.value);
  const tabBg = theme.terminalTabBg || preset?.colors.terminalTabBg || theme.bodyColor;
  const tabActiveBg =
    theme.terminalTabActiveBg || preset?.colors.terminalTabActiveBg || theme.surfaceColor;
  return {
    tabColor: tabBg,
    tabColorSegment: tabActiveBg,
  };
});
const approvalColors = computed(() => {
  const theme = activeTheme.value;
  const isDarkTheme = isDarkHex(theme.bodyColor || '#ffffff');
  return {
    bg: isDarkTheme ? 'rgba(251, 146, 60, 0.18)' : 'rgba(249, 115, 22, 0.14)',
    border: isDarkTheme ? 'rgba(251, 146, 60, 0.4)' : 'rgba(249, 115, 22, 0.3)',
    accent: isDarkTheme ? '#fb923c' : '#f97316',
    accentStrong: isDarkTheme ? '#f97316' : '#ea580c',
    glow: isDarkTheme ? 'rgba(251, 146, 60, 0.24)' : 'rgba(249, 115, 22, 0.16)',
  };
});
const webSessionStyleVars = computed(
  () =>
    ({
      '--web-session-approval-bg': approvalColors.value.bg,
      '--web-session-approval-border': approvalColors.value.border,
      '--web-session-approval-accent': approvalColors.value.accent,
      '--web-session-approval-accent-strong': approvalColors.value.accentStrong,
      '--web-session-approval-glow': approvalColors.value.glow,
    }) as CSSProperties
);
const tabTitleStyle = computed(() => ({
  maxWidth: `${tabTitleMaxWidth.value}px`,
}));
const timelineContentVersion = computed(() =>
  visibleBlocks.value
    .map(block => {
      const toolVersion = block.tool
        ? `${block.tool.id}:${block.tool.status}:${String(block.tool.output ?? '').length}`
        : '';
      return `${block.key}:${block.kind}:${block.text.length}:${block.attachments.length}:${toolVersion}:${block.done ? 1 : 0}`;
    })
    .join('|')
);
const sidebarProjectIdsToLoad = computed(() => {
  const ids = new Set<string>();
  if (props.projectId) {
    ids.add(props.projectId);
  }
  projectStore.recentProjects.forEach(project => {
    if (project.id) {
      ids.add(project.id);
    }
  });
  projectStore.projects.forEach(project => {
    if (project.id) {
      ids.add(project.id);
    }
  });
  return Array.from(ids);
});

function parseTimestamp(value?: string | null) {
  if (!value) {
    return 0;
  }
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function fallbackDraftTitle(agent: 'claude' | 'codex') {
  const baseAgent = agent === 'claude' ? 'Claude' : 'Codex';
  const projectName = projectStore.currentProject?.name?.trim();
  return projectName ? `${baseAgent} · ${projectName}` : baseAgent;
}

function normalizeDraftSession(
  session: Partial<DraftSessionTab>,
  index: number,
  projectId: string
): DraftSessionTab | null {
  const id = String(session.id || '').trim();
  if (!id) {
    return null;
  }
  const agent = session.agent === 'claude' ? 'claude' : 'codex';
  const nowIso = new Date().toISOString();
  return {
    id,
    projectId,
    worktreeId: typeof session.worktreeId === 'string' ? session.worktreeId || null : null,
    orderIndex: Number.MAX_SAFE_INTEGER - index,
    agent,
    title:
      typeof session.title === 'string' && session.title.trim()
        ? session.title.trim()
        : fallbackDraftTitle(agent),
    model:
      typeof session.model === 'string' && session.model.trim()
        ? session.model.trim()
        : defaultModelForAgent(agent),
    reasoningEffort:
      session.reasoningEffort === 'default' ||
      session.reasoningEffort === 'none' ||
      session.reasoningEffort === 'low' ||
      session.reasoningEffort === 'medium' ||
      session.reasoningEffort === 'high' ||
      session.reasoningEffort === 'xhigh'
        ? session.reasoningEffort
        : defaultReasoningEffortForAgent(agent),
    workflowMode: session.workflowMode === 'plan' ? 'plan' : 'default',
    permissionLevel:
      session.permissionLevel === 'default' ||
      session.permissionLevel === 'elevated' ||
      session.permissionLevel === 'yolo'
        ? session.permissionLevel
        : 'elevated',
    cwd: typeof session.cwd === 'string' ? session.cwd : projectStore.currentProject?.path || '',
    nativeSessionId: null,
    status: 'idle',
    hasUnread: false,
    archivedAt: null,
    activityAt:
      typeof session.activityAt === 'string' && session.activityAt.trim()
        ? session.activityAt
        : nowIso,
    lastMessageAt: null,
    sourceKind: typeof session.sourceKind === 'string' ? session.sourceKind : 'codex_app_server',
    syncState:
      session.syncState === 'fresh' ||
      session.syncState === 'stale' ||
      session.syncState === 'missing' ||
      session.syncState === 'syncing' ||
      session.syncState === 'error'
        ? session.syncState
        : 'missing',
    sourceCreatedAt: null,
    sourceUpdatedAt: null,
    lastSyncedAt: null,
    threadPath: null,
    threadPreview: null,
    turnCount: 0,
    itemCount: 0,
    syncError: null,
    createdAt:
      typeof session.createdAt === 'string' && session.createdAt.trim()
        ? session.createdAt
        : nowIso,
    updatedAt:
      typeof session.updatedAt === 'string' && session.updatedAt.trim()
        ? session.updatedAt
        : nowIso,
    usage: {
      inputTokens: 0,
      cachedInputTokens: 0,
      outputTokens: 0,
      cost: 0,
    },
    isDraft: true,
  };
}

function loadPersistedDraftSessions(projectId: string) {
  const stored = persistedDraftSessionsByProject.value[projectId];
  if (!Array.isArray(stored) || stored.length === 0) {
    return [];
  }
  const seen = new Set<string>();
  return stored
    .map((session, index) => normalizeDraftSession(session, index, projectId))
    .filter((session): session is DraftSessionTab => {
      if (!session || seen.has(session.id)) {
        return false;
      }
      seen.add(session.id);
      return true;
    });
}

function persistDraftSessionState(
  projectId: string,
  nextDrafts = draftSessions.value,
  nextActiveDraftId = activeDraftSessionId.value
) {
  if (!projectId) {
    return;
  }
  const normalizedDrafts = nextDrafts
    .map((session, index) => normalizeDraftSession(session, index, projectId))
    .filter((session): session is DraftSessionTab => Boolean(session));
  persistedDraftSessionsByProject.value = normalizedDrafts.length
    ? {
        ...persistedDraftSessionsByProject.value,
        [projectId]: normalizedDrafts,
      }
    : Object.fromEntries(
        Object.entries(persistedDraftSessionsByProject.value).filter(([key]) => key !== projectId)
      );
  const normalizedActiveDraftId = normalizedDrafts.some(session => session.id === nextActiveDraftId)
    ? nextActiveDraftId
    : '';
  persistedActiveDraftSessionIdByProject.value = normalizedActiveDraftId
    ? {
        ...persistedActiveDraftSessionIdByProject.value,
        [projectId]: normalizedActiveDraftId,
      }
    : Object.fromEntries(
        Object.entries(persistedActiveDraftSessionIdByProject.value).filter(
          ([key]) => key !== projectId
        )
      );
}

function replaceDraftSessionState(
  nextDrafts: DraftSessionTab[],
  nextActiveDraftId: string,
  projectId = props.projectId
) {
  draftSessions.value = nextDrafts;
  activeDraftSessionId.value = nextActiveDraftId;
  persistDraftSessionState(projectId, nextDrafts, nextActiveDraftId);
}

function resolveDraftContext(worktreeId?: string | null) {
  const normalizedWorktreeId = String(worktreeId || '').trim();
  const worktree = normalizedWorktreeId
    ? projectStore.worktrees.find(item => item.id === normalizedWorktreeId)
    : null;
  return {
    worktreeId: worktree?.id ?? (normalizedWorktreeId || null),
    cwd: worktree?.path || projectStore.currentProject?.path || currentSession.value?.cwd || '',
  };
}

function buildDraftTitle(agent: 'claude' | 'codex') {
  const baseTitle = fallbackDraftTitle(agent);
  const samePrefixCount = draftSessions.value.filter(
    session => session.title === baseTitle || session.title.startsWith(`${baseTitle} `)
  ).length;
  return samePrefixCount > 0 ? `${baseTitle} ${samePrefixCount + 1}` : baseTitle;
}

function updateDraftSession(draftId: string, updater: (draft: DraftSessionTab) => DraftSessionTab) {
  replaceDraftSessionState(
    draftSessions.value.map(session => (session.id === draftId ? updater(session) : session)),
    activeDraftSessionId.value
  );
}

function updateActiveDraftSession(updater: (draft: DraftSessionTab) => DraftSessionTab) {
  if (!activeDraftSessionId.value) {
    return;
  }
  updateDraftSession(activeDraftSessionId.value, updater);
}

function createDraftSession(forceAgent?: 'claude' | 'codex') {
  const source = currentSession.value;
  const nextAgent = forceAgent ?? source?.agent ?? draftAgent.value;
  const context = resolveDraftContext(
    source?.worktreeId ?? projectStore.selectedWorktreeId ?? null
  );
  const nowIso = new Date().toISOString();
  const draft: DraftSessionTab = {
    id: `draft_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    projectId: props.projectId,
    worktreeId: context.worktreeId,
    orderIndex: Number.MAX_SAFE_INTEGER - draftSessions.value.length,
    agent: nextAgent,
    title: buildDraftTitle(nextAgent),
    model: source?.model || draftModel.value || defaultModelForAgent(nextAgent),
    reasoningEffort:
      source?.reasoningEffort ||
      draftReasoningEffort.value ||
      defaultReasoningEffortForAgent(nextAgent),
    workflowMode: source?.workflowMode || draftWorkflowMode.value,
    permissionLevel: source?.permissionLevel || draftPermissionLevel.value,
    cwd: context.cwd,
    nativeSessionId: null,
    status: 'idle',
    hasUnread: false,
    archivedAt: null,
    activityAt: nowIso,
    lastMessageAt: null,
    sourceKind: 'codex_app_server',
    syncState: 'missing',
    sourceCreatedAt: null,
    sourceUpdatedAt: null,
    lastSyncedAt: null,
    threadPath: null,
    threadPreview: null,
    turnCount: 0,
    itemCount: 0,
    syncError: null,
    createdAt: nowIso,
    updatedAt: nowIso,
    usage: {
      inputTokens: 0,
      cachedInputTokens: 0,
      outputTokens: 0,
      cost: 0,
    },
    isDraft: true,
  };
  replaceDraftSessionState([...draftSessions.value, draft], draft.id);
  webSessionStore.setActiveSession(props.projectId, '');
  return draft;
}

function ensureDefaultDraftSession() {
  if (
    realSessions.value.length > 0 ||
    draftSessions.value.length > 0 ||
    archivedPreviewSession.value
  ) {
    return;
  }
  createDraftSession();
}

function clearArchivedPreviewSession() {
  archivedPreviewSession.value = null;
}

async function openArchivedPreviewSession(session: WebSessionSummary) {
  clearArchivedPreviewSession();
  replaceDraftSessionState(draftSessions.value, '');
  archivedPreviewSession.value = {
    ...session,
    isArchivedPreview: true,
  };
  await webSessionStore.refreshSessionSnapshot(session.id);
}

function activateRealSession(sessionId: string, connect = true) {
  const targetSession = realSessions.value.find(session => session.id === sessionId);
  if (!targetSession) {
    return false;
  }
  clearArchivedPreviewSession();
  replaceDraftSessionState(draftSessions.value, '');
  if (connect) {
    void webSessionStore.ensureSessionConnected(props.projectId, targetSession.id);
  } else {
    webSessionStore.setActiveSession(props.projectId, targetSession.id);
  }
  return true;
}

function removeDraftSession(
  sessionId: string,
  options?: { nextRealSessionId?: string; preserveDraftState?: boolean }
) {
  const nextDrafts = draftSessions.value.filter(session => session.id !== sessionId);
  const removedActive = activeDraftSessionId.value === sessionId;
  const nextActiveDraftId = removedActive
    ? (nextDrafts[nextDrafts.length - 1]?.id ?? '')
    : activeDraftSessionId.value;
  replaceDraftSessionState(nextDrafts, nextActiveDraftId);
  if (!options?.preserveDraftState) {
    webSessionStore.clearDraft(props.projectId, sessionId);
  }
  if (!removedActive) {
    return;
  }
  const nextActiveDraft = nextDrafts[nextDrafts.length - 1] ?? null;
  if (!nextActiveDraft) {
    if (options?.nextRealSessionId && activateRealSession(options.nextRealSessionId, false)) {
      return;
    }
    const nextRealSessionId = realSessions.value[0]?.id;
    if (nextRealSessionId && activateRealSession(nextRealSessionId)) {
      return;
    }
    ensureDefaultDraftSession();
  }
}

function getSessionActivityTimestamp(session: WebSessionSummary) {
  return parseTimestamp(
    session.activityAt || session.lastMessageAt || session.updatedAt || session.createdAt
  );
}

function markSessionViewed(sessionId?: string) {
  const normalizedSessionId = String(sessionId || '').trim();
  if (!props.isActive || !normalizedSessionId) {
    return;
  }

  const latestSeq = webSessionStore.getLatestEventSeq(normalizedSessionId);
  const previousViewedSeq = viewedEventSeqBySession.value[normalizedSessionId] ?? -1;
  if (latestSeq <= previousViewedSeq) {
    return;
  }

  viewedEventSeqBySession.value = {
    ...viewedEventSeqBySession.value,
    [normalizedSessionId]: latestSeq,
  };
  webSessionStore.emitter.emit('web-session:viewed', {
    sessionId: normalizedSessionId,
  });
}

function hasSessionUnread(session: (typeof sessions.value)[number]) {
  if (isDraftSession(session)) {
    return false;
  }
  const latestSeq = webSessionStore.getLatestEventSeq(session.id);
  const viewedSeq = viewedEventSeqBySession.value[session.id] ?? -1;
  if (latestSeq > 0) {
    return latestSeq > viewedSeq;
  }
  return session.hasUnread && (!props.isActive || activeSessionId.value !== session.id);
}

function getProjectName(projectId: string) {
  if (!projectId) {
    return '';
  }
  if (projectStore.currentProject?.id === projectId && projectStore.currentProject.name) {
    return projectStore.currentProject.name;
  }
  return (
    projectStore.projects.find(project => project.id === projectId)?.name ||
    projectStore.recentProjects.find(project => project.id === projectId)?.name ||
    projectId
  );
}

type CrossProjectSessionItem = {
  session: WebSessionSummary;
  projectId: string;
  projectName: string;
  activityAt: number;
  isCurrent: boolean;
  projectIndex?: { index: number; color: string };
};

function withProjectIndexes(items: CrossProjectSessionItem[]) {
  const presentProjectIds = new Set(items.map(item => item.projectId).filter(Boolean));
  const projectIds: string[] = [];
  projectStore.projects.forEach(project => {
    if (project.id && presentProjectIds.has(project.id)) {
      projectIds.push(project.id);
    }
  });
  items.forEach(item => {
    if (item.projectId && !projectIds.includes(item.projectId)) {
      projectIds.push(item.projectId);
    }
  });

  const projectIndex = new Map<string, { index: number; color: string }>();
  projectIds.forEach((projectId, idx) => {
    projectIndex.set(projectId, {
      index: idx + 1,
      color: PROJECT_INDEX_COLORS[idx % PROJECT_INDEX_COLORS.length],
    });
  });

  return items.map(item => ({
    ...item,
    projectIndex: projectIndex.get(item.projectId),
  }));
}

const crossProjectSessions = computed<CrossProjectSessionItem[]>(() => {
  const rawItems: CrossProjectSessionItem[] = [];
  sidebarProjectIdsToLoad.value.forEach(projectId => {
    webSessionStore.getSessions(projectId).forEach(session => {
      rawItems.push({
        session,
        projectId,
        projectName: getProjectName(projectId),
        activityAt: getSessionActivityTimestamp(session),
        isCurrent: projectId === props.projectId && session.id === activeSessionId.value,
      });
    });
  });
  const sorted = rawItems.sort((left, right) => {
    if (right.activityAt !== left.activityAt) {
      return right.activityAt - left.activityAt;
    }
    if (left.isCurrent !== right.isCurrent) {
      return left.isCurrent ? -1 : 1;
    }
    const leftHasUnread = hasSessionUnread(left.session);
    const rightHasUnread = hasSessionUnread(right.session);
    if (leftHasUnread !== rightHasUnread) {
      return leftHasUnread ? -1 : 1;
    }
    const projectNameCompare = left.projectName.localeCompare(right.projectName);
    if (projectNameCompare !== 0) {
      return projectNameCompare;
    }
    if (left.session.orderIndex !== right.session.orderIndex) {
      return left.session.orderIndex - right.session.orderIndex;
    }
    return left.session.id.localeCompare(right.session.id);
  });

  return withProjectIndexes(sorted);
});

const crossProjectArchivedSessions = computed<CrossProjectSessionItem[]>(() => {
  const items = webSessionStore.getArchivedSessions(sidebarProjectIdsToLoad.value).map(session => ({
    session,
    projectId: session.projectId,
    projectName: getProjectName(session.projectId),
    activityAt: getSessionActivityTimestamp(session),
    isCurrent: Boolean(archivedPreviewSession.value?.id === session.id),
  }));
  return withProjectIndexes(items);
});

const archivedSidebarMeta = computed(() =>
  webSessionStore.getArchivedMeta(sidebarProjectIdsToLoad.value)
);

const isSingleSidebarProject = computed(() => {
  const ids = new Set(
    [...crossProjectSessions.value, ...crossProjectArchivedSessions.value]
      .map(item => item.projectId)
      .filter(Boolean)
  );
  return ids.size <= 1;
});

function clamp(min: number, value: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

const maxSidebarWidthByContainer = computed(() => {
  if (!sidebarContainerWidth.value) {
    return MAX_SESSION_SIDEBAR_WIDTH;
  }
  const maxAllowed = Math.max(
    MIN_SESSION_SIDEBAR_WIDTH,
    sidebarContainerWidth.value - MIN_SESSION_MAIN_WIDTH
  );
  return Math.min(MAX_SESSION_SIDEBAR_WIDTH, maxAllowed);
});

const effectiveSidebarWidthPx = computed(() => {
  if (!sidebarContainerWidth.value) {
    return DEFAULT_SESSION_SIDEBAR_WIDTH;
  }
  return clamp(
    MIN_SESSION_SIDEBAR_WIDTH,
    Math.round(sidebarWidthPx.value),
    Math.round(maxSidebarWidthByContainer.value)
  );
});

const showSidebarStatusText = computed(
  () => effectiveSidebarWidthPx.value >= SIDEBAR_STATUS_TEXT_THRESHOLD
);

const agentOptions = [
  { label: 'Codex', value: 'codex' },
  { label: 'Claude', value: 'claude' },
];

const CLAUDE_MODEL_OPTIONS = [
  { label: 'Opus', value: 'opus' },
  { label: 'Sonnet', value: 'sonnet' },
  { label: 'Haiku', value: 'haiku' },
];

const CODEX_MODEL_OPTIONS = [
  { label: 'GPT-5.3 Codex', value: 'gpt-5.3-codex' },
  { label: 'GPT-5.3 Codex Spark', value: 'gpt-5.3-codex-spark' },
  { label: 'GPT-5.4', value: 'gpt-5.4' },
  { label: 'GPT-5.4 mini', value: 'gpt-5.4-mini' },
  { label: 'GPT-5.4 nano', value: 'gpt-5.4-nano' },
  { label: 'GPT-5.4 Pro', value: 'gpt-5.4-pro' },
];

const CUSTOM_MODEL_VALUE = '__custom_model__';

function withCurrentModelOption(
  options: Array<{ label: string; value: string }>,
  currentModel?: string | null
) {
  const normalizedModel = String(currentModel || '').trim();
  if (!normalizedModel) {
    return options;
  }
  if (options.some(option => option.value === normalizedModel)) {
    return options;
  }
  return [
    ...options,
    {
      label: `${normalizedModel} (Current)`,
      value: normalizedModel,
    },
  ];
}

function defaultReasoningEffortForAgent(agent: 'claude' | 'codex') {
  return agent === 'codex' ? 'xhigh' : 'default';
}

function withCurrentReasoningEffortOption(
  options: Array<{ label: string; value: string }>,
  currentEffort?: string | null
) {
  const normalizedEffort = String(currentEffort || '')
    .trim()
    .toLowerCase();
  if (!normalizedEffort) {
    return options;
  }
  if (options.some(option => option.value === normalizedEffort)) {
    return options;
  }
  return [
    ...options,
    {
      label: `${normalizedEffort} (Current)`,
      value: normalizedEffort,
    },
  ];
}

const modelOptions = computed(() => {
  const activeModel = currentSession.value?.model ?? draftModel.value;
  if (selectedAgent.value === 'claude') {
    return [
      ...withCurrentModelOption(CLAUDE_MODEL_OPTIONS, activeModel),
      { label: t('webSession.customModel'), value: CUSTOM_MODEL_VALUE },
    ];
  }
  return [
    ...withCurrentModelOption(CODEX_MODEL_OPTIONS, activeModel),
    { label: t('webSession.customModel'), value: CUSTOM_MODEL_VALUE },
  ];
});

const reasoningEffortOptions = computed(() => {
  const options = [
    { label: t('common.default'), value: 'default' },
    { label: 'None', value: 'none' },
    { label: 'Low', value: 'low' },
    { label: 'Medium', value: 'medium' },
    { label: 'High', value: 'high' },
    { label: 'Xhigh', value: 'xhigh' },
  ];
  const activeEffort = currentSession.value?.reasoningEffort ?? draftReasoningEffort.value;
  return withCurrentReasoningEffortOption(options, activeEffort);
});

const selectedAgent = computed({
  get: () => currentSession.value?.agent ?? draftAgent.value,
  set: value => {
    const next = value as 'claude' | 'codex';
    draftAgent.value = next;
    if (next === 'claude' && draftModel.value.startsWith('gpt-')) {
      draftModel.value = 'opus';
    }
    if (next === 'codex' && !draftModel.value.startsWith('gpt-')) {
      draftModel.value = 'gpt-5.4';
    }
    draftReasoningEffort.value = defaultReasoningEffortForAgent(next);
    if (isDraftSession(currentSession.value)) {
      updateActiveDraftSession(current => ({
        ...current,
        agent: next,
        model:
          next === 'claude' && current.model.startsWith('gpt-')
            ? 'opus'
            : next === 'codex' && !current.model.startsWith('gpt-')
              ? 'gpt-5.4'
              : current.model,
        reasoningEffort: defaultReasoningEffortForAgent(next),
        updatedAt: new Date().toISOString(),
      }));
      return;
    }
    if (currentRealSession.value) {
      void webSessionStore.updateAgent(currentRealSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const selectedModel = computed({
  get: () => currentSession.value?.model ?? draftModel.value,
  set: value => {
    const next = String(value);
    if (next === CUSTOM_MODEL_VALUE) {
      openCustomModelDialog();
      return;
    }
    draftModel.value = next;
    if (isDraftSession(currentSession.value)) {
      updateActiveDraftSession(current => ({
        ...current,
        model: next,
        updatedAt: new Date().toISOString(),
      }));
      return;
    }
    if (currentRealSession.value) {
      void webSessionStore.updateModel(currentRealSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const selectedReasoningEffort = computed<'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh'>({
  get: () => currentSession.value?.reasoningEffort ?? draftReasoningEffort.value,
  set: value => {
    const next = value as 'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh';
    draftReasoningEffort.value = next;
    if (isDraftSession(currentSession.value)) {
      updateActiveDraftSession(current => ({
        ...current,
        reasoningEffort: next,
        updatedAt: new Date().toISOString(),
      }));
      return;
    }
    if (currentRealSession.value) {
      void webSessionStore.updateReasoningEffort(currentRealSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const permissionLevelOptions = computed(() => [
  { label: t('webSession.permissionDefault'), value: 'default' },
  { label: t('webSession.permissionElevated'), value: 'elevated' },
  { label: t('webSession.permissionYolo'), value: 'yolo' },
]);

const selectedWorkflowMode = computed<'default' | 'plan'>({
  get: () => currentSession.value?.workflowMode ?? draftWorkflowMode.value,
  set: value => {
    const next = value as 'default' | 'plan';
    draftWorkflowMode.value = next;
    if (isDraftSession(currentSession.value)) {
      updateActiveDraftSession(current => ({
        ...current,
        workflowMode: next,
        updatedAt: new Date().toISOString(),
      }));
      return;
    }
    if (currentRealSession.value) {
      void webSessionStore.updateWorkflowMode(currentRealSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const selectedPermissionLevel = computed<'default' | 'elevated' | 'yolo'>({
  get: () => currentSession.value?.permissionLevel ?? draftPermissionLevel.value,
  set: value => {
    const next = value as 'default' | 'elevated' | 'yolo';
    draftPermissionLevel.value = next;
    if (isDraftSession(currentSession.value)) {
      updateActiveDraftSession(current => ({
        ...current,
        permissionLevel: next,
        updatedAt: new Date().toISOString(),
      }));
      return;
    }
    if (currentRealSession.value) {
      void webSessionStore.updatePermissionLevel(currentRealSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const refreshTabSortable = useDebounceFn(() => {
  nextTick(() => {
    setupTabSorting();
  });
}, 100);

let tabScrollContainer: HTMLElement | null = null;

function setWorkflowMode(mode: 'default' | 'plan') {
  draftWorkflowMode.value = mode;
  const session = currentSession.value;
  if (!session) {
    return;
  }
  if (isDraftSession(session)) {
    updateActiveDraftSession(current => ({
      ...current,
      workflowMode: mode,
      updatedAt: new Date().toISOString(),
    }));
    return;
  }
  void webSessionStore.updateWorkflowMode(session.id, mode).catch(error => {
    message.error(error instanceof Error ? error.message : t('common.error'));
  });
}

function openCustomModelDialog() {
  const inputValue = ref((currentSession.value?.model ?? draftModel.value).trim());
  dialog.create({
    title: t('webSession.customModelTitle'),
    content: () =>
      h(NInput, {
        value: inputValue.value,
        'onUpdate:value': (value: string) => {
          inputValue.value = value;
        },
        maxlength: 128,
        autofocus: true,
        placeholder: t('webSession.customModelPlaceholder'),
      }),
    positiveText: t('common.save'),
    negativeText: t('common.cancel'),
    showIcon: false,
    maskClosable: false,
    closeOnEsc: true,
    onPositiveClick: async () => {
      const nextModel = inputValue.value.trim();
      if (!nextModel) {
        message.warning(t('webSession.customModelEmpty'));
        return false;
      }
      draftModel.value = nextModel;
      if (!currentSession.value) {
        return true;
      }
      if (isDraftSession(currentSession.value)) {
        updateActiveDraftSession(current => ({
          ...current,
          model: nextModel,
          updatedAt: new Date().toISOString(),
        }));
        return true;
      }
      try {
        await webSessionStore.updateModel(currentSession.value.id, nextModel);
        return true;
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('common.error'));
        return false;
      }
    },
  });
}

function defaultModelForAgent(agent: 'claude' | 'codex') {
  return agent === 'claude' ? 'opus' : 'gpt-5.4';
}

function formatTime(timestamp: number) {
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return '';
  }
  return new Date(timestamp).toLocaleTimeString();
}

function formatDateTime(timestamp: number) {
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return '';
  }
  return new Date(timestamp).toLocaleString();
}

function formatElapsedDuration(startedAt: number, endedAt: number) {
  const diff = Math.max(0, endedAt - startedAt);
  const totalSeconds = Math.floor(diff / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
  }

  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
}

function getLiveTimeText(state: WebSessionLiveState) {
  if (
    typeof state.startedAt === 'number' &&
    Number.isFinite(state.startedAt) &&
    state.startedAt > 0
  ) {
    const endedAt = state.running ? liveStateClockMs.value : state.updatedAt;
    return formatElapsedDuration(state.startedAt, endedAt);
  }
  return formatTime(state.updatedAt);
}

function getLiveTimeTooltip(state: WebSessionLiveState) {
  if (
    typeof state.startedAt === 'number' &&
    Number.isFinite(state.startedAt) &&
    state.startedAt > 0
  ) {
    return formatDateTime(state.startedAt);
  }
  return formatDateTime(state.updatedAt);
}

function stringifyValue(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }
  try {
    const serialized = JSON.stringify(value, null, 2);
    return typeof serialized === 'string' ? serialized : String(value ?? '');
  } catch {
    return String(value ?? '');
  }
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined;
  }
  return value as Record<string, unknown>;
}

function normalizeToolKindValue(value: string | undefined) {
  const normalized = String(value ?? '').trim();
  if (normalized === 'commandExecution') {
    return 'command_execution';
  }
  if (normalized === 'mcpToolCall') {
    return 'mcp_tool_call';
  }
  if (normalized === 'fileChange') {
    return 'file_change';
  }
  if (normalized === 'webSearch') {
    return 'web_search';
  }
  return normalized;
}

function isCompactToolKind(value: string | undefined) {
  return ['command_execution', 'file_change', 'mcp_tool_call', 'web_search'].includes(
    normalizeToolKindValue(value)
  );
}

function compactToolLabel(tool?: { kind?: string; meta?: Record<string, unknown> }) {
  const kind = normalizeToolKindValue(tool?.kind || String(tool?.meta?.kind ?? ''));
  if (kind === 'command_execution') {
    return t('webSession.toolCommandExecution');
  }
  if (kind === 'file_change') {
    return t('webSession.toolFileChange');
  }
  if (kind === 'mcp_tool_call') {
    return t('webSession.toolMcpToolCall');
  }
  if (kind === 'web_search') {
    return t('webSession.toolWebSearch');
  }
  return t('webSession.toolKindDefault');
}

function isCompactTool(
  tool: Pick<NonNullable<WebSessionBlock['tool']>, 'kind' | 'meta' | 'commandGroup'>
) {
  return isCompactToolKind(tool.kind || String(tool.meta?.kind ?? ''));
}

function getCompactToolKind(tool: Pick<NonNullable<WebSessionBlock['tool']>, 'kind' | 'meta'>) {
  return normalizeToolKindValue(tool.kind || String(tool.meta?.kind ?? ''));
}

function getCompactToolSummary(tool: NonNullable<WebSessionBlock['tool']>) {
  const kind = getCompactToolKind(tool);
  const input = asRecord(tool.input);
  const subtitle = String(tool.meta?.subtitle ?? '').trim();

  if (kind === 'command_execution') {
    const command = String(input?.command ?? '').trim();
    return command || subtitle;
  }

  if (kind === 'file_change') {
    const path =
      String(input?.path ?? input?.file_path ?? input?.new_path ?? input?.old_path ?? '').trim() ||
      subtitle;
    if (path) {
      return path;
    }
    const changes = Array.isArray(input?.changes) ? input.changes.length : 0;
    return changes > 0 ? `${changes} change${changes > 1 ? 's' : ''}` : '';
  }

  if (kind === 'mcp_tool_call') {
    const toolName = String(input?.tool_name ?? input?.name ?? '').trim();
    const args = asRecord(input?.arguments);
    const target =
      String(
        args?.url ??
          args?.query ??
          args?.path ??
          args?.file ??
          args?.name ??
          args?.id ??
          input?.server ??
          input?.path ??
          ''
      ).trim() || subtitle;
    if (toolName && target && toolName !== target) {
      return `${toolName} · ${target}`;
    }
    return toolName || target;
  }

  if (kind === 'web_search') {
    const query = String(input?.query ?? '').trim();
    if (query) {
      return query;
    }
    const action = asRecord(input?.action);
    const queries = Array.isArray(action?.queries)
      ? action.queries
          .map(value => String(value ?? '').trim())
          .filter((value): value is string => Boolean(value))
      : [];
    return queries[0] ?? subtitle;
  }

  return subtitle;
}

function getCompactToolCount(tool: NonNullable<WebSessionBlock['tool']>) {
  return Math.max(1, Number(tool.commandGroup?.count ?? 1) || 1);
}

function shouldHideTimelineMeta(item: WebSessionBlock) {
  if (!Number.isFinite(item.timestamp) || item.timestamp <= 0) {
    return true;
  }
  return item.kind === 'tool' && item.tool ? isCompactTool(item.tool) : false;
}

function canPreviewAttachment(attachment: { name: string; mime?: string }) {
  const normalizedMime = attachment.mime?.trim().toLowerCase();
  if (normalizedMime) {
    return normalizedMime.startsWith('image/');
  }
  return IMAGE_ATTACHMENT_NAME_PATTERN.test(attachment.name);
}

function getAttachmentPreviewUrl(attachmentID: string) {
  const normalizedID = String(attachmentID || '').trim();
  if (!normalizedID) {
    return '';
  }
  const path = `/api/v1/web-sessions/attachments/${encodeURIComponent(normalizedID)}`;
  return urlBase ? new URL(path, urlBase).toString() : path;
}

function openAttachmentPreview(attachment: { id: string; name: string; mime?: string }) {
  if (!canPreviewAttachment(attachment)) {
    return;
  }
  activeAttachmentPreview.value = {
    id: attachment.id,
    name: attachment.name,
    url: getAttachmentPreviewUrl(attachment.id),
  };
  showAttachmentPreview.value = true;
}

function handleAttachmentPreviewVisibilityChange(show: boolean) {
  showAttachmentPreview.value = show;
  if (!show) {
    activeAttachmentPreview.value = null;
  }
}

const commandExecutionDetailTitle = computed(() =>
  activeCommandExecutionDetail.value
    ? t('webSession.compactToolDetailTitleWithCount', {
        kind: compactToolLabel(activeCommandExecutionDetail.value),
        count: activeCommandExecutionDetail.value.count,
      })
    : t('webSession.compactToolDetailTitle')
);

const commandExecutionDetailItems = computed(() => {
  if (!activeCommandExecutionDetail.value) {
    return [];
  }
  return [...activeCommandExecutionDetail.value.items].sort((left, right) => {
    const leftTime = Date.parse(left.completedAt || left.startedAt || left.timestamp || '') || 0;
    const rightTime =
      Date.parse(right.completedAt || right.startedAt || right.timestamp || '') || 0;
    return rightTime - leftTime;
  });
});

function buildLocalCommandExecutionDetail(block: WebSessionBlock): CommandExecutionDetail | null {
  if (!block.tool) {
    return null;
  }
  const payload = asRecord(block.payload);
  const rawItems = Array.isArray(payload?.groupItems) ? payload?.groupItems : null;
  if (!rawItems || rawItems.length === 0) {
    return null;
  }
  const items: CommandExecutionDetailItem[] = [];
  rawItems.forEach(item => {
    const record = asRecord(item);
    if (!record) {
      return;
    }
    items.push({
      toolId: String(record.toolId ?? ''),
      kind: String(record.kind ?? ''),
      title: String(record.title ?? ''),
      summary: String(record.summary ?? ''),
      command: String(record.command ?? ''),
      input: record.input,
      output: typeof record.output === 'string' ? record.output : undefined,
      status:
        record.status === 'running' || record.status === 'error' || record.status === 'done'
          ? record.status
          : 'done',
      timestamp: typeof record.timestamp === 'string' ? record.timestamp : '',
      startedAt: typeof record.startedAt === 'string' ? record.startedAt : undefined,
      completedAt: typeof record.completedAt === 'string' ? record.completedAt : undefined,
    });
  });
  if (items.length === 0) {
    return null;
  }
  const groupId = block.tool.commandGroup?.id || block.tool.id;
  return {
    groupId,
    kind: block.tool.kind ?? '',
    title: block.tool.name,
    summary: getCompactToolSummary(block.tool),
    count: Math.max(
      items.length,
      Number(block.tool.commandGroup?.count ?? items.length) || items.length
    ),
    firstSeq: Number(block.tool.commandGroup?.firstSeq ?? 0),
    lastSeq: Number(block.tool.commandGroup?.lastSeq ?? 0),
    status: block.tool.status,
    latestToolId: block.tool.commandGroup?.latestToolId || block.tool.id,
    items,
  };
}

async function openCommandExecutionDetail(block: WebSessionBlock) {
  if (!currentRealSession.value) {
    return;
  }
  const tool = block.tool;
  if (!tool) {
    return;
  }
  const groupId = tool.commandGroup?.id || tool.id;
  if (!groupId) {
    return;
  }

  activeCommandExecutionGroupId.value = groupId;
  showCommandExecutionDetail.value = true;
  loadingCommandExecutionDetail.value = true;
  const requestGroupId = groupId;

  const localDetail = buildLocalCommandExecutionDetail(block);
  if (localDetail) {
    activeCommandExecutionDetail.value = localDetail;
    loadingCommandExecutionDetail.value = false;
    return;
  }

  try {
    const response =
      (await http
        .Get<{
          item?: CommandExecutionDetail;
        }>(
          `/projects/${encodeURIComponent(currentRealSession.value.projectId)}/web-sessions/${encodeURIComponent(currentRealSession.value.id)}/command-groups/${encodeURIComponent(groupId)}`,
          { cacheFor: 0 }
        )
        .send()) ?? {};
    if (activeCommandExecutionGroupId.value === requestGroupId) {
      activeCommandExecutionDetail.value = response.item ?? null;
    }
  } catch (error) {
    if (activeCommandExecutionGroupId.value === requestGroupId) {
      activeCommandExecutionDetail.value = null;
    }
    message.error(
      error instanceof Error && error.message
        ? error.message
        : t('webSession.compactToolLoadFailed')
    );
  } finally {
    if (activeCommandExecutionGroupId.value === requestGroupId) {
      loadingCommandExecutionDetail.value = false;
    }
  }
}

function handleCommandExecutionDetailVisibilityChange(show: boolean) {
  showCommandExecutionDetail.value = show;
  if (!show) {
    activeCommandExecutionDetail.value = null;
    activeCommandExecutionGroupId.value = '';
    loadingCommandExecutionDetail.value = false;
  }
}

function showCommandExecutionInput(item: CommandExecutionDetailItem) {
  const input = asRecord(item.input);
  if (!input) {
    return Boolean(item.input);
  }
  const keys = Object.keys(input);
  if (item.kind === 'command_execution') {
    return !(keys.length === 1 && keys[0] === 'command');
  }
  return keys.length > 0;
}

function formatCommandExecutionDetailTime(item: CommandExecutionDetailItem) {
  const value = Date.parse(item.completedAt || item.startedAt || item.timestamp || '');
  if (!Number.isFinite(value)) {
    return '';
  }
  return formatTime(value);
}

function isToolExpanded(toolId: string) {
  return Boolean(expandedTools.value[toolId]);
}

function toggleToolExpanded(toolId: string) {
  expandedTools.value = {
    ...expandedTools.value,
    [toolId]: !expandedTools.value[toolId],
  };
}

function showPlanActions(toolId: string) {
  return Boolean(
    currentRealSession.value &&
      latestPlanToolId.value === toolId &&
      (!liveState.value.running || inlinePlanChoice.value) &&
      !dismissedPlanActions.value[toolId] &&
      !hasUserMessageAfterLatestPlan.value
  );
}

function setPlanActionsDismissed(toolId: string, dismissed: boolean) {
  if (!toolId) {
    return;
  }
  dismissedPlanActions.value = {
    ...dismissedPlanActions.value,
    [toolId]: dismissed,
  };
}

function toolKindLabel(tool: { name: string; kind?: string }) {
  const kind = normalizeToolKindValue(tool.kind);
  if (!kind) {
    return t('webSession.toolKindDefault');
  }
  if (kind === 'command_execution') {
    return t('webSession.toolCommandExecution');
  }
  if (kind === 'file_change') {
    return t('webSession.toolFileChange');
  }
  if (kind === 'mcp_tool_call') {
    return t('webSession.toolMcpToolCall');
  }
  if (kind === 'tool_use') {
    return t('webSession.toolKindTool');
  }
  if (kind === 'shell') {
    return 'Shell';
  }
  return kind;
}

function formatToolPreview(tool: {
  input?: unknown;
  output?: string;
  kind?: string;
  meta?: Record<string, unknown>;
  commandGroup?: { count: number };
}) {
  if (isCompactTool(tool as NonNullable<WebSessionBlock['tool']>)) {
    return getCompactToolSummary(tool as NonNullable<WebSessionBlock['tool']>);
  }
  const source =
    typeof tool.output === 'string' && tool.output.trim()
      ? tool.output
      : stringifyValue(tool.input);
  return String(source ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 120);
}

function toolStateLabel(tool: { status: 'running' | 'done' | 'error' }) {
  if (tool.status === 'done') {
    return t('webSession.toolDone');
  }
  if (tool.status === 'error') {
    return t('webSession.toolError');
  }
  return t('webSession.toolRunning');
}

function timelineRoleLabel(item: WebSessionBlock) {
  if (item.kind === 'user') {
    return t('terminal.user');
  }
  if (item.kind === 'assistant') {
    return t('terminal.assistant');
  }
  if (item.kind === 'tool') {
    return item.tool?.name || t('webSession.toolKindDefault');
  }
  return t('common.info');
}

function historyInteractionTitle(item: WebSessionBlock) {
  switch (item.detail?.type) {
    case 'approval_request':
      return t('webSession.approvalTitle');
    case 'approval_response':
      return item.detail.action === 'reject'
        ? t('webSession.historyApprovalRejected')
        : t('webSession.historyApprovalApproved');
    case 'user_input_request':
      return t('webSession.userInputTitle');
    case 'user_input_response':
      return t('webSession.historyUserInputSubmitted');
    default:
      return t('common.info');
  }
}

function historyInteractionPrompt(item: WebSessionBlock) {
  if (item.detail?.type === 'user_input_request' && item.detail.questions?.length) {
    return '';
  }
  if (item.detail?.type === 'user_input_response' && item.detail.answers?.length) {
    return '';
  }
  return item.detail?.prompt?.trim() || item.text?.trim() || '';
}

function historyInteractionBadgeClass(item: WebSessionBlock) {
  switch (item.detail?.type) {
    case 'approval_request':
      return 'state-approval-request';
    case 'approval_response':
      return item.detail.action === 'reject' ? 'state-approval-reject' : 'state-approval-approve';
    case 'user_input_request':
      return 'state-user-input-request';
    case 'user_input_response':
      return 'state-user-input-response';
    default:
      return '';
  }
}

function historyInteractionCardClass(item: WebSessionBlock) {
  switch (item.detail?.type) {
    case 'approval_request':
      return 'type-approval-request';
    case 'approval_response':
      return item.detail.action === 'reject' ? 'type-approval-reject' : 'type-approval-approve';
    case 'user_input_request':
      return 'type-user-input-request';
    case 'user_input_response':
      return 'type-user-input-response';
    default:
      return '';
  }
}

function historyQuestionTitle(question: WebSessionUserInputQuestion) {
  return (
    question.header?.trim() || question.question?.trim() || t('webSession.historyQuestionLabel')
  );
}

function formatHistoryAnswerValues(answer: WebSessionHistoryAnswerEntry) {
  if (answer.masked) {
    return answer.values.map(() => t('webSession.historyMaskedAnswer'));
  }
  return answer.values;
}

async function initializeProjectSessions(projectId: string) {
  if (!projectId) {
    return;
  }
  clearArchivedPreviewSession();
  const restoredDrafts = loadPersistedDraftSessions(projectId);
  const storedActiveDraftId = persistedActiveDraftSessionIdByProject.value[projectId] ?? '';
  const activeDraftId = restoredDrafts.some(session => session.id === storedActiveDraftId)
    ? storedActiveDraftId
    : '';
  replaceDraftSessionState(restoredDrafts, activeDraftId, projectId);
  const loadedSessions = await webSessionStore.loadSessions(projectId);
  await webSessionStore.openSocket();
  if (activeDraftId) {
    webSessionStore.setActiveSession(projectId, '');
    return;
  }
  const rememberedSessionId = webSessionStore.getActiveSessionId(projectId);
  const targetSessionId = webSessionStore.hasStoredActiveSession(projectId)
    ? rememberedSessionId
    : loadedSessions[0]?.id;
  if (targetSessionId) {
    await webSessionStore.ensureSessionConnected(projectId, targetSessionId);
    return;
  }
  if (restoredDrafts.length > 0) {
    const fallbackDraftId = restoredDrafts[restoredDrafts.length - 1]?.id ?? '';
    replaceDraftSessionState(restoredDrafts, fallbackDraftId, projectId);
    webSessionStore.setActiveSession(projectId, '');
    return;
  }
  ensureDefaultDraftSession();
}

async function handleSessionSelect(sessionId: string) {
  if (!sessionId) {
    return;
  }
  showMobileTabSelector.value = false;
  if (archivedPreviewSession.value?.id === sessionId) {
    scrollToBottom(true);
    return;
  }
  const draft = draftSessions.value.find(session => session.id === sessionId);
  if (draft) {
    clearArchivedPreviewSession();
    replaceDraftSessionState(draftSessions.value, draft.id);
    webSessionStore.setActiveSession(props.projectId, '');
    scrollToBottom(true);
    return;
  }
  clearArchivedPreviewSession();
  replaceDraftSessionState(draftSessions.value, '');
  await webSessionStore.ensureSessionConnected(props.projectId, sessionId);
  scrollToBottom(true);
}

async function handleSidebarSessionSelect(item: CrossProjectSessionItem) {
  const sessionId = item.session.id;
  if (!sessionId) {
    return;
  }
  try {
    if (item.projectId === props.projectId && sessionId === activeSessionId.value) {
      scrollToBottom(true);
      return;
    }
    clearArchivedPreviewSession();
    await webSessionStore.ensureSessionConnected(item.projectId, sessionId);
    if (item.projectId !== props.projectId) {
      projectStore.addRecentProject(item.projectId);
      await router.push({
        name: 'project',
        params: { id: item.projectId },
      });
      return;
    }
    replaceDraftSessionState(draftSessions.value, '');
    scrollToBottom(true);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleArchivedSidebarSessionSelect(item: CrossProjectSessionItem) {
  if (!item.session.id) {
    return;
  }
  try {
    await openArchivedPreviewSession(item.session);
    scrollToBottom(true);
  } catch (error) {
    clearArchivedPreviewSession();
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleLoadMoreArchived() {
  try {
    await webSessionStore.loadArchivedSessions(sidebarProjectIdsToLoad.value, {
      limit: 20,
    });
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function updateSidebarContainerWidth() {
  const parent = sidebarRootRef.value?.parentElement;
  if (!parent) {
    sidebarContainerWidth.value = 0;
    return;
  }
  sidebarContainerWidth.value = parent.getBoundingClientRect().width;
}

function setupSidebarResizeObserver() {
  sidebarResizeObserver?.disconnect();
  sidebarResizeObserver = null;
  const parent = sidebarRootRef.value?.parentElement;
  if (!parent || typeof ResizeObserver === 'undefined') {
    updateSidebarContainerWidth();
    return;
  }
  sidebarResizeObserver = new ResizeObserver(() => updateSidebarContainerWidth());
  sidebarResizeObserver.observe(parent);
  updateSidebarContainerWidth();
}

function startSidebarResize(event: MouseEvent) {
  if (!sidebarContainerWidth.value) {
    return;
  }
  event.preventDefault();
  isSidebarResizing.value = true;
  const startX = event.clientX;
  const startWidth = effectiveSidebarWidthPx.value;

  function onMouseMove(moveEvent: MouseEvent) {
    const delta = startX - moveEvent.clientX;
    sidebarWidthPx.value = Math.round(
      clamp(MIN_SESSION_SIDEBAR_WIDTH, startWidth + delta, maxSidebarWidthByContainer.value)
    );
  }

  function onMouseUp() {
    isSidebarResizing.value = false;
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }

  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', onMouseUp);
  document.body.style.cursor = 'col-resize';
  document.body.style.userSelect = 'none';
}

async function handleCreateSession(forceAgent?: 'claude' | 'codex') {
  try {
    const source = currentSession.value;
    const agent = forceAgent ?? source?.agent ?? selectedAgent.value;
    const worktreeId = isDraftSession(source)
      ? (source.worktreeId ?? undefined)
      : (projectStore.selectedWorktreeId ?? source?.worktreeId ?? undefined);
    const session = await webSessionStore.createSession(props.projectId, {
      worktreeId,
      agent,
      model: source?.model || draftModel.value || defaultModelForAgent(agent),
      reasoningEffort:
        source?.reasoningEffort ||
        (agent === 'codex' ? selectedReasoningEffort.value : defaultReasoningEffortForAgent(agent)),
      workflowMode: source?.workflowMode || draftWorkflowMode.value,
      permissionLevel: source?.permissionLevel || draftPermissionLevel.value,
      title: isDraftSession(source) ? source.title : undefined,
    });
    if (isDraftSession(source)) {
      webSessionStore.moveDraft(props.projectId, source.id, session.id);
      removeDraftSession(source.id, {
        nextRealSessionId: session.id,
        preserveDraftState: true,
      });
    }
    draftAgent.value = session.agent;
    draftModel.value = session.model;
    draftReasoningEffort.value =
      session.reasoningEffort || defaultReasoningEffortForAgent(session.agent);
    draftWorkflowMode.value = session.workflowMode;
    draftPermissionLevel.value = session.permissionLevel;
    clearArchivedPreviewSession();
    scrollToBottom(true);
    return session;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
    return null;
  }
}

function handleStartDraftSession(forceAgent?: 'claude' | 'codex') {
  clearArchivedPreviewSession();
  const draft = createDraftSession(forceAgent);
  draftAgent.value = draft.agent;
  draftModel.value = draft.model || defaultModelForAgent(draft.agent);
  draftReasoningEffort.value = draft.reasoningEffort || defaultReasoningEffortForAgent(draft.agent);
  draftWorkflowMode.value = draft.workflowMode;
  draftPermissionLevel.value = draft.permissionLevel;
  showMobileTabSelector.value = false;
  contextMenuSession.value = null;
  expandedTools.value = {};
  autoFollowBottom.value = true;
  scrollToBottom(true);
  updateActiveTabIndicator();
  focusComposer();
}

async function handleRenameSession(sessionId: string) {
  const session = sessions.value.find(item => item.id === sessionId);
  if (!session) {
    return;
  }

  const inputValue = ref(session.title);
  dialog.create({
    title: t('webSession.renameTitle'),
    content: () =>
      h(NInput, {
        value: inputValue.value,
        'onUpdate:value': (value: string) => {
          inputValue.value = value;
        },
        maxlength: 64,
        autofocus: true,
        placeholder: t('webSession.renamePlaceholder'),
      }),
    positiveText: t('common.save'),
    negativeText: t('common.cancel'),
    showIcon: false,
    maskClosable: false,
    closeOnEsc: true,
    onPositiveClick: async () => {
      const nextTitle = inputValue.value.trim();
      if (!nextTitle) {
        message.warning(t('webSession.emptyName'));
        return false;
      }
      if (nextTitle === session.title) {
        return true;
      }
      if (isDraftSession(session)) {
        updateDraftSession(session.id, current => ({
          ...current,
          title: nextTitle,
          updatedAt: new Date().toISOString(),
        }));
        message.success(t('webSession.renameSuccess'));
        return true;
      }
      try {
        await webSessionStore.renameSession(session.projectId, sessionId, nextTitle);
        if (isArchivedPreviewSession(session) && archivedPreviewSession.value?.id === session.id) {
          archivedPreviewSession.value = {
            ...archivedPreviewSession.value,
            title: nextTitle,
          };
        }
        message.success(t('webSession.renameSuccess'));
        return true;
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('webSession.renameFailed'));
        return false;
      }
    },
  });
}

async function refreshArchivedSidebar() {
  await webSessionStore.loadArchivedSessions(sidebarProjectIdsToLoad.value, {
    reset: true,
    limit: 20,
  });
}

function handleArchiveSession(sessionId: string) {
  const session = sessions.value.find(item => item.id === sessionId);
  if (!session) {
    return;
  }

  if (isDraftSession(session)) {
    removeDraftSession(sessionId);
    return;
  }
  if (isArchivedPreviewSession(session)) {
    clearArchivedPreviewSession();
    return;
  }

  if (confirmBeforeTerminalClose.value) {
    dialog.warning({
      title: t('webSession.confirmCloseTitle'),
      content: () =>
        h('div', { class: 'web-session-close-confirm' }, [
          h('div', { class: 'web-session-close-confirm__message' }, [
            t('webSession.confirmCloseContent', { title: session.title }),
          ]),
        ]),
      positiveText: t('webSession.confirmCloseButton'),
      negativeText: t('common.cancel'),
      onPositiveClick: async () => performArchiveSession(session),
    });
    return;
  }

  void performArchiveSession(session);
}

async function performArchiveSession(session: WebSessionSummary): Promise<boolean> {
  try {
    await webSessionStore.archiveSession(session.projectId, session.id);
    await refreshArchivedSidebar();
    const nextSession = webSessionStore.getActiveSession(props.projectId);
    if (nextSession?.id) {
      await webSessionStore.ensureSessionConnected(props.projectId, nextSession.id);
    } else if (draftSessions.value.length > 0) {
      replaceDraftSessionState(
        draftSessions.value,
        draftSessions.value[draftSessions.value.length - 1]?.id ?? ''
      );
    } else {
      ensureDefaultDraftSession();
    }
    return true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
    return false;
  }
}

async function performDeleteSession(sessionId: string): Promise<boolean> {
  const session = sessions.value.find(item => item.id === sessionId);
  if (!session) {
    return false;
  }
  try {
    await webSessionStore.deleteSession(session.projectId, sessionId);
    if (isArchivedPreviewSession(session)) {
      clearArchivedPreviewSession();
    } else if (!isDraftSession(session)) {
      await refreshArchivedSidebar();
    }
    const nextSession = webSessionStore.getActiveSession(props.projectId);
    if (nextSession?.id) {
      await webSessionStore.ensureSessionConnected(props.projectId, nextSession.id);
    } else if (draftSessions.value.length > 0) {
      replaceDraftSessionState(
        draftSessions.value,
        draftSessions.value[draftSessions.value.length - 1]?.id ?? ''
      );
    } else {
      ensureDefaultDraftSession();
    }
    return true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
    return false;
  }
}

function openFilePicker() {
  fileInputRef.value?.click();
}

function getTransferImageKey(file: File) {
  const normalizedName = file.name.trim().toLowerCase() || 'clipboard-image';
  const normalizedType = file.type.trim().toLowerCase();
  return [normalizedName, normalizedType, String(file.size)].join(':');
}

function collectImageFiles(
  items: Iterable<File | DataTransferItem>,
  options: { fromDataTransferItem: boolean }
) {
  const imageFiles: File[] = [];
  const seen = new Set<string>();

  for (const entry of items) {
    const file = options.fromDataTransferItem
      ? (entry as DataTransferItem).getAsFile()
      : (entry as File);
    if (!file || !file.type.startsWith('image/')) {
      continue;
    }
    const key = getTransferImageKey(file);
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    imageFiles.push(file);
  }

  return imageFiles;
}

function getImageFilesFromTransfer(dataTransfer: DataTransfer | null) {
  if (!dataTransfer) {
    return [];
  }

  // Clipboard paste can expose the same image through both items and files.
  // Prefer items when available and only fall back to files if items yield nothing.
  const itemFiles = collectImageFiles(Array.from(dataTransfer.items || []), {
    fromDataTransferItem: true,
  });
  if (itemFiles.length > 0) {
    return itemFiles;
  }

  return collectImageFiles(Array.from(dataTransfer.files || []), {
    fromDataTransferItem: false,
  });
}

function hasFileTransfer(dataTransfer: DataTransfer | null) {
  if (!dataTransfer) {
    return false;
  }

  if (Array.from(dataTransfer.items || []).some(item => item.kind === 'file')) {
    return true;
  }

  return (
    Array.from(dataTransfer.files || []).length > 0 ||
    Array.from(dataTransfer.types || []).includes('Files')
  );
}

function resetComposerDragState() {
  composerDragDepth = 0;
  isComposerDragOver.value = false;
}

async function uploadComposerImages(files: File[]) {
  const sessionId = currentDraftSessionId.value;
  if (!sessionId) {
    return;
  }
  for (const file of files) {
    try {
      await webSessionStore.uploadAttachment(props.projectId, sessionId, file);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('common.error'));
    }
  }
}

async function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement | null;
  const files = Array.from(target?.files ?? []).filter(file => file.type.startsWith('image/'));
  if (files.length === 0) {
    return;
  }
  await uploadComposerImages(files);
  if (target) {
    target.value = '';
  }
}

function handleComposerPaste(event: ClipboardEvent) {
  const files = getImageFilesFromTransfer(event.clipboardData);
  if (files.length === 0) {
    return;
  }

  event.preventDefault();
  void uploadComposerImages(files);
}

function handleComposerDragEnter(event: DragEvent) {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  composerDragDepth += 1;
  isComposerDragOver.value = true;
}

function handleComposerDragOver(event: DragEvent) {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy';
  }
  isComposerDragOver.value = true;
}

function handleComposerDragLeave(event: DragEvent) {
  if (!isComposerDragOver.value) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  composerDragDepth = Math.max(0, composerDragDepth - 1);
  if (composerDragDepth === 0) {
    isComposerDragOver.value = false;
  }
}

async function handleComposerDrop(event: DragEvent) {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  const files = getImageFilesFromTransfer(event.dataTransfer);
  resetComposerDragState();
  if (files.length === 0) {
    return;
  }

  await uploadComposerImages(files);
}

function removeAttachment(attachmentId: string) {
  const sessionId = currentDraftSessionId.value;
  if (!sessionId) {
    return;
  }
  webSessionStore.removeDraftAttachment(props.projectId, sessionId, attachmentId);
}

function focusComposer() {
  nextTick(() => {
    composerInputRef.value?.focus();
  });
}

async function prepareSessionForSend(session: WebSessionSummary) {
  if (!session.archivedAt) {
    return {
      session,
      navigateProjectId: '',
    };
  }

  const restored = await webSessionStore.unarchiveSession(session.projectId, session.id);
  await refreshArchivedSidebar();
  clearArchivedPreviewSession();
  await webSessionStore.ensureSessionConnected(restored.projectId, restored.id);

  return {
    session: restored,
    navigateProjectId: restored.projectId !== props.projectId ? restored.projectId : '',
  };
}

async function handleSubmit() {
  if (isRunActive.value || !hasDraftContent.value) {
    return;
  }
  try {
    let session = currentRealSession.value;
    if (!session || isDraftSession(currentSession.value)) {
      const created = await handleCreateSession();
      session = created ?? webSessionStore.getActiveSession(props.projectId);
    }
    if (!session) {
      return;
    }
    const draftSessionId = currentDraftSessionId.value;
    const draftText = composerRawText.value;
    const attachments = [...draftAttachments.value];
    const prepared = await prepareSessionForSend(session);
    session = prepared.session;
    await webSessionStore.sendMessage(
      session.id,
      draftText,
      attachments.map(item => item.id)
    );
    webSessionStore.clearDraft(props.projectId, draftSessionId);
    if (prepared.navigateProjectId) {
      projectStore.addRecentProject(prepared.navigateProjectId);
      await router.push({
        name: 'project',
        params: { id: prepared.navigateProjectId },
      });
    }
    autoFollowBottom.value = true;
    scrollToBottom(true);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handlePreinput(mode: 'redirect' | 'queue') {
  if (!currentRealSession.value || !hasDraftContent.value) {
    return;
  }
  try {
    const attachments = draftAttachments.value;
    await webSessionStore.sendMessage(
      currentRealSession.value.id,
      composerRawText.value,
      attachments.map(item => item.id),
      mode
    );
    webSessionStore.clearDraft(props.projectId, currentRealSession.value.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function handleComposerEnter(event: KeyboardEvent) {
  if (isRunActive.value) {
    if (hasDraftContent.value) {
      event.preventDefault();
      void handlePreinput('redirect');
    }
    return;
  }
  if (!hasDraftContent.value) {
    return;
  }
  event.preventDefault();
  void handleSubmit();
}

function handleUserInputEnter(event: KeyboardEvent) {
  if (event.key !== 'Enter') {
    return;
  }
  if (event.shiftKey || event.ctrlKey || event.altKey || event.metaKey) {
    return;
  }
  if (event.isComposing || event.keyCode === 229) {
    return;
  }
  event.preventDefault();
  event.stopPropagation();
  void handleUserInputSubmit();
}

function pendingModeLabel(mode: WebSessionPendingInput['mode']) {
  return mode === 'redirect' ? t('webSession.pendingRedirect') : t('webSession.pendingQueue');
}

function pendingInputPreview(item: WebSessionPendingInput) {
  const text = item.text.trim();
  if (text) {
    return text.length > 72 ? `${text.slice(0, 72)}...` : text;
  }
  return t('webSession.pendingAttachments', { count: item.attachmentIds.length });
}

function handleRemovePendingInput(pendingId: string) {
  if (!currentRealSession.value) {
    return;
  }
  webSessionStore.removePendingInput(currentRealSession.value.id, pendingId);
}

function userInputPlaceholder(question: WebSessionUserInputQuestion) {
  if (question.options.length === 0) {
    return t('webSession.userInputAnswerPlaceholder');
  }
  if (question.isOther) {
    return t('webSession.userInputOtherPlaceholder');
  }
  return t('webSession.userInputAnswerPlaceholder');
}

function buildUserInputAnswers() {
  const request = pendingUserInput.value;
  if (!request) {
    return null;
  }
  const answers: Record<string, string[]> = {};
  for (const question of request.questions) {
    const values = [...(userInputSelections.value[question.id] ?? [])];
    const freeform = (userInputDrafts.value[question.id] ?? '').trim();
    if (question.options.length === 0) {
      if (freeform) {
        answers[question.id] = [freeform];
      }
      continue;
    }
    if (question.isOther && freeform) {
      values.push(freeform);
    }
    if (values.length > 0) {
      answers[question.id] = values;
    }
  }
  return answers;
}

function formatSessionInteractionError(error: unknown) {
  const rawMessage = error instanceof Error ? error.message.trim() : '';
  if (rawMessage.includes('session is not running')) {
    return t('webSession.recoveredActionExpired');
  }
  return rawMessage || t('common.error');
}

function findInlinePlanChoiceOption(mode: 'execute' | 'plan') {
  if (!inlinePlanChoice.value) {
    return null;
  }
  return (
    inlinePlanChoice.value.options.find(option => option.isExecute === (mode === 'execute')) ?? null
  );
}

async function answerInlinePlanChoice(mode: 'execute' | 'plan') {
  if (!currentRealSession.value || !pendingUserInput.value || !inlinePlanChoice.value) {
    return false;
  }
  const option = findInlinePlanChoiceOption(mode);
  if (!option || !inlinePlanChoice.value.questionId) {
    return false;
  }
  await webSessionStore.answerUserInput(
    currentRealSession.value.id,
    pendingUserInput.value.itemId,
    {
      [inlinePlanChoice.value.questionId]: [option.label],
    }
  );
  userInputSelections.value = {};
  userInputDrafts.value = {};
  return true;
}

async function handlePlanCardImplement() {
  if (!currentRealSession.value) {
    return;
  }

  try {
    const prepared = await prepareSessionForSend(currentRealSession.value);
    const targetSession = prepared.session;

    if (targetSession.workflowMode === 'plan') {
      await webSessionStore.updateWorkflowMode(targetSession.id, 'default');
    }

    const answered = await answerInlinePlanChoice('execute');
    if (answered) {
      return;
    }

    await webSessionStore.sendMessage(targetSession.id, 'Implement the plan.', []);
    if (prepared.navigateProjectId) {
      projectStore.addRecentProject(prepared.navigateProjectId);
      await router.push({
        name: 'project',
        params: { id: prepared.navigateProjectId },
      });
    }
    autoFollowBottom.value = true;
    scrollToBottom(true);
  } catch (error) {
    message.error(formatSessionInteractionError(error));
  }
}

async function handlePlanCardCancel() {
  const toolId = latestPlanToolId.value;
  setPlanActionsDismissed(toolId, true);
  focusComposer();
}

async function handleUserInputSubmit() {
  if (!currentRealSession.value || !pendingUserInput.value) {
    return;
  }
  if (pendingUserInput.value.stale) {
    message.info(pendingUserInput.value.recoveryMessage || t('webSession.recoveredActionExpired'));
    return;
  }
  const answers = buildUserInputAnswers();
  if (!answers) {
    return;
  }
  const hasMissingAnswer = pendingUserInput.value.questions.some(
    question => !Array.isArray(answers[question.id]) || answers[question.id].length === 0
  );
  if (hasMissingAnswer) {
    message.warning(t('webSession.userInputAnswerRequired'));
    return;
  }
  try {
    await webSessionStore.answerUserInput(
      currentRealSession.value.id,
      pendingUserInput.value.itemId,
      answers
    );
    userInputSelections.value = {};
    userInputDrafts.value = {};
  } catch (error) {
    message.error(formatSessionInteractionError(error));
  }
}

async function handleApproval(action: 'approve' | 'reject') {
  if (!currentRealSession.value || !pendingApproval.value) {
    return;
  }
  if (pendingApproval.value.stale) {
    message.info(pendingApproval.value.recoveryMessage || t('webSession.recoveredActionExpired'));
    return;
  }
  try {
    if (action === 'approve') {
      await webSessionStore.approveSession(currentRealSession.value.id);
      return;
    }
    await webSessionStore.rejectSession(currentRealSession.value.id);
  } catch (error) {
    message.error(formatSessionInteractionError(error));
  }
}

async function handleAbortCurrent() {
  if (!currentRealSession.value) {
    return;
  }
  try {
    await webSessionStore.abortSession(currentRealSession.value.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function syncScrollToBottom() {
  const container = timelineScrollRef.value;
  if (!container) {
    return;
  }
  container.scrollTop = Math.max(0, container.scrollHeight - container.clientHeight);
  autoFollowBottom.value = true;
  showJumpToBottom.value = false;
}

function scheduleScrollToBottom(force = false) {
  nextTick(() => {
    const run = () => {
      const container = timelineScrollRef.value;
      if (!container) {
        return;
      }
      if (force || autoFollowBottom.value) {
        syncScrollToBottom();
      } else {
        updateBottomState(container);
      }
    };

    if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
      run();
      return;
    }

    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(run);
    });
  });
}

function scrollToBottom(force = false) {
  if (!force && !autoFollowBottom.value) {
    return;
  }
  scheduleScrollToBottom(force);
}

function handleLiveCardClick() {
  scrollToBottom(true);
}

function updateBottomState(container: HTMLDivElement) {
  const nearBottom = container.scrollHeight - (container.scrollTop + container.clientHeight) < 160;
  autoFollowBottom.value = nearBottom;
  showJumpToBottom.value = !nearBottom;
}

function restoreHistoryAnchor() {
  const anchor = pendingHistoryAnchor.value;
  const container = timelineScrollRef.value;
  if (!anchor || !container || currentSession.value?.id !== anchor.sessionId) {
    return false;
  }
  container.scrollTop = anchor.previousTop + (container.scrollHeight - anchor.previousHeight);
  pendingHistoryAnchor.value = null;
  updateBottomState(container);
  return true;
}

function handleTimelineScroll(event: Event) {
  const container = event.currentTarget as HTMLDivElement | null;
  if (!container) {
    return;
  }
  const nearTop = container.scrollTop < 120;
  updateBottomState(container);
  if (
    nearTop &&
    !pendingHistoryAnchor.value &&
    currentRealSession.value &&
    historyMeta.value.hasMore &&
    !historyMeta.value.loading
  ) {
    pendingHistoryAnchor.value = {
      sessionId: currentRealSession.value.id,
      previousHeight: container.scrollHeight,
      previousTop: container.scrollTop,
    };
    void webSessionStore.loadMoreHistory(currentRealSession.value.id).catch(error => {
      pendingHistoryAnchor.value = null;
      console.error('[Web Session] Failed to load more history', error);
    });
  }
}

function ensureTimelineHistoryFilled() {
  const container = timelineScrollRef.value;
  if (
    !container ||
    !currentRealSession.value ||
    pendingHistoryAnchor.value ||
    historyMeta.value.loading ||
    !historyMeta.value.hasMore
  ) {
    return;
  }
  const lacksScrollableOverflow = container.scrollHeight <= container.clientHeight + 24;
  if (!lacksScrollableOverflow) {
    return;
  }
  void webSessionStore.loadMoreHistory(currentRealSession.value.id).catch(error => {
    console.error('[Web Session] Failed to auto-fill history', error);
  });
}

function recalcTabTitleWidth(explicitWidth?: number) {
  if (typeof explicitWidth === 'number') {
    tabsContainerWidth.value = explicitWidth;
  }
  const containerWidth =
    typeof explicitWidth === 'number' ? explicitWidth : tabsContainerWidth.value;
  if (!containerWidth) {
    tabTitleMaxWidth.value = MAX_TAB_TITLE_WIDTH;
    return;
  }
  const sessionCount = Math.max(sessions.value.length, 1);
  let activeOffset = TABS_CONTAINER_STATIC_OFFSET;
  if (containerWidth - activeOffset < SHARED_WIDTH_HIDE_THRESHOLD) {
    activeOffset = TABS_CONTAINER_MIN_OFFSET;
  }
  const availableWidth = Math.max(containerWidth - activeOffset, 0);
  const rawWidth = availableWidth / sessionCount - TAB_LABEL_EXTRA_SPACE;
  tabTitleMaxWidth.value = Math.round(Math.min(MAX_TAB_TITLE_WIDTH, Math.max(56, rawWidth)));
}

function updateActiveTabIndicator() {
  nextTick(() => {
    activeTabIndicatorStyle.value =
      !isMobile.value && activeSessionId.value
        ? calculateCardTabIndicatorStyle(tabsContainerRef.value)
        : hiddenCardTabIndicatorStyle();
  });
}

function setupTabScrollListener() {
  cleanupTabScrollListener();
  nextTick(() => {
    if (isMobile.value) {
      return;
    }
    const container = tabsContainerRef.value;
    if (!container) {
      return;
    }
    const scrollContainer = container.querySelector('.v-x-scroll') as HTMLElement | null;
    if (scrollContainer) {
      tabScrollContainer = scrollContainer;
      scrollContainer.addEventListener('scroll', updateActiveTabIndicator);
    }
  });
}

function cleanupTabScrollListener() {
  if (tabScrollContainer) {
    tabScrollContainer.removeEventListener('scroll', updateActiveTabIndicator);
    tabScrollContainer = null;
  }
}

function createTabProps(session: (typeof sessions.value)[number]): HTMLAttributes {
  const isActive = activeSessionId.value === session.id;
  const theme = activeTheme.value;
  const preset = getPresetById(currentPresetId.value);
  const hideHeaderBorder = theme.terminalHeaderBorder === false;
  const props: HTMLAttributes = {
    onContextmenu: (event: MouseEvent) => handleTabContextMenu(event, session),
  };
  const classes: string[] = [];

  if (hasSessionUnviewedApproval(session)) {
    classes.push('has-unviewed-approval');
    if (isActive && hideHeaderBorder) {
      props.style = {
        borderBottom: 'none',
      };
    }
  } else if (hasSessionUnviewedCompletion(session)) {
    classes.push('has-unviewed-completion');
    if (isActive && hideHeaderBorder) {
      props.style = {
        borderBottom: 'none',
      };
    }
  } else if (isActive) {
    props.style = {
      backgroundColor:
        theme.terminalTabActiveBg || preset?.colors.terminalTabActiveBg || theme.surfaceColor,
      ...(hideHeaderBorder ? { borderBottom: 'none' } : {}),
    };
  } else {
    props.style = {
      backgroundColor: theme.terminalTabBg || preset?.colors.terminalTabBg || theme.bodyColor,
    };
  }

  if (classes.length > 0) {
    props.class = classes.join(' ');
  }
  return props;
}

function getSessionAssistantStateClass(session: (typeof sessions.value)[number]) {
  if (isDraftSession(session)) {
    return 'waiting_input';
  }
  const live = webSessionStore.getLiveState(session.id);
  switch (live.phase) {
    case 'starting':
    case 'thinking':
    case 'tool':
      return 'working';
    case 'waiting_approval':
      return 'waiting_approval';
    case 'waiting_input':
      return 'waiting_input';
    case 'done':
    case 'idle':
      return 'waiting_input';
    default:
      return 'unknown';
  }
}

function getSessionStatusLabel(session: (typeof sessions.value)[number]) {
  switch (getSessionAssistantStateClass(session)) {
    case 'working':
      return t('terminal.aiStatusWorking');
    case 'waiting_approval':
      return t('terminal.aiStatusWaitingApproval');
    case 'waiting_input':
      return t('terminal.aiStatusWaitingInput');
    default:
      return '';
  }
}

function getSessionPillStateClass(session: (typeof sessions.value)[number]) {
  if (hasSessionUnviewedCompletion(session)) {
    return 'completion';
  }
  return getSessionAssistantStateClass(session);
}

function getSessionStatusEmoji(session: (typeof sessions.value)[number]) {
  switch (getSessionAssistantStateClass(session)) {
    case 'working':
      return '🤔';
    case 'waiting_approval':
      return '✋';
    case 'waiting_input':
      return '✓';
    default:
      return '';
  }
}

function getSessionAssistantIcon(session: (typeof sessions.value)[number]) {
  return getAssistantIconByType(session.agent === 'claude' ? 'claude-code' : 'codex');
}

function getSessionStatusTooltip(session: (typeof sessions.value)[number]) {
  const label = getSessionStatusLabel(session);
  const agentName = session.agent === 'claude' ? 'Claude Code' : 'Codex';
  if (isDraftSession(session)) {
    return agentName;
  }
  const suffix =
    session.syncState === 'stale'
      ? t('webSession.syncStateStale')
      : session.syncState === 'error' && session.syncError
        ? session.syncError
        : '';
  const base = label ? `${agentName} · ${label}` : agentName;
  return suffix ? `${base} · ${suffix}` : base;
}

function getSidebarSessionSubtitle(item: CrossProjectSessionItem) {
  if (!showSidebarStatusText.value) {
    return '';
  }
  return getSessionStatusLabel(item.session);
}

function getSidebarSessionAccentColor(item: CrossProjectSessionItem) {
  const assistantState = getSessionAssistantStateClass(item.session);
  if (hasSessionUnread(item.session) && assistantState === 'waiting_input') {
    return '#10b981';
  }
  switch (assistantState) {
    case 'working':
      return '#8b5cf6';
    case 'waiting_approval':
      return '#f79009';
    case 'waiting_input':
      return '#9ca3af';
    default:
      if (item.session.status === 'err') {
        return '#f04438';
      }
      return 'rgba(15, 23, 42, 0.08)';
  }
}

function getSidebarSessionClasses(item: CrossProjectSessionItem): string[] {
  const assistantState = getSessionAssistantStateClass(item.session);
  if (hasSessionUnread(item.session) && assistantState === 'waiting_input') {
    return ['session-sidebar-completion'];
  }
  switch (assistantState) {
    case 'working':
      return ['session-sidebar-working'];
    case 'waiting_approval':
      return ['session-sidebar-approval'];
    case 'waiting_input':
      return ['session-sidebar-idle'];
    default:
      if (item.session.status === 'err') {
        return ['session-sidebar-error'];
      }
      return [];
  }
}

function getSessionPillSizeClass() {
  const width = tabTitleMaxWidth.value;
  if (width < 60) {
    return 'pill-size-icon-only';
  }
  if (width < 90) {
    return 'pill-size-icon-emoji';
  }
  return 'pill-size-full';
}

function shouldShowSessionStatusDot(session: (typeof sessions.value)[number]) {
  if (isDraftSession(session)) {
    return false;
  }
  return session.status === 'err' || session.syncState === 'stale';
}

function getSessionStatusDotClass(session: (typeof sessions.value)[number]) {
  if (session.syncState === 'stale') {
    return 'stale';
  }
  return session.status;
}

function hasSessionUnviewedApproval(session: (typeof sessions.value)[number]) {
  return hasSessionUnread(session) && getSessionAssistantStateClass(session) === 'waiting_approval';
}

function hasSessionUnviewedCompletion(session: (typeof sessions.value)[number]) {
  if (!hasSessionUnread(session) || hasSessionUnviewedApproval(session)) {
    return false;
  }
  return getSessionAssistantStateClass(session) === 'waiting_input' && session.status !== 'err';
}

function handleTabContextMenu(event: MouseEvent, session: (typeof sessions.value)[number]) {
  event.preventDefault();
  event.stopPropagation();
  contextMenuSession.value = session;
  contextMenuX.value = event.clientX;
  contextMenuY.value = event.clientY;
}

async function handleContextMenuSelect(key: string | number) {
  const action = String(key);
  const session = contextMenuSession.value;
  contextMenuSession.value = null;
  if (action === 'new') {
    handleStartDraftSession();
    return;
  }
  if (!session) {
    return;
  }
  if (action === 'rename') {
    await handleRenameSession(session.id);
    return;
  }
  if (action === 'sync') {
    await handleSyncSession(session.id);
    return;
  }
  if (action === 'archive') {
    handleArchiveSession(session.id);
    return;
  }
  if (action === 'delete') {
    dialog.warning({
      title: t('common.delete'),
      content: t('webSession.deleteConfirm', { title: session.title }),
      positiveText: t('common.delete'),
      negativeText: t('common.cancel'),
      onPositiveClick: async () => performDeleteSession(session.id),
    });
  }
}

async function handleSyncSession(sessionId: string) {
  const session = sessions.value.find(item => item.id === sessionId);
  if (!session) {
    return;
  }
  try {
    await webSessionStore.syncSession(session.projectId, sessionId);
    message.success(t('webSession.syncSuccess'));
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('webSession.syncFailed'));
  }
}

function handleMobileTabSelect(key: string | number) {
  void handleSessionSelect(String(key));
}

function goToPrevSession() {
  if (!hasPrevSession.value) {
    return;
  }
  const session = sessions.value[currentSessionIndex.value - 1];
  if (session) {
    void handleSessionSelect(session.id);
  }
}

function goToNextSession() {
  if (!hasNextSession.value) {
    return;
  }
  const session = sessions.value[currentSessionIndex.value + 1];
  if (session) {
    void handleSessionSelect(session.id);
  }
}

function setupTabSorting() {
  if (isMobile.value) {
    destroyTabSorting();
    return;
  }
  const container = tabsContainerRef.value;
  if (
    !container ||
    sessions.value.length <= 1 ||
    draftSessions.value.length > 0 ||
    archivedPreviewSession.value
  ) {
    destroyTabSorting();
    return;
  }
  const wrapper = container.querySelector('.n-tabs-wrapper') as HTMLElement | null;
  if (!wrapper) {
    destroyTabSorting();
    return;
  }
  if (tabDragSortable.value) {
    if (tabDragSortable.value.el === wrapper) {
      tabDragSortable.value.option(
        'disabled',
        sessions.value.length <= 1 || draftSessions.value.length > 0 || archivedPreviewSession.value
      );
      return;
    }
    destroyTabSorting();
  }
  tabDragSortable.value = Sortable.create(wrapper, {
    animation: 150,
    direction: 'horizontal',
    draggable: '.n-tabs-tab-wrapper',
    handle: '.n-tabs-tab',
    filter: '.n-tabs-tab__close',
    preventOnFilter: false,
    ghostClass: 'web-session-tab-ghost',
    chosenClass: 'web-session-tab-chosen',
    dragClass: 'web-session-tab-dragging',
    onEnd: handleTabDragEnd,
  });
  tabDragSortable.value.option(
    'disabled',
    sessions.value.length <= 1 || draftSessions.value.length > 0 || archivedPreviewSession.value
  );
}

function destroyTabSorting() {
  if (tabDragSortable.value) {
    tabDragSortable.value.destroy();
    tabDragSortable.value = null;
  }
}

function handleTabDragEnd(event: SortableEvent) {
  if (draftSessions.value.length > 0 || archivedPreviewSession.value) {
    return;
  }
  const fromIndex = event.oldDraggableIndex ?? event.oldIndex ?? -1;
  const toIndex = event.newDraggableIndex ?? event.newIndex ?? -1;
  if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) {
    return;
  }
  void webSessionStore.moveSession(props.projectId, fromIndex, toIndex).catch(error => {
    message.error(error instanceof Error ? error.message : t('common.error'));
  });
  nextTick(() => {
    updateActiveTabIndicator();
  });
}

watch(
  () => props.projectId,
  projectId => {
    if (projectId) {
      void initializeProjectSessions(projectId);
    }
  },
  { immediate: true }
);

watch(
  sidebarProjectIdsToLoad,
  projectIds => {
    projectIds.forEach(projectId => {
      if (!projectId || loadedSidebarProjectIds.has(projectId)) {
        return;
      }
      loadedSidebarProjectIds.add(projectId);
      void webSessionStore.loadSessions(projectId).catch(error => {
        loadedSidebarProjectIds.delete(projectId);
        console.error('[Web Session] Failed to preload sidebar sessions', projectId, error);
      });
    });
  },
  { immediate: true }
);

watch(
  sidebarProjectIdsToLoad,
  projectIds => {
    void webSessionStore
      .loadArchivedSessions(projectIds, {
        reset: true,
        limit: 20,
      })
      .catch(error => {
        console.error('[Web Session] Failed to preload archived sidebar sessions', error);
      });
  },
  { immediate: true }
);

watch(
  () => sidebarContainerWidth.value,
  () => {
    if (!showCrossProjectSidebar.value) {
      return;
    }
    sidebarWidthPx.value = clamp(
      MIN_SESSION_SIDEBAR_WIDTH,
      sidebarWidthPx.value,
      maxSidebarWidthByContainer.value
    );
  }
);

watch(
  showCrossProjectSidebar,
  visible => {
    if (!visible) {
      sidebarResizeObserver?.disconnect();
      sidebarResizeObserver = null;
      sidebarContainerWidth.value = 0;
      return;
    }
    nextTick(() => {
      setupSidebarResizeObserver();
    });
  },
  { immediate: true }
);

watch(
  () => currentSession.value?.id,
  sessionId => {
    stopWebSessionCatchUp('session-change');
    pendingHistoryAnchor.value = null;
    handleCommandExecutionDetailVisibilityChange(false);
    if (!sessionId) {
      showMobileTabSelector.value = false;
      return;
    }
    const session = currentSession.value;
    if (!session) {
      return;
    }
    draftAgent.value = session.agent;
    draftModel.value = session.model || defaultModelForAgent(session.agent);
    draftReasoningEffort.value =
      session.reasoningEffort || defaultReasoningEffortForAgent(session.agent);
    draftWorkflowMode.value = session.workflowMode;
    draftPermissionLevel.value = session.permissionLevel;
    expandedTools.value = {};
    autoFollowBottom.value = true;
    scrollToBottom(true);
    updateActiveTabIndicator();
    if (!isDraftSession(session)) {
      markSessionViewed(session.id);
    }
  },
  { immediate: true }
);

watch(
  () => props.isActive,
  active => {
    if (!active) {
      return;
    }
    markSessionViewed(currentRealSession.value?.id);
  },
  { immediate: true }
);

watch(currentSessionLatestEventSeq, () => {
  markSessionViewed(currentRealSession.value?.id);
});

watch(
  () =>
    sessions.value
      .map(
        session =>
          `${session.id}:${session.orderIndex}:${session.status}:${session.hasUnread}:${getSessionAssistantStateClass(session)}`
      )
      .join('|'),
  () => {
    nextTick(() => {
      recalcTabTitleWidth();
      updateActiveTabIndicator();
      setupTabScrollListener();
      if (isMobile.value) {
        destroyTabSorting();
      } else {
        refreshTabSortable();
      }
    });
  },
  { immediate: true }
);

watch(
  () => isMobile.value,
  mobile => {
    if (mobile) {
      showMobileTabSelector.value = false;
      cleanupTabScrollListener();
      destroyTabSorting();
      activeTabIndicatorStyle.value = hiddenCardTabIndicatorStyle();
      return;
    }
    nextTick(() => {
      setupTabScrollListener();
      refreshTabSortable();
      updateActiveTabIndicator();
    });
  },
  { immediate: true }
);

watch(timelineContentVersion, async () => {
  await nextTick();
  if (restoreHistoryAnchor()) {
    markSessionViewed(currentRealSession.value?.id);
    ensureTimelineHistoryFilled();
    return;
  }
  const container = timelineScrollRef.value;
  if (!container) {
    return;
  }
  if (autoFollowBottom.value) {
    syncScrollToBottom();
  } else {
    updateBottomState(container);
  }
  markSessionViewed(currentRealSession.value?.id);
  ensureTimelineHistoryFilled();
});

useResizeObserver(timelineListRef, () => {
  if (!currentSession.value) {
    return;
  }
  scheduleScrollToBottom();
});

useResizeObserver(timelineScrollRef, entries => {
  const container = entries[0]?.target as HTMLDivElement | undefined;
  if (!container || !currentSession.value) {
    return;
  }
  if (autoFollowBottom.value) {
    scheduleScrollToBottom(true);
  } else {
    updateBottomState(container);
  }
});

watch(
  () => selectedAgent.value,
  value => {
    if (!draftModel.value || (value === 'claude' && draftModel.value.startsWith('gpt-'))) {
      draftModel.value = defaultModelForAgent(value);
    }
    if (value === 'codex' && !draftModel.value.startsWith('gpt-')) {
      draftModel.value = defaultModelForAgent(value);
    }
  }
);

useResizeObserver(tabsContainerRef, entries => {
  const entry = entries[0];
  if (!entry) {
    return;
  }
  const width = entry.contentRect.width;
  if (width !== tabsContainerWidth.value) {
    recalcTabTitleWidth(width);
    updateActiveTabIndicator();
  }
});

onMounted(() => {
  liveStateClockTimer = window.setInterval(() => {
    liveStateClockMs.value = Date.now();
  }, LIVE_TIME_TICK_MS);
  if (projectStore.projects.length === 0) {
    void projectStore.fetchProjects().catch(error => {
      console.error('[Web Session] Failed to preload projects', error);
    });
  }
  window.addEventListener('focus', handleWebSessionWindowFocus);
  window.addEventListener('pageshow', handleWebSessionWindowPageShow);
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', handleWebSessionDocumentVisibilityChange);
  }
  nextTick(() => {
    setupSidebarResizeObserver();
    recalcTabTitleWidth();
    setupTabScrollListener();
    updateActiveTabIndicator();
    if (currentSession.value) {
      syncScrollToBottom();
    }
  });
});

onBeforeUnmount(() => {
  if (liveStateClockTimer != null) {
    window.clearInterval(liveStateClockTimer);
    liveStateClockTimer = null;
  }
  stopWebSessionCatchUp('unmount');
  resetComposerDragState();
  cleanupTabScrollListener();
  destroyTabSorting();
  sidebarResizeObserver?.disconnect();
  sidebarResizeObserver = null;
  window.removeEventListener('focus', handleWebSessionWindowFocus);
  window.removeEventListener('pageshow', handleWebSessionWindowPageShow);
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', handleWebSessionDocumentVisibilityChange);
  }
});
</script>

<style scoped>
.web-session-panel {
  --web-session-approval-bg: rgba(247, 144, 9, 0.25);
  --web-session-approval-border: rgba(247, 144, 9, 0.5);
  height: 100%;
  overflow: hidden;
}

.panel-main {
  height: 100%;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: var(--app-surface-color, var(--n-card-color, #fff));
}

.panel-body {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.panel-content {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 12px 0;
  flex-shrink: 0;
  background-color: var(--app-surface-color, var(--n-card-color, #fff));
  color: var(--app-text-color, var(--n-text-color-1, #1f1f1f));
  border-bottom: var(--kanban-terminal-header-border, 1px solid var(--n-border-color));
  position: relative;
  z-index: 1;
}

.tabs-container {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  padding-right: 8px;
  position: relative;
}

.tabs-container :deep(.n-tabs) {
  width: 100%;
}

.tabs-container :deep(.n-tabs-pane-wrapper) {
  display: none;
}

.tabs-container :deep(.n-tab-pane) {
  padding: 0 !important;
}

.tabs-container :deep(.n-tabs-tab) {
  cursor: grab;
  user-select: none;
}

.tabs-container :deep(.n-tabs-tab:active) {
  cursor: grabbing;
}

.active-tab-indicator {
  position: absolute;
  bottom: 8px;
  left: 0;
  height: 2px;
  background-color: var(--n-primary-color);
  border-radius: 1px;
  transition:
    transform 0.3s cubic-bezier(0.4, 0, 0.2, 1),
    width 0.3s cubic-bezier(0.4, 0, 0.2, 1),
    opacity 0.3s ease;
  z-index: 2;
}

.panel-header :deep(.n-tabs) {
  --n-tab-border-color: var(--n-border-color, rgba(0, 0, 0, 0.1));
  --n-tab-text-color: var(--app-text-color, var(--n-text-color-2, #666));
  --n-tab-text-color-hover: var(--app-text-color, var(--n-text-color-1, #333));
  --n-tab-text-color-active: var(--app-text-color, var(--n-text-color-1, #333));
}

.panel-header :deep(.n-tabs .n-tabs-card-tabs) {
  background-color: transparent;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab) {
  background-color: var(--kanban-terminal-tab-bg, #ffffff) !important;
  color: var(--n-tab-text-color);
  border-color: var(--n-tab-border-color);
  transition:
    background-color 0.2s ease,
    color 0.2s ease;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.n-tabs-tab--active) {
  background-color: var(--kanban-terminal-tab-active-bg, #e8e8e8) !important;
  color: var(--n-tab-text-color-active);
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
}

.tab-title {
  display: inline-block;
  max-width: min(160px, 20vw);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
  background-color: var(--n-text-color-disabled, #c0c4d8);
  box-shadow: 0 0 0 1px var(--n-box-shadow-color, rgba(15, 17, 26, 0.08));
}

.status-dot.running {
  background-color: var(--kanban-terminal-status-connecting, var(--n-color-warning, #f79009));
  box-shadow: 0 0 0 1px rgba(247, 144, 9, 0.25);
}

.status-dot.done {
  background-color: var(--kanban-terminal-status-ready, var(--n-color-success, #12b76a));
  box-shadow: 0 0 0 1px rgba(18, 183, 106, 0.25);
}

.status-dot.err {
  background-color: var(--kanban-terminal-status-error, var(--n-color-error, #f04438));
  box-shadow: 0 0 0 1px rgba(240, 68, 56, 0.25);
}

.status-dot.aborting {
  background-color: var(--n-warning-color, #f59e0b);
  box-shadow: 0 0 0 1px rgba(245, 158, 11, 0.25);
}

.status-dot.stale {
  background-color: #f79009;
  box-shadow: 0 0 0 1px rgba(247, 144, 9, 0.25);
}

.ai-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0 6px;
  margin-bottom: 2px;
  border-radius: 999px;
  font-size: 10px;
  line-height: 16px;
  background-color: #eef2ff;
  color: #6366f1;
  transition: all 0.2s ease;
}

.ai-status-pill.pill-size-full .ai-status-emoji {
  display: none;
}

.ai-status-pill.pill-size-icon-emoji .ai-status-text {
  display: none;
}

.ai-status-pill.pill-size-icon-emoji .ai-status-emoji {
  display: inline;
  font-size: 10px;
  line-height: 1;
}

.ai-status-pill.pill-size-icon-only .ai-status-text,
.ai-status-pill.pill-size-icon-only .ai-status-emoji {
  display: none;
}

.ai-status-pill.pill-size-icon-only {
  padding: 0 4px;
}

.ai-status-pill.state-working {
  background-color: #eadffc;
  color: #7c3aed;
}

.ai-status-pill.state-waiting_approval {
  background-color: #fed7aa;
  color: #f79009;
}

.ai-status-pill.state-completion {
  background-color: rgba(255, 255, 255, 0.84);
  color: #475467;
  border: 1px solid rgba(16, 185, 129, 0.2);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.65);
}

.ai-status-pill.state-waiting_input {
  background-color: #eceef2;
  color: #475467;
}

.ai-status-pill.state-unknown {
  background-color: #f1f5f9;
  color: #94a3b8;
  padding: 0 4px;
}

.ai-status-pill.state-unknown .ai-status-text,
.ai-status-pill.state-unknown .ai-status-emoji {
  display: none;
}

.ai-status-icon {
  display: inline-flex;
  align-items: center;
  line-height: 1;
}

.ai-status-icon :deep(svg) {
  display: block;
}

.ai-status-emoji {
  font-size: 10px;
  line-height: 1;
}

.empty-tabs-label {
  font-size: 13px;
  color: var(--n-text-color-3);
  padding-bottom: 6px;
}

/* TODO: unify terminal/web-session completion tab theming without reusing terminal CSS vars directly. */
.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-unviewed-completion) {
  background-color: rgba(16, 185, 129, 0.16) !important;
  border-color: rgba(16, 185, 129, 0.42) !important;
}

.panel-header
  :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-unviewed-completion.n-tabs-tab--active) {
  background-color: rgba(16, 185, 129, 0.22) !important;
  border-color: rgba(16, 185, 129, 0.54) !important;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  padding-right: 4px;
  margin-left: auto;
}

.new-session-button {
  min-width: 32px;
  width: 32px;
  padding-left: 0 !important;
  padding-right: 0 !important;
}

.mobile-tab-selector {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  padding-bottom: 6px;
}

.mobile-nav-btn,
.mobile-tab-trigger {
  border: 1px solid var(--n-border-color);
  background: var(--app-surface-color, #fff);
  color: var(--app-text-color, var(--n-text-color-2, #666));
  height: 30px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition:
    background-color 0.2s ease,
    color 0.2s ease,
    border-color 0.2s ease,
    transform 0.18s ease;
}

.mobile-nav-btn {
  width: 30px;
  padding: 0;
}

.mobile-nav-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
  transform: none;
}

.mobile-tab-trigger {
  flex: 1;
  min-width: 0;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px;
}

.mobile-tab-title {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.mobile-tab-arrow {
  transition: transform 0.2s ease;
}

.mobile-tab-arrow.is-open {
  transform: rotate(180deg);
}

.agent-select {
  width: 112px;
}

.session-sidebar-shell {
  display: flex;
  min-height: 0;
}

.session-sidebar {
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  background: var(--app-surface-color, var(--n-card-color, #fff));
  padding: 8px;
  display: flex;
  flex-direction: column;
}

.session-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 4px 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--n-primary-color) 8%, var(--n-border-color));
}

.session-sidebar-title-wrap {
  min-width: 0;
}

.session-sidebar-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.session-sidebar-subtitle {
  margin-top: 1px;
  font-size: 10px;
  color: var(--n-text-color-3);
}

.session-sidebar-count {
  min-width: 24px;
  height: 24px;
  padding: 0 6px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  color: var(--n-primary-color);
  font-size: 11px;
  font-weight: 700;
}

.session-sidebar-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 2px 2px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.session-sidebar-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.session-sidebar-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 6px;
  font-size: 11px;
  font-weight: 700;
  color: var(--n-text-color-2);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.session-sidebar-section-count {
  font-variant-numeric: tabular-nums;
}

.session-sidebar-section-empty {
  padding: 8px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--n-border-color) 26%, transparent);
  font-size: 11px;
  color: var(--n-text-color-3);
}

.session-sidebar-empty {
  padding: 20px 12px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.session-sidebar-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 12%, var(--n-border-color));
  border-left: 4px solid var(--session-sidebar-accent, rgba(15, 23, 42, 0.08));
  background: var(--app-surface-color, #fff);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    transform 0.18s ease,
    box-shadow 0.18s ease;
}

.session-sidebar-item:hover {
  transform: none;
  box-shadow: 0 6px 16px rgba(15, 23, 42, 0.12);
}

.session-sidebar-item.is-active {
  border-color: color-mix(
    in srgb,
    var(--session-sidebar-accent, var(--n-primary-color)) 44%,
    var(--n-border-color)
  );
  background: linear-gradient(
    135deg,
    color-mix(
        in srgb,
        var(--session-sidebar-accent, var(--n-primary-color)) 14%,
        var(--app-surface-color, #fff)
      )
      0%,
    color-mix(
        in srgb,
        var(--session-sidebar-accent, var(--n-primary-color)) 6%,
        var(--app-surface-color, #fff)
      )
      100%
  );
  box-shadow: 0 6px 16px
    color-mix(in srgb, var(--session-sidebar-accent, var(--n-primary-color)) 20%, transparent);
}

.session-sidebar-item.is-archived {
  border-style: dashed;
}

.session-sidebar-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}

.session-sidebar-title-line {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.session-sidebar-agent-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 999px;
  background: transparent;
  color: var(--n-primary-color);
  flex-shrink: 0;
}

.session-sidebar-agent-icon :deep(svg) {
  display: block;
}

.session-sidebar-item-title {
  min-width: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.session-sidebar-state-text {
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  font-weight: 500;
  color: var(--n-text-color-3);
}

.session-sidebar-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.session-archived-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 38px;
  height: 18px;
  padding: 0 6px;
  border-radius: 999px;
  background: color-mix(in srgb, #94a3b8 16%, transparent);
  color: color-mix(in srgb, #334155 78%, var(--n-text-color-2));
  font-size: 10px;
  font-weight: 700;
}

.project-index-badge.session-project-badge {
  width: 18px;
  height: 18px;
  font-size: 10px;
  border-width: 1px;
  margin-left: 2px;
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.12),
    0 3px 8px color-mix(in srgb, var(--badge-color, #3b82f6) 16%, transparent);
}

.project-index-badge.session-project-badge.is-single-project {
  visibility: hidden;
  pointer-events: none;
}

.session-current-indicator {
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  border-radius: 50%;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  color: #ffffff;
  border: 1px solid rgba(59, 130, 246, 0.9);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.4);
}

.session-current-indicator.is-hidden {
  opacity: 0;
  pointer-events: none;
}

.session-current-indicator svg {
  display: block;
}

.session-sidebar-working {
  background: color-mix(in srgb, #8b5cf6 8%, var(--app-surface-color, #fff));
}

.session-sidebar-approval {
  border-color: rgba(247, 144, 9, 0.44);
  background: rgba(247, 144, 9, 0.14);
}

.session-sidebar-item.session-sidebar-approval.is-active,
.session-sidebar-item.session-sidebar-approval.is-active:hover {
  border-color: rgba(247, 144, 9, 0.6);
  background: rgba(247, 144, 9, 0.22);
  box-shadow: none;
}

.session-sidebar-completion {
  background: color-mix(in srgb, #10b981 10%, var(--app-surface-color, #fff));
}

.session-sidebar-idle {
  background: color-mix(in srgb, #9ca3af 4%, var(--app-surface-color, #fff));
}

.session-sidebar-error {
  background: color-mix(in srgb, #f04438 8%, var(--app-surface-color, #fff));
}

.session-sidebar-load-more {
  width: 100%;
  border: 1px dashed color-mix(in srgb, var(--n-primary-color) 24%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-primary-color) 5%, transparent);
  color: var(--n-primary-color);
  border-radius: 8px;
  padding: 8px 10px;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    opacity 0.18s ease;
}

.session-sidebar-load-more:hover:not(:disabled) {
  background: color-mix(in srgb, var(--n-primary-color) 9%, transparent);
  border-color: color-mix(in srgb, var(--n-primary-color) 38%, var(--n-border-color));
}

.session-sidebar-load-more:disabled {
  opacity: 0.6;
  cursor: wait;
}

.terminal-resizer {
  flex-shrink: 0;
  width: 6px;
  margin: 0 -3px;
  cursor: col-resize;
  position: relative;
  z-index: 1;
}

.resizer-handle {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 2px;
  height: 24px;
  border-radius: 1px;
  background-color: transparent;
  transition:
    background-color 0.15s ease,
    height 0.15s ease,
    opacity 0.15s ease;
  opacity: 0;
}

.terminal-resizer:hover .resizer-handle {
  background-color: var(--n-border-color, #d0d0d0);
  height: 40px;
  opacity: 1;
}

.terminal-resizer.is-dragging .resizer-handle {
  background-color: var(--n-primary-color, #3b82f6);
  height: 60px;
  opacity: 1;
}

.timeline-shell {
  position: relative;
  flex: 1;
  min-height: 0;
}

.timeline-scroll {
  height: 100%;
  overflow-y: auto;
  overscroll-behavior: contain;
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--n-primary-color) 10%, transparent),
      transparent 26%
    ),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--n-primary-color) 2%, var(--app-body-color, #f7f8fa)),
      var(--app-surface-color, #fff)
    );
}

.timeline-list {
  min-height: 100%;
  padding: 24px 24px 28px;
}

.history-loading {
  display: flex;
  justify-content: center;
  padding: 4px 0 16px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.timeline-loading-shell {
  display: flex;
  flex-direction: column;
  gap: 18px;
  max-width: 940px;
}

.timeline-loading-card {
  height: 52px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--n-border-color) 88%, transparent);
  background: linear-gradient(
    90deg,
    color-mix(in srgb, var(--app-surface-color, #fff) 92%, #eef2f7) 0%,
    color-mix(in srgb, var(--app-surface-color, #fff) 76%, #e3eaf5) 48%,
    color-mix(in srgb, var(--app-surface-color, #fff) 92%, #eef2f7) 100%
  );
  background-size: 220% 100%;
  animation: web-session-sync-loading 1.2s ease-in-out infinite;
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.06);
}

@keyframes web-session-sync-loading {
  0% {
    background-position: 100% 0;
  }
  100% {
    background-position: -100% 0;
  }
}

.timeline-intro {
  max-width: 640px;
  padding: 16px 16px 18px;
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 14%, var(--n-border-color));
  border-radius: 12px;
  background: var(--app-surface-color, #fff);
}

.timeline-intro-badge {
  display: inline-flex;
  align-items: center;
  padding: 5px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  color: var(--n-primary-color);
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
}

.timeline-intro-title {
  margin-top: 14px;
  font-size: 18px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.timeline-intro-text {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--n-text-color-3);
}

.timeline-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 20px;
}

.timeline-item.kind-user {
  align-items: flex-end;
}

.timeline-item.kind-system {
  align-items: flex-start;
}

.timeline-item.kind-tool {
  align-items: flex-start;
}

.item-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.timeline-item.kind-user .item-meta {
  justify-content: flex-end;
}

.item-bubble {
  max-width: min(860px, 84%);
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 10%, var(--n-border-color));
  border-radius: 12px;
  background: var(--app-surface-color, #fff);
  padding: 15px 16px;
}

.timeline-item.kind-user .item-bubble {
  background: color-mix(in srgb, var(--n-primary-color) 10%, rgba(255, 255, 255, 0.92));
  border-color: color-mix(in srgb, var(--n-primary-color) 22%, var(--n-border-color));
  border-top-right-radius: 8px;
}

.timeline-item.kind-system .item-bubble {
  max-width: min(780px, 100%);
  background: color-mix(in srgb, var(--app-surface-color, #fff) 92%, var(--n-primary-color) 8%);
  border-style: dashed;
}

.timeline-history-card-shell {
  width: min(860px, 100%);
}

.item-bubble.level-error {
  border-color: color-mix(in srgb, var(--n-error-color) 35%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-error-color) 7%, rgba(255, 255, 255, 0.9));
}

.item-bubble.level-warn {
  border-color: color-mix(in srgb, var(--n-warning-color) 35%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-warning-color) 10%, rgba(255, 255, 255, 0.92));
}

.item-text {
  min-width: 0;
}

.attachment-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.attachment-pill,
.draft-attachment-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  font-size: 12px;
}

.attachment-preview-trigger {
  min-width: 0;
  max-width: 100%;
  display: inline-flex;
  align-items: center;
  padding: 0;
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: zoom-in;
  transition: color 0.2s ease;
}

.attachment-preview-trigger:hover {
  color: var(--n-primary-color);
}

.attachment-preview-trigger.is-static {
  cursor: default;
}

.attachment-preview-trigger.is-static:hover {
  color: inherit;
}

.attachment-preview-trigger-text {
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.attachment-hover-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  width: min(40vw, 320px);
  min-width: 160px;
  min-height: 120px;
}

.attachment-hover-image {
  display: block;
  max-width: 100%;
  max-height: min(36vh, 240px);
  border-radius: 10px;
  object-fit: contain;
}

.attachment-preview-modal-body {
  display: flex;
  align-items: center;
  justify-content: center;
  max-height: calc(88vh - 96px);
}

.attachment-preview-modal-image {
  display: block;
  max-width: 100%;
  max-height: calc(88vh - 96px);
  border-radius: 12px;
  object-fit: contain;
}

.command-execution-detail-summary {
  margin-bottom: 12px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.command-execution-detail-loading,
.command-execution-detail-empty {
  padding: 16px 4px 8px;
  font-size: 13px;
  color: var(--n-text-color-3);
}

.command-execution-detail-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.command-execution-detail-item {
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 12%, var(--n-border-color));
  border-radius: 12px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 96%, var(--n-primary-color) 4%);
  overflow: hidden;
}

.command-execution-detail-item[open] {
  border-color: color-mix(in srgb, var(--n-primary-color) 22%, var(--n-border-color));
}

.command-execution-detail-item-summary {
  list-style: none;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  cursor: pointer;
}

.command-execution-detail-item-summary::-webkit-details-marker {
  display: none;
}

.command-execution-detail-item-label {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 56px;
  padding: 4px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  color: var(--n-primary-color);
  font-size: 11px;
  font-weight: 700;
}

.command-execution-detail-item-command {
  min-width: 0;
  font-family: 'SFMono-Regular', 'JetBrains Mono', 'Consolas', monospace;
  font-size: 12px;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.command-execution-detail-item-time {
  font-size: 11px;
  color: var(--n-text-color-3);
}

.command-execution-detail-item-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 0 14px 14px;
}

.timeline-tool-shell {
  width: min(860px, 84%);
  max-width: 100%;
}

.tool-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 14px;
}

.tool-card {
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 14%, var(--n-border-color));
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 94%, var(--n-primary-color) 6%);
  overflow: hidden;
}

.tool-card.is-plan-tool {
  border-color: rgba(14, 116, 144, 0.22);
  background:
    linear-gradient(
      135deg,
      rgba(236, 253, 245, 0.98) 0%,
      rgba(240, 249, 255, 0.96) 52%,
      rgba(255, 255, 255, 0.98) 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 18px 36px rgba(8, 47, 73, 0.08);
}

.tool-card.is-plan-tool.is-static-plan-tool {
  overflow: hidden;
  position: relative;
}

.tool-card.is-plan-tool.is-static-plan-tool::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 4px;
  background: linear-gradient(90deg, #14b8a6 0%, #0ea5e9 55%, #38bdf8 100%);
}

.timeline-tool-card {
  width: 100%;
}

.tool-header {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  padding: 12px 14px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.tool-header-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tool-header-leading {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.tool-kind {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-primary-color) 9%, transparent);
  color: var(--n-primary-color);
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

.tool-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tool-state-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

.tool-state-badge.state-running {
  background: rgba(139, 92, 246, 0.12);
  color: #7c3aed;
}

.tool-state-badge.state-done {
  background: rgba(16, 185, 129, 0.12);
  color: #059669;
}

.tool-state-badge.state-error {
  background: rgba(239, 68, 68, 0.12);
  color: #dc2626;
}

.tool-state-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.tool-state-badge.state-running .tool-state-dot {
  animation: livePulse 1.4s ease-in-out infinite;
}

.tool-preview {
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tool-body {
  padding: 0 14px 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.command-tool-card {
  border-color: color-mix(in srgb, #0ea5e9 20%, var(--n-border-color));
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--app-surface-color, #fff) 92%, rgba(14, 165, 233, 0.08)) 0%,
    var(--app-surface-color, #fff) 100%
  );
}

.command-tool-card.state-running {
  border-color: rgba(124, 58, 237, 0.22);
}

.command-tool-card.state-done {
  border-color: rgba(5, 150, 105, 0.2);
}

.command-tool-card.state-error {
  border-color: rgba(220, 38, 38, 0.22);
}

.command-tool-button {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 14px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.command-tool-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.command-tool-topline {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}

.command-tool-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.command-tool-count {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(14, 165, 233, 0.12);
  color: #0369a1;
  font-size: 11px;
  font-weight: 700;
}

.command-tool-time {
  font-size: 11px;
  color: var(--n-text-color-3);
}

.command-tool-command {
  min-width: 0;
  font-family: 'SFMono-Regular', 'JetBrains Mono', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.45;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.plan-tool-body {
  padding: 18px 18px 20px;
  gap: 18px;
}

.plan-tool-header {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.plan-tool-badge {
  display: inline-flex;
  align-items: center;
  padding: 6px 10px;
  border-radius: 999px;
  background: rgba(20, 184, 166, 0.12);
  color: #0f766e;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.plan-tool-caption {
  font-size: 12px;
  line-height: 1.5;
  color: #155e75;
}

.tool-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tool-section-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--n-text-color-3);
  text-transform: uppercase;
}

.tool-code {
  margin: 0;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.5;
  background: color-mix(in srgb, var(--n-primary-color) 8%, transparent);
  border-radius: 8px;
  padding: 10px;
}

.plan-tool-content {
  padding: 18px 20px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.86);
  border: 1px solid rgba(14, 116, 144, 0.1);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.7),
    0 10px 24px rgba(14, 116, 144, 0.06);
}

.plan-tool-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 6px;
  margin-top: 2px;
  background: transparent;
  border: 0;
  box-shadow: none;
}

.plan-tool-action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  justify-content: flex-end;
  margin-left: auto;
}

.plan-tool-action-primary,
.plan-tool-action-secondary {
  min-width: 148px;
}

.runtime-strip {
  margin-top: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.live-card,
.approval-card {
  border: 1px solid var(--n-border-color);
  border-radius: 10px;
  background: var(--app-surface-color, #fff);
}

.live-card {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 12px;
  width: 100%;
  appearance: none;
  text-align: left;
  color: inherit;
  font: inherit;
  cursor: pointer;
  overflow: hidden;
  isolation: isolate;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease,
    transform 0.18s ease,
    box-shadow 0.18s ease;
}

.live-card::before {
  content: none;
}

.live-card::after {
  content: none;
}

.live-card:hover {
  box-shadow: none;
}

.live-card:active {
  box-shadow: none;
}

.live-card:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--n-primary-color) 72%, white);
  outline-offset: 2px;
}

.live-card.phase-starting,
.live-card.phase-thinking,
.live-card.phase-tool {
  border-color: var(--n-border-color);
  background: var(--app-surface-color, #fff);
  box-shadow: none;
}

.live-card.phase-starting::before,
.live-card.phase-thinking::before,
.live-card.phase-tool::before {
  animation: none;
}

.live-card.phase-starting::after,
.live-card.phase-thinking::after,
.live-card.phase-tool::after {
  animation: none;
}

.live-card.phase-waiting_approval,
.live-card.phase-waiting_plan_approval,
.live-card.phase-waiting_input {
  border-color: var(--n-border-color);
  background: var(--app-surface-color, #fff);
  box-shadow: none;
}

.live-card.phase-waiting_approval::before,
.live-card.phase-waiting_plan_approval::before,
.live-card.phase-waiting_input::before {
  animation: none;
}

.live-card.phase-done {
  border-color: var(--n-border-color);
  background: var(--app-surface-color, #fff);
  box-shadow: none;
}

.live-card.phase-done::before {
  animation: none;
}

.live-card.phase-error {
  border-color: var(--n-border-color);
  background: var(--app-surface-color, #fff);
  box-shadow: none;
}

.live-card-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  position: relative;
  z-index: 1;
}

.live-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  position: relative;
  z-index: 1;
}

.live-activity {
  display: inline-flex;
  align-items: flex-end;
  gap: 3px;
  height: 16px;
  padding: 0 2px;
}

.live-activity-bar {
  width: 3px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
  transform-origin: center bottom;
  animation: liveBars 0.95s ease-in-out infinite;
  opacity: 0.9;
}

.live-activity-bar:nth-child(2) {
  animation-delay: 0.14s;
}

.live-activity-bar:nth-child(3) {
  animation-delay: 0.28s;
}

.live-jump-hint {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-primary-color) 12%, transparent);
  color: var(--n-primary-color);
  font-size: 10px;
  font-weight: 600;
  white-space: nowrap;
  opacity: 0;
  transform: translateX(6px);
  transition:
    opacity 0.18s ease,
    transform 0.18s ease,
    background-color 0.18s ease;
}

.live-jump-hint::before {
  content: '↓';
  font-size: 11px;
  line-height: 1;
}

.live-card:hover .live-jump-hint,
.live-card:focus-visible .live-jump-hint,
.live-card.show-jump-hint .live-jump-hint {
  opacity: 1;
  transform: translateX(0);
}

.live-orb {
  position: relative;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--n-text-color-3) 70%, transparent);
  box-shadow: none;
  animation: none;
  flex-shrink: 0;
}

.live-orb::after {
  content: none;
}

.live-card.phase-waiting_input .live-orb,
.live-card.phase-waiting_plan_approval .live-orb,
.live-card.phase-waiting_approval .live-orb,
.approval-badge {
  background: color-mix(in srgb, var(--n-border-color) 72%, var(--app-surface-color, #fff));
}

.live-card.phase-waiting_input .live-orb,
.live-card.phase-waiting_plan_approval .live-orb,
.live-card.phase-waiting_approval .live-orb {
  box-shadow: none;
}

.live-card.phase-waiting_plan_approval .live-orb::after,
.live-card.phase-waiting_approval .live-orb::after,
.live-card.phase-waiting_input .live-orb::after {
  content: none;
}

.live-card.phase-done .live-orb {
  background: color-mix(in srgb, var(--n-text-color-3) 70%, transparent);
  box-shadow: none;
  animation: none;
}

.live-card.phase-done .live-orb::after {
  content: none;
}

.live-card.phase-error .live-orb {
  background: color-mix(in srgb, var(--n-text-color-3) 70%, transparent);
  box-shadow: none;
  animation: none;
}

.live-card.phase-error .live-orb::after {
  content: none;
}

.live-copy {
  min-width: 0;
}

.live-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.live-detail {
  margin-top: 2px;
  font-size: 11px;
  line-height: 1.45;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-height: 16px;
}

.live-detail.is-placeholder {
  color: color-mix(in srgb, var(--n-text-color-3) 78%, transparent);
}

.live-time,
.approval-time {
  font-size: 11px;
  color: var(--n-text-color-3);
  flex-shrink: 0;
}

.approval-card {
  position: relative;
  overflow: hidden;
  border-color: var(--n-border-color);
  background: var(--app-surface-color, #fff);
  padding: 11px 12px;
  box-shadow: none;
}

.approval-card:not(.history-interaction-card)::before {
  content: none;
}

.approval-card > * {
  position: relative;
  z-index: 1;
}

.history-interaction-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.history-interaction-card.type-approval-approve {
  border-color: var(--n-border-color);
  background: var(--app-surface-color, #fff);
}

.history-interaction-card.type-approval-reject {
  border-color: var(--n-border-color);
  background: var(--app-surface-color, #fff);
}

.approval-card.is-stale {
  border: 1px dashed var(--n-border-color);
  background: var(--app-surface-color, #fff);
  box-shadow: none;
}

.approval-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.approval-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  font-size: 11px;
  font-weight: 600;
}

.approval-badge.state-approval-approve {
  background: color-mix(in srgb, var(--n-border-color) 72%, var(--app-surface-color, #fff));
}

.approval-badge.state-approval-reject {
  background: color-mix(in srgb, var(--n-border-color) 72%, var(--app-surface-color, #fff));
}

.approval-badge.state-user-input-request,
.approval-badge.state-user-input-response {
  background: color-mix(in srgb, var(--n-border-color) 72%, var(--app-surface-color, #fff));
}

.approval-prompt {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  white-space: pre-wrap;
}

.history-interaction-prompt {
  margin-top: 0;
}

.approval-note {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
}

.approval-actions {
  margin-top: 10px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.user-input-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.history-question-list,
.history-answer-list {
  gap: 8px;
}

.user-input-question {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 2px;
}

.user-input-question + .user-input-question {
  border-top: 1px dashed var(--n-border-color);
  padding-top: 10px;
}

.history-question-card,
.history-answer-card {
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--n-border-color);
  background: var(--app-surface-color, #fff);
}

.user-input-question-header {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.user-input-question-copy {
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color-2);
}

.user-input-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.user-input-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.user-input-option-label {
  font-weight: 600;
}

.user-input-option-description {
  padding-left: 24px;
  font-size: 11px;
  line-height: 1.45;
  color: var(--n-text-color-3);
}

.history-option-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.history-option-row {
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--app-surface-color, #fff);
  border: 1px solid var(--n-border-color);
}

.history-option-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.history-option-description,
.history-question-note {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color-3);
}

.history-answer-values {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.history-answer-chip {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 5px 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-border-color) 72%, var(--app-surface-color, #fff));
  border: 1px solid var(--n-border-color);
  font-size: 12px;
  line-height: 1.4;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.composer {
  border-top: 1px solid var(--n-border-color);
  padding: 8px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.composer-shell {
  border-radius: 12px;
  padding: 5px 6px 4px;
  background: var(--app-surface-color, #fff);
  transition:
    background-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.composer-shell.is-drag-over {
  background: color-mix(in srgb, var(--n-primary-color) 5%, var(--app-surface-color, #fff));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--n-primary-color) 16%, transparent);
}

.composer-config {
  display: flex;
  align-items: center;
  width: 100%;
  margin-bottom: 2px;
  padding-bottom: 3px;
  border-bottom: 1px solid color-mix(in srgb, var(--n-border-color) 72%, transparent);
}

.composer-config-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
}

.composer-mode-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.composer-select {
  width: 138px;
  flex-shrink: 0;
}

.reasoning-select {
  width: 106px;
}

.composer-mode-switch {
  flex-shrink: 0;
}

.composer-mode-switch :deep(.n-button) {
  min-width: 54px;
}

.permission-select {
  width: 144px;
  flex-shrink: 0;
}

.permission-select :deep(.n-base-selection-label) {
  font-size: 12px;
}

.composer-path {
  min-width: 0;
  flex: 1;
  font-size: 10px;
  color: var(--n-text-color-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: right;
}

.composer-input {
  flex: 1;
}

.composer-input :deep(.n-input-wrapper) {
  background: transparent !important;
  box-shadow: none !important;
  padding-left: 0 !important;
  padding-right: 0 !important;
}

.composer-input :deep(.n-input__border),
.composer-input :deep(.n-input__state-border) {
  display: none !important;
}

.composer-input :deep(.n-input__textarea-el) {
  min-height: 42px !important;
  font-size: 14px;
  line-height: 1.55;
}

.composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-top: 0;
}

.composer-footer-left,
.composer-footer-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.composer-footer-left {
  min-width: 0;
  margin-left: -2px;
}

.composer-icon-btn {
  width: 24px;
  height: 24px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--n-text-color-3);
  cursor: pointer;
  appearance: none;
  -webkit-appearance: none;
  box-shadow: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition:
    background-color 0.2s ease,
    color 0.2s ease;
}

.composer-icon-btn:hover {
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  color: var(--n-primary-color);
}

.composer-hint {
  min-width: 0;
  font-size: 10px;
  line-height: 1.15;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.composer-send-btn,
.composer-stop-btn,
.composer-queue-btn {
  min-width: 84px;
}

.draft-attachments {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 2px;
}

.pending-inputs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 2px;
}

.pending-input-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  max-width: 100%;
  padding: 4px 6px;
  border: 1px solid color-mix(in srgb, var(--n-border-color) 82%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 98%, var(--n-primary-color) 2%);
}

.pending-input-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
}

.pending-input-badge.mode-redirect {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.pending-input-badge.mode-queue {
  background: rgba(99, 102, 241, 0.12);
  color: #4f46e5;
}

.pending-input-preview {
  min-width: 0;
  flex: 1;
  font-size: 11px;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pending-input-remove {
  border: none;
  background: transparent;
  color: var(--n-text-color-3);
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  flex-shrink: 0;
}

.draft-attachment-remove {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
}

.hidden-file-input {
  display: none;
}

:global(.web-session-tab-ghost) {
  opacity: 0.4;
}

:global(.web-session-tab-chosen .n-tabs-tab) {
  box-shadow: 0 0 0 1px var(--n-color-primary);
}

:global(.web-session-tab-dragging .n-tabs-tab) {
  cursor: grabbing !important;
}

@media (max-width: 900px) {
  .panel-header {
    gap: 8px;
    padding-right: 8px;
  }

  .header-actions {
    gap: 6px;
  }

  .runtime-strip {
    margin-top: 14px;
  }

  .item-bubble {
    max-width: 100%;
  }

  .composer-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .composer-footer-left,
  .composer-footer-right {
    width: 100%;
    justify-content: space-between;
  }

  .composer-config-row {
    flex-wrap: wrap;
  }

  .composer-path {
    width: 100%;
    text-align: left;
  }
}

@media (max-width: 640px) {
  .panel-header {
    padding: 6px 8px 0;
  }

  .header-actions {
    gap: 4px;
  }

  .header-actions :deep(.n-button) {
    padding-left: 10px;
    padding-right: 10px;
  }

  .composer-select {
    width: calc(50% - 4px);
  }

  .composer-mode-switch {
    width: auto;
  }

  .composer-mode-switch :deep(.n-button) {
    flex: 1;
  }

  .permission-select {
    width: 148px;
  }

  .composer-mode-row {
    width: 100%;
    justify-content: space-between;
  }

  .pending-inputs {
    gap: 5px;
  }

  .composer-path {
    width: 100%;
  }

  .timeline-list {
    padding: 14px 12px 20px;
  }

  .composer {
    padding: 8px;
  }

  .runtime-strip {
    margin-top: 14px;
  }

  .live-card,
  .approval-card,
  .composer-shell {
    border-radius: 10px;
  }
}

@keyframes livePulse {
  0% {
    transform: scale(1);
    opacity: 1;
  }

  50% {
    transform: scale(1.18);
    opacity: 0.72;
  }

  100% {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes liveRipple {
  0% {
    transform: scale(0.5);
    opacity: 0.56;
  }

  70% {
    opacity: 0;
  }

  100% {
    transform: scale(1.9);
    opacity: 0;
  }
}

@keyframes liveBars {
  0%,
  100% {
    transform: scaleY(0.55);
    opacity: 0.45;
  }

  50% {
    transform: scaleY(1.85);
    opacity: 1;
  }
}

@keyframes liveSweep {
  0% {
    transform: translateX(-130%);
  }

  55% {
    transform: translateX(130%);
  }

  100% {
    transform: translateX(130%);
  }
}

@keyframes liveTrack {
  0% {
    transform: translateX(0);
  }

  100% {
    transform: translateX(380%);
  }
}

@media (max-width: 720px) {
  .command-execution-detail-item-summary {
    grid-template-columns: 1fr auto;
    align-items: start;
  }

  .command-execution-detail-item-label {
    grid-column: 1 / -1;
    width: fit-content;
  }

  .command-execution-detail-item-command {
    white-space: normal;
    word-break: break-word;
  }

  .command-execution-detail-item-time {
    justify-self: end;
  }
}

@media (prefers-reduced-motion: reduce) {
  .live-card,
  .live-jump-hint,
  .live-activity-bar,
  .live-orb,
  .live-orb::after,
  .live-card::before,
  .live-card::after,
  .tool-state-badge.state-running .tool-state-dot {
    animation: none !important;
    transition: none !important;
  }
}
</style>
