package websession

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"code-kanban/utils"
)

type claudeHookAnswerFile struct {
	Answers            map[string]string `json:"answers,omitempty"`
	PermissionDecision string            `json:"permissionDecision,omitempty"`
}

type claudePreToolUseRequest struct {
	SessionID string         `json:"session_id"`
	ToolUseID string         `json:"tool_use_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type claudePreToolUseResponse struct {
	HookSpecificOutput claudePreToolUseHookSpecificOutput `json:"hookSpecificOutput"`
}

type claudePreToolUseHookSpecificOutput struct {
	HookEventName            string         `json:"hookEventName"`
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]any `json:"updatedInput,omitempty"`
}

func claudeAskUserUpdatedInput(
	toolInput map[string]any,
	questions []toolRequestQuestion,
	answers map[string][]string,
) map[string]any {
	updated := cloneMap(toolInput)
	if updated == nil {
		updated = map[string]any{}
	}
	answerMap := make(map[string]string)
	for _, question := range questions {
		questionID := strings.TrimSpace(question.ID)
		values := answers[questionID]
		if len(values) == 0 {
			continue
		}
		key := strings.TrimSpace(firstNonEmpty(question.Question, question.Header, questionID))
		if key == "" {
			continue
		}
		answerMap[key] = strings.Join(values, ", ")
	}
	updated["answers"] = answerMap
	return updated
}

func (m *Manager) writeClaudeHookAnswer(
	sessionID string,
	toolUseID string,
	payload claudeHookAnswerFile,
) error {
	if m.store == nil {
		return fmt.Errorf("store is not configured")
	}
	if err := os.MkdirAll(m.store.claudeHookDir(sessionID), 0o755); err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(m.store.claudeHookAnswerPath(sessionID, toolUseID), encoded, 0o644)
}

func (m *Manager) readClaudeHookAnswer(sessionID, toolUseID string) (claudeHookAnswerFile, error) {
	data, err := os.ReadFile(m.store.claudeHookAnswerPath(sessionID, toolUseID))
	if err != nil {
		return claudeHookAnswerFile{}, err
	}
	var payload claudeHookAnswerFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return claudeHookAnswerFile{}, err
	}
	return payload, nil
}

func (m *Manager) deleteClaudeHookAnswer(sessionID, toolUseID string) {
	if m.store == nil {
		return
	}
	_ = os.Remove(m.store.claudeHookAnswerPath(sessionID, toolUseID))
}

func (m *Manager) ensureClaudeHookServer() (string, error) {
	m.claudeHookOnce.Do(func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			m.claudeHookErr = err
			return
		}
		m.claudeHookToken = utils.NewID()
		mux := http.NewServeMux()
		mux.HandleFunc("/claude-hooks/pre-tool-use", m.handleClaudePreToolUseHook)
		server := &http.Server{Handler: mux}
		m.claudeHookServer = server
		m.claudeHookBaseURL = "http://" + listener.Addr().String()
		m.claudeHookSettingsPath = filepath.Join(m.store.rootDir, "claude-hook-settings.json")

		settings := m.claudeHookSettings()
		encoded, err := json.Marshal(settings)
		if err != nil {
			_ = listener.Close()
			m.claudeHookErr = err
			return
		}
		if err := os.WriteFile(m.claudeHookSettingsPath, encoded, 0o644); err != nil {
			_ = listener.Close()
			m.claudeHookErr = err
			return
		}
		go func() {
			_ = server.Serve(listener)
		}()
	})
	if m.claudeHookErr != nil {
		return "", m.claudeHookErr
	}
	return m.claudeHookSettingsPath, nil
}

func (m *Manager) ensureCCRClaudeHookSettings() error {
	if _, err := m.ensureClaudeHookServer(); err != nil {
		return err
	}
	m.ccrHookMu.Lock()
	defer m.ccrHookMu.Unlock()
	if m.ccrHookReady {
		return nil
	}
	m.ccrHookErr = m.writeCCRClaudeHookSettings()
	if m.ccrHookErr == nil {
		m.ccrHookReady = true
	}
	return m.ccrHookErr
}

func (m *Manager) writeCCRClaudeHookSettings() error {
	if strings.TrimSpace(m.cfg.CCRConfigPath) == "" {
		return fmt.Errorf("claude code router config path is not configured")
	}
	data, err := os.ReadFile(m.cfg.CCRConfigPath)
	if err != nil {
		return fmt.Errorf("read claude code router config: %w", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse claude code router config: %w", err)
	}
	if config == nil {
		config = map[string]any{}
	}
	claudeSettings, _ := config["claudeCodeSettings"].(map[string]any)
	if claudeSettings == nil {
		claudeSettings = map[string]any{}
		config["claudeCodeSettings"] = claudeSettings
	}
	injectClaudeHookSettings(claudeSettings, m.claudeHookBaseURL, m.claudeHookToken)
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(m.cfg.CCRConfigPath, encoded, 0o644)
}

func (m *Manager) claudeHookSettings() map[string]any {
	return map[string]any{
		"allowedHttpHookUrls": []string{
			m.claudeHookBaseURL,
		},
		"hooks": map[string]any{
			"PreToolUse": codeKanbanClaudeHookEntries(m.claudeHookBaseURL, m.claudeHookToken),
		},
	}
}

func codeKanbanClaudeHookEntries(baseURL, token string) []map[string]any {
	return []map[string]any{
		{
			"matcher": "AskUserQuestion",
			"hooks": []map[string]any{
				{
					"type": "http",
					"url":  baseURL + "/claude-hooks/pre-tool-use",
					"headers": map[string]any{
						"Authorization": "Bearer " + token,
					},
				},
			},
		},
		{
			"matcher": "ExitPlanMode",
			"hooks": []map[string]any{
				{
					"type": "http",
					"url":  baseURL + "/claude-hooks/pre-tool-use",
					"headers": map[string]any{
						"Authorization": "Bearer " + token,
					},
				},
			},
		},
	}
}

func injectClaudeHookSettings(settings map[string]any, baseURL, token string) {
	settings["allowedHttpHookUrls"] = appendUniqueStringValues(settings["allowedHttpHookUrls"], baseURL)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	existingEntries, _ := hooks["PreToolUse"].([]any)
	filtered := make([]any, 0, len(existingEntries)+2)
	for _, entry := range existingEntries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		matcher, _ := entryMap["matcher"].(string)
		if matcher == "AskUserQuestion" || matcher == "ExitPlanMode" {
			if cleaned, keep := removeCodeKanbanClaudeHooks(entryMap); keep {
				filtered = append(filtered, cleaned)
			}
			continue
		}
		filtered = append(filtered, entry)
	}
	for _, entry := range codeKanbanClaudeHookEntries(baseURL, token) {
		filtered = append(filtered, entry)
	}
	hooks["PreToolUse"] = filtered
}

func removeCodeKanbanClaudeHooks(entry map[string]any) (map[string]any, bool) {
	rawHooks, ok := entry["hooks"].([]any)
	if !ok {
		return entry, true
	}
	filteredHooks := make([]any, 0, len(rawHooks))
	for _, hook := range rawHooks {
		if isCodeKanbanClaudeHook(hook) {
			continue
		}
		filteredHooks = append(filteredHooks, hook)
	}
	if len(filteredHooks) == 0 {
		return nil, false
	}
	cleaned := make(map[string]any, len(entry))
	for key, value := range entry {
		cleaned[key] = value
	}
	cleaned["hooks"] = filteredHooks
	return cleaned, true
}

func isCodeKanbanClaudeHook(hook any) bool {
	hookMap, ok := hook.(map[string]any)
	if !ok {
		return false
	}
	hookURL, _ := hookMap["url"].(string)
	return strings.Contains(hookURL, "/claude-hooks/pre-tool-use")
}

func appendUniqueStringValues(current any, values ...string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	switch typed := current.(type) {
	case []any:
		for _, item := range typed {
			if value, ok := item.(string); ok {
				add(value)
			}
		}
	case []string:
		for _, value := range typed {
			add(value)
		}
	case string:
		add(typed)
	}
	for _, value := range values {
		add(value)
	}
	return result
}

func (m *Manager) handleClaudePreToolUseHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")); token == "" || token != m.claudeHookToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()

	var request claudePreToolUseRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	toolName := strings.TrimSpace(request.ToolName)
	if toolName != "AskUserQuestion" && toolName != "ExitPlanMode" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response := claudePreToolUseResponse{
		HookSpecificOutput: claudePreToolUseHookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "defer",
		},
	}
	if answerFile, err := m.readClaudeHookAnswer(strings.TrimSpace(request.SessionID), strings.TrimSpace(request.ToolUseID)); err == nil {
		if toolName == "ExitPlanMode" && strings.TrimSpace(answerFile.PermissionDecision) != "" {
			response.HookSpecificOutput.PermissionDecision = strings.TrimSpace(answerFile.PermissionDecision)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		if toolName != "AskUserQuestion" || len(answerFile.Answers) == 0 {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		answers := make(map[string][]string, len(answerFile.Answers))
		questions := decodeToolQuestions(request.ToolInput["questions"])
		for _, question := range questions {
			key := strings.TrimSpace(firstNonEmpty(question.Question, question.Header))
			if key == "" {
				continue
			}
			value := strings.TrimSpace(answerFile.Answers[key])
			if value == "" {
				continue
			}
			answers[firstNonEmpty(question.ID, question.Question, question.Header)] = []string{value}
		}
		response.HookSpecificOutput.PermissionDecision = "allow"
		response.HookSpecificOutput.UpdatedInput = claudeAskUserUpdatedInput(request.ToolInput, questions, answers)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
