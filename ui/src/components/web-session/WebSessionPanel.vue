<template>
  <div class="web-session-panel" :style="webSessionStyleVars">
    <WebSessionCompletionNotifier />
    <WebSessionApprovalNotifier />
    <WebSessionImportDialog
      v-if="props.projectId"
      v-model:show="showImportDialog"
      :project-id="props.projectId"
      :pending-session-id="importingCodexSessionId"
      @import-session="handleImportCodexSession"
      @open-existing-session="handleOpenImportedCodexSession"
    />

    <div class="panel-main">
      <div class="panel-body">
        <div class="panel-content">
          <div class="panel-header">
            <n-dropdown
              v-if="isMobile"
              trigger="manual"
              :placement="mobileTabDropdownPlacement"
              :show="showMobileTabSelector"
              :value="activeSessionId"
              :options="mobileTabOptions"
              :menu-props="mobileTabDropdownMenuProps"
              :node-props="getMobileTabOptionNodeProps"
              :render-label="renderMobileTabOptionLabel"
              :x="mobileTabDropdownX"
              :y="mobileTabDropdownY"
              @select="handleMobileTabSelect"
              @clickoutside="handleMobileTabDropdownClickoutside"
            />
            <div
              v-if="isMobile && (sessions.length > 0 || archivedPreviewSession)"
              class="mobile-tab-selector"
            >
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
              <button
                ref="mobileTabTriggerRef"
                type="button"
                class="mobile-tab-trigger"
                :title="currentSession ? getSessionStatusTooltip(currentSession) : undefined"
                @pointerdown.stop.prevent="handleMobileTabTriggerClick"
                @click.stop.prevent
                @keydown.enter.stop.prevent="handleMobileTabTriggerClick"
                @keydown.space.stop.prevent="handleMobileTabTriggerClick"
              >
                <span class="mobile-tab-trigger-main">
                  <span class="mobile-tab-title">{{ activeSessionTitle }}</span>
                  <span
                    v-if="activeSessionStatusLabel"
                    class="ai-status-pill mobile-tab-trigger-status"
                    :class="`state-${activeSessionAttentionStateClass}`"
                  >
                    <span class="mobile-tab-trigger-status-text">
                      {{ activeSessionStatusLabel }}
                    </span>
                  </span>
                  <span
                    v-if="activeSessionHasWorkflowPlanBadge"
                    class="mobile-tab-trigger-plan-badge"
                    aria-hidden="true"
                  ></span>
                </span>
                <n-icon class="mobile-tab-arrow" :class="{ 'is-open': showMobileTabSelector }">
                  <ChevronDownOutline />
                </n-icon>
              </button>
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
                :value="activeTabSessionId"
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
                        v-if="isSessionArchiving(session.id)"
                        class="tab-action-spinner"
                        aria-hidden="true"
                      ></span>
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

            <div v-else-if="!archivedPreviewSession" class="empty-tabs-label">
              {{ emptyStateTitle }}
            </div>

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
              <n-dropdown
                v-if="isMobile"
                class="mobile-header-action-menu"
                trigger="click"
                placement="bottom-end"
                :options="mobileActionMenuOptions"
                @select="handleMobileActionMenuSelect"
              >
                <n-button
                  secondary
                  size="small"
                  class="new-session-button"
                  :title="t('common.more')"
                  :aria-label="t('common.more')"
                >
                  <template #icon>
                    <n-icon><EllipsisHorizontalOutline /></n-icon>
                  </template>
                </n-button>
              </n-dropdown>
              <n-tooltip v-else trigger="hover" placement="bottom" :delay="100">
                <template #trigger>
                  <n-button
                    text
                    size="small"
                    class="desktop-header-icon-button"
                    :title="t('webSession.newSession')"
                    :aria-label="t('webSession.newSession')"
                    @click="handleStartDraftSession()"
                  >
                    <template #icon>
                      <n-icon><AddOutline /></n-icon>
                    </template>
                  </n-button>
                </template>
                {{ t('webSession.newSession') }}
              </n-tooltip>
              <n-tooltip v-if="!isMobile" trigger="hover" placement="bottom" :delay="100">
                <template #trigger>
                  <n-button
                    text
                    size="small"
                    class="desktop-header-icon-button"
                    :title="t('webSession.importCodexSession')"
                    :aria-label="t('webSession.importCodexSession')"
                    @click="openImportDialog()"
                  >
                    <template #icon>
                      <n-icon><TimeOutline /></n-icon>
                    </template>
                  </n-button>
                </template>
                {{ t('webSession.importCodexSession') }}
              </n-tooltip>
            </div>
          </div>

          <div v-if="currentSession" class="timeline-shell">
            <div
              ref="timelineScrollRef"
              class="timeline-scroll"
              @click.capture="handleTimelineLinkClick"
              @scroll="handleTimelineScroll"
            >
              <div ref="timelineListRef" class="timeline-list">
                <div v-if="historyMeta.loading" class="history-loading">
                  {{
                    currentRealSession?.syncState === 'syncing'
                      ? t('webSession.syncLoading')
                      : t('common.loading')
                  }}
                </div>

                <div
                  v-if="
                    visibleBlocks.length === 0 &&
                    !historyMeta.loading &&
                    currentRealSession?.syncState !== 'syncing'
                  "
                  class="timeline-intro"
                >
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
                    <span class="item-time" :title="formatDateTime(item.timestamp)">{{
                      formatTime(item.timestamp)
                    }}</span>
                  </div>

                  <div
                    v-if="item.kind === 'tool' && item.tool && isPlanTool(item.tool)"
                    class="timeline-tool-shell plan-tool-shell"
                  >
                    <div
                      class="tool-card timeline-tool-card is-plan-tool is-static-plan-tool"
                      :class="{
                        'is-raw-capable': shouldShowPlanRawToggle(item),
                        'is-raw-active': isTimelineRawBlockActive(item, 'plan'),
                      }"
                      :data-raw-toggle-card="
                        shouldShowPlanRawToggle(item)
                          ? getTimelineRawModeKey(item, 'plan')
                          : undefined
                      "
                      :tabindex="shouldShowPlanRawToggle(item) ? 0 : undefined"
                      @click="activateTimelineRawBlock(item, 'plan')"
                      @focusin="activateTimelineRawBlock(item, 'plan')"
                      @keydown.enter.self.prevent="activateTimelineRawBlock(item, 'plan')"
                      @keydown.space.self.prevent="activateTimelineRawBlock(item, 'plan')"
                    >
                      <button
                        v-if="shouldShowTimelineRawToggle(item, 'plan')"
                        type="button"
                        class="timeline-display-toggle"
                        :class="{ 'is-active': isBlockRawMode(item, 'plan') }"
                        :title="t('terminal.rawMode')"
                        @click.stop="toggleBlockRawMode(item, 'plan')"
                      >
                        raw
                      </button>
                      <div class="tool-body plan-tool-body">
                        <div class="plan-tool-header">
                          <span class="plan-tool-badge">{{ t('webSession.planCardBadge') }}</span>
                          <span class="plan-tool-caption">{{
                            t('webSession.planCardCaption')
                          }}</span>
                        </div>
                        <div v-if="item.tool.output" class="plan-tool-content">
                          <pre
                            v-if="isBlockRawMode(item, 'plan')"
                            class="timeline-raw-text plan-tool-content--raw"
                          ><code>{{ item.tool.output }}</code></pre>
                          <div
                            v-else
                            v-memo="getPlanToolMarkdownMemoDeps(item)"
                            class="chat-markdown"
                            v-html="
                              renderMarkdown(
                                getPlanToolMarkdownText(item),
                                getPlanToolMarkdownRenderOptions(item)
                              )
                            "
                          ></div>
                        </div>
                        <div v-if="showPlanActions(item.tool.id)" class="plan-tool-actions">
                          <div class="plan-tool-action-row">
                            <n-button
                              size="small"
                              type="primary"
                              class="plan-tool-action-primary"
                              :loading="isSubmittingPlanExecution"
                              :disabled="isSubmittingMessage"
                              @click="handlePlanCardImplement"
                            >
                              {{ t('webSession.planActionImplement') }}
                            </n-button>
                            <n-button
                              size="small"
                              secondary
                              class="plan-tool-action-secondary"
                              :disabled="isSubmittingMessage"
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
                            <span class="command-tool-time" :title="formatDateTime(item.timestamp)">
                              {{ formatTime(item.timestamp) }}
                            </span>
                          </span>
                          <span
                            class="command-tool-command"
                            :title="getCompactToolSummary(item.tool)"
                          >
                            {{ getCompactToolDisplaySummary(item.tool) }}
                          </span>
                        </span>
                        <span class="tool-state-badge" :class="`state-${item.tool.status}`">
                          <span class="tool-state-dot"></span>
                          {{ toolStateLabel(item.tool) }}
                        </span>
                      </button>
                    </div>

                    <div
                      v-else
                      class="tool-card timeline-tool-card"
                      :class="toolCardClass(item.tool)"
                    >
                      <button
                        type="button"
                        class="tool-header"
                        @click="toggleToolExpanded(item.tool)"
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
                        <div v-if="isImageViewTool(item.tool)" class="tool-section">
                          <div class="tool-section-label">
                            {{ t('webSession.imageViewPreview') }}
                          </div>
                          <div class="image-view-preview-card">
                            <div class="image-view-preview-meta">
                              <span class="image-view-preview-name">
                                <n-icon size="14"><ImageOutline /></n-icon>
                                <span>{{ getImageViewDisplayName(item.tool) }}</span>
                              </span>
                              <span
                                v-if="getImageViewDisplayPath(item.tool)"
                                class="image-view-preview-path"
                                :title="getImageViewDisplayPath(item.tool)"
                              >
                                {{ getImageViewDisplayPath(item.tool) }}
                              </span>
                            </div>
                            <div class="image-view-preview-frame">
                              <div
                                v-if="getImageViewPreviewState(item.tool) !== 'ready'"
                                class="image-view-preview-status"
                                :class="{
                                  'is-error': getImageViewPreviewState(item.tool) === 'error',
                                }"
                              >
                                {{
                                  getImageViewPreviewState(item.tool) === 'error'
                                    ? t('webSession.imageViewLoadFailed')
                                    : t('webSession.imageViewLoading')
                                }}
                              </div>
                              <img
                                v-if="
                                  getImageViewPreviewSrc(item.tool) &&
                                  getImageViewPreviewState(item.tool) !== 'error'
                                "
                                :src="getImageViewPreviewSrc(item.tool)"
                                :alt="getImageViewDisplayName(item.tool)"
                                class="image-view-preview-image"
                                :class="{
                                  'is-ready': getImageViewPreviewState(item.tool) === 'ready',
                                }"
                                loading="lazy"
                                @load="handleImageViewPreviewLoad(item.tool.id)"
                                @error="handleImageViewPreviewError(item.tool.id)"
                              />
                            </div>
                          </div>
                        </div>
                        <div v-if="item.tool.input" class="tool-section">
                          <div class="tool-section-label">{{ t('webSession.toolInput') }}</div>
                          <pre class="tool-code">{{ stringifyValue(item.tool.input) }}</pre>
                        </div>
                        <div v-if="item.tool.output" class="tool-section">
                          <div class="tool-section-label">{{ t('webSession.toolOutput') }}</div>
                          <pre class="tool-code">{{ item.tool.output }}</pre>
                        </div>
                        <div
                          v-else-if="shouldShowToolPendingPlaceholder(item.tool)"
                          class="tool-section"
                        >
                          <div class="tool-section-label">{{ t('webSession.toolOutput') }}</div>
                          <pre class="tool-code">{{ t('common.loading') }}</pre>
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
                        <span class="approval-time" :title="formatDateTime(item.timestamp)">
                          {{ formatTime(item.timestamp) }}
                        </span>
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
                    :class="[
                      item.level ? `level-${item.level}` : undefined,
                      item.itemType ? `type-${item.itemType}` : undefined,
                      {
                        'is-raw-capable': shouldShowMessageRawToggle(item),
                        'is-raw-active': isTimelineRawBlockActive(item, 'message'),
                      },
                    ]"
                    :data-raw-toggle-card="
                      shouldShowMessageRawToggle(item)
                        ? getTimelineRawModeKey(item, 'message')
                        : undefined
                    "
                    :tabindex="shouldShowMessageRawToggle(item) ? 0 : undefined"
                    @mouseenter="handleMessageBubbleMouseEnter(item)"
                    @mouseleave="handleMessageBubbleMouseLeave(item)"
                    @click="handleMessageBubbleClick(item)"
                    @focusin="activateTimelineRawBlock(item, 'message')"
                    @focusout="handleMessageBubbleFocusOut(item, $event)"
                    @keydown.enter.self.prevent="activateTimelineRawBlock(item, 'message')"
                    @keydown.space.self.prevent="activateTimelineRawBlock(item, 'message')"
                  >
                    <button
                      v-if="shouldShowTimelineRawToggle(item, 'message')"
                      type="button"
                      class="timeline-display-toggle"
                      :class="{ 'is-active': isBlockRawMode(item, 'message') }"
                      :title="t('terminal.rawMode')"
                      @click.stop="toggleBlockRawMode(item, 'message')"
                    >
                      raw
                    </button>
                    <pre
                      v-if="shouldShowMessageRawToggle(item) && isBlockRawMode(item, 'message')"
                      class="item-text item-text--raw timeline-raw-text"
                    ><code>{{ item.text }}</code></pre>
                    <div
                      v-else-if="getDisplayBlockText(item)"
                      v-memo="getMessageMarkdownMemoDeps(item)"
                      class="item-text chat-markdown"
                      v-html="
                        renderMarkdown(
                          getMessageMarkdownText(item),
                          getMessageMarkdownRenderOptions(item)
                        )
                      "
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
                          placement="bottom-start"
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
                      `phase-${displayLiveState.phase}`,
                      {
                        'show-jump-hint': showJumpToBottom,
                      },
                    ]"
                    :aria-label="liveCardAriaLabel"
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
                      <n-tooltip placement="top-end" :delay="120">
                        <template #trigger>
                          <span class="live-time">{{ getLiveTimeText(displayLiveState) }}</span>
                        </template>
                        <div class="live-time-tooltip">
                          <div
                            v-for="item in getLiveTimeTooltipItems(displayLiveState)"
                            :key="item.key"
                            class="live-time-tooltip-row"
                          >
                            <span class="live-time-tooltip-label">{{ item.label }}</span>
                            <span class="live-time-tooltip-value">{{ item.value }}</span>
                          </div>
                        </div>
                      </n-tooltip>
                    </div>
                  </button>

                  <div
                    v-if="pendingApproval"
                    class="approval-card"
                    :class="{ 'is-stale': pendingApproval.stale }"
                  >
                    <div class="approval-card-header">
                      <span class="approval-badge">{{ t('webSession.approvalTitle') }}</span>
                      <span
                        class="approval-time"
                        :title="formatDateTime(pendingApproval.requestedAt)"
                        >{{ formatTime(pendingApproval.requestedAt) }}</span
                      >
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
                      <span
                        class="approval-time"
                        :title="formatDateTime(pendingUserInput.requestedAt)"
                        >{{ formatTime(pendingUserInput.requestedAt) }}</span
                      >
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
                        v-if="question.options.length > 0 && question.multiSelect"
                        v-model:value="userInputSelections[question.id]"
                        :disabled="isUserInputInteractionDisabled"
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
                      <n-radio-group
                        v-else-if="question.options.length > 0"
                        :value="userInputSelections[question.id]?.[0] || null"
                        :disabled="isUserInputInteractionDisabled"
                        class="user-input-options"
                        @update:value="handleUserInputSingleSelect(question.id, $event)"
                      >
                        <div
                          v-for="option in question.options"
                          :key="`${question.id}:${option.label}`"
                          class="user-input-option"
                        >
                          <n-radio :value="option.label">
                            <span class="user-input-option-label">{{ option.label }}</span>
                          </n-radio>
                          <div v-if="option.description" class="user-input-option-description">
                            {{ option.description }}
                          </div>
                        </div>
                      </n-radio-group>
                      <n-input
                        v-if="question.isOther || question.options.length === 0"
                        v-model:value="userInputDrafts[question.id]"
                        :type="question.isSecret ? 'password' : 'text'"
                        size="small"
                        :disabled="isUserInputInteractionDisabled"
                        :show-password-on="question.isSecret ? 'mousedown' : undefined"
                        :placeholder="userInputPlaceholder(question)"
                        @keydown="handleUserInputEnter"
                      />
                    </div>
                    <div class="approval-actions">
                      <n-button
                        size="small"
                        type="primary"
                        :loading="isSubmittingUserInput"
                        :disabled="isUserInputInteractionDisabled"
                        @click="handleUserInputSubmit"
                      >
                        {{ t('webSession.userInputSubmit') }}
                      </n-button>
                      <n-button
                        size="small"
                        tertiary
                        :disabled="isUserInputInteractionDisabled"
                        @click="handleAbortCurrent"
                      >
                        {{ t('webSession.stop') }}
                      </n-button>
                    </div>
                    <div
                      v-if="showUserInputSubmitSlowHint"
                      class="approval-note"
                      aria-live="polite"
                    >
                      {{ t('webSession.userInputSubmitSlow') }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="!currentSession" class="empty-state">
            <n-empty :description="emptyStateDescription" />
          </div>

          <div
            class="composer"
            :class="{
              'is-drag-over': isComposerDragOver,
              'is-mobile-expanded': isMobile && isMobileComposerExpanded,
              'is-mobile-focused': isMobileComposerFocused,
            }"
            @paste.capture="handleComposerPaste"
            @dragenter="handleComposerDragEnter"
            @dragover="handleComposerDragOver"
            @dragleave="handleComposerDragLeave"
            @drop="handleComposerDrop"
          >
            <input
              ref="fileInputRef"
              type="file"
              accept="image/*"
              multiple
              class="hidden-file-input"
              @change="handleFileChange"
            />

            <div v-if="isMobile" class="composer-mobile-summary">
              <button
                type="button"
                class="composer-mobile-toggle"
                :aria-expanded="isMobileComposerExpanded"
                @click="toggleMobileComposerExpanded"
              >
                <span class="composer-mobile-toggle-copy">
                  <span class="composer-mobile-toggle-chips">
                    <span
                      v-for="token in mobileComposerSummaryTokens"
                      :key="token.key"
                      class="composer-mobile-toggle-chip"
                    >
                      {{ token.label }}
                    </span>
                  </span>
                </span>
                <n-icon
                  class="composer-mobile-toggle-arrow"
                  :class="{ 'is-open': isMobileComposerExpanded }"
                >
                  <ChevronDownOutline />
                </n-icon>
              </button>
            </div>

            <div
              v-if="!isMobile || isMobileComposerExpanded"
              class="composer-config"
              :class="{ 'is-mobile': isMobile }"
            >
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
                <div class="composer-auto-continue">
                  <n-checkbox v-model:checked="webSessionAutoContinueEnabledValue" size="small">
                    {{ t('webSession.infiniteRetry') }}
                  </n-checkbox>
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
                  placement="bottom-start"
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

            <div class="composer-input-shell" :class="{ 'is-mobile': isMobile }">
              <n-input
                ref="composerInputRef"
                v-model:value="composerText"
                type="textarea"
                class="composer-input"
                :autosize="composerAutosize"
                :placeholder="composerPlaceholder"
                @mousedown.stop
                @touchstart.stop
                @focus="handleComposerFocus"
                @blur="handleComposerBlur"
                @keydown.enter.exact="handleComposerEnter"
              />
            </div>

            <div class="composer-footer" :class="{ 'is-mobile': isMobile }">
              <div v-if="isMobile" class="composer-footer-left composer-footer-left-mobile">
                <n-popover
                  v-model:show="showQuickInputPopover"
                  trigger="manual"
                  placement="top-start"
                  content-style="padding: 4px 0;"
                  @clickoutside="handleMobileQuickInputClickOutside"
                >
                  <template #trigger>
                    <button
                      type="button"
                      class="composer-icon-btn composer-icon-btn-mobile"
                      :title="quickInputButtonTitle"
                      :aria-label="quickInputButtonTitle"
                      @pointerdown.stop.prevent="handleMobileQuickInputTrigger"
                      @click.stop.prevent
                      @keydown.enter.stop.prevent="handleMobileQuickInputTrigger"
                      @keydown.space.stop.prevent="handleMobileQuickInputTrigger"
                    >
                      <n-icon size="18"><FlashOutline /></n-icon>
                    </button>
                  </template>
                  <div class="quick-input-popover-card">
                    <div class="quick-input-popover-header">
                      <n-checkbox
                        v-model:checked="quickInputDirectSendEnabled"
                        size="small"
                        @mousedown.stop
                        @touchstart.stop
                        @click.stop
                      >
                        {{ t('webSession.quickInputDirectSend') }}
                      </n-checkbox>
                    </div>
                    <div v-if="quickInputItems.length === 0" class="quick-input-empty">
                      {{ t('webSession.quickInputEmpty') }}
                    </div>
                    <div v-else class="quick-input-scroll">
                      <div class="quick-input-item-list">
                        <button
                          v-for="text in quickInputItems"
                          :key="text"
                          type="button"
                          class="quick-input-item"
                          :class="{ 'is-selected': isQuickInputSelected(text) }"
                          @click="handleQuickInputApply(text)"
                        >
                          <span class="quick-input-item-text">{{ text }}</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </n-popover>
                <button
                  type="button"
                  class="composer-icon-btn composer-icon-btn-mobile composer-icon-btn-mobile-secondary"
                  :title="t('webSession.attachImage')"
                  :aria-label="t('webSession.attachImage')"
                  @pointerdown.stop.prevent="handleMobileAttachmentTrigger"
                  @click.stop.prevent
                  @keydown.enter.stop.prevent="handleMobileAttachmentTrigger"
                  @keydown.space.stop.prevent="handleMobileAttachmentTrigger"
                >
                  <n-icon size="18"><ImageOutline /></n-icon>
                </button>
              </div>
              <div v-if="!isMobile" class="composer-footer-left">
                <n-popover
                  v-model:show="showQuickInputPopover"
                  trigger="click"
                  placement="top-start"
                  content-style="padding: 4px 0;"
                >
                  <template #trigger>
                    <button
                      type="button"
                      class="composer-icon-btn"
                      :title="quickInputButtonTitle"
                      :aria-label="quickInputButtonTitle"
                    >
                      <n-icon size="14"><FlashOutline /></n-icon>
                    </button>
                  </template>
                  <div class="quick-input-popover-card">
                    <div class="quick-input-popover-header">
                      <n-checkbox
                        v-model:checked="quickInputDirectSendEnabled"
                        size="small"
                        @mousedown.stop
                        @touchstart.stop
                        @click.stop
                      >
                        {{ t('webSession.quickInputDirectSend') }}
                      </n-checkbox>
                    </div>
                    <div v-if="quickInputItems.length === 0" class="quick-input-empty">
                      {{ t('webSession.quickInputEmpty') }}
                    </div>
                    <div v-else class="quick-input-scroll">
                      <div class="quick-input-item-list">
                        <button
                          v-for="text in quickInputItems"
                          :key="text"
                          type="button"
                          class="quick-input-item"
                          :class="{ 'is-selected': isQuickInputSelected(text) }"
                          @click="handleQuickInputApply(text)"
                        >
                          <span class="quick-input-item-text">{{ text }}</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </n-popover>
                <button type="button" class="composer-icon-btn" @click="openFilePicker">
                  <n-icon size="14"><ImageOutline /></n-icon>
                </button>
                <span class="composer-hint">{{ composerHint }}</span>
              </div>

              <div class="composer-footer-right">
                <n-tooltip v-if="contextUsageIndicator" trigger="hover" placement="top">
                  <template #trigger>
                    <span
                      class="composer-context-pill"
                      :class="`state-${contextUsageIndicator.state}`"
                    >
                      {{ contextUsageIndicator.label }}
                    </span>
                  </template>
                  <div class="composer-context-tooltip">
                    <div class="composer-context-tooltip-title">
                      {{ contextUsageIndicator.title }}
                    </div>
                    <div
                      v-for="line in contextUsageIndicator.lines"
                      :key="line"
                      class="composer-context-tooltip-line"
                    >
                      {{ line }}
                    </div>
                  </div>
                </n-tooltip>
                <n-button
                  v-if="isRunActive"
                  secondary
                  type="warning"
                  class="composer-stop-btn"
                  @click="handleAbortCurrent"
                >
                  {{ t('webSession.stop') }}
                </n-button>
                <template v-if="isRunActive">
                  <n-button
                    secondary
                    class="composer-queue-btn"
                    :disabled="!canStageDuringRun"
                    @click="handlePreinput('queue')"
                  >
                    {{ t('webSession.preinputQueue') }}
                  </n-button>
                  <n-button
                    type="primary"
                    class="composer-send-btn"
                    :disabled="!canStageDuringRun"
                    @click="handlePreinput('redirect')"
                  >
                    {{ t('webSession.preinputRedirect') }}
                  </n-button>
                </template>
                <n-popover
                  v-else
                  trigger="manual"
                  placement="top-end"
                  :show="showSendConflictWarning"
                  :show-arrow="true"
                  :disabled="!showSendConflictWarning"
                >
                  <template #trigger>
                    <n-button
                      type="primary"
                      :class="[
                        'composer-send-btn',
                        { 'is-confirm-armed': isSendConflictConfirmationArmed },
                      ]"
                      :loading="isSubmittingMessage && !isSubmittingPlanExecution"
                      :disabled="!canSend"
                      @click="handleSubmit"
                    >
                      {{
                        isSendConflictConfirmationArmed
                          ? t('webSession.sendEmphatic')
                          : t('webSession.send')
                      }}
                    </n-button>
                  </template>
                  <div class="composer-send-confirm-popover-card" role="status" aria-live="polite">
                    <div class="composer-send-confirm-title">
                      {{ t('webSession.sendConflictWarningTitle') }}
                    </div>
                    <div class="composer-send-confirm-body">
                      {{ sendConflictWarningBody }}
                    </div>
                  </div>
                </n-popover>
              </div>
            </div>
            <TransferProgressDialog
              v-if="composerTransferCard"
              :message="composerTransferCard.message"
              :detail="composerTransferCard.detail"
              :progress="composerTransferCard.progress"
              :tone="composerTransferCard.tone"
              :card-style="composerTransferDialogStyle"
            />
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
                <SplitDropdownControl
                  class="session-sidebar-scope-control"
                  :label="sidebarScopeLabel"
                  :options="sidebarScopeOptions"
                  :title="sidebarScopeAriaLabel"
                  :menu-title="sidebarScopeAriaLabel"
                  :aria-label="sidebarScopeAriaLabel"
                  flat
                  @main-click="toggleSidebarScope"
                  @select="handleSidebarScopeSelect"
                >
                  <template #prefix>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                      <path
                        d="M4 7h16M4 12h10M4 17h8"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                  </template>
                </SplitDropdownControl>
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
                    {
                      'has-workflow-plan-badge': shouldShowSessionWorkflowPlanBadge(item.session),
                      'is-archiving': isSessionArchiving(item.session.id),
                    },
                    { 'is-active': item.isCurrent },
                  ]"
                  :style="{ '--session-sidebar-accent': getSidebarSessionAccentColor(item) }"
                  :title="getSidebarSessionTitle(item)"
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
                      v-if="isSessionArchiving(item.session.id)"
                      class="session-sidebar-spinner"
                      aria-hidden="true"
                    ></span>
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
                    {
                      'has-workflow-plan-badge': shouldShowSessionWorkflowPlanBadge(item.session),
                    },
                    { 'is-active': item.isCurrent },
                  ]"
                  :style="{ '--session-sidebar-accent': getSidebarSessionAccentColor(item) }"
                  :title="getSidebarSessionTitle(item)"
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
            <span
              class="command-execution-detail-item-time"
              :title="formatCommandExecutionDetailDateTime(detailItem)"
            >
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
  type Component,
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
import { useRoute, useRouter } from 'vue-router';
import { useDebounceFn, useEventListener, useResizeObserver, useStorage } from '@vueuse/core';
import { storeToRefs } from 'pinia';
import {
  NCheckbox,
  NIcon,
  NInput,
  useDialog,
  useMessage,
  type DialogReactive,
  type DropdownOption,
} from 'naive-ui';
import {
  AddOutline,
  ArchiveOutline,
  ChevronBackOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  CreateOutline,
  EllipsisHorizontalOutline,
  FlashOutline,
  ImageOutline,
  RefreshCircleOutline,
  RefreshOutline,
  TimeOutline,
  TrashOutline,
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
import type {
  WebSessionCodexRuntimeConfig,
  WebSessionContextWindowSource,
  WebSessionSummary,
} from '@/types/models';
import {
  calculateCardTabIndicatorStyle,
  hiddenCardTabIndicatorStyle,
} from '@/utils/cardTabIndicator';
import { getDefaultTerminalTheme, getTerminalThemeById } from '@/constants/terminalThemes';
import { getAssistantIconByType } from '@/utils/assistantIcon';
import { hexToRgba, isDarkHex } from '@/utils/color';
import { renderMarkdown } from '@/utils/markdown';
import { resolveNavigableHref } from '@/utils/messageLinkNavigation';
import {
  buildImagePlaceholder,
  buildImageViewPreviewUrl,
  insertImagePlaceholdersAtCursor,
  parseImageViewToolOutput,
  resolveImageAttachmentDisplayName,
  stripImagePlaceholdersFromText,
  resolveImageViewDisplayName,
} from '@/utils/webSessionImages';
import { urlBase } from '@/api';
import { http } from '@/api/http';
import { webSessionApi } from '@/api/webSession';
import TransferProgressDialog from '@/components/common/TransferProgressDialog.vue';
import SplitDropdownControl from '@/components/common/SplitDropdownControl.vue';
import WebSessionApprovalNotifier from '@/components/web-session/WebSessionApprovalNotifier.vue';
import WebSessionCompletionNotifier from '@/components/web-session/WebSessionCompletionNotifier.vue';
import WebSessionImportDialog from '@/components/web-session/WebSessionImportDialog.vue';
import {
  buildTimelineRawModeKey,
  pruneActiveTimelineRawBlockKey,
  resolveActivatedTimelineRawBlockKey,
  shouldClearActiveTimelineRawBlockKey,
  shouldShowTimelineRawToggle as shouldShowTimelineRawToggleForBlock,
  toggleExclusiveTimelineRawBlock,
  type TimelineRawSurface,
} from '@/components/web-session/webSessionRawToggle';
import { createWebSessionStreamingMarkdownController } from '@/components/web-session/webSessionStreamingMarkdown';
import {
  getWebSessionSidebarTone,
  getWebSessionTabTone,
} from '@/components/web-session/sessionVisualState';
import {
  beginWebSessionSubmit,
  buildWebSessionSubmitOwnerId,
  endWebSessionSubmit,
  getWebSessionSubmitEntry,
  isWebSessionSubmitting,
  resolveOptimisticWebSessionLiveState,
  shouldShowWebSessionExecuteFeedback,
  transferWebSessionSubmit,
  type WebSessionSubmitKind,
  type WebSessionSubmitState,
} from '@/components/web-session/webSessionSubmitState';
import {
  buildWebSessionUserInputSubmitOwnerId,
  hasMissingWebSessionUserInputAnswers,
  scheduleWebSessionUserInputSlowHint,
} from '@/components/web-session/webSessionUserInputSubmit';
import {
  buildWebSessionUserInputDraftSyncKey,
  reconcileWebSessionUserInputLocalState,
} from '@/components/web-session/webSessionUserInputDraftSync';
import {
  buildWebSessionSendConfirmationSignature,
  findWebSessionSendConflicts,
  resolveWebSessionSendConfirmation,
  type WebSessionSendConfirmationState,
} from '@/components/web-session/webSessionSendGuard';
import {
  resolveWebSessionDisplayState,
  resolveWebSessionSidebarSortTimestamp,
  type WebSessionDisplayState,
} from '@/components/web-session/webSessionSessionState';
import {
  formatWebSessionDateTime,
  formatWebSessionTimestamp,
} from '@/components/web-session/webSessionTimeFormat';
import {
  buildOrderedTabSessions,
  resolveActiveTabSessionId,
  resolveUnderlyingTabSessionId,
  sortMobileCurrentSessions,
} from '@/components/web-session/webSessionTabOrder';
import {
  buildWebSessionMobileTabDescriptors,
  MOBILE_NEW_SESSION_OPTION_KEY,
  type MobileSessionCategory,
} from '@/components/web-session/webSessionMobileTabOptions';
import {
  collapseProjectDraftTabs,
  pickPreferredDraftTab,
  resolveStartDraftSessionDecision,
} from '@/components/web-session/webSessionDraftTabs';
import {
  normalizeWebSessionSidebarScope,
  resolveWebSessionSidebarProjectIds,
  resolveWebSessionSidebarToggleScope,
  type WebSessionSidebarScope,
} from '@/components/web-session/webSessionSidebarScope';
import { normalizeWebSessionSyncState } from '@/utils/webSessionSyncState';
import { createWebSessionSnapshotLoadController } from '@/utils/webSessionSnapshotLoadController';
import { buildWorkspaceRouteQuery, inferWorkspaceRouteTab } from '@/utils/workspaceRoute';
import {
  buildWebSessionRouteQuery,
  getWebSessionRouteSessionId,
  isWebSessionRouteQuerySynced,
  resolveWebSessionDeepLinkTarget,
  shouldPreserveWebSessionRouteSessionId,
} from '@/utils/webSessionRoute';

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
const TAB_ORDER_STORAGE_KEY = 'workspace-web-session-tab-order';
const TAB_MRU_STORAGE_KEY = 'workspace-web-session-tab-mru';
const SIDEBAR_SCOPE_STORAGE_KEY = 'workspace-web-session-sidebar-scope';
const LIVE_TIME_TICK_MS = 1000;
const DEFAULT_CODEX_CONTEXT_WINDOW_TOKENS = 400000;
const WEB_SESSION_SEND_CONFIRM_TTL_MS = 5000;
const MOBILE_TAB_SELECTOR_CLICKOUTSIDE_GUARD_MS = 220;
const STREAMING_MARKDOWN_RENDER_OPTIONS = Object.freeze({
  disableCodeHighlight: true,
});
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

const emit = defineEmits<{
  (event: 'mobile-composer-focus-change', focused: boolean): void;
  (event: 'request-mobile-view', view: 'webSession'): void;
}>();

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

type MobileTabSessionOption = DropdownOption & {
  kind: 'session';
  key: string;
  label: string;
  section: MobileSessionCategory;
  session: SessionTab;
  displayState: WebSessionDisplayState;
  tooltip: string;
};

type MobileTabActionOption = DropdownOption & {
  kind: 'new-session';
  key: typeof MOBILE_NEW_SESSION_OPTION_KEY;
  label: string;
  section: 'current';
};

type MobileTabRenderOption = {
  type: 'render';
  key: string;
  render: () => ReturnType<typeof h>;
  props?: HTMLAttributes;
};

type MobileTabDropdownOption =
  | MobileTabSessionOption
  | MobileTabActionOption
  | MobileTabRenderOption;
type MobileTabSelectorSource = 'header' | 'bottom-nav';
type MobileTabSelectorAnchor = {
  source: MobileTabSelectorSource;
  x: number;
  y: number;
  width: number;
};

function isMobileTabSessionOption(
  option: DropdownOption | MobileTabDropdownOption
): option is MobileTabSessionOption {
  return (option as MobileTabSessionOption | undefined)?.kind === 'session';
}

function isMobileTabActionOption(
  option: DropdownOption | MobileTabDropdownOption
): option is MobileTabActionOption {
  return (option as MobileTabActionOption | undefined)?.kind === 'new-session';
}

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

type LiveTimeTooltipItem = {
  key: string;
  label: string;
  value: string;
};

type ImageViewPreviewState = 'loading' | 'ready' | 'error';

function isDraftSession(session: SessionTab | null | undefined): session is DraftSessionTab {
  return Boolean(session && 'isDraft' in session && session.isDraft);
}

function isArchivedPreviewSession(
  session: SessionTab | null | undefined
): session is ArchivedPreviewSessionTab {
  return Boolean(session && 'isArchivedPreview' in session && session.isArchivedPreview);
}

function isAbortLikeError(error: unknown) {
  return Boolean(
    error &&
    typeof error === 'object' &&
    'name' in error &&
    String((error as { name?: unknown }).name || '') === 'AbortError'
  );
}

const webSessionStore = useWebSessionStore();
const projectStore = useProjectStore();
const settingsStore = useSettingsStore();
const route = useRoute();
const router = useRouter();
const dialog = useDialog();
const message = useMessage();
const { locale, t } = useLocale();
const { isMobile } = useResponsive();
const {
  activeTheme,
  currentPresetId,
  confirmBeforeTerminalClose,
  showWebSessionReasoning,
  webSessionAutoContinueScope,
  webSessionAutoContinuePreset,
  webSessionStreamingMarkdownThrottleMs,
  effectiveTerminalThemeId,
  webSessionQuickInput,
  webSessionQuickInputDirectSend,
} = storeToRefs(settingsStore);
const persistedDraftSessionsByProject = useStorage<Record<string, DraftSessionTab[]>>(
  DRAFT_SESSION_STORAGE_KEY,
  {}
);
const persistedActiveDraftSessionIdByProject = useStorage<Record<string, string>>(
  ACTIVE_DRAFT_SESSION_STORAGE_KEY,
  {}
);
const persistedTabOrderByProject = useStorage<Record<string, string[]>>(TAB_ORDER_STORAGE_KEY, {});
const persistedTabMruByProject = useStorage<Record<string, string[]>>(TAB_MRU_STORAGE_KEY, {});
const routeWebSessionId = computed(() => getWebSessionRouteSessionId(route.query));
const routeWorkspaceTab = computed(() => inferWorkspaceRouteTab(route.query));

const tabsContainerRef = ref<HTMLElement | null>(null);
const mobileTabTriggerRef = ref<HTMLButtonElement | null>(null);
const timelineScrollRef = ref<HTMLDivElement | null>(null);
const timelineListRef = ref<HTMLDivElement | null>(null);
const fileInputRef = ref<HTMLInputElement | null>(null);
const composerInputRef = ref<InstanceType<typeof NInput> | null>(null);
const sidebarRootRef = ref<HTMLElement | null>(null);
const autoFollowBottom = ref(true);
const showJumpToBottom = ref(false);
const expandedTools = ref<Record<string, boolean>>({});
const imageViewPreviewSrcByToolId = ref<Record<string, string>>({});
const imageViewPreviewStateByToolId = ref<Record<string, ImageViewPreviewState>>({});
const showMobileTabSelector = ref(false);
const mobileTabSelectorAnchor = shallowRef<MobileTabSelectorAnchor | null>(null);
const showQuickInputPopover = ref(false);
const showImportDialog = ref(false);
const contextMenuSession = ref<SessionTab | null>(null);
const contextMenuX = ref(0);
const contextMenuY = ref(0);
const activeTabIndicatorStyle = ref(hiddenCardTabIndicatorStyle());
const tabsContainerWidth = ref(0);
const tabTitleMaxWidth = ref(MAX_TAB_TITLE_WIDTH);
const isComposerDragOver = ref(false);
const isMobileComposerExpanded = ref(false);
const isMobileComposerFocused = ref(false);
const showAttachmentPreview = ref(false);
const activeAttachmentPreview = ref<{
  id: string;
  name: string;
  url: string;
} | null>(null);
const codexRuntimeConfig = ref<WebSessionCodexRuntimeConfig | null>(null);
const codexRuntimeConfigReady = ref(false);
const showCommandExecutionDetail = ref(false);
const loadingCommandExecutionDetail = ref(false);
const activeCommandExecutionDetail = ref<CommandExecutionDetail | null>(null);
const activeCommandExecutionGroupId = ref('');
const dismissedPlanActions = ref<Record<string, boolean>>({});
const rawTimelineBlocks = ref<Record<string, boolean>>({});
const activeRawTimelineBlockKey = ref('');
const streamingMarkdownTextByKey = ref<Record<string, string>>({});
const userInputSelections = ref<Record<string, string[]>>({});
const userInputDrafts = ref<Record<string, string>>({});
const submitStateBySessionId = ref<WebSessionSubmitState>({});
const userInputSubmitStateByOwnerId = ref<WebSessionSubmitState>({});
const userInputSlowStateByOwnerId = ref<WebSessionSubmitState>({});
const archiveStateBySessionId = ref<WebSessionSubmitState>({});
const importingCodexSessionId = ref('');
const sendConfirmationState = ref<WebSessionSendConfirmationState | null>(null);
const liveCardContinuePending = ref(false);
const optimisticUnreadClearedVersionBySession = ref<Record<string, number>>({});
const webSessionCatchUpActive = ref(false);
const isProjectSessionInitializing = ref(false);
const pendingRouteActivationSessionId = ref('');
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
let sendConfirmationTimer: number | null = null;
const loadedSidebarProjectIds = new Set<string>();
const streamingMarkdownController = createWebSessionStreamingMarkdownController({
  delayMs: webSessionStreamingMarkdownThrottleMs.value,
  onStateChange: state => {
    streamingMarkdownTextByKey.value = state;
  },
});
const sidebarContainerWidth = ref(0);
const isSidebarResizing = ref(false);
const sidebarWidthPx = useStorage<number>(
  'workspace-web-session-sidebar-width',
  DEFAULT_SESSION_SIDEBAR_WIDTH
);
const persistedSidebarScope = useStorage<string>(SIDEBAR_SCOPE_STORAGE_KEY, 'all');
let sidebarResizeObserver: ResizeObserver | null = null;
const composerTransferErrorMessage = ref('');
const composerTransferErrorDetail = ref('');
let composerTransferErrorTimer: number | null = null;
let cancelUserInputSlowHint: (() => void) | null = null;
let activeUserInputSlowHintOwnerId = '';
let mobileQuickInputOpenedAt = 0;
let mobileTabSelectorOpenedAt = 0;
const realSessionSnapshotLoadController = createWebSessionSnapshotLoadController();

const IMAGE_ATTACHMENT_NAME_PATTERN = /\.(png|jpe?g|gif|webp|bmp|svg|tiff?)$/i;

const draftAgent = ref<'claude' | 'codex'>('codex');
const draftModel = ref('gpt-5.4');
const draftReasoningEffort = ref<'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh'>('xhigh');
const draftWorkflowMode = ref<'default' | 'plan'>('default');
const draftPermissionLevel = ref<'default' | 'elevated' | 'yolo'>('elevated');
const draftSessions = ref<DraftSessionTab[]>([]);
const activeDraftSessionId = ref('');
const activeArchivedPreviewId = ref('');
const archivedPreviewSession = ref<ArchivedPreviewSessionTab | null>(null);
const tabOrderIds = ref<string[]>([]);
const tabMruIds = ref<string[]>([]);

const realSessions = computed<SessionTab[]>(() =>
  webSessionStore.getSessions(props.projectId).map(session => ({
    ...session,
    isDraft: false as const,
  }))
);
const nonArchivedVisibleSessions = computed<SessionTab[]>(() => [
  ...realSessions.value,
  ...draftSessions.value,
]);
const allVisibleSessions = computed<SessionTab[]>(() => [
  ...sessions.value,
  ...(archivedPreviewSession.value ? [archivedPreviewSession.value] : []),
]);
const visibleSessionById = computed(() => {
  const map = new Map<string, SessionTab>();
  allVisibleSessions.value.forEach(session => {
    map.set(session.id, session);
  });
  return map;
});
const sessions = computed<SessionTab[]>(() =>
  buildOrderedTabSessions(tabOrderIds.value, nonArchivedVisibleSessions.value)
);
const currentSession = computed<SessionTab | null>(() => {
  if (
    activeArchivedPreviewId.value &&
    archivedPreviewSession.value?.id === activeArchivedPreviewId.value
  ) {
    return archivedPreviewSession.value;
  }
  if (activeDraftSessionId.value) {
    return draftSessions.value.find(session => session.id === activeDraftSessionId.value) ?? null;
  }
  const activeRealId = webSessionStore.getActiveSessionId(props.projectId);
  return realSessions.value.find(session => session.id === activeRealId) ?? null;
});
const currentRealSession = computed<WebSessionSummary | null>(() => {
  const session = currentSession.value;
  return session && !isDraftSession(session) ? session : null;
});
const sendGuardProjectId = computed(() => currentRealSession.value?.projectId || props.projectId);
const currentDraftSessionId = computed(() => currentSession.value?.id ?? '');
const currentSessionAutoRetryEnabled = computed(() =>
  Boolean(currentSession.value?.autoRetryEnabled)
);
const webSessionAutoContinueEnabledValue = computed({
  get: () => currentSessionAutoRetryEnabled.value,
  set: value => {
    const next = value === true;
    const session = currentSession.value;
    if (!session) {
      return;
    }
    if (isDraftSession(session)) {
      updateActiveDraftSession(current => ({
        ...current,
        autoRetryEnabled: next,
        autoRetryScope: webSessionAutoContinueScope.value,
        autoRetryPreset: webSessionAutoContinuePreset.value,
        updatedAt: new Date().toISOString(),
      }));
      return;
    }
    if (currentRealSession.value) {
      void webSessionStore
        .updateAutoRetry(currentRealSession.value.id, {
          enabled: next,
          scope: webSessionAutoContinueScope.value,
          preset: webSessionAutoContinuePreset.value,
        })
        .catch(error => {
          message.error(error instanceof Error ? error.message : t('common.error'));
        });
    }
  },
});
const composerText = computed({
  get: () => webSessionStore.getDraft(props.projectId, currentDraftSessionId.value).text,
  set: value => {
    const sessionId = currentDraftSessionId.value;
    if (!sessionId) {
      return;
    }
    webSessionStore.setDraftText(props.projectId, sessionId, value);
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
  const session = currentRealSession.value;
  const sessionId = session?.id;
  if (!sessionId) {
    stopWebSessionCatchUp(`${reason}-no-session`);
    return;
  }

  beginWebSessionCatchUp(reason);
  const token = ++webSessionCatchUpToken;

  try {
    if (session?.projectId && !session.archivedAt) {
      await webSessionStore.loadSessions(session.projectId, true);
    }
    const snapshot = await webSessionStore.loadSessionSnapshot(session.projectId, sessionId, {
      rememberActive: !isArchivedPreviewSession(currentSession.value),
      preserveArchivedPosition: isArchivedPreviewSession(currentSession.value),
    });
    if (isArchivedPreviewSession(currentSession.value) && snapshot?.session) {
      archivedPreviewSession.value = {
        ...snapshot.session,
        isArchivedPreview: true,
      };
    }
    syncArchivedPreviewSessionSummary(sessionId);
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

function shouldShowToolPendingPlaceholder(tool: NonNullable<WebSessionBlock['tool']>) {
  if (tool.status !== 'running') {
    return false;
  }
  if (isCompactTool(tool)) {
    return !getCompactToolSummary(tool).trim();
  }
  const hasOutput = typeof tool.output === 'string' && tool.output.trim().length > 0;
  if (hasOutput) {
    return false;
  }
  if (tool.input == null) {
    return true;
  }
  return stringifyValue(tool.input).trim().length === 0;
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
function shouldShowMessageRawToggle(block: WebSessionBlock) {
  if (block.kind !== 'user' && block.kind !== 'assistant') {
    return false;
  }
  return Boolean(block.text?.trim());
}
function getDisplayBlockText(block: WebSessionBlock) {
  if (!block.text) {
    return '';
  }
  return stripImagePlaceholdersFromText(block.text, block.attachments.length);
}
function shouldShowPlanRawToggle(block: WebSessionBlock) {
  return Boolean(
    block.kind === 'tool' && block.tool && isPlanTool(block.tool) && block.tool.output?.trim()
  );
}
type StreamingMarkdownSurface = 'message' | 'plan';

function buildStreamingMarkdownKey(block: WebSessionBlock, surface: StreamingMarkdownSurface) {
  return `${block.key}:${surface}`;
}

function isStreamingMessageMarkdownBlock(block: WebSessionBlock) {
  return block.kind === 'assistant' && liveState.value.running && block.done !== true;
}

function isStreamingPlanMarkdownBlock(block: WebSessionBlock) {
  return Boolean(
    block.kind === 'tool' && block.tool && isPlanTool(block.tool) && block.tool.status === 'running'
  );
}

function getStreamingMarkdownText(block: WebSessionBlock, surface: StreamingMarkdownSurface) {
  if (surface === 'plan') {
    return block.tool?.output ?? '';
  }
  return getDisplayBlockText(block);
}

function getEffectiveStreamingMarkdownText(
  block: WebSessionBlock,
  surface: StreamingMarkdownSurface
) {
  const fallback = getStreamingMarkdownText(block, surface);
  return streamingMarkdownTextByKey.value[buildStreamingMarkdownKey(block, surface)] ?? fallback;
}

function getMessageMarkdownText(block: WebSessionBlock) {
  if (!isStreamingMessageMarkdownBlock(block)) {
    return getDisplayBlockText(block);
  }
  return getEffectiveStreamingMarkdownText(block, 'message');
}

function getMessageMarkdownRenderOptions(block: WebSessionBlock) {
  return isStreamingMessageMarkdownBlock(block) ? STREAMING_MARKDOWN_RENDER_OPTIONS : undefined;
}

function getMessageMarkdownMemoDeps(block: WebSessionBlock) {
  const rawMode = isBlockRawMode(block, 'message');
  return rawMode
    ? ['message-raw', block.text]
    : [
        'message-markdown',
        getMessageMarkdownText(block),
        isStreamingMessageMarkdownBlock(block) ? 1 : 0,
      ];
}

function getPlanToolMarkdownText(block: WebSessionBlock) {
  if (!isStreamingPlanMarkdownBlock(block)) {
    return block.tool?.output ?? '';
  }
  return getEffectiveStreamingMarkdownText(block, 'plan');
}

function getPlanToolMarkdownRenderOptions(block: WebSessionBlock) {
  return isStreamingPlanMarkdownBlock(block) ? STREAMING_MARKDOWN_RENDER_OPTIONS : undefined;
}

function getPlanToolMarkdownMemoDeps(block: WebSessionBlock) {
  const rawMode = isBlockRawMode(block, 'plan');
  return rawMode
    ? ['plan-raw', block.tool?.output ?? '']
    : [
        'plan-markdown',
        getPlanToolMarkdownText(block),
        isStreamingPlanMarkdownBlock(block) ? 1 : 0,
      ];
}

function getTimelineRawModeKey(block: WebSessionBlock, surface: TimelineRawSurface) {
  return buildTimelineRawModeKey({
    sessionId: currentSession.value?.id,
    surface,
    blockKey: block.key,
  });
}
function isTimelineRawBlockActive(block: WebSessionBlock, surface: TimelineRawSurface) {
  return activeRawTimelineBlockKey.value === getTimelineRawModeKey(block, surface);
}
function activateTimelineRawBlock(block: WebSessionBlock, surface: TimelineRawSurface) {
  const rawCapable =
    surface === 'message' ? shouldShowMessageRawToggle(block) : shouldShowPlanRawToggle(block);
  activeRawTimelineBlockKey.value = resolveActivatedTimelineRawBlockKey(
    rawCapable,
    getTimelineRawModeKey(block, surface)
  );
}
function deactivateTimelineRawBlock(block: WebSessionBlock, surface: TimelineRawSurface) {
  const key = getTimelineRawModeKey(block, surface);
  if (activeRawTimelineBlockKey.value === key) {
    activeRawTimelineBlockKey.value = '';
  }
}
function handleMessageBubbleMouseEnter(block: WebSessionBlock) {
  if (isMobile.value || !shouldShowMessageRawToggle(block)) {
    return;
  }
  activateTimelineRawBlock(block, 'message');
}
function handleMessageBubbleMouseLeave(block: WebSessionBlock) {
  if (isMobile.value || !shouldShowMessageRawToggle(block)) {
    return;
  }
  deactivateTimelineRawBlock(block, 'message');
}
function handleMessageBubbleClick(block: WebSessionBlock) {
  if (!isMobile.value || !shouldShowMessageRawToggle(block)) {
    return;
  }
  activateTimelineRawBlock(block, 'message');
}
function handleMessageBubbleFocusOut(block: WebSessionBlock, event: FocusEvent) {
  if (!shouldShowMessageRawToggle(block)) {
    return;
  }
  const currentTarget = event.currentTarget;
  const relatedTarget = event.relatedTarget;
  if (
    currentTarget instanceof Element &&
    relatedTarget instanceof Node &&
    currentTarget.contains(relatedTarget)
  ) {
    return;
  }
  deactivateTimelineRawBlock(block, 'message');
}
function shouldShowTimelineRawToggle(block: WebSessionBlock, surface: TimelineRawSurface) {
  const rawCapable =
    surface === 'message' ? shouldShowMessageRawToggle(block) : shouldShowPlanRawToggle(block);
  return shouldShowTimelineRawToggleForBlock({
    activeKey: activeRawTimelineBlockKey.value,
    rawKey: getTimelineRawModeKey(block, surface),
    rawCapable,
    rawMode: isBlockRawMode(block, surface),
  });
}
function isBlockRawMode(block: WebSessionBlock, surface: TimelineRawSurface) {
  return !!rawTimelineBlocks.value[getTimelineRawModeKey(block, surface)];
}
function toggleBlockRawMode(block: WebSessionBlock, surface: TimelineRawSurface) {
  const key = getTimelineRawModeKey(block, surface);
  rawTimelineBlocks.value = toggleExclusiveTimelineRawBlock(rawTimelineBlocks.value, key);
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
    return hasReasoningContent(block) || shouldShowToolPendingPlaceholder(block.tool);
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
    return shouldShowToolPendingPlaceholder(block.tool);
  }
  return true;
}

const visibleBlocks = computed(() =>
  blocks.value.filter((block, index) => {
    if (
      !showWebSessionReasoning.value &&
      isReasoningBlock(block) &&
      currentSession.value?.agent !== 'codex'
    ) {
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
const visibleRawTimelineBlockKeys = computed(() => {
  const keys: string[] = [];
  visibleBlocks.value.forEach(block => {
    if (shouldShowMessageRawToggle(block)) {
      keys.push(getTimelineRawModeKey(block, 'message'));
    }
    if (shouldShowPlanRawToggle(block)) {
      keys.push(getTimelineRawModeKey(block, 'plan'));
    }
  });
  return keys;
});
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
const streamingMarkdownTargets = computed(() =>
  visibleBlocks.value.flatMap(block => {
    const targets: Array<{ key: string; text: string }> = [];
    if (isStreamingMessageMarkdownBlock(block)) {
      const text = getDisplayBlockText(block);
      if (text) {
        targets.push({
          key: buildStreamingMarkdownKey(block, 'message'),
          text,
        });
      }
    }
    if (isStreamingPlanMarkdownBlock(block)) {
      const text = block.tool?.output ?? '';
      if (text) {
        targets.push({
          key: buildStreamingMarkdownKey(block, 'plan'),
          text,
        });
      }
    }
    return targets;
  })
);
const pendingApproval = computed(() =>
  currentRealSession.value ? webSessionStore.getPendingApproval(currentRealSession.value.id) : null
);
const pendingUserInput = computed(() =>
  currentRealSession.value ? webSessionStore.getPendingUserInput(currentRealSession.value.id) : null
);
const pendingUserInputSyncKey = computed(() =>
  buildWebSessionUserInputDraftSyncKey(currentRealSession.value?.id, pendingUserInput.value)
);
const currentUserInputSubmitOwnerId = computed(() =>
  currentRealSession.value && pendingUserInput.value
    ? buildWebSessionUserInputSubmitOwnerId(
        currentRealSession.value.id,
        pendingUserInput.value.itemId
      )
    : ''
);
const isSubmittingUserInput = computed(() =>
  isWebSessionSubmitting(userInputSubmitStateByOwnerId.value, currentUserInputSubmitOwnerId.value)
);
const isUserInputSubmitSlow = computed(() =>
  isWebSessionSubmitting(userInputSlowStateByOwnerId.value, currentUserInputSubmitOwnerId.value)
);
const isUserInputInteractionDisabled = computed(
  () => Boolean(pendingUserInput.value?.stale) || isSubmittingUserInput.value
);
const showUserInputSubmitSlowHint = computed(
  () => isSubmittingUserInput.value && isUserInputSubmitSlow.value
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
    liveState.value.phase === 'waiting_plan_approval' &&
    !pendingApproval.value &&
    Boolean(latestPlanToolId.value) &&
    !hasUserMessageAfterLatestPlan.value
);
const currentSubmitEntry = computed(() =>
  getWebSessionSubmitEntry(submitStateBySessionId.value, currentDraftSessionId.value)
);
const currentSubmitShowsExecuteFeedback = computed(() =>
  shouldShowWebSessionExecuteFeedback(currentSubmitEntry.value)
);
const isOptimisticExecuteFeedbackActive = computed(
  () => currentSubmitShowsExecuteFeedback.value && !liveState.value.running
);
const displayLiveState = computed(() =>
  resolveOptimisticWebSessionLiveState(liveState.value, currentSubmitEntry.value)
);
const isSubmittingPlanExecution = computed(() => currentSubmitEntry.value?.kind === 'execute_plan');
const showRuntimeStrip = computed(() => {
  if (pendingApproval.value || pendingUserInput.value) {
    return true;
  }
  if (isPlanWaitingApprovalState.value && !isOptimisticExecuteFeedbackActive.value) {
    return false;
  }
  if (displayLiveState.value.phase === 'idle') {
    return false;
  }
  if (
    displayLiveState.value.phase === 'done' &&
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
const draftAttachments = computed(() =>
  webSessionStore.getDraftAttachments(props.projectId, currentDraftSessionId.value)
);
const sendConflictSessions = computed(() => {
  const projectId = sendGuardProjectId.value;
  if (!projectId) {
    return [];
  }
  return findWebSessionSendConflicts({
    currentSessionId: currentRealSession.value?.id ?? '',
    sessions: webSessionStore.getSessions(projectId).map(session => ({
      id: session.id,
      title: session.title,
      workflowMode: session.workflowMode,
      livePhase: webSessionStore.getLiveState(session.id).phase,
    })),
  });
});
const draftAttachmentUpload = computed(() =>
  webSessionStore.getDraftAttachmentUpload(props.projectId, currentDraftSessionId.value)
);
const isDraftAttachmentUploading = computed(() => Boolean(draftAttachmentUpload.value));
const activeTerminalTheme = computed(() => {
  return getTerminalThemeById(effectiveTerminalThemeId.value) || getDefaultTerminalTheme();
});
const composerTransferCard = computed(() => {
  if (draftAttachmentUpload.value) {
    const upload = draftAttachmentUpload.value;
    return {
      tone: 'progress' as const,
      message:
        upload.totalFiles > 1
          ? t('webSession.attachmentUploadingBatch', {
              current: upload.currentFileIndex,
              total: upload.totalFiles,
            })
          : t('webSession.attachmentUploading'),
      detail: '',
      progress: upload.percent ?? 0,
    };
  }
  if (composerTransferErrorMessage.value) {
    return {
      tone: 'error' as const,
      message: composerTransferErrorMessage.value,
      detail: '',
      progress: null,
    };
  }
  return null;
});
const composerTransferDialogStyle = computed(() => {
  const theme = activeTerminalTheme.value.theme;
  const background = theme.background || '#0f111a';
  const foreground = theme.foreground || '#f6f8ff';

  return {
    '--terminal-transfer-card-bg': hexToRgba(background, 0.94),
    '--terminal-transfer-card-fg': foreground,
    '--terminal-transfer-card-border': hexToRgba(foreground, 0.18),
    '--terminal-transfer-card-track': hexToRgba(foreground, 0.14),
  } as CSSProperties;
});
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
const currentComposerSubmitKind = computed(() => resolveComposerSubmitKind());
const sendConfirmationSignature = computed(() =>
  buildWebSessionSendConfirmationSignature({
    ownerId: currentDraftSessionId.value,
    text: composerText.value,
    attachmentIds: draftAttachments.value.map(item => item.id),
    conflictSessionIds: sendConflictSessions.value.map(session => session.id),
  })
);
const planImplementConfirmationSignature = computed(() =>
  buildWebSessionSendConfirmationSignature({
    ownerId: currentRealSession.value?.id ?? '',
    text: '__implement_plan__',
    attachmentIds: [],
    conflictSessionIds: sendConflictSessions.value.map(session => session.id),
  })
);
const isSendConflictConfirmationArmed = computed(
  () =>
    currentComposerSubmitKind.value === 'execute_send' &&
    sendConflictSessions.value.length > 0 &&
    Boolean(
      sendConfirmationState.value &&
      sendConfirmationState.value.signature === sendConfirmationSignature.value
    )
);
const isSubmittingMessage = computed(() =>
  isWebSessionSubmitting(submitStateBySessionId.value, currentDraftSessionId.value)
);
const isRunActive = computed(() => liveState.value.running);
const hasDraftContent = computed(
  () => composerText.value.trim().length > 0 || draftAttachments.value.length > 0
);
const canSend = computed(
  () =>
    !isRunActive.value &&
    !isSubmittingMessage.value &&
    hasDraftContent.value &&
    !isDraftAttachmentUploading.value
);
const canStageDuringRun = computed(
  () => isRunActive.value && hasDraftContent.value && !isDraftAttachmentUploading.value
);
const composerAutosize = computed(() =>
  isMobile.value ? { minRows: 1, maxRows: 5 } : { minRows: 2, maxRows: 7 }
);
const composerPlaceholder = computed(() =>
  isMobile.value
    ? locale.value === 'zh-CN'
      ? '输入消息'
      : 'Type a message'
    : t('webSession.inputPlaceholder')
);
const composerHint = computed(() => {
  if (isDraftAttachmentUploading.value) {
    return t('webSession.composerHintUploading');
  }
  if (hasRecoveredRuntimeRequest.value) {
    return t('webSession.composerHintRecovered');
  }
  if (
    pendingApproval.value ||
    liveState.value.phase === 'waiting_approval' ||
    liveState.value.phase === 'waiting_plan_approval'
  ) {
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
const quickInputPinnedItems = computed(() => webSessionQuickInput.value.pinned);
const quickInputRecentItems = computed(() => {
  const pinned = new Set(quickInputPinnedItems.value);
  return webSessionQuickInput.value.recent.filter(text => !pinned.has(text));
});
const hasQuickInputOptions = computed(
  () => quickInputPinnedItems.value.length > 0 || quickInputRecentItems.value.length > 0
);
const quickInputButtonTitle = computed(() =>
  hasQuickInputOptions.value ? t('webSession.quickInput') : t('webSession.quickInputUnavailable')
);
const quickInputDirectSendEnabled = computed({
  get: () => webSessionQuickInputDirectSend.value,
  set: value => {
    settingsStore.updateWebSessionQuickInputDirectSend(value === true);
  },
});
const quickInputItems = computed(() => [
  ...quickInputPinnedItems.value,
  ...quickInputRecentItems.value,
]);
const normalizedComposerText = computed(() => composerText.value.trim());
const selectedAgentLabel = computed(
  () =>
    agentOptions.find(option => option.value === selectedAgent.value)?.label ?? selectedAgent.value
);
const selectedModelLabel = computed(
  () => String(selectedModel.value || '').trim() || t('common.default')
);
const selectedReasoningEffortLabel = computed(
  () =>
    reasoningEffortOptions.value.find(option => option.value === selectedReasoningEffort.value)
      ?.label ?? selectedReasoningEffort.value
);
const selectedWorkflowModeLabel = computed(() =>
  selectedWorkflowMode.value === 'plan'
    ? t('webSession.workflowPlan')
    : t('webSession.workflowDefault')
);
const selectedPermissionLevelLabel = computed(() => {
  switch (selectedPermissionLevel.value) {
    case 'elevated':
      return t('webSession.permissionElevated');
    case 'yolo':
      return t('webSession.permissionYolo');
    default:
      return t('webSession.permissionDefault');
  }
});
const mobileComposerSummaryTokens = computed(() => {
  const tokens = [
    { key: 'agent', label: selectedAgentLabel.value },
    { key: 'model', label: selectedModelLabel.value },
  ];
  if (selectedAgent.value === 'codex') {
    tokens.push({ key: 'reasoning', label: selectedReasoningEffortLabel.value });
  }
  tokens.push(
    { key: 'workflow', label: selectedWorkflowModeLabel.value },
    { key: 'permission', label: selectedPermissionLevelLabel.value }
  );
  if (currentSessionAutoRetryEnabled.value) {
    tokens.push({ key: 'auto-continue', label: t('webSession.infiniteRetry') });
  }
  return tokens;
});
const tokenNumberFormatter = new Intl.NumberFormat();
const contextUsageDisclaimer =
  '这个数据是我从codex那边读的然后原样显示，数据可能并不准确，但我也不知道为什么他这样给显示，有明白的大佬麻烦告知';
const contextUsageIndicator = computed(() => {
  const session = currentSession.value;
  if (!session) {
    return null;
  }

  if (session.agent === 'codex' && isDraftSession(session) && !codexRuntimeConfigReady.value) {
    return null;
  }

  const runtimeConfig = codexRuntimeConfig.value;
  const sessionSource =
    session.contextWindowSource === 'config' ||
    session.contextWindowSource === 'default' ||
    session.contextWindowSource === 'unavailable'
      ? session.contextWindowSource
      : session.agent === 'codex'
        ? ('default' as WebSessionContextWindowSource)
        : ('unavailable' as WebSessionContextWindowSource);
  const source = runtimeConfig?.source ?? sessionSource;

  const contextWindowTokens =
    typeof runtimeConfig?.contextWindowTokens === 'number' &&
    Number.isFinite(runtimeConfig.contextWindowTokens)
      ? Math.max(0, runtimeConfig.contextWindowTokens)
      : typeof session.contextWindowTokens === 'number' &&
          Number.isFinite(session.contextWindowTokens)
        ? Math.max(0, session.contextWindowTokens)
        : session.agent === 'codex' && isDraftSession(session)
          ? DEFAULT_CODEX_CONTEXT_WINDOW_TOKENS
          : null;
  const compactLimitTokens =
    typeof runtimeConfig?.compactLimitTokens === 'number' &&
    Number.isFinite(runtimeConfig.compactLimitTokens)
      ? Math.max(0, runtimeConfig.compactLimitTokens)
      : contextWindowTokens;

  if (session.agent !== 'codex' || !contextWindowTokens || !compactLimitTokens) {
    return {
      state: 'unavailable',
      label: t('webSession.contextUsageLabelUnavailable'),
      title: t('webSession.contextUsageUnavailableTitle'),
      lines: [t('webSession.contextUsageUnavailableDescription')],
    };
  }

  const estimateInputTokens = Number(session.contextEstimate.inputTokens || 0);
  const estimateCachedInputTokens = Number(session.contextEstimate.cachedInputTokens || 0);
  const estimateOutputTokens = Number(session.contextEstimate.outputTokens || 0);
  const usedTokens = Math.max(0, Number(session.contextEstimate.usedTokens || 0));
  const totalInputTokens = Number(session.usage.inputTokens || 0);
  const totalCachedInputTokens = Number(session.usage.cachedInputTokens || 0);
  const totalOutputTokens = Number(session.usage.outputTokens || 0);
  const totalUsedTokens = Math.max(0, totalInputTokens + totalOutputTokens);
  const remainingEstimateTokens = Math.max(0, compactLimitTokens - usedTokens);
  const remainingPercent =
    compactLimitTokens > 0 ? Math.round((remainingEstimateTokens / compactLimitTokens) * 100) : 0;
  const sourceLabel =
    source === 'config'
      ? t('webSession.contextUsageSourceConfig')
      : t('webSession.contextUsageSourceDefault');
  const estimateMode =
    session.contextEstimateMode === 'latest_turn_delta'
      ? 'latest_turn_delta'
      : session.contextEstimateMode === 'since_compaction'
        ? 'since_compaction'
        : 'cumulative_total';
  const estimateModeLabel =
    estimateMode === 'latest_turn_delta'
      ? t('webSession.contextUsageModeLatestTurnDelta')
      : estimateMode === 'since_compaction'
        ? t('webSession.contextUsageModeSinceCompaction')
        : t('webSession.contextUsageModeCumulativeTotal');
  const estimateNote =
    estimateMode === 'latest_turn_delta'
      ? t('webSession.contextUsageNoteLatestTurnDelta')
      : estimateMode === 'since_compaction'
        ? t('webSession.contextUsageNoteSinceCompaction')
        : t('webSession.contextUsageNoteCumulativeTotal');

  return {
    state: remainingPercent <= 10 ? 'warning' : remainingPercent <= 25 ? 'active' : 'idle',
    label: t('webSession.contextUsageLabel', {
      percent: remainingPercent,
    }),
    title: t('webSession.contextUsageTitle'),
    lines: [
      contextUsageDisclaimer,
      t('webSession.contextUsageRemainingEstimate', {
        count: tokenNumberFormatter.format(remainingEstimateTokens),
      }),
      t('webSession.contextUsageEstimatedUsed', {
        count: tokenNumberFormatter.format(usedTokens),
      }),
      t('webSession.contextUsageWindow', {
        count: tokenNumberFormatter.format(contextWindowTokens),
      }),
      t('webSession.contextUsageCompactLimit', {
        count: tokenNumberFormatter.format(compactLimitTokens),
      }),
      t('webSession.contextUsageSource', {
        source: sourceLabel,
      }),
      t('webSession.contextUsageMode', {
        mode: estimateModeLabel,
      }),
      t('webSession.contextUsageEstimatedBreakdown', {
        input: tokenNumberFormatter.format(estimateInputTokens),
        cached: tokenNumberFormatter.format(estimateCachedInputTokens),
        output: tokenNumberFormatter.format(estimateOutputTokens),
      }),
      t('webSession.contextUsageTotalUsed', {
        count: tokenNumberFormatter.format(totalUsedTokens),
      }),
      t('webSession.contextUsageTotalBreakdown', {
        input: tokenNumberFormatter.format(totalInputTokens),
        cached: tokenNumberFormatter.format(totalCachedInputTokens),
        output: tokenNumberFormatter.format(totalOutputTokens),
      }),
      estimateNote,
    ],
  };
});

function emitMobileComposerFocusChange(focused: boolean) {
  if (isMobileComposerFocused.value === focused) {
    return;
  }
  isMobileComposerFocused.value = focused;
  emit('mobile-composer-focus-change', focused);
}

function toggleMobileComposerExpanded() {
  if (!isMobile.value) {
    return;
  }
  isMobileComposerExpanded.value = !isMobileComposerExpanded.value;
}

function handleMobileQuickInputClickOutside() {
  if (Date.now() - mobileQuickInputOpenedAt < 180) {
    return;
  }
  showQuickInputPopover.value = false;
}

function handleMobileQuickInputTrigger() {
  if (!isMobile.value) {
    return;
  }
  const nextShow = !showQuickInputPopover.value;
  showQuickInputPopover.value = nextShow;
  if (nextShow) {
    mobileQuickInputOpenedAt = Date.now();
  }
}

function handleMobileAttachmentTrigger() {
  if (!isMobile.value) {
    return;
  }
  openFilePicker();
}

function clearComposerTransferError() {
  if (composerTransferErrorTimer != null) {
    window.clearTimeout(composerTransferErrorTimer);
    composerTransferErrorTimer = null;
  }
  composerTransferErrorMessage.value = '';
  composerTransferErrorDetail.value = '';
}

function beginSessionSubmit(ownerId: string, kind: WebSessionSubmitKind) {
  submitStateBySessionId.value = beginWebSessionSubmit(submitStateBySessionId.value, ownerId, {
    kind,
  });
}

function endSessionSubmit(ownerId: string) {
  submitStateBySessionId.value = endWebSessionSubmit(submitStateBySessionId.value, ownerId);
}

function transferSessionSubmit(fromOwnerId: string, toOwnerId: string) {
  submitStateBySessionId.value = transferWebSessionSubmit(
    submitStateBySessionId.value,
    fromOwnerId,
    toOwnerId
  );
}

function clearUserInputSlowHintTimer(ownerId = '') {
  const normalizedOwnerId = buildWebSessionSubmitOwnerId(ownerId);
  if (
    normalizedOwnerId &&
    activeUserInputSlowHintOwnerId &&
    activeUserInputSlowHintOwnerId !== normalizedOwnerId
  ) {
    return;
  }
  if (cancelUserInputSlowHint) {
    cancelUserInputSlowHint();
    cancelUserInputSlowHint = null;
  }
  activeUserInputSlowHintOwnerId = '';
}

function beginUserInputSubmit(ownerId: string) {
  const normalizedOwnerId = buildWebSessionSubmitOwnerId(ownerId);
  if (!normalizedOwnerId) {
    return;
  }
  userInputSubmitStateByOwnerId.value = beginWebSessionSubmit(
    userInputSubmitStateByOwnerId.value,
    normalizedOwnerId
  );
  userInputSlowStateByOwnerId.value = endWebSessionSubmit(
    userInputSlowStateByOwnerId.value,
    normalizedOwnerId
  );
  clearUserInputSlowHintTimer();
  activeUserInputSlowHintOwnerId = normalizedOwnerId;
  cancelUserInputSlowHint = scheduleWebSessionUserInputSlowHint(normalizedOwnerId, slowOwnerId => {
    cancelUserInputSlowHint = null;
    activeUserInputSlowHintOwnerId = '';
    userInputSlowStateByOwnerId.value = beginWebSessionSubmit(
      userInputSlowStateByOwnerId.value,
      slowOwnerId
    );
  });
}

function endUserInputSubmit(ownerId: string) {
  const normalizedOwnerId = buildWebSessionSubmitOwnerId(ownerId);
  if (!normalizedOwnerId) {
    return;
  }
  userInputSubmitStateByOwnerId.value = endWebSessionSubmit(
    userInputSubmitStateByOwnerId.value,
    normalizedOwnerId
  );
  clearUserInputSlowHintTimer(normalizedOwnerId);
  userInputSlowStateByOwnerId.value = endWebSessionSubmit(
    userInputSlowStateByOwnerId.value,
    normalizedOwnerId
  );
}

function clearSendConflictConfirmationTimer() {
  if (sendConfirmationTimer != null) {
    window.clearTimeout(sendConfirmationTimer);
    sendConfirmationTimer = null;
  }
}

function setSendConflictConfirmationState(nextState: WebSessionSendConfirmationState | null) {
  clearSendConflictConfirmationTimer();
  sendConfirmationState.value = nextState;
  if (!nextState) {
    return;
  }
  const delay = Math.max(0, nextState.expiresAt - Date.now());
  sendConfirmationTimer = window.setTimeout(() => {
    sendConfirmationTimer = null;
    if (sendConfirmationState.value?.signature === nextState.signature) {
      sendConfirmationState.value = null;
    }
  }, delay);
}

function clearSendConflictConfirmation() {
  setSendConflictConfirmationState(null);
}

function showComposerTransferError(detail?: string) {
  clearComposerTransferError();
  composerTransferErrorMessage.value = t('webSession.attachmentUploadFailed');
  composerTransferErrorDetail.value = String(detail || '').trim();
  composerTransferErrorTimer = window.setTimeout(() => {
    composerTransferErrorTimer = null;
    clearComposerTransferError();
  }, 900);
}

async function handleQuickInputApply(text: string) {
  showQuickInputPopover.value = false;
  const applied = await applyQuickInputText(text);
  if (!applied || !quickInputDirectSendEnabled.value) {
    return;
  }
  await triggerPrimaryComposerAction();
}

function isQuickInputSelected(text: string) {
  return normalizedComposerText.value.length > 0 && normalizedComposerText.value === text.trim();
}

function resolveComposerSubmitKind(): WebSessionSubmitKind {
  const workflowMode = currentSession.value?.workflowMode ?? draftWorkflowMode.value;
  return workflowMode === 'plan' ? 'plan_message' : 'execute_send';
}

const liveStateLabel = computed(() => {
  if (hasRecoveredRuntimeRequest.value) {
    return t('webSession.liveRecovered');
  }
  switch (displayLiveState.value.phase) {
    case 'starting':
      return t('webSession.liveStarting');
    case 'thinking':
      return t('webSession.liveThinking');
    case 'retrying':
      if (displayLiveState.value.retry?.attempt && displayLiveState.value.retry?.maxAttempts) {
        return t('webSession.liveRetryingProgress', {
          attempt: displayLiveState.value.retry.attempt,
          max: displayLiveState.value.retry.maxAttempts,
        });
      }
      return t('webSession.liveRetrying');
    case 'tool':
      if (isCompactToolKind(displayLiveState.value.tool?.kind)) {
        const count = Math.max(1, Number(displayLiveState.value.tool?.count ?? 1) || 1);
        const label = compactToolLabel(displayLiveState.value.tool);
        const toolLabel = count > 1 ? `${label} x${count}` : label;
        return t('webSession.liveTool', { tool: toolLabel });
      }
      return t('webSession.liveTool', { tool: displayLiveState.value.tool?.name || 'Tool' });
    case 'waiting_approval':
    case 'waiting_plan_approval':
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
  if (isOptimisticExecuteFeedbackActive.value) {
    return '';
  }
  if (pendingApproval.value?.prompt) {
    return pendingApproval.value.prompt;
  }
  if (
    displayLiveState.value.phase === 'waiting_approval' ||
    displayLiveState.value.phase === 'waiting_plan_approval'
  ) {
    return t('webSession.liveWaitingApprovalDetail');
  }
  if (pendingUserInput.value?.prompt) {
    return pendingUserInput.value.prompt;
  }
  if (displayLiveState.value.phase === 'retrying' && displayLiveState.value.retry?.message) {
    const message = displayLiveState.value.retry.message.trim();
    if (message && message !== liveStateLabel.value) {
      return message;
    }
  }
  if (displayLiveState.value.phase === 'tool' && displayLiveState.value.tool?.summary) {
    return displayLiveState.value.tool.summary;
  }
  if (displayLiveState.value.phase === 'tool' && displayLiveState.value.tool?.kind) {
    return displayLiveState.value.tool.kind;
  }
  if (displayLiveState.value.phase === 'error' && displayLiveState.value.errorMessage) {
    return displayLiveState.value.errorMessage;
  }
  return '';
});
const liveStateSecondaryText = computed(() => {
  if (liveStateDetail.value) {
    return liveStateDetail.value;
  }
  switch (displayLiveState.value.phase) {
    case 'starting':
      return t('webSession.liveStartingDetail');
    case 'thinking':
      return t('webSession.liveThinkingDetail');
    case 'retrying':
      return t('webSession.liveRetryingDetail');
    case 'tool':
      return compactToolLabel(displayLiveState.value.tool);
    case 'waiting_approval':
    case 'waiting_plan_approval':
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
  ['starting', 'thinking', 'retrying', 'tool'].includes(displayLiveState.value.phase)
);
const shouldAutoContinueOnLiveCardClick = computed(
  () =>
    liveState.value.phase === 'error' &&
    Boolean(currentRealSession.value) &&
    !liveCardContinuePending.value
);
const liveCardAriaLabel = computed(() =>
  shouldAutoContinueOnLiveCardClick.value ? 'continue' : t('webSession.jumpToBottom')
);
const underlyingTabSessionId = computed(() =>
  resolveUnderlyingTabSessionId({
    activeDraftSessionId: activeDraftSessionId.value,
    activeRealSessionId: webSessionStore.getActiveSessionId(props.projectId),
  })
);
const activeTabSessionId = computed(() =>
  resolveActiveTabSessionId({
    activeArchivedPreviewId: activeArchivedPreviewId.value,
    activeDraftSessionId: activeDraftSessionId.value,
    activeRealSessionId: webSessionStore.getActiveSessionId(props.projectId),
  })
);
const activeSessionId = computed(() => currentSession.value?.id ?? '');
const emptyStateTitle = computed(() => t('webSession.draftTitle'));
const emptyStateDescription = computed(() => t('webSession.draftDescription'));
const activeSessionTitle = computed(() => currentSession.value?.title ?? emptyStateTitle.value);
const activeSessionStatusLabel = computed(() =>
  currentSession.value ? getSessionStatusLabel(currentSession.value) : ''
);
const activeSessionAttentionStateClass = computed(() =>
  currentSession.value ? getSessionAttentionStateClass(currentSession.value) : 'unknown'
);
const activeSessionHasWorkflowPlanBadge = computed(() =>
  shouldShowSessionWorkflowPlanBadge(currentSession.value)
);
const sidebarScope = computed<WebSessionSidebarScope>({
  get: () => normalizeWebSessionSidebarScope(persistedSidebarScope.value),
  set: value => {
    persistedSidebarScope.value = normalizeWebSessionSidebarScope(value);
  },
});
const showCrossProjectSidebar = computed(() => !isMobile.value && props.showSidebar);
const sidebarScopeOptions = computed<DropdownOption[]>(() => [
  {
    key: 'all',
    label: t('webSession.sidebarScopeAll'),
  },
  {
    key: 'current',
    label: t('webSession.sidebarScopeCurrentProject'),
  },
]);
const sidebarScopeLabel = computed(() =>
  sidebarScope.value === 'current'
    ? t('webSession.sidebarScopeCurrentProject')
    : t('webSession.sidebarScopeAll')
);
const sidebarScopeAriaLabel = computed(() =>
  t('webSession.sidebarScopeAria', { scope: sidebarScopeLabel.value })
);
const sidebarScopeToggleLabel = computed(() =>
  resolveWebSessionSidebarToggleScope(sidebarScope.value) === 'current'
    ? t('webSession.sidebarScopeCurrentProject')
    : t('webSession.sidebarScopeAll')
);
const sidebarScopeToggleTitle = computed(() =>
  t('webSession.sidebarScopeToggle', {
    current: sidebarScopeLabel.value,
    next: sidebarScopeToggleLabel.value,
  })
);
const mobileSessionCategory = ref<MobileSessionCategory>('current');
const mobileCurrentSessions = computed<SessionTab[]>(() => {
  if (sidebarScope.value !== 'all') {
    return sortMobileCurrentSessions(sessions.value, session =>
      resolveWebSessionSidebarSortTimestamp(session)
    );
  }

  const draftSessions = sessions.value.filter(isDraftSession);
  const crossProjectCurrentSessions = crossProjectSessions.value.map(
    item => item.session as SessionTab
  );
  return sortMobileCurrentSessions([...draftSessions, ...crossProjectCurrentSessions], session =>
    resolveWebSessionSidebarSortTimestamp(session)
  );
});
const mobileArchivedProjectIds = computed(() => sidebarVisibleProjectIds.value);
const mobileArchivedScopeKey = computed(() => mobileArchivedProjectIds.value.join('|'));
const mobileArchivedMeta = computed(() => archivedSidebarMeta.value);
const mobileArchivedSessions = computed<SessionTab[]>(() =>
  crossProjectArchivedSessions.value.map(item => item.session as SessionTab)
);
const mobileVisibleSessions = computed<SessionTab[]>(() =>
  mobileSessionCategory.value === 'archived'
    ? mobileArchivedSessions.value
    : mobileCurrentSessions.value
);
const mobileProjectBadgeById = computed(() => {
  const ids = new Set(sidebarProjectIdsToLoad.value.filter(Boolean));
  const ordered: string[] = [];
  projectStore.projects.forEach(project => {
    if (project.id && ids.has(project.id) && !ordered.includes(project.id)) {
      ordered.push(project.id);
    }
  });
  projectStore.recentProjects.forEach(project => {
    if (project.id && ids.has(project.id) && !ordered.includes(project.id)) {
      ordered.push(project.id);
    }
  });
  sidebarProjectIdsToLoad.value.forEach(projectId => {
    if (projectId && !ordered.includes(projectId)) {
      ordered.push(projectId);
    }
  });
  return new Map(
    ordered.map((projectId, index) => [
      projectId,
      {
        index: index + 1,
        color: PROJECT_INDEX_COLORS[index % PROJECT_INDEX_COLORS.length],
      },
    ])
  );
});
const mobileNavigationSessions = computed<SessionTab[]>(() =>
  isArchivedPreviewSession(currentSession.value)
    ? mobileArchivedSessions.value
    : mobileCurrentSessions.value
);
const mobileTabDropdownPlacement = computed(() =>
  mobileTabSelectorAnchor.value?.source === 'bottom-nav' ? 'top' : 'bottom-start'
);
const mobileTabDropdownX = computed(() => mobileTabSelectorAnchor.value?.x ?? 0);
const mobileTabDropdownY = computed(() => mobileTabSelectorAnchor.value?.y ?? 0);
const currentSessionIndex = computed(() =>
  mobileNavigationSessions.value.findIndex(session => session.id === activeSessionId.value)
);
const hasPrevSession = computed(() => currentSessionIndex.value > 0);
const hasNextSession = computed(
  () =>
    currentSessionIndex.value >= 0 &&
    currentSessionIndex.value < mobileNavigationSessions.value.length - 1
);

watch(
  pendingUserInputSyncKey,
  syncKey => {
    const request = pendingUserInput.value;
    if (!syncKey || !request) {
      userInputSelections.value = {};
      userInputDrafts.value = {};
      return;
    }
    const nextState = reconcileWebSessionUserInputLocalState(request.questions, {
      selections: userInputSelections.value,
      drafts: userInputDrafts.value,
    });
    userInputSelections.value = nextState.selections;
    userInputDrafts.value = nextState.drafts;
  },
  { immediate: true }
);

watch(currentUserInputSubmitOwnerId, (nextOwnerId, previousOwnerId) => {
  if (previousOwnerId && previousOwnerId !== nextOwnerId) {
    endUserInputSubmit(previousOwnerId);
  }
});

const mobileTabOptions = computed<MobileTabDropdownOption[]>(() => {
  const options = buildWebSessionMobileTabDescriptors({
    section: mobileSessionCategory.value,
    sessions: mobileVisibleSessions.value,
    hasArchivedLoadMore: mobileArchivedMeta.value.hasMore,
    isArchivedLoading: mobileArchivedMeta.value.loading,
  }).map(descriptor => {
    switch (descriptor.kind) {
      case 'header':
        return {
          type: 'render' as const,
          key: descriptor.key,
          render: renderMobileTabCategoryHeader,
          props: {
            class: 'mobile-tab-category-header-render',
          },
        };
      case 'session': {
        const displayState = getSessionDisplayState(descriptor.session);
        return {
          kind: 'session' as const,
          label: descriptor.session.title,
          key: descriptor.key,
          section: descriptor.section,
          session: descriptor.session,
          displayState,
          tooltip: getSessionStatusTooltip(descriptor.session),
        };
      }
      case 'empty':
        return {
          type: 'render' as const,
          key: descriptor.key,
          render: renderMobileTabEmptyState,
          props: {
            class: 'mobile-tab-empty-render',
          },
        };
      case 'load-more':
        return {
          type: 'render' as const,
          key: descriptor.key,
          render: renderMobileTabLoadMore,
          props: {
            class: 'mobile-tab-load-more-render',
          },
        };
      case 'new-session':
        return {
          kind: 'new-session' as const,
          key: descriptor.key,
          label: t('webSession.newSession'),
          section: 'current' as const,
        };
    }
  }) as MobileTabDropdownOption[];

  options.push({
    type: 'render',
    key: `mobile-session-scope-toggle:${sidebarScope.value}:${mobileSessionCategory.value}`,
    render: renderMobileTabScopeToggle,
    props: {
      class: 'mobile-tab-scope-toggle-render',
    },
  });

  return options;
});

function mobileTabDropdownMenuProps() {
  const anchor = mobileTabSelectorAnchor.value;
  const isBottomNav = anchor?.source === 'bottom-nav';
  return {
    class: ['web-session-mobile-dropdown', isBottomNav && 'is-from-bottom-nav']
      .filter(Boolean)
      .join(' '),
    style: {
      width: isBottomNav
        ? 'min(320px, calc(100vw - 24px))'
        : `${Math.max(anchor?.width ?? 0, 220)}px`,
      maxWidth: 'calc(100vw - 24px)',
      '--mobile-tab-dropdown-origin': isBottomNav ? 'center bottom' : 'left top',
    } as CSSProperties,
  };
}

function getMobileTabOptionNodeProps(option: DropdownOption): HTMLAttributes {
  const mobileOption = option as MobileTabDropdownOption;
  const classes = ['web-session-mobile-option'];
  if (isMobileTabActionOption(mobileOption)) {
    classes.push('is-action', 'is-new-session');
    return {
      class: classes.join(' '),
      title: mobileOption.label,
    };
  }
  if (!isMobileTabSessionOption(mobileOption)) {
    return {
      class: classes.join(' '),
    };
  }
  if (mobileOption.key === activeSessionId.value) {
    classes.push('is-selected');
  }
  if (mobileOption.displayState.hasUnviewedApproval) {
    classes.push('is-approval');
  } else if (mobileOption.displayState.hasUnviewedCompletion) {
    classes.push('is-completion');
  }
  return {
    class: classes.join(' '),
    title: mobileOption.tooltip,
  };
}

function renderMobileTabCategoryHeader() {
  return h('div', { class: 'mobile-tab-category-header' }, [
    renderMobileTabCategoryButton('current'),
    renderMobileTabCategoryButton('archived'),
  ]);
}

function renderMobileTabCategoryButton(section: 'current' | 'archived') {
  const active = mobileSessionCategory.value === section;
  const count =
    section === 'current' ? mobileCurrentSessions.value.length : mobileArchivedMeta.value.total;
  const label =
    section === 'current' ? t('webSession.currentSessions') : t('webSession.archivedSessions');

  return h(
    'button',
    {
      type: 'button',
      class: ['mobile-tab-category-button', active && 'is-active'],
      onClick: (event: MouseEvent) => {
        event.preventDefault();
        event.stopPropagation();
        void setMobileSessionCategory(section);
      },
      onMousedown: (event: MouseEvent) => {
        event.preventDefault();
        event.stopPropagation();
      },
    },
    [
      h('span', { class: 'mobile-tab-category-button-label' }, label),
      h('span', { class: 'mobile-tab-category-button-count' }, String(count)),
    ]
  );
}

function renderMobileTabEmptyState() {
  return h(
    'div',
    { class: 'mobile-tab-empty-state' },
    mobileSessionCategory.value === 'archived'
      ? mobileArchivedMeta.value.loading
        ? t('common.loading')
        : t('webSession.archivedSessionsEmpty')
      : t('webSession.currentSessionsEmpty')
  );
}

function renderMobileTabLoadMore() {
  return h(
    'button',
    {
      type: 'button',
      class: ['mobile-tab-load-more', mobileArchivedMeta.value.loading && 'is-loading'],
      disabled: mobileArchivedMeta.value.loading,
      onClick: (event: MouseEvent) => {
        event.preventDefault();
        event.stopPropagation();
        void loadMoreMobileArchivedSessions();
      },
      onMousedown: (event: MouseEvent) => {
        event.preventDefault();
        event.stopPropagation();
      },
    },
    mobileArchivedMeta.value.loading ? t('common.loading') : t('webSession.loadMoreArchived')
  );
}

function renderMobileTabScopeToggle() {
  return h('div', { class: 'mobile-tab-scope-toggle-anchor', 'aria-hidden': 'true' }, [
    h(
      'button',
      {
        type: 'button',
        class: ['mobile-tab-scope-toggle-button', sidebarScope.value === 'current' && 'is-current'],
        title: sidebarScopeToggleTitle.value,
        'aria-label': sidebarScopeToggleTitle.value,
        'aria-pressed': sidebarScope.value === 'current',
        onClick: (event: MouseEvent) => {
          event.preventDefault();
          event.stopPropagation();
          toggleSidebarScope();
        },
        onMousedown: (event: MouseEvent) => {
          event.preventDefault();
          event.stopPropagation();
        },
      },
      [
        h('span', { class: 'mobile-tab-scope-toggle-icon', 'aria-hidden': 'true' }, [
          h(
            'svg',
            {
              width: '18',
              height: '18',
              viewBox: '0 0 24 24',
              fill: 'none',
            },
            [
              h('rect', {
                x: '4.5',
                y: '5',
                width: '9',
                height: '6',
                rx: '1.5',
                stroke: 'currentColor',
                'stroke-width': '1.8',
              }),
              h('rect', {
                x: '10.5',
                y: '13',
                width: '9',
                height: '6',
                rx: '1.5',
                stroke: 'currentColor',
                'stroke-width': '1.8',
                opacity: '0.92',
              }),
            ]
          ),
        ]),
        h('span', {
          class: 'mobile-tab-scope-toggle-indicator',
          'aria-hidden': 'true',
        }),
      ]
    ),
  ]);
}

function renderMobileTabOptionLabel(option: DropdownOption) {
  const mobileOption = option as MobileTabDropdownOption;
  if (isMobileTabActionOption(mobileOption)) {
    return h('div', { class: 'mobile-tab-action-option-body' }, [
      h('span', { class: 'mobile-tab-action-option-icon', 'aria-hidden': 'true' }, [h(AddOutline)]),
      h('span', { class: 'mobile-tab-action-option-title' }, mobileOption.label),
    ]);
  }
  if (!isMobileTabSessionOption(mobileOption)) {
    return '';
  }
  const { displayState } = mobileOption;
  const projectBadge = getMobileTabOptionProjectBadge(mobileOption.session);

  return h('div', { class: 'mobile-tab-option-body' }, [
    h('span', { class: 'mobile-tab-option-agent-shell', title: mobileOption.tooltip }, [
      h(
        'span',
        {
          class: [
            'mobile-tab-option-agent-badge',
            getMobileTabOptionAgentBadgeStateClass(mobileOption.session, displayState),
          ],
        },
        [
          h('span', {
            class: ['ai-status-icon', 'mobile-tab-option-agent-icon'],
            innerHTML: getSessionAssistantIcon(mobileOption.session),
          }),
        ]
      ),
      displayState.showStatusDot && displayState.statusDotClass
        ? h('span', {
            class: ['status-dot', 'mobile-tab-option-badge-dot', displayState.statusDotClass],
          })
        : null,
      shouldShowSessionWorkflowPlanBadge(mobileOption.session)
        ? h('span', {
            class: 'mobile-tab-option-plan-badge',
            'aria-hidden': 'true',
          })
        : null,
    ]),
    h(
      'span',
      { class: 'mobile-tab-option-title', title: mobileOption.tooltip },
      mobileOption.session.title
    ),
    projectBadge
      ? h(
          'span',
          {
            class: 'mobile-tab-option-project-badge',
            style: {
              '--badge-color': projectBadge.color,
            },
            title: getProjectName(mobileOption.session.projectId || props.projectId),
          },
          String(projectBadge.index)
        )
      : null,
  ]);
}

function getMobileTabOptionAgentBadgeStateClass(
  session: SessionTab,
  displayState: WebSessionDisplayState
) {
  if (!isDraftSession(session) && session.status === 'err') {
    return 'state-error';
  }
  return `state-${displayState.attentionStateClass}`;
}

function getMobileTabOptionProjectBadge(session: SessionTab) {
  const projectId = session.projectId || props.projectId;
  if (!projectId) {
    return null;
  }
  return mobileProjectBadgeById.value.get(projectId) ?? null;
}

async function setMobileSessionCategory(section: 'current' | 'archived') {
  mobileSessionCategory.value = section;
  if (section !== 'archived') {
    return;
  }
  if (
    mobileArchivedMeta.value.loading ||
    !mobileArchivedScopeKey.value ||
    webSessionStore.hasArchivedScope(mobileArchivedProjectIds.value)
  ) {
    return;
  }
  try {
    await ensureArchivedScopeLoaded(mobileArchivedProjectIds.value, 20);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function loadMoreMobileArchivedSessions() {
  if (
    !mobileArchivedScopeKey.value ||
    mobileArchivedMeta.value.loading ||
    !mobileArchivedMeta.value.hasMore
  ) {
    return;
  }
  try {
    await webSessionStore.loadArchivedSessions(mobileArchivedProjectIds.value, {
      limit: 20,
    });
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function syncMobileSessionCategoryToCurrentSession() {
  mobileSessionCategory.value = isArchivedPreviewSession(currentSession.value)
    ? 'archived'
    : 'current';
}

function buildMobileTabSelectorAnchor(
  anchorEl: HTMLElement,
  source: MobileTabSelectorSource
): MobileTabSelectorAnchor {
  const rect = anchorEl.getBoundingClientRect();
  if (source === 'bottom-nav') {
    return {
      source,
      x: Math.round(rect.left + rect.width / 2),
      y: Math.max(8, Math.round(rect.top) - 8),
      width: Math.round(rect.width),
    };
  }
  return {
    source,
    x: Math.round(rect.left),
    y: Math.round(rect.bottom) + 4,
    width: Math.round(rect.width),
  };
}

function closeMobileSessionSelector() {
  showMobileTabSelector.value = false;
}

function openMobileSessionSelectorFromElement(
  anchorEl: HTMLElement,
  source: MobileTabSelectorSource
) {
  if (!isMobile.value) {
    return;
  }
  syncMobileSessionCategoryToCurrentSession();
  if (mobileSessionCategory.value === 'archived') {
    void setMobileSessionCategory('archived');
  }
  mobileTabSelectorAnchor.value = buildMobileTabSelectorAnchor(anchorEl, source);
  mobileTabSelectorOpenedAt = Date.now();
  showMobileTabSelector.value = true;
}

function handleMobileTabTriggerClick() {
  const anchorEl = mobileTabTriggerRef.value;
  if (!anchorEl) {
    return;
  }
  if (showMobileTabSelector.value && mobileTabSelectorAnchor.value?.source === 'header') {
    closeMobileSessionSelector();
    return;
  }
  openMobileSessionSelectorFromElement(anchorEl, 'header');
}

function handleMobileTabDropdownClickoutside() {
  if (Date.now() - mobileTabSelectorOpenedAt < MOBILE_TAB_SELECTOR_CLICKOUTSIDE_GUARD_MS) {
    return;
  }
  closeMobileSessionSelector();
}

function requestMobileViewForBottomNavSelector() {
  if (mobileTabSelectorAnchor.value?.source !== 'bottom-nav') {
    return;
  }
  emit('request-mobile-view', 'webSession');
}

function renderDropdownIcon(icon: Component) {
  return () => h(NIcon, null, { default: () => h(icon) });
}

function buildSessionActionOptions(session: SessionTab | null): DropdownOption[] {
  const canClaudeSync =
    !!session &&
    !isDraftSession(session) &&
    session.agent === 'claude' &&
    (Boolean(session.nativeSessionId) || Boolean(session.threadPath));
  const canCodexSync =
    !!session &&
    !isDraftSession(session) &&
    session.agent === 'codex' &&
    Boolean(session.nativeSessionId);

  const options: DropdownOption[] = [
    {
      label: t('webSession.newSession'),
      key: 'new',
      icon: renderDropdownIcon(AddOutline),
    },
    {
      label: t('webSession.importCodexSession'),
      key: 'import',
      icon: renderDropdownIcon(TimeOutline),
    },
    {
      label: t('common.edit'),
      key: 'rename',
      icon: renderDropdownIcon(CreateOutline),
      disabled: !session || isDraftSession(session),
    },
    {
      label: t('webSession.archiveAction'),
      key: 'archive',
      icon: renderDropdownIcon(ArchiveOutline),
      disabled:
        !session ||
        isDraftSession(session) ||
        isArchivedPreviewSession(session) ||
        isSessionArchiving(session.id),
    },
    {
      label: t('common.delete'),
      key: 'delete',
      icon: renderDropdownIcon(TrashOutline),
      disabled: !session,
    },
  ];

  if (canClaudeSync || canCodexSync) {
    options.splice(2, 0, {
      label:
        session?.agent === 'claude'
          ? t('webSession.syncSessionAction')
          : t('webSession.syncFromTerminal'),
      key: 'sync',
      icon: renderDropdownIcon(RefreshOutline),
      disabled: session?.agent === 'claude' ? !canClaudeSync : !canCodexSync,
    });
  }

  if (canCodexSync) {
    options.splice(3, 0, {
      label: t('webSession.deepSyncFromTerminal'),
      key: 'deep-sync',
      icon: renderDropdownIcon(RefreshCircleOutline),
      disabled: !canCodexSync,
    });
  }

  return options;
}

const contextMenuOptions = computed<DropdownOption[]>(() =>
  buildSessionActionOptions(contextMenuSession.value)
);

const mobileActionMenuOptions = computed<DropdownOption[]>(() =>
  buildSessionActionOptions(currentSession.value)
);

async function handleSessionActionSelect(action: string, session: SessionTab | null) {
  if (action === 'new') {
    await handleStartDraftSession();
    return;
  }
  if (action === 'import') {
    openImportDialog();
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
    confirmSyncSession(session, 'fast');
    return;
  }
  if (action === 'deep-sync') {
    confirmSyncSession(session, 'deep');
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

async function handleMobileActionMenuSelect(key: string | number) {
  await handleSessionActionSelect(String(key), currentSession.value);
}

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
const approvalTabColors = computed(() => {
  const theme = activeTheme.value;
  const isDarkTheme = isDarkHex(theme.bodyColor || '#ffffff');
  if (isDarkTheme) {
    return {
      bg: 'var(--web-session-approval-bg)',
      border: 'var(--web-session-approval-border)',
      activeBg:
        'color-mix(in srgb, var(--web-session-approval-bg, rgba(247, 144, 9, 0.16)) 78%, var(--app-surface-color, #fff) 22%)',
      activeBorder:
        'color-mix(in srgb, var(--web-session-approval-border, rgba(247, 144, 9, 0.42)) 88%, transparent 12%)',
    };
  }
  return {
    bg: 'rgba(247, 144, 9, 0.14)',
    border: 'rgba(247, 144, 9, 0.44)',
    activeBg: 'rgba(247, 144, 9, 0.22)',
    activeBorder: 'rgba(247, 144, 9, 0.6)',
  };
});
const planApprovalColors = computed(() => {
  const theme = activeTheme.value;
  const isDarkTheme = isDarkHex(theme.bodyColor || '#ffffff');
  return {
    bg: isDarkTheme ? 'rgba(34, 211, 238, 0.18)' : 'rgba(6, 182, 212, 0.14)',
    border: isDarkTheme ? 'rgba(34, 211, 238, 0.4)' : 'rgba(6, 182, 212, 0.3)',
    accent: isDarkTheme ? '#22d3ee' : '#0891b2',
    accentStrong: isDarkTheme ? '#06b6d4' : '#0e7490',
    glow: isDarkTheme ? 'rgba(34, 211, 238, 0.24)' : 'rgba(6, 182, 212, 0.16)',
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
      '--web-session-approval-tab-bg': approvalTabColors.value.bg,
      '--web-session-approval-tab-border': approvalTabColors.value.border,
      '--web-session-approval-tab-active-bg': approvalTabColors.value.activeBg,
      '--web-session-approval-tab-active-border': approvalTabColors.value.activeBorder,
      '--web-session-plan-approval-bg': planApprovalColors.value.bg,
      '--web-session-plan-approval-border': planApprovalColors.value.border,
      '--web-session-plan-approval-accent': planApprovalColors.value.accent,
      '--web-session-plan-approval-accent-strong': planApprovalColors.value.accentStrong,
      '--web-session-plan-approval-glow': planApprovalColors.value.glow,
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
const sidebarVisibleProjectIds = computed(() =>
  resolveWebSessionSidebarProjectIds({
    scope: sidebarScope.value,
    currentProjectId: props.projectId,
    allProjectIds: sidebarProjectIdsToLoad.value,
  })
);

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
    autoRetryEnabled: session.autoRetryEnabled === true,
    autoRetryScope:
      session.autoRetryScope === 'network_and_rate_limit' ||
      session.autoRetryScope === 'all_failures'
        ? session.autoRetryScope
        : webSessionAutoContinueScope.value,
    autoRetryPreset:
      session.autoRetryPreset === 'aggressive_stop' || session.autoRetryPreset === 'sustain_60s'
        ? session.autoRetryPreset
        : webSessionAutoContinuePreset.value,
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
    syncState: normalizeWebSessionSyncState(session.syncState),
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
    contextEstimate: {
      inputTokens: 0,
      cachedInputTokens: 0,
      outputTokens: 0,
      usedTokens: 0,
    },
    contextEstimateMode: 'cumulative_total',
    lastContextCompactionAt: null,
    contextWindowTokens: agent === 'codex' ? DEFAULT_CODEX_CONTEXT_WINDOW_TOKENS : null,
    contextWindowSource: agent === 'codex' ? 'default' : 'unavailable',
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

function normalizeSessionIdList(value: unknown) {
  if (!Array.isArray(value) || value.length === 0) {
    return [];
  }
  const seen = new Set<string>();
  return value
    .map(item => String(item || '').trim())
    .filter(sessionId => {
      if (!sessionId || seen.has(sessionId)) {
        return false;
      }
      seen.add(sessionId);
      return true;
    });
}

function loadPersistedTabOrderIds(projectId: string) {
  return normalizeSessionIdList(persistedTabOrderByProject.value[projectId]);
}

function loadPersistedTabMruIds(projectId: string) {
  return normalizeSessionIdList(persistedTabMruByProject.value[projectId]);
}

function getVisibleTabIds() {
  return sessions.value.map(session => session.id);
}

function getDefaultTabOrderIds(visibleIds = getVisibleTabIds()) {
  const visibleSet = new Set(visibleIds);
  const ids = [
    ...realSessions.value.map(session => session.id).filter(sessionId => visibleSet.has(sessionId)),
    ...draftSessions.value
      .map(session => session.id)
      .filter(sessionId => visibleSet.has(sessionId)),
  ];
  return ids;
}

function normalizeTabOrderIds(orderIds: string[], visibleIds = getVisibleTabIds()) {
  const defaultIds = getDefaultTabOrderIds(visibleIds);
  const visibleSet = new Set(defaultIds);
  const next: string[] = [];

  normalizeSessionIdList(orderIds).forEach(sessionId => {
    if (!visibleSet.has(sessionId) || next.includes(sessionId)) {
      return;
    }
    next.push(sessionId);
  });

  defaultIds.forEach(sessionId => {
    if (!next.includes(sessionId)) {
      next.push(sessionId);
    }
  });

  return next;
}

function normalizeTabMruIds(
  mruIds: string[],
  visibleIds = getVisibleTabIds(),
  orderIds = normalizeTabOrderIds(tabOrderIds.value, visibleIds)
) {
  const visibleSet = new Set(visibleIds);
  const next: string[] = [];

  normalizeSessionIdList(mruIds).forEach(sessionId => {
    if (!visibleSet.has(sessionId) || next.includes(sessionId)) {
      return;
    }
    next.push(sessionId);
  });

  orderIds.forEach(sessionId => {
    if (visibleSet.has(sessionId) && !next.includes(sessionId)) {
      next.push(sessionId);
    }
  });

  return next;
}

function persistTabNavigationState(
  projectId: string,
  nextOrderIds = tabOrderIds.value,
  nextMruIds = tabMruIds.value,
  visibleIds = getVisibleTabIds()
) {
  if (!projectId) {
    return;
  }

  const normalizedOrderIds = normalizeTabOrderIds(nextOrderIds, visibleIds);
  const normalizedMruIds = normalizeTabMruIds(nextMruIds, visibleIds, normalizedOrderIds);
  const persistableIds = normalizedOrderIds.filter(sessionId => {
    const session = visibleSessionById.value.get(sessionId);
    return session && !isArchivedPreviewSession(session);
  });
  const persistableMruIds = normalizedMruIds.filter(sessionId => {
    const session = visibleSessionById.value.get(sessionId);
    return session && !isArchivedPreviewSession(session);
  });

  persistedTabOrderByProject.value = persistableIds.length
    ? {
        ...persistedTabOrderByProject.value,
        [projectId]: persistableIds,
      }
    : Object.fromEntries(
        Object.entries(persistedTabOrderByProject.value).filter(([key]) => key !== projectId)
      );

  persistedTabMruByProject.value = persistableMruIds.length
    ? {
        ...persistedTabMruByProject.value,
        [projectId]: persistableMruIds,
      }
    : Object.fromEntries(
        Object.entries(persistedTabMruByProject.value).filter(([key]) => key !== projectId)
      );
}

function replaceTabNavigationState(
  nextOrderIds: string[],
  nextMruIds: string[],
  projectId = props.projectId,
  visibleIds = getVisibleTabIds()
) {
  const normalizedOrderIds = normalizeTabOrderIds(nextOrderIds, visibleIds);
  const normalizedMruIds = normalizeTabMruIds(nextMruIds, visibleIds, normalizedOrderIds);
  tabOrderIds.value = normalizedOrderIds;
  tabMruIds.value = normalizedMruIds;
  persistTabNavigationState(projectId, normalizedOrderIds, normalizedMruIds, visibleIds);
}

function syncTabNavigationState(
  projectId = props.projectId,
  options?: { orderIds?: string[]; mruIds?: string[]; visibleIds?: string[] }
) {
  const visibleIds = options?.visibleIds ?? getVisibleTabIds();
  replaceTabNavigationState(
    options?.orderIds ?? tabOrderIds.value,
    options?.mruIds ?? tabMruIds.value,
    projectId,
    visibleIds
  );
}

function rememberTabVisit(sessionId: string, projectId = props.projectId) {
  const normalizedSessionId = String(sessionId || '').trim();
  if (!normalizedSessionId) {
    return;
  }
  const visibleIds = getVisibleTabIds();
  if (!visibleIds.includes(normalizedSessionId)) {
    return;
  }
  replaceTabNavigationState(
    tabOrderIds.value,
    [normalizedSessionId, ...tabMruIds.value.filter(id => id !== normalizedSessionId)],
    projectId,
    visibleIds
  );
}

function insertTabAfter(
  sessionId: string,
  afterId = underlyingTabSessionId.value,
  projectId = props.projectId
) {
  const visibleIds = getVisibleTabIds();
  if (!visibleIds.includes(sessionId)) {
    return;
  }
  const nextOrderIds = normalizeTabOrderIds(
    tabOrderIds.value.filter(id => id !== sessionId),
    visibleIds.filter(id => id !== sessionId)
  );
  let insertIndex = nextOrderIds.length;
  if (afterId) {
    const anchorIndex = nextOrderIds.indexOf(afterId);
    insertIndex = anchorIndex >= 0 ? anchorIndex + 1 : nextOrderIds.length;
  }
  nextOrderIds.splice(insertIndex, 0, sessionId);
  replaceTabNavigationState(
    nextOrderIds,
    [sessionId, ...tabMruIds.value.filter(id => id !== sessionId)],
    projectId,
    visibleIds
  );
}

function replaceTabIdInNavigationState(
  fromId: string,
  toId: string,
  projectId = props.projectId,
  visibleIds = Array.from(new Set([...getVisibleTabIds().filter(id => id !== fromId), toId]))
) {
  const nextOrderIds = tabOrderIds.value.map(sessionId =>
    sessionId === fromId ? toId : sessionId
  );
  const nextMruIds = tabMruIds.value.map(sessionId => (sessionId === fromId ? toId : sessionId));
  replaceTabNavigationState(nextOrderIds, nextMruIds, projectId, visibleIds);
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
  if (nextActiveDraftId) {
    activeArchivedPreviewId.value = '';
  }
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
  const anchorId = underlyingTabSessionId.value;
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
    permissionLevel:
      (source?.permissionLevel === 'default' && nextAgent === 'claude'
        ? 'elevated'
        : source?.permissionLevel) || draftPermissionLevel.value,
    autoRetryEnabled: source?.autoRetryEnabled === true,
    autoRetryScope:
      source?.autoRetryEnabled === true ? source.autoRetryScope : webSessionAutoContinueScope.value,
    autoRetryPreset:
      source?.autoRetryEnabled === true
        ? source.autoRetryPreset
        : webSessionAutoContinuePreset.value,
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
    contextEstimate: {
      inputTokens: 0,
      cachedInputTokens: 0,
      outputTokens: 0,
      usedTokens: 0,
    },
    contextEstimateMode: 'cumulative_total',
    lastContextCompactionAt: null,
    contextWindowTokens: nextAgent === 'codex' ? DEFAULT_CODEX_CONTEXT_WINDOW_TOKENS : null,
    contextWindowSource: nextAgent === 'codex' ? 'default' : 'unavailable',
    isDraft: true,
  };
  replaceDraftSessionState([...draftSessions.value, draft], draft.id);
  insertTabAfter(draft.id, anchorId);
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
  if (activeArchivedPreviewId.value === archivedPreviewSession.value?.id) {
    activeArchivedPreviewId.value = '';
  }
  archivedPreviewSession.value = null;
}

function syncArchivedPreviewSessionSummary(sessionId: string) {
  if (!archivedPreviewSession.value || archivedPreviewSession.value.id !== sessionId) {
    return;
  }
  const latest =
    webSessionStore
      .getArchivedSessions(mobileArchivedProjectIds.value)
      .find(item => item.id === sessionId) ??
    webSessionStore
      .getArchivedSessions(sidebarVisibleProjectIds.value)
      .find(item => item.id === sessionId) ??
    archivedPreviewSession.value;
  archivedPreviewSession.value = {
    ...latest,
    isArchivedPreview: true,
  };
}

function resolveNextTabAfterClose(sessionId: string) {
  const nextVisibleIds = getVisibleTabIds().filter(id => id !== sessionId);
  const nextOrderIds = normalizeTabOrderIds(
    tabOrderIds.value.filter(id => id !== sessionId),
    nextVisibleIds
  );
  const nextMruIds = normalizeTabMruIds(
    tabMruIds.value.filter(id => id !== sessionId),
    nextVisibleIds,
    nextOrderIds
  );
  if (nextMruIds[0]) {
    return nextMruIds[0];
  }
  const currentOrderIds = normalizeTabOrderIds(tabOrderIds.value, getVisibleTabIds());
  const closedIndex = currentOrderIds.indexOf(sessionId);
  if (closedIndex < 0) {
    return nextOrderIds[0] ?? '';
  }
  return nextOrderIds[closedIndex] ?? nextOrderIds[closedIndex - 1] ?? nextOrderIds[0] ?? '';
}

async function activateTabById(
  sessionId: string,
  options?: { connectReal?: boolean; routeDriven?: boolean }
) {
  const session = visibleSessionById.value.get(sessionId);
  if (!session) {
    return false;
  }
  if (!options?.routeDriven) {
    pendingRouteActivationSessionId.value = '';
  }

  if (isDraftSession(session)) {
    realSessionSnapshotLoadController.cancel();
    replaceDraftSessionState(draftSessions.value, session.id);
    activeArchivedPreviewId.value = '';
    webSessionStore.setActiveSession(props.projectId, '');
    rememberTabVisit(session.id);
    return true;
  } else if (isArchivedPreviewSession(session)) {
    realSessionSnapshotLoadController.cancel();
    activeArchivedPreviewId.value = session.id;
    return true;
  } else {
    replaceDraftSessionState(draftSessions.value, '');
    activeArchivedPreviewId.value = '';
    rememberTabVisit(session.id);
    if (options?.connectReal === false) {
      realSessionSnapshotLoadController.cancel();
      webSessionStore.setActiveSession(props.projectId, session.id);
      return true;
    } else {
      return await connectVisibleRealSession(props.projectId, session.id);
    }
  }
}

function buildProjectRouteLocation(projectId: string, sessionId = '') {
  return {
    name: 'project' as const,
    params: { id: projectId },
    query: buildWebSessionRouteQuery(buildWorkspaceRouteQuery(route.query, 'web'), sessionId),
  };
}

async function syncWebSessionRouteSessionId(sessionId = '') {
  if (isWebSessionRouteQuerySynced(route.query, sessionId)) {
    return;
  }
  await router.replace({
    query: buildWebSessionRouteQuery(route.query, sessionId),
  });
}

async function openArchivedPreviewSession(
  session: WebSessionSummary,
  options?: { snapshotLoaded?: boolean; routeDriven?: boolean }
) {
  if (!options?.routeDriven) {
    pendingRouteActivationSessionId.value = '';
  }
  const previousPreviewId = archivedPreviewSession.value?.id ?? '';
  if (previousPreviewId && previousPreviewId !== session.id) {
    clearArchivedPreviewSession();
  }
  archivedPreviewSession.value = {
    ...session,
    isArchivedPreview: true,
  };
  activeArchivedPreviewId.value = session.id;
  if (!options?.snapshotLoaded) {
    const snapshot = await webSessionStore.loadSessionSnapshot(session.projectId, session.id, {
      rememberActive: false,
      preserveArchivedPosition: true,
    });
    if (snapshot?.session) {
      archivedPreviewSession.value = {
        ...snapshot.session,
        isArchivedPreview: true,
      };
    }
  }
  syncArchivedPreviewSessionSummary(session.id);
}

async function connectVisibleRealSession(projectId: string, sessionId: string) {
  if (!projectId || !sessionId) {
    return false;
  }
  const snapshotLoad = realSessionSnapshotLoadController.begin();
  activeArchivedPreviewId.value = '';
  webSessionStore.setActiveSession(projectId, sessionId);
  try {
    await webSessionStore.loadSessionSnapshot(projectId, sessionId, {
      rememberActive: false,
      signal: snapshotLoad.signal,
    });
    return realSessionSnapshotLoadController.isCurrent(snapshotLoad);
  } catch (error) {
    if (isAbortLikeError(error) || !realSessionSnapshotLoadController.isCurrent(snapshotLoad)) {
      return false;
    }
    throw error;
  } finally {
    realSessionSnapshotLoadController.release(snapshotLoad);
  }
}

async function activateSessionFromRoute(
  projectId: string,
  requestedSessionId: string,
  options?: {
    loadedSessions?: WebSessionSummary[];
    showError?: boolean;
  }
) {
  const routeTarget = resolveWebSessionDeepLinkTarget({
    currentProjectId: projectId,
    requestedSessionId,
    loadedSessions: options?.loadedSessions ?? realSessions.value,
  });

  if (routeTarget.action === 'none') {
    return false;
  }

  if (routeTarget.action === 'activate-loaded') {
    const handled = await activateTabById(routeTarget.sessionId, { routeDriven: true });
    if (handled) {
      pendingRouteActivationSessionId.value = '';
    }
    return handled;
  }

  if (routeTarget.action !== 'load-snapshot') {
    return false;
  }

  const snapshotLoad = realSessionSnapshotLoadController.begin();
  try {
    const snapshot = await webSessionStore.loadSessionSnapshot(projectId, routeTarget.sessionId, {
      rememberActive: false,
      signal: snapshotLoad.signal,
      preserveArchivedPosition: true,
    });
    if (!realSessionSnapshotLoadController.isCurrent(snapshotLoad)) {
      return false;
    }
    const snapshotTarget = resolveWebSessionDeepLinkTarget({
      currentProjectId: projectId,
      requestedSessionId: routeTarget.sessionId,
      snapshotSession: snapshot?.session ?? null,
    });

    if (snapshotTarget.action === 'activate-real') {
      const handled = await activateTabById(snapshotTarget.sessionId, {
        connectReal: false,
        routeDriven: true,
      });
      if (handled) {
        pendingRouteActivationSessionId.value = '';
      }
      return handled;
    }

    if (snapshotTarget.action === 'open-archived-preview' && snapshot?.session) {
      await openArchivedPreviewSession(snapshot.session, {
        snapshotLoaded: true,
        routeDriven: true,
      });
      pendingRouteActivationSessionId.value = '';
      return true;
    }

    pendingRouteActivationSessionId.value = '';
    await syncWebSessionRouteSessionId('');
  } catch (error) {
    if (isAbortLikeError(error) || !realSessionSnapshotLoadController.isCurrent(snapshotLoad)) {
      return false;
    }
    pendingRouteActivationSessionId.value = '';
    await syncWebSessionRouteSessionId('');
    if (options?.showError !== false) {
      message.error(error instanceof Error ? error.message : t('common.error'));
    }
  } finally {
    realSessionSnapshotLoadController.release(snapshotLoad);
  }

  return false;
}

function removeDraftSessionRecord(sessionId: string, options?: { preserveDraftState?: boolean }) {
  const removedActive = activeDraftSessionId.value === sessionId;
  const nextActiveDraftId = removedActive ? '' : activeDraftSessionId.value;
  replaceDraftSessionState(
    draftSessions.value.filter(session => session.id !== sessionId),
    nextActiveDraftId
  );
  if (!options?.preserveDraftState) {
    webSessionStore.clearDraft(props.projectId, sessionId);
  }
}

async function closeTabById(
  sessionId: string,
  closer: () => Promise<void> | void,
  options?: { syncNavigationOnly?: boolean }
) {
  const wasActive = activeSessionId.value === sessionId;
  const fallbackTabId = wasActive ? resolveNextTabAfterClose(sessionId) : '';

  await closer();

  syncTabNavigationState();

  if (wasActive) {
    if (fallbackTabId && (await activateTabById(fallbackTabId))) {
      return;
    }
    ensureDefaultDraftSession();
    if (activeSessionId.value) {
      rememberTabVisit(activeSessionId.value);
    }
    return;
  }

  if (options?.syncNavigationOnly) {
    syncTabNavigationState();
  }
}

function markSessionViewed(sessionId?: string) {
  const normalizedSessionId = String(sessionId || '').trim();
  if (!props.isActive || !normalizedSessionId) {
    return;
  }
  const session = visibleSessionById.value.get(normalizedSessionId);
  if (session && !isDraftSession(session)) {
    optimisticUnreadClearedVersionBySession.value = {
      ...optimisticUnreadClearedVersionBySession.value,
      [normalizedSessionId]: getSessionUnreadVersion(session),
    };
  }
  webSessionStore.emitter.emit('web-session:viewed', {
    sessionId: normalizedSessionId,
  });
}

function getSessionUnreadVersion(session: WebSessionSummary) {
  return parseTimestamp(
    session.statusUpdatedAt ||
      session.assistantStateUpdatedAt ||
      session.updatedAt ||
      session.activityAt ||
      session.lastMessageAt ||
      session.createdAt
  );
}

function hasSessionUnread(session: (typeof sessions.value)[number]) {
  if (isDraftSession(session)) {
    return false;
  }
  if (!session.hasUnread) {
    return false;
  }
  const optimisticClearedVersion = optimisticUnreadClearedVersionBySession.value[session.id] ?? 0;
  return getSessionUnreadVersion(session) > optimisticClearedVersion;
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
  isCurrent: boolean;
  projectIndex?: { index: number; color: string };
};

function buildSidebarProjectOrder(items: Array<Pick<CrossProjectSessionItem, 'projectId'>>) {
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
  return projectIds;
}

function withProjectIndexes(
  items: CrossProjectSessionItem[],
  projectIds = buildSidebarProjectOrder(items)
) {
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
  sidebarVisibleProjectIds.value.forEach(projectId => {
    webSessionStore.getSessions(projectId).forEach(session => {
      rawItems.push({
        session,
        projectId,
        projectName: getProjectName(projectId),
        isCurrent: projectId === props.projectId && session.id === activeSessionId.value,
      });
    });
  });
  const projectIds = buildSidebarProjectOrder(rawItems);
  const sorted = [...rawItems].sort((left, right) => {
    const rightTimestamp = resolveWebSessionSidebarSortTimestamp(right.session);
    const leftTimestamp = resolveWebSessionSidebarSortTimestamp(left.session);
    if (rightTimestamp !== leftTimestamp) {
      return rightTimestamp - leftTimestamp;
    }
    if (left.session.orderIndex !== right.session.orderIndex) {
      return left.session.orderIndex - right.session.orderIndex;
    }
    return left.session.id.localeCompare(right.session.id);
  });

  return withProjectIndexes(sorted, projectIds);
});

const crossProjectArchivedSessions = computed<CrossProjectSessionItem[]>(() => {
  const items = webSessionStore
    .getArchivedSessions(sidebarVisibleProjectIds.value)
    .map(session => ({
      session,
      projectId: session.projectId,
      projectName: getProjectName(session.projectId),
      isCurrent: activeArchivedPreviewId.value === session.id,
    }));
  return withProjectIndexes(items);
});

const archivedSidebarMeta = computed(() =>
  webSessionStore.getArchivedMeta(sidebarVisibleProjectIds.value)
);

async function ensureArchivedScopeLoaded(projectIds: string[], limit = 20) {
  if (!projectIds.length || webSessionStore.hasArchivedScope(projectIds)) {
    return;
  }
  await webSessionStore.loadArchivedSessions(projectIds, {
    reset: true,
    limit,
  });
}

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
    { label: 'Off', value: 'none' },
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
    if (next === 'claude' && draftPermissionLevel.value === 'default') {
      draftPermissionLevel.value = 'elevated';
    }
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
        permissionLevel:
          next === 'claude' && current.permissionLevel === 'default'
            ? 'elevated'
            : current.permissionLevel,
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
  ...(selectedAgent.value === 'claude'
    ? []
    : [{ label: t('webSession.permissionDefault'), value: 'default' }]),
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
  get: () => {
    const value = currentSession.value?.permissionLevel ?? draftPermissionLevel.value;
    if (selectedAgent.value === 'claude' && value === 'default') {
      return 'elevated';
    }
    return value;
  },
  set: value => {
    const next =
      selectedAgent.value === 'claude' && value === 'default'
        ? 'elevated'
        : (value as 'default' | 'elevated' | 'yolo');
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
  return formatWebSessionTimestamp(timestamp, locale.value);
}

function formatDateTime(timestamp: number) {
  return formatWebSessionDateTime(timestamp, locale.value);
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

function isValidTimestamp(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0;
}

function getLiveStateEndedAt(state: WebSessionLiveState) {
  return state.running ? liveStateClockMs.value : state.updatedAt;
}

function getLiveRunStartedAt(state: WebSessionLiveState) {
  return isValidTimestamp(state.startedAt) ? state.startedAt : undefined;
}

function getLiveToolStartedAt(state: WebSessionLiveState) {
  return isValidTimestamp(state.tool?.startedAt) ? state.tool.startedAt : undefined;
}

function getLiveOperationStartedAt(state: WebSessionLiveState) {
  if (state.phase === 'tool') {
    return getLiveToolStartedAt(state) ?? getLiveRunStartedAt(state);
  }
  if (state.phase === 'waiting_approval') {
    return isValidTimestamp(pendingApproval.value?.requestedAt)
      ? pendingApproval.value.requestedAt
      : getLiveRunStartedAt(state);
  }
  if (state.phase === 'waiting_input') {
    return isValidTimestamp(pendingUserInput.value?.requestedAt)
      ? pendingUserInput.value.requestedAt
      : getLiveRunStartedAt(state);
  }
  if (
    state.phase === 'starting' ||
    state.phase === 'thinking' ||
    state.phase === 'waiting_plan_approval'
  ) {
    return getLiveRunStartedAt(state);
  }
  return getLiveRunStartedAt(state);
}

function isUserOriginatedInteractionBlock(block: WebSessionBlock) {
  return (
    block.kind === 'user' ||
    block.detail?.type === 'approval_response' ||
    block.detail?.type === 'user_input_response'
  );
}

function getLivePreviousInteractionAt(state: WebSessionLiveState) {
  const endedAt = getLiveStateEndedAt(state);
  for (let index = blocks.value.length - 1; index >= 0; index -= 1) {
    const block = blocks.value[index];
    if (!isValidTimestamp(block.timestamp) || block.timestamp > endedAt) {
      continue;
    }
    if (isUserOriginatedInteractionBlock(block)) {
      return block.timestamp;
    }
  }
  return undefined;
}

function getLiveElapsedText(state: WebSessionLiveState) {
  const startedAt = getLiveOperationStartedAt(state);
  if (!startedAt) {
    return '';
  }
  return formatElapsedDuration(startedAt, getLiveStateEndedAt(state));
}

function getLiveTimeText(state: WebSessionLiveState) {
  const elapsed = getLiveElapsedText(state);
  if (elapsed) {
    return elapsed;
  }
  return formatTime(state.updatedAt);
}

function getLiveTimeTooltipItems(state: WebSessionLiveState): LiveTimeTooltipItem[] {
  const startedAt = getLiveOperationStartedAt(state);
  const elapsed = getLiveElapsedText(state);
  const previousInteractionAt = getLivePreviousInteractionAt(state);
  const endedAt = getLiveStateEndedAt(state);
  const items: LiveTimeTooltipItem[] = [];

  if (startedAt) {
    items.push({
      key: 'started-at',
      label: t('webSession.liveTooltipStartedAt'),
      value: formatDateTime(startedAt),
    });
  }

  if (elapsed) {
    items.push({
      key: 'elapsed',
      label: t('webSession.liveTooltipElapsed'),
      value: elapsed,
    });
  }

  if (previousInteractionAt) {
    items.push({
      key: 'since-previous-interaction',
      label: t('webSession.liveTooltipSincePreviousInteraction'),
      value: formatElapsedDuration(previousInteractionAt, endedAt),
    });
  }

  if (items.length > 0) {
    return items;
  }

  const updatedAt = formatDateTime(state.updatedAt);
  if (updatedAt) {
    return [
      {
        key: 'updated-at',
        label: t('webSession.liveTooltipStartedAt'),
        value: updatedAt,
      },
    ];
  }

  return [];
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

function extractToolWorkingDirectory(input: unknown) {
  const record = asRecord(input);
  if (!record) {
    return '';
  }

  const direct = String(record.cwd ?? record.workdir ?? '').trim();
  if (direct) {
    return direct;
  }

  const args = asRecord(record.arguments);
  return String(args?.cwd ?? args?.workdir ?? '').trim();
}

function getImageViewToolData(tool?: NonNullable<WebSessionBlock['tool']>) {
  if (!tool) {
    return null;
  }
  return parseImageViewToolOutput(tool.output);
}

function isImageViewTool(tool?: NonNullable<WebSessionBlock['tool']>) {
  return Boolean(getImageViewToolData(tool));
}

function getImageViewDisplayName(tool?: NonNullable<WebSessionBlock['tool']>) {
  const data = getImageViewToolData(tool);
  return data ? resolveImageViewDisplayName(data.path) : '';
}

function getImageViewDisplayPath(tool?: NonNullable<WebSessionBlock['tool']>) {
  return getImageViewToolData(tool)?.path ?? '';
}

function getImageViewPreviewSrc(tool?: NonNullable<WebSessionBlock['tool']>) {
  if (!tool) {
    return '';
  }
  return imageViewPreviewSrcByToolId.value[tool.id] ?? '';
}

function getImageViewPreviewState(tool?: NonNullable<WebSessionBlock['tool']>) {
  if (!tool) {
    return 'loading' as const;
  }
  return imageViewPreviewStateByToolId.value[tool.id] ?? 'loading';
}

function ensureImageViewPreview(tool: NonNullable<WebSessionBlock['tool']>) {
  if (imageViewPreviewSrcByToolId.value[tool.id]) {
    if (imageViewPreviewStateByToolId.value[tool.id] === 'error') {
      imageViewPreviewStateByToolId.value = {
        ...imageViewPreviewStateByToolId.value,
        [tool.id]: 'loading',
      };
    }
    return;
  }

  const data = getImageViewToolData(tool);
  if (!data) {
    return;
  }

  const previewSrc = buildImageViewPreviewUrl(data.path, {
    cwd: data.cwd || extractToolWorkingDirectory(tool.input) || currentRealSession.value?.cwd,
  });
  if (!previewSrc) {
    return;
  }

  imageViewPreviewSrcByToolId.value = {
    ...imageViewPreviewSrcByToolId.value,
    [tool.id]: previewSrc,
  };
  imageViewPreviewStateByToolId.value = {
    ...imageViewPreviewStateByToolId.value,
    [tool.id]: 'loading',
  };
}

function handleImageViewPreviewLoad(toolId: string) {
  imageViewPreviewStateByToolId.value = {
    ...imageViewPreviewStateByToolId.value,
    [toolId]: 'ready',
  };
}

function handleImageViewPreviewError(toolId: string) {
  imageViewPreviewStateByToolId.value = {
    ...imageViewPreviewStateByToolId.value,
    [toolId]: 'error',
  };
}

function normalizeToolKindValue(value: string | undefined) {
  const normalized = String(value ?? '').trim();
  if (normalized === 'commandExecution') {
    return 'command_execution';
  }
  if (normalized === 'contextCompaction') {
    return 'context_compaction';
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

function isContextCompactionToolKind(value: string | undefined) {
  return normalizeToolKindValue(value) === 'context_compaction';
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

function toolCardClass(tool: Pick<NonNullable<WebSessionBlock['tool']>, 'kind' | 'meta'>) {
  return {
    'is-context-compaction-tool': isContextCompactionToolKind(
      tool.kind || String(tool.meta?.kind ?? '')
    ),
  };
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

function contextCompactionPreview(tool: { output?: string; meta?: Record<string, unknown> }) {
  const preview = String(tool.output ?? tool.meta?.subtitle ?? '')
    .replace(/\s+/g, ' ')
    .trim();
  if (preview) {
    return preview.slice(0, 120);
  }
  return t('webSession.contextCompactionFallbackPreview');
}

function getCompactToolDisplaySummary(tool: NonNullable<WebSessionBlock['tool']>) {
  const summary = getCompactToolSummary(tool).trim();
  if (summary) {
    return summary;
  }
  if (shouldShowToolPendingPlaceholder(tool)) {
    return t('common.loading');
  }
  return t('webSession.compactToolNoSummary');
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

function formatCommandExecutionDetailDateTime(item: CommandExecutionDetailItem) {
  const value = Date.parse(item.completedAt || item.startedAt || item.timestamp || '');
  if (!Number.isFinite(value)) {
    return '';
  }
  return formatDateTime(value);
}

function isToolExpanded(toolId: string) {
  return Boolean(expandedTools.value[toolId]);
}

function toggleToolExpanded(tool: NonNullable<WebSessionBlock['tool']>) {
  const nextExpanded = !expandedTools.value[tool.id];
  if (nextExpanded && isImageViewTool(tool)) {
    ensureImageViewPreview(tool);
  }

  expandedTools.value = {
    ...expandedTools.value,
    [tool.id]: nextExpanded,
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

function beginSessionArchive(sessionId: string) {
  archiveStateBySessionId.value = beginWebSessionSubmit(archiveStateBySessionId.value, sessionId);
}

function endSessionArchive(sessionId: string) {
  archiveStateBySessionId.value = endWebSessionSubmit(archiveStateBySessionId.value, sessionId);
}

function isSessionArchiving(sessionId: string) {
  return isWebSessionSubmitting(archiveStateBySessionId.value, sessionId);
}

function toolKindLabel(tool: { name: string; kind?: string; output?: string }) {
  if (isImageViewTool(tool as NonNullable<WebSessionBlock['tool']>)) {
    return t('webSession.toolImageView');
  }
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
  if (kind === 'context_compaction') {
    return t('webSession.toolContextCompaction');
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
  if (isContextCompactionToolKind(tool.kind || String(tool.meta?.kind ?? ''))) {
    return contextCompactionPreview(tool);
  }
  const imageViewData = getImageViewToolData(tool as NonNullable<WebSessionBlock['tool']>);
  if (imageViewData) {
    return resolveImageViewDisplayName(imageViewData.path);
  }
  if (isCompactTool(tool as NonNullable<WebSessionBlock['tool']>)) {
    return getCompactToolSummary(tool as NonNullable<WebSessionBlock['tool']>);
  }
  const source =
    typeof tool.output === 'string' && tool.output.trim()
      ? tool.output
      : stringifyValue(tool.input);
  const preview = String(source ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 120);
  if (preview) {
    return preview;
  }
  if (shouldShowToolPendingPlaceholder(tool as NonNullable<WebSessionBlock['tool']>)) {
    return t('common.loading');
  }
  return '';
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
  isProjectSessionInitializing.value = true;
  realSessionSnapshotLoadController.cancel();
  try {
    clearArchivedPreviewSession();
    activeArchivedPreviewId.value = '';
    tabOrderIds.value = loadPersistedTabOrderIds(projectId);
    tabMruIds.value = loadPersistedTabMruIds(projectId);
    const restoredDraftState = collapseProjectDraftTabs({
      drafts: loadPersistedDraftSessions(projectId),
      activeDraftId: persistedActiveDraftSessionIdByProject.value[projectId] ?? '',
      orderIds: tabOrderIds.value,
      mruIds: tabMruIds.value,
    });
    restoredDraftState.removedDraftIds.forEach(draftId => {
      webSessionStore.clearDraft(projectId, draftId);
    });
    tabOrderIds.value = restoredDraftState.orderIds;
    tabMruIds.value = restoredDraftState.mruIds;
    const restoredDrafts = restoredDraftState.drafts;
    const activeDraftId = restoredDraftState.activeDraftId;
    replaceDraftSessionState(restoredDrafts, activeDraftId, projectId);
    const loadedSessions = await webSessionStore.loadSessions(projectId);
    syncTabNavigationState(projectId, {
      orderIds: tabOrderIds.value,
      mruIds: tabMruIds.value,
    });
    await webSessionStore.openEventStream();
    if (routeWebSessionId.value) {
      pendingRouteActivationSessionId.value = routeWebSessionId.value;
      const handled = await activateSessionFromRoute(projectId, routeWebSessionId.value, {
        loadedSessions,
      });
      if (handled) {
        return;
      }
    }
    if (activeDraftId) {
      await activateTabById(activeDraftId, { connectReal: false });
      return;
    }
    const rememberedSessionId = webSessionStore.getActiveSessionId(projectId);
    const targetSessionId =
      loadedSessions.find(session => session.id === rememberedSessionId)?.id ??
      loadedSessions[0]?.id;
    if (targetSessionId) {
      try {
        await activateTabById(targetSessionId);
      } catch (error) {
        console.warn('[Web Session] Failed to initialize current session', {
          projectId,
          sessionId: targetSessionId,
          error,
        });
      }
      return;
    }
    if (restoredDrafts.length > 0) {
      const fallbackDraftId =
        tabMruIds.value.find(sessionId =>
          restoredDrafts.some(session => session.id === sessionId)
        ) ??
        restoredDrafts[restoredDrafts.length - 1]?.id ??
        '';
      if (fallbackDraftId) {
        await activateTabById(fallbackDraftId, { connectReal: false });
      }
      return;
    }
    ensureDefaultDraftSession();
  } finally {
    isProjectSessionInitializing.value = false;
  }
}

async function handleSessionSelect(sessionId: string) {
  if (!sessionId) {
    return;
  }
  closeMobileSessionSelector();
  if (sessionId === activeSessionId.value) {
    pendingRouteActivationSessionId.value = '';
    const session = currentSession.value;
    void syncWebSessionRouteSessionId(
      session && !isDraftSession(session) && session.projectId === props.projectId ? session.id : ''
    ).catch(error => {
      console.error('[Web Session] Failed to sync route session id', error);
    });
    rememberTabVisit(sessionId);
    scrollToBottom(true);
    return;
  }
  try {
    if (await activateTabById(sessionId)) {
      scrollToBottom(true);
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleSidebarSessionSelect(item: CrossProjectSessionItem) {
  const sessionId = item.session.id;
  if (!sessionId) {
    return;
  }
  try {
    if (item.projectId === props.projectId && sessionId === activeSessionId.value) {
      pendingRouteActivationSessionId.value = '';
      void syncWebSessionRouteSessionId(sessionId).catch(error => {
        console.error('[Web Session] Failed to sync route session id', error);
      });
      scrollToBottom(true);
      return;
    }
    if (item.projectId !== props.projectId) {
      webSessionStore.setActiveSession(item.projectId, sessionId);
      projectStore.addRecentProject(item.projectId);
      await router.push(buildProjectRouteLocation(item.projectId, sessionId));
      return;
    }
    if (await activateTabById(sessionId)) {
      scrollToBottom(true);
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleArchivedSidebarSessionSelect(item: CrossProjectSessionItem) {
  if (!item.session.id) {
    return;
  }
  try {
    if (item.projectId !== props.projectId) {
      projectStore.addRecentProject(item.projectId);
      await router.push(buildProjectRouteLocation(item.projectId, item.session.id));
      return;
    }
    if (archivedPreviewSession.value?.id === item.session.id) {
      pendingRouteActivationSessionId.value = '';
      void syncWebSessionRouteSessionId(item.session.id).catch(error => {
        console.error('[Web Session] Failed to sync route session id', error);
      });
      activeArchivedPreviewId.value = item.session.id;
      scrollToBottom(true);
      return;
    }
    await openArchivedPreviewSession(item.session);
    scrollToBottom(true);
  } catch (error) {
    clearArchivedPreviewSession();
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleLoadMoreArchived() {
  try {
    await webSessionStore.loadArchivedSessions(sidebarVisibleProjectIds.value, {
      limit: 20,
    });
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function handleSidebarScopeSelect(key: string | number) {
  sidebarScope.value = normalizeWebSessionSidebarScope(key);
}

function toggleSidebarScope() {
  sidebarScope.value = resolveWebSessionSidebarToggleScope(sidebarScope.value);
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

function openImportDialog() {
  closeMobileSessionSelector();
  contextMenuSession.value = null;
  showImportDialog.value = true;
}

function promptSyncExistingImportedSession(session: WebSessionSummary) {
  dialog.warning({
    title: t('webSession.importCodexSessionReuseTitle'),
    content: t('webSession.importCodexSessionReuseContent', {
      title: session.title,
    }),
    positiveText: t('webSession.syncFromTerminal'),
    negativeText: t('webSession.importCodexSessionReuseSkip'),
    onPositiveClick: async () => handleSyncSession(session.id, 'fast', false),
  });
}

async function handleOpenImportedCodexSession(session: WebSessionSummary) {
  try {
    showImportDialog.value = false;

    let target = session;
    if (session.archivedAt) {
      target = await webSessionStore.unarchiveSession(session.projectId, session.id);
      await refreshArchivedSidebar();
      if (archivedPreviewSession.value?.id === session.id) {
        clearArchivedPreviewSession();
        activeArchivedPreviewId.value = '';
      }
    }

    await activateTabById(target.id);
    scrollToBottom(true);
    promptSyncExistingImportedSession(target);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleImportCodexSession(sessionId: string) {
  if (!props.projectId || !sessionId || importingCodexSessionId.value) {
    return;
  }

  importingCodexSessionId.value = sessionId;
  try {
    const result = await webSessionStore.importSession(props.projectId, sessionId, 'fast');

    if (result.reused) {
      await refreshArchivedSidebar();
      if (archivedPreviewSession.value?.id === result.session.id) {
        clearArchivedPreviewSession();
        activeArchivedPreviewId.value = '';
      }
    }

    showImportDialog.value = false;
    await activateTabById(result.session.id, { connectReal: false });
    scrollToBottom(true);
    message.success(t('webSession.importCodexSessionSuccess'));

    if (result.reused && !result.synced) {
      promptSyncExistingImportedSession(result.session);
    }
  } catch (error) {
    message.error(
      error instanceof Error ? error.message : t('webSession.importCodexSessionFailed')
    );
  } finally {
    importingCodexSessionId.value = '';
  }
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
      permissionLevel:
        (source?.permissionLevel === 'default' && agent === 'claude'
          ? 'elevated'
          : source?.permissionLevel) || draftPermissionLevel.value,
      autoRetryEnabled: source?.autoRetryEnabled === true,
      autoRetryScope:
        source?.autoRetryEnabled === true
          ? source.autoRetryScope
          : webSessionAutoContinueScope.value,
      autoRetryPreset:
        source?.autoRetryEnabled === true
          ? source.autoRetryPreset
          : webSessionAutoContinuePreset.value,
    });
    if (isDraftSession(source)) {
      webSessionStore.moveDraft(props.projectId, source.id, session.id);
      replaceTabIdInNavigationState(source.id, session.id);
      removeDraftSessionRecord(source.id, {
        preserveDraftState: true,
      });
    }
    draftAgent.value = session.agent;
    draftModel.value = session.model;
    draftReasoningEffort.value =
      session.reasoningEffort || defaultReasoningEffortForAgent(session.agent);
    draftWorkflowMode.value = session.workflowMode;
    draftPermissionLevel.value = session.permissionLevel;
    await activateTabById(session.id, { connectReal: false });
    scrollToBottom(true);
    return session;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
    return null;
  }
}

async function handleStartDraftSession(forceAgent?: 'claude' | 'codex') {
  const decision = resolveStartDraftSessionDecision(draftSessions.value, {
    activeDraftId: activeDraftSessionId.value,
    mruIds: tabMruIds.value,
  });
  if (decision.kind === 'reuse') {
    await activateTabById(decision.draft.id, { connectReal: false });
    closeMobileSessionSelector();
    contextMenuSession.value = null;
    expandedTools.value = {};
    autoFollowBottom.value = true;
    scrollToBottom(true);
    updateActiveTabIndicator();
    focusComposer();
    if (decision.shouldNotifyExistingDraft) {
      message.info(t('webSession.existingDraftSessionNotice'));
    }
    return;
  }
  const draft = createDraftSession(forceAgent);
  draftAgent.value = draft.agent;
  draftModel.value = draft.model || defaultModelForAgent(draft.agent);
  draftReasoningEffort.value = draft.reasoningEffort || defaultReasoningEffortForAgent(draft.agent);
  draftWorkflowMode.value = draft.workflowMode;
  draftPermissionLevel.value = draft.permissionLevel;
  closeMobileSessionSelector();
  contextMenuSession.value = null;
  expandedTools.value = {};
  autoFollowBottom.value = true;
  scrollToBottom(true);
  updateActiveTabIndicator();
  focusComposer();
}

async function handleRenameSession(sessionId: string) {
  const session = visibleSessionById.value.get(sessionId);
  if (!session || isDraftSession(session)) {
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
  await webSessionStore.loadArchivedSessions(sidebarVisibleProjectIds.value, {
    reset: true,
    limit: 20,
  });
}

function handleArchiveSession(sessionId: string) {
  if (isSessionArchiving(sessionId)) {
    return;
  }
  const session = visibleSessionById.value.get(sessionId);
  if (!session) {
    return;
  }

  if (isDraftSession(session)) {
    void closeTabById(sessionId, () => {
      removeDraftSessionRecord(sessionId);
    });
    return;
  }
  if (isArchivedPreviewSession(session)) {
    void closeTabById(sessionId, () => {
      clearArchivedPreviewSession();
    });
    return;
  }

  if (confirmBeforeTerminalClose.value) {
    let archiveConfirmDialog: DialogReactive | null = null;
    archiveConfirmDialog = dialog.warning({
      title: t('webSession.confirmCloseTitle'),
      content: () =>
        h('div', { class: 'web-session-close-confirm' }, [
          h('div', { class: 'web-session-close-confirm__message' }, [
            t('webSession.confirmCloseContent', { title: session.title }),
          ]),
        ]),
      positiveText: t('webSession.confirmCloseButton'),
      negativeText: t('common.cancel'),
      onPositiveClick: async () => {
        if (archiveConfirmDialog?.loading) {
          return false;
        }
        if (archiveConfirmDialog) {
          archiveConfirmDialog.loading = true;
        }
        try {
          return await performArchiveSession(session);
        } finally {
          if (archiveConfirmDialog) {
            archiveConfirmDialog.loading = false;
          }
        }
      },
    });
    return;
  }

  void performArchiveSession(session);
}

async function performArchiveSession(session: WebSessionSummary): Promise<boolean> {
  if (isSessionArchiving(session.id)) {
    return false;
  }
  beginSessionArchive(session.id);
  try {
    await closeTabById(session.id, async () => {
      await webSessionStore.archiveSession(session.projectId, session.id);
    });
    await refreshArchivedSidebar();
    return true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
    return false;
  } finally {
    endSessionArchive(session.id);
  }
}

async function performDeleteSession(sessionId: string): Promise<boolean> {
  const session = visibleSessionById.value.get(sessionId);
  if (!session) {
    return false;
  }
  try {
    await closeTabById(sessionId, async () => {
      if (isArchivedPreviewSession(session)) {
        clearArchivedPreviewSession();
        return;
      }
      if (isDraftSession(session)) {
        removeDraftSessionRecord(sessionId);
        return;
      }
      await webSessionStore.deleteSession(session.projectId, sessionId);
    });
    if (!isDraftSession(session) && !isArchivedPreviewSession(session)) {
      await refreshArchivedSidebar();
    }
    return true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
    return false;
  }
}

function openFilePicker() {
  showQuickInputPopover.value = false;
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
  clearComposerTransferError();
  const result = await webSessionStore.uploadAttachments(props.projectId, sessionId, files);
  if (result.attachments.length > 0) {
    insertUploadedImagePlaceholders(result.attachments.length);
  }
  if (result.errors.length === 0) {
    return;
  }

  const detail = result.errors[0]?.message || '';
  showComposerTransferError(detail);
  result.errors.forEach(error => {
    const errorMessage = error.fileName ? `${error.fileName}: ${error.message}` : error.message;
    message.error(errorMessage || t('common.error'));
  });
}

async function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement | null;
  const files = Array.from(target?.files ?? []).filter(file => file.type.startsWith('image/'));
  if (files.length === 0) {
    return;
  }
  try {
    await uploadComposerImages(files);
  } finally {
    if (target) {
      target.value = '';
    }
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

function isComposerTextareaElement(value: unknown): value is HTMLTextAreaElement {
  return (
    typeof value === 'object' &&
    value !== null &&
    'setSelectionRange' in value &&
    typeof (value as HTMLTextAreaElement).setSelectionRange === 'function'
  );
}

function getComposerTextarea() {
  const rawTextarea = composerInputRef.value?.textareaElRef as
    | unknown
    | { value?: unknown }
    | null
    | undefined;

  if (isComposerTextareaElement(rawTextarea)) {
    return rawTextarea as HTMLTextAreaElement;
  }

  const nestedTextarea =
    rawTextarea && typeof rawTextarea === 'object' && 'value' in rawTextarea
      ? rawTextarea.value
      : null;
  if (isComposerTextareaElement(nestedTextarea)) {
    return nestedTextarea;
  }

  return null;
}

async function applyQuickInputText(text: string) {
  const sessionId = currentDraftSessionId.value;
  if (!sessionId) {
    return false;
  }

  webSessionStore.setDraftText(props.projectId, sessionId, text);

  await nextTick();
  composerInputRef.value?.focus();
  const textarea = getComposerTextarea();
  if (textarea) {
    textarea.setSelectionRange(text.length, text.length);
  }
  return true;
}

function insertUploadedImagePlaceholders(uploadedCount: number) {
  const sessionId = currentDraftSessionId.value;
  if (!sessionId || uploadedCount <= 0) {
    return;
  }

  const attachmentCount = webSessionStore.getDraftAttachments(props.projectId, sessionId).length;
  const firstIndex = attachmentCount - uploadedCount + 1;
  if (firstIndex <= 0) {
    return;
  }

  const placeholders = Array.from({ length: uploadedCount }, (_, index) =>
    buildImagePlaceholder(firstIndex + index)
  );
  const textarea = getComposerTextarea();
  const nextComposer = insertImagePlaceholdersAtCursor(
    composerText.value,
    textarea?.selectionStart ?? composerText.value.length,
    textarea?.selectionEnd ?? textarea?.selectionStart ?? composerText.value.length,
    placeholders
  );

  webSessionStore.setDraftText(props.projectId, sessionId, nextComposer.text);

  nextTick(() => {
    composerInputRef.value?.focus();
    const nextTextarea = getComposerTextarea();
    if (nextTextarea) {
      nextTextarea.setSelectionRange(nextComposer.cursor, nextComposer.cursor);
    }
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
  if (restored.projectId === props.projectId) {
    await activateTabById(restored.id);
  } else {
    webSessionStore.setActiveSession(restored.projectId, restored.id);
  }

  return {
    session: restored,
    navigateProjectId: restored.projectId !== props.projectId ? restored.projectId : '',
  };
}

async function continueErroredSession(session: WebSessionSummary) {
  const prepared = await prepareSessionForSend(session);
  await webSessionStore.sendMessage(prepared.session.id, 'continue', []);
  if (prepared.navigateProjectId) {
    projectStore.addRecentProject(prepared.navigateProjectId);
    await router.push(buildProjectRouteLocation(prepared.navigateProjectId, prepared.session.id));
  }
  autoFollowBottom.value = true;
  scrollToBottom(true);
}

async function handleSubmit() {
  const initialSubmitOwnerId = currentDraftSessionId.value;
  if (
    !initialSubmitOwnerId ||
    isSubmittingMessage.value ||
    isRunActive.value ||
    isDraftAttachmentUploading.value ||
    !hasDraftContent.value
  ) {
    return;
  }
  const submitKind = resolveComposerSubmitKind();
  if (submitKind === 'execute_send') {
    if (!ensureSendConflictConfirmed(sendConfirmationSignature.value)) {
      return;
    }
  } else {
    clearSendConflictConfirmation();
  }
  let submitOwnerId = initialSubmitOwnerId;
  beginSessionSubmit(submitOwnerId, submitKind);
  try {
    let session = currentRealSession.value;
    if (!session || isDraftSession(currentSession.value)) {
      const created = await handleCreateSession();
      session = created ?? webSessionStore.getActiveSession(props.projectId);
      if (created?.id && created.id !== submitOwnerId) {
        transferSessionSubmit(submitOwnerId, created.id);
        submitOwnerId = created.id;
      }
    }
    if (!session) {
      return;
    }
    const draftSessionId = currentDraftSessionId.value;
    const draftText = composerText.value;
    const attachments = [...draftAttachments.value];
    const prepared = await prepareSessionForSend(session);
    session = prepared.session;
    if (session.id !== submitOwnerId) {
      transferSessionSubmit(submitOwnerId, session.id);
      submitOwnerId = session.id;
    }
    await webSessionStore.sendMessage(
      session.id,
      draftText,
      attachments.map(item => item.id)
    );
    settingsStore.recordWebSessionRecentInput(draftText);
    void settingsStore.syncWebSessionQuickInputToServer();
    webSessionStore.clearDraft(props.projectId, draftSessionId);
    if (prepared.navigateProjectId) {
      projectStore.addRecentProject(prepared.navigateProjectId);
      await router.push(buildProjectRouteLocation(prepared.navigateProjectId, session.id));
    }
    autoFollowBottom.value = true;
    isMobileComposerExpanded.value = false;
    scrollToBottom(true);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  } finally {
    endSessionSubmit(submitOwnerId);
  }
}

async function handlePreinput(mode: 'redirect' | 'queue') {
  if (!currentRealSession.value || isDraftAttachmentUploading.value || !hasDraftContent.value) {
    return;
  }
  try {
    const draftText = composerText.value;
    const attachments = draftAttachments.value;
    await webSessionStore.sendMessage(
      currentRealSession.value.id,
      draftText,
      attachments.map(item => item.id),
      mode
    );
    settingsStore.recordWebSessionRecentInput(draftText);
    void settingsStore.syncWebSessionQuickInputToServer();
    webSessionStore.clearDraft(props.projectId, currentRealSession.value.id);
    isMobileComposerExpanded.value = false;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function triggerPrimaryComposerAction() {
  if (isDraftAttachmentUploading.value) {
    return;
  }
  if (isRunActive.value) {
    if (canStageDuringRun.value) {
      await handlePreinput('redirect');
    }
    return;
  }
  if (canSend.value) {
    await handleSubmit();
  }
}

function handleComposerFocus() {
  if (!isMobile.value) {
    return;
  }
  isMobileComposerExpanded.value = false;
  emitMobileComposerFocusChange(true);
}

function handleComposerBlur() {
  if (!isMobile.value) {
    return;
  }
  emitMobileComposerFocusChange(false);
}

function handleComposerEnter(event: KeyboardEvent) {
  if (isDraftAttachmentUploading.value) {
    if (hasDraftContent.value) {
      event.preventDefault();
    }
    return;
  }
  if (!hasDraftContent.value) {
    return;
  }
  event.preventDefault();
  void triggerPrimaryComposerAction();
}

function getSendConflictSessionTitle(title: string) {
  const normalized = String(title || '').trim();
  return normalized || t('terminal.untitledSession');
}

function formatSendConflictSessionList(
  sessions: Array<{
    title: string;
  }>
) {
  const titles = sessions.slice(0, 2).map(session => getSendConflictSessionTitle(session.title));
  if (sessions.length <= 2) {
    return titles.join(locale.value === 'zh-CN' ? '、' : ', ');
  }
  return t('webSession.sendConflictListOverflow', {
    first: titles[0],
    second: titles[1],
    remaining: sessions.length - 2,
  });
}

function buildSendConflictWarningBody(
  sessions: Array<{
    title: string;
  }>
) {
  if (sessions.length === 0) {
    return '';
  }
  const formatted = formatSendConflictSessionList(sessions);
  if (sessions.length === 1) {
    return t('webSession.sendConflictWarningBodySingle', { sessions: formatted });
  }
  return t('webSession.sendConflictWarningBodyMultiple', {
    count: sessions.length,
    sessions: formatted,
  });
}

function ensureSendConflictConfirmed(
  signature: string,
  options?: {
    notifyOnBlock?: boolean;
  }
) {
  const confirmation = resolveWebSessionSendConfirmation({
    conflicts: sendConflictSessions.value,
    currentState: sendConfirmationState.value,
    signature,
    now: Date.now(),
    ttlMs: WEB_SESSION_SEND_CONFIRM_TTL_MS,
  });
  setSendConflictConfirmationState(confirmation.nextState);
  if (!confirmation.shouldProceed && options?.notifyOnBlock) {
    const warningBody = buildSendConflictWarningBody(sendConflictSessions.value);
    if (warningBody) {
      message.warning(warningBody);
    }
  }
  return confirmation.shouldProceed;
}

const showSendConflictWarning = computed(
  () => isSendConflictConfirmationArmed.value && sendConflictSessions.value.length > 0
);
const sendConflictWarningBody = computed(() =>
  showSendConflictWarning.value ? buildSendConflictWarningBody(sendConflictSessions.value) : ''
);

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
  if (isSubmittingUserInput.value) {
    return;
  }
  void handleUserInputSubmit();
}

function handleUserInputSingleSelect(questionId: string, value: string | null) {
  const normalizedQuestionId = String(questionId || '').trim();
  if (!normalizedQuestionId) {
    return;
  }
  const normalizedValue = String(value || '').trim();
  userInputSelections.value = {
    ...userInputSelections.value,
    [normalizedQuestionId]: normalizedValue ? [normalizedValue] : [],
  };
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

async function handleRemovePendingInput(pendingId: string) {
  if (!currentRealSession.value) {
    return;
  }
  try {
    await webSessionStore.removePendingInput(currentRealSession.value.id, pendingId);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
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
  if (!currentRealSession.value || isSubmittingMessage.value) {
    return;
  }
  if (
    !ensureSendConflictConfirmed(planImplementConfirmationSignature.value, { notifyOnBlock: true })
  ) {
    return;
  }
  let submitOwnerId = currentRealSession.value.id;
  beginSessionSubmit(submitOwnerId, 'execute_plan');
  try {
    const prepared = await prepareSessionForSend(currentRealSession.value);
    const targetSession = prepared.session;
    if (targetSession.id !== submitOwnerId) {
      transferSessionSubmit(submitOwnerId, targetSession.id);
      submitOwnerId = targetSession.id;
    }

    if (targetSession.workflowMode === 'plan') {
      await webSessionStore.updateWorkflowMode(targetSession.id, 'default');
    }

    const answered = await answerInlinePlanChoice('execute');
    if (!answered) {
      await webSessionStore.sendMessage(targetSession.id, 'Implement the plan.', []);
    }

    if (prepared.navigateProjectId) {
      projectStore.addRecentProject(prepared.navigateProjectId);
      await router.push(buildProjectRouteLocation(prepared.navigateProjectId, targetSession.id));
    }
    autoFollowBottom.value = true;
    scrollToBottom(true);
  } catch (error) {
    message.error(formatSessionInteractionError(error));
  } finally {
    endSessionSubmit(submitOwnerId);
  }
}

async function handlePlanCardCancel() {
  const toolId = latestPlanToolId.value;
  setPlanActionsDismissed(toolId, true);
  focusComposer();
}

async function handleUserInputSubmit() {
  if (!currentRealSession.value || !pendingUserInput.value || isSubmittingUserInput.value) {
    return;
  }
  const sessionId = currentRealSession.value.id;
  const request = pendingUserInput.value;
  if (request.stale) {
    message.info(request.recoveryMessage || t('webSession.recoveredActionExpired'));
    return;
  }
  const answers = buildUserInputAnswers();
  if (!answers) {
    return;
  }
  if (hasMissingWebSessionUserInputAnswers(request.questions, answers)) {
    message.warning(t('webSession.userInputAnswerRequired'));
    return;
  }
  const submitOwnerId = buildWebSessionUserInputSubmitOwnerId(sessionId, request.itemId);
  if (!submitOwnerId) {
    return;
  }
  beginUserInputSubmit(submitOwnerId);
  let answered = false;
  try {
    await webSessionStore.answerUserInput(sessionId, request.itemId, answers);
    answered = true;
  } catch (error) {
    message.error(formatSessionInteractionError(error));
  } finally {
    if (!answered || currentUserInputSubmitOwnerId.value !== submitOwnerId) {
      endUserInputSubmit(submitOwnerId);
    }
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

async function handleLiveCardClick() {
  if (shouldAutoContinueOnLiveCardClick.value && currentRealSession.value) {
    liveCardContinuePending.value = true;
    try {
      await continueErroredSession(currentRealSession.value);
      return;
    } catch (error) {
      message.error(formatSessionInteractionError(error));
    } finally {
      liveCardContinuePending.value = false;
    }
  }
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

function getClickedTimelineAnchor(
  target: EventTarget | null,
  currentTarget: EventTarget | null
): HTMLAnchorElement | null {
  if (!(target instanceof Element) || !(currentTarget instanceof HTMLElement)) {
    return null;
  }

  const anchor = target.closest('a[href]');
  if (!(anchor instanceof HTMLAnchorElement) || !currentTarget.contains(anchor)) {
    return null;
  }

  return anchor.closest('.chat-markdown') ? anchor : null;
}

function handleTimelineLinkClick(event: MouseEvent) {
  if (event.defaultPrevented || typeof window === 'undefined') {
    return;
  }

  const anchor = getClickedTimelineAnchor(event.target, event.currentTarget);
  if (!anchor) {
    return;
  }

  event.preventDefault();
  const href = resolveNavigableHref(anchor.getAttribute('href') ?? '', window.location.href);
  if (!href) {
    message.warning(t('common.invalidLink'));
    return;
  }

  dialog.warning({
    title: t('common.openLinkTitle'),
    content: () =>
      h('div', { class: 'web-session-close-confirm' }, [
        h('div', { class: 'web-session-close-confirm__message' }, [t('common.openLinkMessage')]),
        h('code', { class: 'web-session-close-confirm__href' }, href),
      ]),
    positiveText: t('common.openInNewTab'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => {
      const opened = window.open(href, '_blank', 'noopener,noreferrer');
      if (!opened) {
        message.error(t('common.openLinkFailed'));
      }
    },
  });
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
      !isMobile.value && activeTabSessionId.value
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

function shouldShowSessionWorkflowPlanBadge(
  session: Pick<WebSessionSummary, 'workflowMode'> | null | undefined
) {
  return session?.workflowMode === 'plan';
}

function createTabProps(session: (typeof sessions.value)[number]): HTMLAttributes {
  const isActive = activeTabSessionId.value === session.id;
  const theme = activeTheme.value;
  const preset = getPresetById(currentPresetId.value);
  const hideHeaderBorder = theme.terminalHeaderBorder === false;
  const props: HTMLAttributes = {
    onContextmenu: (event: MouseEvent) => handleTabContextMenu(event, session),
  };
  const classes: string[] = [];

  if (isSessionArchiving(session.id)) {
    classes.push('is-archiving');
  }
  if (isArchivedPreviewSession(session)) {
    classes.push('is-tab-drag-locked');
  }

  if (usesSessionPlanApprovalTone(session)) {
    classes.push('has-unviewed-plan-approval');
    if (isActive && hideHeaderBorder) {
      props.style = {
        borderBottom: 'none',
      };
    }
  } else if (usesSessionApprovalTone(session)) {
    classes.push('has-unviewed-approval');
    if (isActive && hideHeaderBorder) {
      props.style = {
        borderBottom: 'none',
      };
    }
  } else if (usesSessionCompletionTone(session)) {
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
  if (shouldShowSessionWorkflowPlanBadge(session)) {
    props.class = [props.class, 'has-workflow-plan-badge'].filter(Boolean).join(' ');
  }
  return props;
}

function getSessionDisplayState(session: SessionTab): WebSessionDisplayState {
  return resolveWebSessionDisplayState({
    isDraft: isDraftSession(session),
    hasUnread: hasSessionUnread(session),
    status: session.status,
    syncState: session.syncState,
    livePhase: isDraftSession(session) ? null : webSessionStore.getLiveState(session.id).phase,
    assistantState: session.assistantState,
  });
}

function getSessionLabelState(session: (typeof sessions.value)[number]) {
  return getSessionDisplayState(session).assistantStateClass;
}

function getSessionVisualInput(session: (typeof sessions.value)[number]) {
  if (isDraftSession(session)) {
    return null;
  }
  return {
    phase: webSessionStore.getLiveState(session.id).phase,
    hasUnread: hasSessionUnread(session),
    status: session.status,
  } as const;
}

function getSessionStatusLabel(session: (typeof sessions.value)[number]) {
  const labelKey = getSessionDisplayState(session).statusLabelKey;
  return labelKey ? t(labelKey) : '';
}

function getSessionPillStateClass(session: (typeof sessions.value)[number]) {
  return getSessionDisplayState(session).pillStateClass;
}

function getSessionAttentionStateClass(session: (typeof sessions.value)[number]) {
  return getSessionDisplayState(session).attentionStateClass;
}

function getSessionStatusEmoji(session: (typeof sessions.value)[number]) {
  return getSessionDisplayState(session).statusEmoji;
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
  const suffix = session.syncState === 'error' && session.syncError ? session.syncError : '';
  const base = label ? `${agentName} · ${label}` : agentName;
  return suffix ? `${base} · ${suffix}` : base;
}

function getSessionHoverTimeText(
  session: Pick<WebSessionSummary, 'updatedAt' | 'lastMessageAt' | 'createdAt'> | null | undefined
) {
  if (!session) {
    return '';
  }
  const timestamp = parseTimestamp(session.updatedAt || session.lastMessageAt || session.createdAt);
  return timestamp > 0 ? formatDateTime(timestamp) : '';
}

function joinSessionHoverParts(parts: Array<string | null | undefined>) {
  return parts
    .map(part => String(part ?? '').trim())
    .filter(Boolean)
    .join(' · ');
}

function getSidebarSessionSubtitle(item: CrossProjectSessionItem) {
  if (!showSidebarStatusText.value) {
    return '';
  }
  return getSessionStatusLabel(item.session);
}

function getSidebarSessionTitle(item: CrossProjectSessionItem) {
  return joinSessionHoverParts([
    item.projectName,
    item.session.title,
    getSidebarSessionSubtitle(item),
    getSessionHoverTimeText(item.session),
  ]);
}

function getSidebarSessionAccentColor(item: CrossProjectSessionItem) {
  const visualInput = getSessionVisualInput(item.session);
  const tone = visualInput ? getWebSessionSidebarTone(visualInput) : 'default';
  switch (tone) {
    case 'working':
      return '#8b5cf6';
    case 'approval':
      return approvalColors.value.accent;
    case 'plan_approval':
      return planApprovalColors.value.accent;
    case 'completion':
      return '#10b981';
    case 'idle':
      return '#9ca3af';
    case 'error':
      return '#f04438';
    default:
      return 'rgba(15, 23, 42, 0.08)';
  }
}

function getSidebarSessionClasses(item: CrossProjectSessionItem): string[] {
  const visualInput = getSessionVisualInput(item.session);
  const tone = visualInput ? getWebSessionSidebarTone(visualInput) : 'default';
  switch (tone) {
    case 'working':
      return ['session-sidebar-working'];
    case 'approval':
      return ['session-sidebar-approval'];
    case 'plan_approval':
      return ['session-sidebar-plan-approval'];
    case 'completion':
      return ['session-sidebar-completion'];
    case 'idle':
      return ['session-sidebar-idle'];
    case 'error':
      return ['session-sidebar-error'];
    default:
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
  return getSessionDisplayState(session).showStatusDot;
}

function getSessionStatusDotClass(session: (typeof sessions.value)[number]) {
  return getSessionDisplayState(session).statusDotClass ?? session.status;
}

function getSessionTabTone(session: (typeof sessions.value)[number]) {
  const visualInput = getSessionVisualInput(session);
  return visualInput ? getWebSessionTabTone(visualInput) : 'default';
}

function usesSessionApprovalTone(session: (typeof sessions.value)[number]) {
  return getSessionTabTone(session) === 'approval';
}

function usesSessionPlanApprovalTone(session: (typeof sessions.value)[number]) {
  return getSessionTabTone(session) === 'plan_approval';
}

function usesSessionCompletionTone(session: (typeof sessions.value)[number]) {
  return getSessionTabTone(session) === 'completion';
}

function handleTabContextMenu(event: MouseEvent, session: (typeof sessions.value)[number]) {
  event.preventDefault();
  event.stopPropagation();
  contextMenuSession.value = session;
  contextMenuX.value = event.clientX;
  contextMenuY.value = event.clientY;
}

async function handleContextMenuSelect(key: string | number) {
  const session = contextMenuSession.value;
  contextMenuSession.value = null;
  await handleSessionActionSelect(String(key), session);
}

function syncModeLabel(mode: 'fast' | 'deep') {
  return mode === 'deep'
    ? t('settings.webSessionSyncModeDeep')
    : t('settings.webSessionSyncModeFast');
}

function confirmSyncSession(session: WebSessionSummary, mode: 'fast' | 'deep') {
  const clearExisting = ref(false);
  const isClaude = session.agent === 'claude';
  dialog.warning({
    title: t('webSession.syncConfirmTitle'),
    content: () =>
      h('div', { class: 'web-session-close-confirm' }, [
        h('div', { class: 'web-session-close-confirm__message' }, [
          isClaude
            ? t('webSession.syncConfirmContentClaude')
            : t('webSession.syncConfirmContent', { mode: syncModeLabel(mode) }),
        ]),
        h(
          'div',
          { class: 'web-session-close-confirm__checkbox' },
          h(
            NCheckbox,
            {
              checked: clearExisting.value,
              'onUpdate:checked': (value: boolean) => {
                clearExisting.value = value;
              },
            },
            { default: () => t('webSession.syncClearExisting') }
          )
        ),
      ]),
    positiveText: isClaude
      ? t('webSession.syncSessionAction')
      : mode === 'deep'
        ? t('webSession.deepSyncFromTerminal')
        : t('webSession.syncFromTerminal'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => handleSyncSession(session.id, mode, clearExisting.value),
  });
}

async function handleSyncSession(
  sessionId: string,
  mode: 'fast' | 'deep' = 'fast',
  clearExisting = false
) {
  const session = visibleSessionById.value.get(sessionId);
  if (!session) {
    return;
  }
  try {
    await webSessionStore.syncSession(session.projectId, sessionId, mode, clearExisting, {
      rememberActive: !isArchivedPreviewSession(session),
    });
    syncArchivedPreviewSessionSummary(sessionId);
    message.success(
      mode === 'deep' ? t('webSession.deepSyncSuccess') : t('webSession.syncSuccess')
    );
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('webSession.syncFailed'));
  }
}

function handleMobileTabSelect(_key: string | number, option: DropdownOption) {
  const mobileOption = option as MobileTabDropdownOption;
  requestMobileViewForBottomNavSelector();
  if (isMobileTabActionOption(mobileOption)) {
    closeMobileSessionSelector();
    void handleStartDraftSession();
    return;
  }
  if (!isMobileTabSessionOption(mobileOption)) {
    return;
  }
  if (mobileOption.section === 'archived') {
    closeMobileSessionSelector();
    if (archivedPreviewSession.value?.id === mobileOption.session.id) {
      activeArchivedPreviewId.value = mobileOption.session.id;
      scrollToBottom(true);
      return;
    }
    void openArchivedPreviewSession(mobileOption.session).then(
      () => {
        scrollToBottom(true);
      },
      error => {
        clearArchivedPreviewSession();
        message.error(error instanceof Error ? error.message : t('common.error'));
      }
    );
    return;
  }
  void handleSessionSelect(String(mobileOption.session.id));
}

function goToPrevSession() {
  if (!hasPrevSession.value) {
    return;
  }
  const session = mobileNavigationSessions.value[currentSessionIndex.value - 1];
  if (session) {
    if (session.archivedAt) {
      void openArchivedPreviewSession(session).then(
        () => {
          scrollToBottom(true);
        },
        error => {
          clearArchivedPreviewSession();
          message.error(error instanceof Error ? error.message : t('common.error'));
        }
      );
      return;
    }
    void handleSessionSelect(session.id);
  }
}

function goToNextSession() {
  if (!hasNextSession.value) {
    return;
  }
  const session = mobileNavigationSessions.value[currentSessionIndex.value + 1];
  if (session) {
    if (session.archivedAt) {
      void openArchivedPreviewSession(session).then(
        () => {
          scrollToBottom(true);
        },
        error => {
          clearArchivedPreviewSession();
          message.error(error instanceof Error ? error.message : t('common.error'));
        }
      );
      return;
    }
    void handleSessionSelect(session.id);
  }
}

function setupTabSorting() {
  if (isMobile.value) {
    destroyTabSorting();
    return;
  }
  const container = tabsContainerRef.value;
  if (!container || sessions.value.length <= 1) {
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
      tabDragSortable.value.option('disabled', sessions.value.length <= 1);
      return;
    }
    destroyTabSorting();
  }
  tabDragSortable.value = Sortable.create(wrapper, {
    animation: 150,
    direction: 'horizontal',
    draggable: '.n-tabs-tab-wrapper',
    handle: '.n-tabs-tab:not(.is-tab-drag-locked)',
    filter: '.n-tabs-tab__close',
    preventOnFilter: false,
    ghostClass: 'web-session-tab-ghost',
    chosenClass: 'web-session-tab-chosen',
    dragClass: 'web-session-tab-dragging',
    onEnd: handleTabDragEnd,
  });
  tabDragSortable.value.option('disabled', sessions.value.length <= 1);
}

function destroyTabSorting() {
  if (tabDragSortable.value) {
    tabDragSortable.value.destroy();
    tabDragSortable.value = null;
  }
}

function handleTabDragEnd(event: SortableEvent) {
  const fromIndex = event.oldDraggableIndex ?? event.oldIndex ?? -1;
  const toIndex = event.newDraggableIndex ?? event.newIndex ?? -1;
  if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) {
    return;
  }
  const previousOrderIds = [...tabOrderIds.value];
  const previousMruIds = [...tabMruIds.value];
  const reorderedSessions = [...sessions.value];
  const [movingSession] = reorderedSessions.splice(fromIndex, 1);
  if (!movingSession) {
    return;
  }
  reorderedSessions.splice(toIndex, 0, movingSession);
  replaceTabNavigationState(
    reorderedSessions.map(session => session.id),
    previousMruIds
  );
  if (isArchivedPreviewSession(movingSession)) {
    replaceTabNavigationState(previousOrderIds, previousMruIds);
    nextTick(() => {
      updateActiveTabIndicator();
    });
    return;
  }
  if (isDraftSession(movingSession)) {
    nextTick(() => {
      updateActiveTabIndicator();
    });
    return;
  }
  const reorderedRealSessions = reorderedSessions.filter(
    session => !isDraftSession(session) && !isArchivedPreviewSession(session)
  );
  const realIndex = reorderedRealSessions.findIndex(session => session.id === movingSession.id);
  const previousRealSessionId = reorderedRealSessions[realIndex - 1]?.id ?? '';
  const nextRealSessionId = reorderedRealSessions[realIndex + 1]?.id ?? '';
  void webSessionStore
    .moveSession(props.projectId, movingSession.id, previousRealSessionId, nextRealSessionId)
    .catch(error => {
      replaceTabNavigationState(previousOrderIds, previousMruIds);
      message.error(error instanceof Error ? error.message : t('common.error'));
    });
  nextTick(() => {
    updateActiveTabIndicator();
  });
}

async function loadCodexRuntimeConfig() {
  try {
    codexRuntimeConfig.value = await webSessionApi.runtimeConfig();
  } catch (error) {
    codexRuntimeConfig.value = null;
    console.warn('[Web Session] Failed to load Codex runtime config', error);
  } finally {
    codexRuntimeConfigReady.value = true;
  }
}

watch(
  () => props.projectId,
  projectId => {
    clearSendConflictConfirmation();
    if (projectId) {
      void initializeProjectSessions(projectId);
    }
  },
  { immediate: true }
);

watch(
  () => routeWebSessionId.value,
  sessionId => {
    if (!sessionId) {
      pendingRouteActivationSessionId.value = '';
      return;
    }
    if (!props.projectId) {
      return;
    }
    if (isProjectSessionInitializing.value) {
      pendingRouteActivationSessionId.value = sessionId;
      return;
    }
    const session = currentSession.value;
    if (
      session &&
      !isDraftSession(session) &&
      session.id === sessionId &&
      session.projectId === props.projectId
    ) {
      pendingRouteActivationSessionId.value = '';
      return;
    }
    pendingRouteActivationSessionId.value = sessionId;
    void activateSessionFromRoute(props.projectId, sessionId).catch(error => {
      console.error('[Web Session] Failed to activate session from route', error);
    });
  }
);

watch([sendConfirmationSignature, planImplementConfirmationSignature], signatures => {
  if (!sendConfirmationState.value) {
    return;
  }
  const activeSignatures = signatures.filter(Boolean);
  if (!activeSignatures.includes(sendConfirmationState.value.signature)) {
    clearSendConflictConfirmation();
  }
});

watch(
  () => sessions.value.map(session => session.id).join('|'),
  () => {
    if (isProjectSessionInitializing.value) {
      return;
    }
    syncTabNavigationState();
  }
);

watch(
  () => activeTabSessionId.value,
  sessionId => {
    if (
      !sessionId ||
      isProjectSessionInitializing.value ||
      !sessions.value.some(session => session.id === sessionId) ||
      tabMruIds.value[0] === sessionId
    ) {
      return;
    }
    rememberTabVisit(sessionId);
  }
);

watch(
  sidebarVisibleProjectIds,
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
  sidebarVisibleProjectIds,
  projectIds => {
    void ensureArchivedScopeLoaded(projectIds, 20).catch(error => {
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
    streamingMarkdownController.clear();
    pendingHistoryAnchor.value = null;
    handleCommandExecutionDetailVisibilityChange(false);
    rawTimelineBlocks.value = {};
    activeRawTimelineBlockKey.value = '';
    syncMobileSessionCategoryToCurrentSession();
    if (!sessionId) {
      closeMobileSessionSelector();
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
  [() => props.isActive, () => currentRealSession.value?.id ?? ''],
  ([isActive, sessionId]) => {
    webSessionStore.setEventSessionFocus(isActive ? sessionId : '');
  },
  { immediate: true }
);

watch(
  visibleRawTimelineBlockKeys,
  keys => {
    activeRawTimelineBlockKey.value = pruneActiveTimelineRawBlockKey(
      activeRawTimelineBlockKey.value,
      keys
    );
  },
  { immediate: true }
);

watch(
  streamingMarkdownTargets,
  targets => {
    streamingMarkdownController.sync(targets);
  },
  { immediate: true, deep: true }
);

watch(
  () => webSessionStreamingMarkdownThrottleMs.value,
  value => {
    streamingMarkdownController.setDelayMs(value);
  },
  { immediate: true }
);

useEventListener(typeof document !== 'undefined' ? document : undefined, 'pointerdown', event => {
  const target = event.target;
  const clickedInsideRawCard =
    target instanceof Element && Boolean(target.closest('[data-raw-toggle-card]'));
  if (shouldClearActiveTimelineRawBlockKey(activeRawTimelineBlockKey.value, clickedInsideRawCard)) {
    activeRawTimelineBlockKey.value = '';
  }
});

useEventListener(typeof window !== 'undefined' ? window : undefined, 'resize', () => {
  if (!showMobileTabSelector.value) {
    return;
  }
  closeMobileSessionSelector();
});

watch(
  [() => currentSession.value, routeWorkspaceTab, routeWebSessionId],
  ([session, workspaceTab]) => {
    const sessionIsDraft = Boolean(session && isDraftSession(session));
    if (
      shouldPreserveWebSessionRouteSessionId({
        workspaceTab,
        pendingRouteSessionId: pendingRouteActivationSessionId.value,
        currentProjectId: props.projectId,
        currentSessionId: session?.id,
        currentSessionProjectId: session && !sessionIsDraft ? session.projectId : '',
        currentSessionIsDraft: sessionIsDraft,
      })
    ) {
      return;
    }
    const nextRouteSessionId =
      workspaceTab === 'web' && session && !sessionIsDraft && session.projectId === props.projectId
        ? session.id
        : '';
    void syncWebSessionRouteSessionId(nextRouteSessionId).catch(error => {
      console.error('[Web Session] Failed to sync route session id', error);
    });
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
  [() => webSessionAutoContinueScope.value, () => webSessionAutoContinuePreset.value],
  ([scope, preset]) => {
    if (!isDraftSession(currentSession.value)) {
      return;
    }
    if (
      currentSession.value.autoRetryScope === scope &&
      currentSession.value.autoRetryPreset === preset
    ) {
      return;
    }
    updateActiveDraftSession(current => ({
      ...current,
      autoRetryScope: scope,
      autoRetryPreset: preset,
      updatedAt: new Date().toISOString(),
    }));
  }
);

watch(
  [
    () => currentRealSession.value?.id ?? '',
    () => currentRealSession.value?.autoRetryEnabled === true,
    () => webSessionAutoContinueScope.value,
    () => webSessionAutoContinuePreset.value,
  ],
  ([sessionId, enabled, scope, preset]) => {
    const session = currentRealSession.value;
    if (!sessionId || !session || !enabled) {
      return;
    }
    if (session.autoRetryScope === scope && session.autoRetryPreset === preset) {
      return;
    }
    void webSessionStore
      .updateAutoRetry(sessionId, {
        enabled: true,
        scope,
        preset,
      })
      .catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
  }
);

watch(
  () =>
    sessions.value
      .map(
        session =>
          `${session.id}:${session.orderIndex}:${session.status}:${session.hasUnread}:${getSessionLabelState(session)}:${getSessionPillStateClass(session)}`
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
      closeMobileSessionSelector();
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

watch(currentDraftSessionId, () => {
  clearComposerTransferError();
  clearSendConflictConfirmation();
  showQuickInputPopover.value = false;
});

watch(
  () => currentSession.value?.id,
  () => {
    showQuickInputPopover.value = false;
    if (isMobile.value) {
      isMobileComposerExpanded.value = false;
    }
  }
);

watch(
  () => isMobile.value,
  mobile => {
    isMobileComposerExpanded.value = false;
    if (!mobile) {
      emitMobileComposerFocusChange(false);
    }
  },
  { immediate: true }
);

watch(
  () => props.isActive,
  active => {
    if (!active) {
      emitMobileComposerFocusChange(false);
      return;
    }
    if (!isDocumentVisible() || !currentRealSession.value?.id) {
      return;
    }
    void refreshWebSessionCatchUp('panel-active');
  }
);

watch(
  () => webSessionStore.eventRecoveryVersion,
  version => {
    if (version <= 0 || !props.isActive || !isDocumentVisible()) {
      return;
    }
    void refreshWebSessionCatchUp('event-stream-recovered');
  }
);

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
  void settingsStore.loadWebSessionQuickInput();
  void loadCodexRuntimeConfig();
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
  realSessionSnapshotLoadController.cancel();
  streamingMarkdownController.clear();
  if (liveStateClockTimer != null) {
    window.clearInterval(liveStateClockTimer);
    liveStateClockTimer = null;
  }
  clearUserInputSlowHintTimer();
  userInputSubmitStateByOwnerId.value = {};
  userInputSlowStateByOwnerId.value = {};
  emitMobileComposerFocusChange(false);
  clearComposerTransferError();
  clearSendConflictConfirmation();
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

defineExpose({
  closeMobileSessionSelector,
  openMobileSessionSelectorFromElement,
});
</script>

<style scoped>
.web-session-panel {
  --web-session-approval-bg: rgba(247, 144, 9, 0.25);
  --web-session-approval-border: rgba(247, 144, 9, 0.5);
  --web-session-plan-approval-bg: rgba(6, 182, 212, 0.14);
  --web-session-plan-approval-border: rgba(6, 182, 212, 0.3);
  box-sizing: border-box;
  height: 100%;
  padding-bottom: var(--workspace-mobile-websession-inset, 0px);
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
  position: relative;
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

.tabs-container :deep(.n-tabs-tab.is-tab-drag-locked),
.tabs-container :deep(.n-tabs-tab.is-tab-drag-locked:active) {
  cursor: default;
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

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-workflow-plan-badge) {
  position: relative;
  overflow: visible;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-workflow-plan-badge)::before {
  content: '';
  position: absolute;
  top: 8px;
  left: -1px;
  z-index: 2;
  width: 14px;
  height: 2px;
  background: #0ea5e9;
  transform: rotate(54deg);
  transform-origin: center center;
  pointer-events: none;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-workflow-plan-badge)::after {
  content: '';
  position: absolute;
  top: 8px;
  left: -1px;
  z-index: 2;
  width: 14px;
  height: 2px;
  background: #0ea5e9;
  transform: rotate(-54deg);
  transform-origin: center center;
  pointer-events: none;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.n-tabs-tab--active) {
  background-color: var(--kanban-terminal-tab-active-bg, #e8e8e8) !important;
  color: var(--n-tab-text-color-active);
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.is-archiving) {
  cursor: wait;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.is-archiving .n-tabs-tab__close) {
  display: none;
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

.tab-action-spinner,
.session-sidebar-spinner {
  width: 11px;
  height: 11px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 1.75px solid color-mix(in srgb, var(--n-primary-color) 24%, transparent);
  border-top-color: var(--n-primary-color);
  animation: web-session-action-spin 0.72s linear infinite;
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

.ai-status-pill.state-approval,
.ai-status-pill.state-waiting_approval {
  background-color: #fed7aa;
  color: #f79009;
}

.ai-status-pill.state-waiting_plan_approval {
  background-color: rgba(34, 211, 238, 0.14);
  color: #0891b2;
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

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-unviewed-approval) {
  background-color: var(--web-session-approval-tab-bg, rgba(247, 144, 9, 0.16)) !important;
  border-color: var(--web-session-approval-tab-border, rgba(247, 144, 9, 0.42)) !important;
}

.panel-header
  :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-unviewed-approval.n-tabs-tab--active) {
  background-color: var(
    --web-session-approval-tab-active-bg,
    color-mix(
      in srgb,
      var(--web-session-approval-tab-bg, rgba(247, 144, 9, 0.16)) 78%,
      var(--app-surface-color, #fff) 22%
    )
  ) !important;
  border-color: var(
    --web-session-approval-tab-active-border,
    color-mix(
      in srgb,
      var(--web-session-approval-tab-border, rgba(247, 144, 9, 0.42)) 88%,
      transparent 12%
    )
  ) !important;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-unviewed-plan-approval) {
  background-color: var(--web-session-plan-approval-bg, rgba(6, 182, 212, 0.14)) !important;
  border-color: var(--web-session-plan-approval-border, rgba(6, 182, 212, 0.3)) !important;
}

.panel-header
  :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.has-unviewed-plan-approval.n-tabs-tab--active) {
  background-color: color-mix(
    in srgb,
    var(--web-session-plan-approval-bg, rgba(6, 182, 212, 0.14)) 78%,
    var(--app-surface-color, #fff) 22%
  ) !important;
  border-color: color-mix(
    in srgb,
    var(--web-session-plan-approval-border, rgba(6, 182, 212, 0.3)) 88%,
    transparent 12%
  ) !important;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  padding-right: 4px;
  padding-bottom: 4px;
  margin-left: auto;
}

.mobile-header-action-menu {
  display: flex;
  align-items: center;
}

.new-session-button {
  min-width: 44px;
  padding-left: 0 !important;
  padding-right: 0 !important;
}

.desktop-header-icon-button {
  width: 30px;
  min-width: 30px;
  height: 34px;
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

.mobile-tab-trigger-main {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
}

.mobile-tab-title {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.mobile-tab-trigger-status {
  flex-shrink: 0;
  min-width: 0;
  max-width: 92px;
  padding: 0 6px;
  height: 18px;
  font-size: 10px;
  line-height: 1;
}

.mobile-tab-trigger-status-text {
  display: block;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.mobile-tab-trigger-plan-badge {
  position: relative;
  flex-shrink: 0;
  width: 12px;
  height: 12px;
}

.mobile-tab-trigger-plan-badge::before,
.mobile-tab-trigger-plan-badge::after {
  content: '';
  position: absolute;
  top: 5px;
  left: -1px;
  width: 14px;
  height: 2px;
  background: #0ea5e9;
  border-radius: 999px;
  transform-origin: center center;
}

.mobile-tab-trigger-plan-badge::before {
  transform: rotate(54deg);
}

.mobile-tab-trigger-plan-badge::after {
  transform: rotate(-54deg);
}

.mobile-tab-arrow {
  transition: transform 0.2s ease;
}

.mobile-tab-arrow.is-open {
  transform: rotate(180deg);
}

:global(.web-session-mobile-dropdown) {
  box-sizing: border-box;
  width: 100%;
  max-width: calc(100vw - 24px);
  max-height: min(72vh, 460px);
  overflow-y: auto;
  overscroll-behavior: contain;
  -webkit-overflow-scrolling: touch;
  transform-origin: var(--mobile-tab-dropdown-origin, left top);
}

:global(.web-session-mobile-dropdown .n-dropdown-option-body) {
  min-height: var(--n-option-height);
  height: auto;
  line-height: normal;
  align-items: center;
  padding-top: 4px;
  padding-bottom: 4px;
}

:global(.web-session-mobile-dropdown .n-dropdown-option-body__label) {
  min-width: 0;
  width: 100%;
  white-space: normal;
}

:global(.web-session-mobile-dropdown .mobile-tab-category-header-render) {
  position: sticky;
  top: -4px;
  z-index: 2;
  padding: 0 6px;
  background: var(--n-color, var(--app-surface-color, #fff));
}

:global(.web-session-mobile-dropdown .mobile-tab-category-header) {
  display: flex;
  gap: 0;
  padding: 2px;
  border: 1px solid var(--n-border-color);
  border-radius: 10px;
  background: #f3f4f6;
}

:global(.web-session-mobile-dropdown .mobile-tab-category-button) {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--app-text-color, var(--n-text-color));
  border-radius: 8px;
  min-height: 32px;
  padding: 0 10px;
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
}

:global(.web-session-mobile-dropdown .mobile-tab-category-button.is-active) {
  background: var(--app-surface-color, #fff);
  color: var(--n-primary-color);
}

:global(.web-session-mobile-dropdown .mobile-tab-category-button-label) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.web-session-mobile-dropdown .mobile-tab-category-button-count) {
  flex-shrink: 0;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(148, 163, 184, 0.16);
  color: inherit;
  font-size: 11px;
  line-height: 1;
}

:global(.web-session-mobile-dropdown .mobile-tab-empty-render) {
  padding: 8px 2px 2px;
}

:global(.web-session-mobile-dropdown .mobile-tab-empty-state) {
  padding: 12px 10px;
  border-radius: 10px;
  color: var(--n-text-color-3);
  background: color-mix(in srgb, var(--n-border-color) 10%, transparent);
  font-size: 12px;
  text-align: center;
}

:global(.web-session-mobile-dropdown .mobile-tab-load-more-render) {
  padding: 6px 2px 2px;
}

:global(.web-session-mobile-dropdown .mobile-tab-load-more) {
  width: 100%;
  border: 1px solid var(--n-border-color);
  border-radius: 10px;
  background: color-mix(in srgb, var(--n-primary-color) 8%, transparent);
  color: var(--n-primary-color);
  min-height: 36px;
  font-size: 12px;
  font-weight: 600;
}

:global(.web-session-mobile-dropdown .mobile-tab-load-more.is-loading) {
  cursor: progress;
  opacity: 0.82;
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-render) {
  position: sticky;
  bottom: -4px;
  z-index: 2;
  height: 0;
  padding: 0;
  pointer-events: none;
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-anchor) {
  position: relative;
  width: 100%;
  height: 0;
  pointer-events: none;
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-button) {
  position: relative;
  position: absolute;
  right: 6px;
  bottom: 6px;
  width: 32px;
  height: 32px;
  border: 1px solid color-mix(in srgb, var(--n-border-color) 88%, rgba(15, 23, 42, 0.08));
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 92%, var(--n-primary-color) 8%);
  color: color-mix(in srgb, var(--n-text-color-2) 84%, var(--n-primary-color) 16%);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.12);
  cursor: pointer;
  transition:
    transform 0.18s ease,
    box-shadow 0.18s ease,
    border-color 0.18s ease,
    background-color 0.18s ease,
    color 0.18s ease;
  pointer-events: auto;
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-button:hover) {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--n-primary-color) 24%, var(--n-border-color));
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.16);
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-button.is-current) {
  border-color: color-mix(in srgb, var(--n-primary-color) 36%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-primary-color) 14%, var(--app-surface-color, #fff));
  color: color-mix(in srgb, var(--n-primary-color) 72%, var(--n-text-color-1));
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-button:focus-visible) {
  outline: none;
  box-shadow:
    0 0 0 3px color-mix(in srgb, var(--n-primary-color) 18%, transparent),
    0 12px 28px rgba(15, 23, 42, 0.16);
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

:global(.web-session-mobile-dropdown .mobile-tab-scope-toggle-indicator) {
  position: absolute;
  right: 7px;
  bottom: 7px;
  width: 7px;
  height: 7px;
  border-radius: 999px;
  border: 1.5px solid color-mix(in srgb, var(--app-surface-color, #fff) 80%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-text-color-2) 30%, transparent);
  transition:
    background-color 0.18s ease,
    transform 0.18s ease;
}

:global(
  .web-session-mobile-dropdown
    .mobile-tab-scope-toggle-button.is-current
    .mobile-tab-scope-toggle-indicator
) {
  background: var(--n-primary-color);
  transform: scale(1.08);
}

:global(
  .web-session-mobile-dropdown .web-session-mobile-option.is-action > .n-dropdown-option-body
) {
  padding-top: 8px;
  padding-bottom: 8px;
}

:global(.web-session-mobile-dropdown .mobile-tab-action-option-body) {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  width: 100%;
  padding: 0 8px;
  color: var(--n-primary-color);
  font-size: 13px;
  font-weight: 700;
}

:global(.web-session-mobile-dropdown .mobile-tab-action-option-icon) {
  width: 28px;
  height: 28px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--n-primary-color) 12%, transparent);
  color: var(--n-primary-color);
  flex-shrink: 0;
}

:global(.web-session-mobile-dropdown .mobile-tab-action-option-icon svg) {
  width: 16px;
  height: 16px;
}

:global(.web-session-mobile-dropdown .mobile-tab-action-option-title) {
  min-width: 0;
  color: inherit;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-body) {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  column-gap: 10px;
  min-width: 0;
  width: 100%;
  padding: 0 8px;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-shell) {
  position: relative;
  display: inline-flex;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge) {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-working) {
  background: #eadffc;
  color: #7c3aed;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-approval),
:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-waiting_approval) {
  background: #fed7aa;
  color: #f79009;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-waiting_plan_approval) {
  background: rgba(34, 211, 238, 0.14);
  color: #0891b2;
  border-color: rgba(6, 182, 212, 0.18);
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-completion) {
  background: rgba(16, 185, 129, 0.12);
  color: #059669;
  border-color: rgba(16, 185, 129, 0.18);
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-waiting_input) {
  background: #eceef2;
  color: #667085;
  border-color: rgba(71, 84, 103, 0.08);
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-unknown) {
  background: #f1f5f9;
  color: #94a3b8;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-badge.state-error) {
  background: rgba(240, 68, 56, 0.14);
  color: #f04438;
  border-color: rgba(240, 68, 56, 0.18);
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-icon) {
  line-height: 1;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-agent-icon svg) {
  display: block;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-badge-dot) {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 8px;
  height: 8px;
  border: 2px solid var(--app-surface-color, #fff);
  box-sizing: content-box;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-plan-badge) {
  position: absolute;
  left: -3px;
  bottom: -1px;
  width: 12px;
  height: 12px;
  pointer-events: none;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-plan-badge::before),
:global(.web-session-mobile-dropdown .mobile-tab-option-plan-badge::after) {
  content: '';
  position: absolute;
  top: 5px;
  left: -1px;
  width: 14px;
  height: 2px;
  background: #0ea5e9;
  border-radius: 999px;
  transform-origin: center center;
}

:global(.web-session-mobile-dropdown .mobile-tab-option-plan-badge::before) {
  transform: rotate(54deg);
}

:global(.web-session-mobile-dropdown .mobile-tab-option-plan-badge::after) {
  transform: rotate(-54deg);
}

:global(
  .web-session-mobile-dropdown
    .web-session-mobile-option.is-approval
    > .n-dropdown-option-body::before
) {
  background: color-mix(
    in srgb,
    var(--web-session-approval-accent, #f79009) 16%,
    var(--app-surface-color, #fff)
  );
}

:global(
  .web-session-mobile-dropdown .web-session-mobile-option.is-approval > .n-dropdown-option-body
) {
  background: color-mix(
    in srgb,
    var(--web-session-approval-accent, #f79009) 8%,
    var(--app-surface-color, #fff)
  );
}

:global(
  .web-session-mobile-dropdown .web-session-mobile-option.is-approval .mobile-tab-option-title
) {
  color: color-mix(in srgb, var(--web-session-approval-accent-strong, #b54708) 78%, #111827);
}

:global(
  .web-session-mobile-dropdown
    .web-session-mobile-option.is-completion
    > .n-dropdown-option-body::before
) {
  background: color-mix(in srgb, #10b981 14%, var(--app-surface-color, #fff));
}

:global(
  .web-session-mobile-dropdown .web-session-mobile-option.is-completion > .n-dropdown-option-body
) {
  background: color-mix(in srgb, #10b981 7%, var(--app-surface-color, #fff));
}

:global(
  .web-session-mobile-dropdown
    .web-session-mobile-option.is-selected
    > .n-dropdown-option-body::before
) {
  background: color-mix(in srgb, var(--n-primary-color) 14%, var(--app-surface-color, #fff));
}

:global(
  .web-session-mobile-dropdown .web-session-mobile-option.is-selected > .n-dropdown-option-body
) {
  background: color-mix(in srgb, var(--n-primary-color) 10%, var(--app-surface-color, #fff));
}

:global(
  .web-session-mobile-dropdown .web-session-mobile-option.is-selected .mobile-tab-option-title
) {
  color: color-mix(in srgb, var(--n-primary-color) 72%, #111827);
}

:global(
  .web-session-mobile-dropdown
    .web-session-mobile-option.is-selected
    .mobile-tab-option-project-badge
) {
  background: color-mix(in srgb, var(--n-primary-color) 78%, #3b82f6);
}

:global(.web-session-mobile-dropdown .mobile-tab-option-title) {
  min-width: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  white-space: normal;
  overflow-wrap: anywhere;
  word-break: break-word;
  line-height: 1.35;
  color: var(--app-text-color, var(--n-text-color));
}

:global(.web-session-mobile-dropdown .mobile-tab-option-project-badge) {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--badge-color, #3b82f6);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  flex-shrink: 0;
}

.agent-select {
  width: 112px;
}

.session-sidebar-shell {
  display: flex;
  min-height: 0;
}

.session-sidebar {
  box-sizing: border-box;
  min-height: 0;
  overflow: hidden;
  border: none;
  border-left: 1px solid var(--n-border-color);
  border-radius: 0;
  background: transparent;
  padding: 4px 0 8px 12px;
  display: flex;
  flex-direction: column;
}

.session-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 2px 0 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--n-primary-color) 8%, var(--n-border-color));
}

.session-sidebar-title-wrap {
  min-width: 0;
}

.session-sidebar-scope-control {
  flex-shrink: 0;
}

.session-sidebar-scope-control:deep(.split-dropdown-control) {
  border-radius: 6px;
}

.session-sidebar-scope-control:deep(.split-dropdown-main),
.session-sidebar-scope-control:deep(.split-dropdown-menu) {
  height: 28px;
  font-size: 11px;
}

.session-sidebar-scope-control:deep(.split-dropdown-main) {
  padding: 0 8px;
  gap: 5px;
}

.session-sidebar-scope-control:deep(.split-dropdown-menu) {
  padding: 0 7px;
}

.session-sidebar-scope-control:deep(.split-dropdown-icon) {
  width: 12px;
  height: 12px;
}

.session-sidebar-scope-control:deep(.split-dropdown-icon svg) {
  width: 12px;
  height: 12px;
}

.session-sidebar-count {
  min-width: 22px;
  height: 22px;
  padding: 0 5px;
  margin-right: 4px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  color: var(--n-primary-color);
  font-size: 10px;
  font-weight: 700;
}

.session-sidebar-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 6px 0 2px;
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

.session-sidebar-item.has-workflow-plan-badge {
  position: relative;
  overflow: visible;
}

.session-sidebar-item.has-workflow-plan-badge::before {
  content: '';
  position: absolute;
  top: 10px;
  left: -6px;
  z-index: 2;
  width: 18px;
  height: 2px;
  background: var(--session-sidebar-accent, #0ea5e9);
  transform: rotate(54deg);
  transform-origin: center center;
  pointer-events: none;
}

.session-sidebar-item.has-workflow-plan-badge::after {
  content: '';
  position: absolute;
  top: 10px;
  left: -6px;
  z-index: 2;
  width: 18px;
  height: 2px;
  background: var(--session-sidebar-accent, #0ea5e9);
  transform: rotate(-54deg);
  transform-origin: center center;
  pointer-events: none;
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

.session-sidebar-item.is-archiving {
  cursor: wait;
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

@keyframes web-session-action-spin {
  to {
    transform: rotate(360deg);
  }
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

.session-sidebar-item.is-active .session-archived-pill {
  background: color-mix(in srgb, var(--n-primary-color) 18%, rgba(255, 255, 255, 0.92));
  color: color-mix(in srgb, var(--n-primary-color) 88%, #ffffff 12%);
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--n-primary-color) 26%, transparent),
    0 1px 2px rgba(59, 130, 246, 0.14);
}

.project-index-badge.session-project-badge {
  width: 18px;
  height: 18px;
  font-size: 10px;
  color: #ffffff;
  background: var(--badge-color, #3b82f6);
  background-image: none;
  border: 1px solid
    color-mix(in srgb, var(--badge-color, #3b82f6) 78%, var(--app-surface-color, #fff) 22%);
  margin-left: 2px;
  box-shadow: none;
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
  background: var(--n-primary-color);
  color: #ffffff;
  border: 1px solid
    color-mix(in srgb, var(--n-primary-color) 78%, var(--app-surface-color, #fff) 22%);
  box-shadow: none;
  animation: none;
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

.session-sidebar-plan-approval {
  border-color: var(--web-session-plan-approval-border, rgba(6, 182, 212, 0.3));
  background: var(--web-session-plan-approval-bg, rgba(6, 182, 212, 0.14));
}

.session-sidebar-item.session-sidebar-plan-approval.is-active,
.session-sidebar-item.session-sidebar-plan-approval.is-active:hover {
  border-color: color-mix(
    in srgb,
    var(--web-session-plan-approval-accent-strong, #0e7490) 14%,
    var(--web-session-plan-approval-border, rgba(6, 182, 212, 0.3)) 86%
  );
  border-left-color: var(--web-session-plan-approval-accent, #0891b2);
  background: linear-gradient(
    135deg,
    color-mix(
        in srgb,
        var(--web-session-plan-approval-bg, rgba(6, 182, 212, 0.14)) 92%,
        var(--app-surface-color, #fff) 8%
      )
      0%,
    color-mix(
        in srgb,
        var(--web-session-plan-approval-bg, rgba(6, 182, 212, 0.14)) 76%,
        var(--app-surface-color, #fff) 24%
      )
      100%
  );
  box-shadow:
    inset 0 0 0 1px
      color-mix(in srgb, var(--web-session-plan-approval-accent, #0891b2) 16%, transparent),
    0 6px 18px color-mix(in srgb, var(--web-session-plan-approval-accent, #0891b2) 14%, transparent);
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
  position: relative;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.item-bubble.is-raw-active {
  border-color: color-mix(in srgb, var(--n-primary-color) 34%, var(--n-border-color));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--n-primary-color) 10%, transparent);
}

.item-bubble.is-raw-capable:focus-visible {
  outline: none;
  border-color: color-mix(in srgb, var(--n-primary-color) 34%, var(--n-border-color));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--n-primary-color) 12%, transparent);
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

.item-bubble.type-run_fail {
  border-color: color-mix(in srgb, var(--n-error-color) 52%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-error-color) 11%, rgba(255, 255, 255, 0.9));
}

.item-bubble.type-note.level-error {
  border-color: color-mix(in srgb, var(--n-warning-color) 48%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-warning-color) 14%, rgba(255, 255, 255, 0.92));
}

.item-bubble.level-warn {
  border-color: color-mix(in srgb, var(--n-warning-color) 35%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-warning-color) 10%, rgba(255, 255, 255, 0.92));
}

.item-bubble.is-raw-active,
.item-bubble.is-raw-capable:focus-visible {
  border-color: color-mix(in srgb, var(--n-primary-color) 34%, var(--n-border-color));
}

.item-text {
  min-width: 0;
}

.item-text--raw {
  padding: 12px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--n-primary-color) 6%, transparent);
}

.timeline-display-toggle {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 4;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--n-text-color-2);
  font-size: 10px;
  line-height: 1;
  font-weight: 600;
  letter-spacing: 0.02em;
  cursor: pointer;
  transition:
    color 0.18s ease,
    opacity 0.18s ease;
}

.timeline-display-toggle:hover {
  color: var(--n-primary-color);
  opacity: 1;
}

.timeline-display-toggle.is-active {
  color: var(--n-primary-color);
}

.timeline-raw-text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: 'SFMono-Regular', 'JetBrains Mono', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.timeline-raw-text code {
  font-family: inherit;
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
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.tool-card.is-raw-capable {
  cursor: pointer;
}

.tool-card.is-raw-active {
  border-color: color-mix(in srgb, var(--n-primary-color) 34%, var(--n-border-color));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--n-primary-color) 10%, transparent);
}

.tool-card.is-raw-capable:focus-visible {
  outline: none;
  border-color: color-mix(in srgb, var(--n-primary-color) 34%, var(--n-border-color));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--n-primary-color) 12%, transparent);
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

.tool-card.is-raw-active,
.tool-card.is-raw-capable:focus-visible {
  border-color: color-mix(in srgb, var(--n-primary-color) 34%, var(--n-border-color));
}

.tool-card.is-context-compaction-tool {
  border-color: rgba(37, 99, 235, 0.24);
  background:
    linear-gradient(
      135deg,
      rgba(239, 246, 255, 0.98) 0%,
      rgba(219, 234, 254, 0.92) 48%,
      rgba(255, 255, 255, 0.98) 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 16px 32px rgba(30, 64, 175, 0.08);
}

.tool-card.is-context-compaction-tool .tool-kind {
  background: rgba(37, 99, 235, 0.12);
  color: #2563eb;
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

.image-view-preview-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 12%, var(--n-border-color));
  border-radius: 12px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 94%, var(--n-primary-color) 6%);
}

.image-view-preview-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.image-view-preview-name {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.image-view-preview-path {
  font-family: 'SFMono-Regular', 'JetBrains Mono', 'Consolas', monospace;
  font-size: 11px;
  line-height: 1.5;
  color: var(--n-text-color-3);
  word-break: break-all;
}

.image-view-preview-frame {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 180px;
  border-radius: 12px;
  overflow: hidden;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--app-surface-color, #fff) 88%, var(--n-primary-color) 12%) 0%,
    color-mix(in srgb, var(--app-surface-color, #fff) 96%, var(--n-primary-color) 4%) 100%
  );
}

.image-view-preview-status {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 180px;
  padding: 18px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color-3);
  text-align: center;
}

.image-view-preview-status.is-error {
  color: var(--n-error-color, #dc2626);
}

.image-view-preview-image {
  display: block;
  max-width: 100%;
  max-height: min(56vh, 520px);
  border-radius: 10px;
  object-fit: contain;
  opacity: 0;
  transition: opacity 0.18s ease;
}

.image-view-preview-image.is-ready {
  opacity: 1;
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

.plan-tool-content--raw {
  padding: 0;
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
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(
    120deg,
    transparent 0%,
    rgba(255, 255, 255, 0.02) 32%,
    rgba(255, 255, 255, 0.34) 50%,
    rgba(255, 255, 255, 0.02) 68%,
    transparent 100%
  );
  transform: translateX(-130%);
  opacity: 0;
  pointer-events: none;
}

.live-card::after {
  content: '';
  position: absolute;
  left: -36%;
  bottom: 0;
  width: 36%;
  height: 3px;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(167, 139, 250, 0.18) 8%,
    rgba(139, 92, 246, 0.95) 48%,
    rgba(167, 139, 250, 0.4) 82%,
    transparent 100%
  );
  opacity: 0;
  pointer-events: none;
}

.live-card:hover {
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.12);
}

.live-card:active {
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.1);
}

.live-card:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--n-primary-color) 72%, white);
  outline-offset: 2px;
}

.live-card.phase-starting,
.live-card.phase-thinking,
.live-card.phase-tool {
  border-color: rgba(139, 92, 246, 0.24);
  background:
    linear-gradient(
      135deg,
      rgba(139, 92, 246, 0.11) 0%,
      rgba(139, 92, 246, 0.03) 52%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(139, 92, 246, 0.08);
}

.live-card.phase-starting::before,
.live-card.phase-thinking::before,
.live-card.phase-tool::before {
  opacity: 0.82;
  animation: liveSweep 1.9s linear infinite;
}

.live-card.phase-starting::after,
.live-card.phase-thinking::after,
.live-card.phase-tool::after {
  opacity: 1;
  animation: liveTrack 1.45s linear infinite;
}

.live-card.phase-retrying {
  border-color: rgba(245, 158, 11, 0.3);
  background:
    linear-gradient(
      135deg,
      rgba(245, 158, 11, 0.13) 0%,
      rgba(245, 158, 11, 0.04) 52%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(245, 158, 11, 0.08);
}

.live-card.phase-retrying::before {
  opacity: 0.72;
  animation: liveSweep 2.2s linear infinite;
}

.live-card.phase-retrying::after {
  opacity: 1;
  animation: liveTrack 1.65s linear infinite;
}

.live-card.phase-waiting_approval,
.live-card.phase-waiting_input {
  border-color: color-mix(in srgb, var(--web-session-approval-border) 82%, var(--n-border-color));
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--web-session-approval-accent) 12%, transparent) 0%,
      transparent 42%
    ),
    linear-gradient(
      135deg,
      color-mix(in srgb, var(--web-session-approval-bg) 24%, var(--app-surface-color, #fff)) 0%,
      color-mix(in srgb, var(--web-session-approval-bg) 11%, var(--app-surface-color, #fff)) 54%,
      var(--app-surface-color, #fff) 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 6px 18px color-mix(in srgb, var(--web-session-approval-glow) 70%, transparent);
}

.live-card.phase-waiting_approval::before,
.live-card.phase-waiting_input::before {
  opacity: 0.26;
  animation: liveSweep 3.2s ease-in-out infinite;
}

.live-card.phase-waiting_plan_approval {
  border-color: color-mix(
    in srgb,
    var(--web-session-plan-approval-border) 82%,
    var(--n-border-color)
  );
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--web-session-plan-approval-accent) 12%, transparent) 0%,
      transparent 42%
    ),
    linear-gradient(
      135deg,
      color-mix(in srgb, var(--web-session-plan-approval-bg) 24%, var(--app-surface-color, #fff)) 0%,
      color-mix(in srgb, var(--web-session-plan-approval-bg) 11%, var(--app-surface-color, #fff))
        54%,
      var(--app-surface-color, #fff) 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 6px 18px color-mix(in srgb, var(--web-session-plan-approval-glow) 70%, transparent);
}

.live-card.phase-waiting_plan_approval::before {
  opacity: 0.26;
  animation: liveSweep 3.2s ease-in-out infinite;
}

.live-card.phase-done {
  border-color: rgba(16, 185, 129, 0.24);
  background:
    linear-gradient(
      135deg,
      rgba(16, 185, 129, 0.12) 0%,
      rgba(16, 185, 129, 0.035) 48%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(16, 185, 129, 0.08);
}

.live-card.phase-done::before {
  opacity: 0.38;
  animation: liveSweep 4.2s ease-in-out infinite;
}

.live-card.phase-error {
  border-color: rgba(239, 68, 68, 0.24);
  background:
    linear-gradient(
      135deg,
      rgba(239, 68, 68, 0.11) 0%,
      rgba(239, 68, 68, 0.03) 48%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(239, 68, 68, 0.08);
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
  background: #8b5cf6;
  box-shadow: 0 0 0 5px rgba(139, 92, 246, 0.16);
  animation: livePulse 1.05s ease-in-out infinite;
  flex-shrink: 0;
}

.live-orb::after {
  content: '';
  position: absolute;
  inset: -9px;
  border-radius: 50%;
  background: rgba(139, 92, 246, 0.22);
  opacity: 0;
  animation: liveRipple 1.35s ease-out infinite;
}

.live-card.phase-waiting_input .live-orb,
.live-card.phase-waiting_approval .live-orb,
.approval-badge {
  background: linear-gradient(
    135deg,
    var(--web-session-approval-accent) 0%,
    var(--web-session-approval-accent-strong) 100%
  );
}

.live-card.phase-waiting_plan_approval .live-orb {
  background: linear-gradient(
    135deg,
    var(--web-session-plan-approval-accent) 0%,
    var(--web-session-plan-approval-accent-strong) 100%
  );
}

.live-card.phase-waiting_input .live-orb,
.live-card.phase-waiting_approval .live-orb {
  box-shadow: 0 0 0 5px color-mix(in srgb, var(--web-session-approval-glow) 82%, transparent);
}

.live-card.phase-waiting_plan_approval .live-orb {
  box-shadow: 0 0 0 5px color-mix(in srgb, var(--web-session-plan-approval-glow) 82%, transparent);
}

.live-card.phase-waiting_approval .live-orb::after,
.live-card.phase-waiting_input .live-orb::after {
  background: color-mix(in srgb, var(--web-session-approval-accent) 28%, transparent);
}

.live-card.phase-waiting_plan_approval .live-orb::after {
  background: color-mix(in srgb, var(--web-session-plan-approval-accent) 28%, transparent);
}

.live-card.phase-retrying .live-orb {
  background: #f59e0b;
  box-shadow: 0 0 0 5px rgba(245, 158, 11, 0.14);
}

.live-card.phase-retrying .live-orb::after {
  background: rgba(245, 158, 11, 0.2);
}

.live-card.phase-done .live-orb {
  background: #10b981;
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.12);
  animation: livePulse 2.8s ease-in-out infinite;
}

.live-card.phase-done .live-orb::after {
  background: rgba(16, 185, 129, 0.18);
  animation-duration: 2.6s;
}

.live-card.phase-error .live-orb {
  background: #ef4444;
  box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.12);
  animation: none;
}

.live-card.phase-error .live-orb::after {
  background: rgba(239, 68, 68, 0.18);
  opacity: 0.35;
  animation: none;
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

.live-time {
  display: inline-flex;
  align-items: center;
  font-variant-numeric: tabular-nums;
}

.live-time-tooltip {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 220px;
}

.live-time-tooltip-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.live-time-tooltip-label {
  font-size: 11px;
  color: var(--n-text-color-3);
  white-space: nowrap;
}

.live-time-tooltip-value {
  min-width: 0;
  font-size: 12px;
  color: var(--n-text-color-1);
  text-align: right;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.approval-card {
  position: relative;
  overflow: hidden;
  border-color: color-mix(in srgb, var(--web-session-approval-border) 88%, transparent);
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--web-session-approval-accent) 10%, transparent) 0%,
      transparent 45%
    ),
    linear-gradient(
      135deg,
      color-mix(in srgb, var(--web-session-approval-border) 14%, var(--app-surface-color, #fff)) 0%,
      color-mix(in srgb, var(--web-session-approval-border) 4%, var(--app-surface-color, #fff)) 58%,
      var(--app-surface-color, #fff) 100%
    );
  padding: 11px 12px;
  box-shadow: 0 10px 24px color-mix(in srgb, var(--web-session-approval-glow) 42%, transparent);
}

.approval-card:not(.history-interaction-card)::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 4px;
  background: linear-gradient(
    180deg,
    var(--web-session-approval-accent) 0%,
    var(--web-session-approval-accent-strong) 100%
  );
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
  border-color: rgba(16, 185, 129, 0.28);
  background: color-mix(in srgb, rgba(16, 185, 129, 0.08) 70%, var(--app-surface-color, #fff));
}

.history-interaction-card.type-approval-reject {
  border-color: rgba(239, 68, 68, 0.28);
  background: color-mix(in srgb, rgba(239, 68, 68, 0.08) 70%, var(--app-surface-color, #fff));
}

.approval-card.is-stale {
  border: 1px dashed
    color-mix(in srgb, var(--web-session-approval-border) 72%, var(--n-border-color));
  background: color-mix(in srgb, var(--web-session-approval-bg) 58%, transparent);
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
  color: #fff;
  font-size: 11px;
  font-weight: 600;
}

.approval-badge.state-approval-approve {
  background: #10b981;
}

.approval-badge.state-approval-reject {
  background: #ef4444;
}

.approval-badge.state-user-input-request,
.approval-badge.state-user-input-response {
  background: linear-gradient(
    135deg,
    var(--web-session-approval-accent) 0%,
    var(--web-session-approval-accent-strong) 100%
  );
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
  color: color-mix(in srgb, var(--web-session-approval-accent-strong) 82%, #111827);
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
  position: relative;
  transition:
    background-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.composer.is-mobile-focused {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--n-primary-color) 20%, transparent);
}

.composer.is-drag-over {
  background: color-mix(in srgb, var(--n-primary-color) 5%, var(--app-surface-color, #fff));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--n-primary-color) 16%, transparent);
}

.composer-mobile-summary {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 4px;
}

.composer-mobile-toggle {
  width: 100%;
  border: 1px solid color-mix(in srgb, var(--n-border-color) 84%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 96%, var(--n-primary-color) 4%);
  color: inherit;
  padding: 8px 10px;
  appearance: none;
  -webkit-appearance: none;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  text-align: left;
  cursor: pointer;
}

.composer-mobile-toggle-copy {
  flex: 1;
  min-width: 0;
}

.composer-mobile-toggle-chips {
  display: flex;
  flex-wrap: nowrap;
  gap: 6px;
  overflow-x: auto;
  scrollbar-width: none;
}

.composer-mobile-toggle-chips::-webkit-scrollbar {
  display: none;
}

.composer-mobile-toggle-chip {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  color: var(--n-text-color-2);
  font-size: 11px;
  line-height: 1.4;
}

.composer-mobile-toggle-arrow {
  flex-shrink: 0;
  margin-top: 2px;
  transition: transform 0.2s ease;
}

.composer-mobile-toggle-arrow.is-open {
  transform: rotate(180deg);
}

.web-session-close-confirm {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.web-session-close-confirm__message {
  line-height: 1.5;
}

.web-session-close-confirm__href {
  display: block;
  max-width: 100%;
  padding: 8px 10px;
  border-radius: 8px;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  background: var(--n-code-color);
}

.web-session-close-confirm__checkbox {
  display: flex;
  align-items: center;
}

.composer-config {
  display: flex;
  align-items: center;
  width: 100%;
  margin-bottom: 2px;
  padding-bottom: 3px;
  border-bottom: 1px solid color-mix(in srgb, var(--n-border-color) 72%, transparent);
}

.composer-config.is-mobile {
  margin-bottom: 6px;
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

.composer-auto-continue {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  white-space: nowrap;
}

.composer-auto-continue :deep(.n-checkbox) {
  display: inline-flex;
  align-items: center;
}

.composer-auto-continue :deep(.n-checkbox__label) {
  font-size: 12px;
  color: var(--n-text-color-2);
}

.composer-input-shell {
  position: relative;
}

.composer-input-shell.is-mobile {
  min-height: 96px;
}

.composer-input-shell.is-mobile .composer-input :deep(.n-input__textarea-el) {
  min-height: 96px !important;
  padding-bottom: 34px !important;
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

.composer-footer.is-mobile {
  margin-top: 4px;
  justify-content: space-between;
  align-items: center;
}

.composer-footer-left,
.composer-footer-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.composer-context-pill {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  border: none;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 88%, transparent);
  color: var(--n-text-color-2);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.01em;
  white-space: nowrap;
  cursor: help;
}

.composer-context-pill.state-active {
  background: rgba(245, 158, 11, 0.08);
  color: #b45309;
}

.composer-context-pill.state-warning {
  background: rgba(239, 68, 68, 0.08);
  color: #b91c1c;
}

.composer-context-pill.state-unavailable {
  opacity: 0.8;
}

.composer-context-tooltip {
  display: grid;
  gap: 6px;
  min-width: 240px;
  max-width: 320px;
}

.composer-context-tooltip-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--n-text-color);
}

.composer-context-tooltip-line {
  font-size: 12px;
  line-height: 1.45;
  color: var(--n-text-color-2);
}

.composer-footer-left {
  min-width: 0;
  margin-left: -2px;
}

.composer-footer-left-mobile {
  margin-left: 0;
  margin-right: auto;
  width: auto;
  justify-content: flex-start;
  align-self: flex-start;
  gap: 6px;
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

.composer-icon-btn-mobile {
  width: 40px;
  height: 40px;
  border-radius: 999px;
  background: transparent;
  border: none;
  box-shadow: none;
  color: var(--n-text-color-3);
}

.composer-icon-btn-mobile-secondary {
  margin-left: 0;
}

.composer-icon-btn-mobile:hover,
.composer-icon-btn-mobile:active {
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

.composer-send-confirm-title {
  font-size: 12px;
  font-weight: 700;
  color: color-mix(in srgb, var(--web-session-approval-accent-strong) 88%, #111827);
}

.composer-send-confirm-body {
  font-size: 12px;
  line-height: 1.45;
  color: color-mix(in srgb, var(--web-session-approval-accent-strong) 72%, #1f2937);
}

.composer-send-btn,
.composer-stop-btn,
.composer-queue-btn {
  min-width: 84px;
}

.composer-send-btn.is-confirm-armed {
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--web-session-approval-border) 78%, transparent);
}

.composer-send-confirm-popover-card {
  display: grid;
  gap: 4px;
  max-width: min(320px, 72vw);
  padding: 2px 0;
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

.quick-input-popover-card {
  width: min(320px, 74vw);
  box-sizing: border-box;
  padding: 0;
}

.quick-input-popover-header {
  display: flex;
  align-items: center;
  padding: 8px 10px 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--n-border-color) 78%, transparent);
}

.quick-input-popover-header :deep(.n-checkbox__label) {
  font-size: 12px;
}

.quick-input-scroll {
  max-height: min(48vh, 320px);
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
  padding: 4px 6px 4px 4px;
  box-sizing: border-box;
}

.quick-input-empty {
  padding: 10px 8px;
  color: var(--n-text-color-3, #8a8f98);
  font-size: 11px;
  line-height: 1.45;
  text-align: center;
}

.quick-input-item-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.quick-input-item {
  display: block;
  width: 100%;
  padding: 6px 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    background-color 0.18s ease,
    color 0.18s ease;
}

.quick-input-item:hover {
  background: color-mix(in srgb, var(--app-surface-color, #fff) 92%, var(--n-primary-color) 8%);
}

.quick-input-item:focus-visible {
  outline: none;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 92%, var(--n-primary-color) 8%);
}

.quick-input-item.is-selected {
  color: var(--n-primary-color);
}

.quick-input-item-text {
  display: -webkit-box;
  overflow: hidden;
  color: var(--n-text-color-1, #222);
  font-size: 12px;
  line-height: 1.4;
  white-space: normal;
  word-break: break-word;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.quick-input-item.is-selected .quick-input-item-text {
  color: inherit;
  font-weight: 600;
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

  .composer-footer-right {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .composer-footer.is-mobile {
    flex-direction: row;
    align-items: flex-end;
  }

  .composer-footer.is-mobile .composer-footer-left-mobile {
    width: auto;
    flex: 0 0 auto;
    justify-content: flex-start;
    align-self: auto;
  }

  .composer-footer.is-mobile .composer-footer-right {
    width: auto;
    min-width: 0;
    flex: 1 1 auto;
    margin-left: auto;
    justify-content: flex-end;
  }

  .composer-config-row {
    flex-wrap: wrap;
  }

  .composer-path {
    width: 100%;
    text-align: left;
  }
}

@media (max-width: 767px) {
  .timeline-tool-shell.plan-tool-shell {
    width: 100%;
  }

  .plan-tool-body {
    padding: 14px 14px 16px;
    gap: 14px;
  }

  .plan-tool-header {
    align-items: flex-start;
    gap: 8px;
  }

  .plan-tool-caption {
    flex: 1 0 100%;
  }

  .plan-tool-content {
    padding: 14px 14px 16px;
    border-radius: 14px;
  }

  .plan-tool-actions {
    padding-top: 2px;
  }

  .plan-tool-action-row {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  .plan-tool-action-primary,
  .plan-tool-action-secondary {
    width: 100%;
    min-width: 0;
  }

  .timeline-display-toggle {
    top: 8px;
    right: 8px;
    font-size: 9px;
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

  .composer-mobile-summary {
    margin-bottom: 2px;
  }

  .composer-mobile-toggle {
    padding: 8px 9px;
  }

  .composer-mobile-toggle-chip {
    font-size: 10px;
  }

  .composer-config.is-mobile {
    margin-bottom: 4px;
  }

  .composer-config.is-mobile .composer-config-row {
    flex-wrap: wrap;
  }

  .composer-config.is-mobile .composer-path {
    flex-basis: 100%;
    text-align: left;
    order: 10;
  }

  .composer-config.is-mobile .composer-auto-continue {
    margin-left: auto;
  }

  .runtime-strip {
    margin-top: 14px;
  }

  .live-card,
  .approval-card {
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
