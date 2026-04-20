package websession

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type codexSkillRoot struct {
	Path   string
	Source CodexSkillSource
}

type codexSkillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type codexSkillAgentConfig struct {
	Interface struct {
		DisplayName      string `yaml:"display_name"`
		ShortDescription string `yaml:"short_description"`
		DefaultPrompt    string `yaml:"default_prompt"`
	} `yaml:"interface"`
	DisplayName      string `yaml:"display_name"`
	ShortDescription string `yaml:"short_description"`
	DefaultPrompt    string `yaml:"default_prompt"`
}

func (m *Manager) ListCodexSkills() ([]CodexSkillSummary, error) {
	return listCodexSkillsFromRoots(resolveCodexSkillRoots(), m.logger), nil
}

func listCodexSkillsFromRoots(roots []codexSkillRoot, logger *zap.Logger) []CodexSkillSummary {
	selected := make(map[string]CodexSkillSummary)

	for _, root := range roots {
		for _, skill := range scanCodexSkillRoot(root, logger) {
			key := strings.ToLower(strings.TrimSpace(skill.Name))
			if key == "" {
				continue
			}
			current, exists := selected[key]
			if !exists || shouldReplaceCodexSkill(current, skill) {
				selected[key] = skill
			}
		}
	}

	items := make([]CodexSkillSummary, 0, len(selected))
	for _, skill := range selected {
		items = append(items, skill)
	}

	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(items[i].DisplayName))
		right := strings.ToLower(strings.TrimSpace(items[j].DisplayName))
		if left == right {
			return strings.ToLower(strings.TrimSpace(items[i].Name)) <
				strings.ToLower(strings.TrimSpace(items[j].Name))
		}
		return left < right
	})

	return items
}

func shouldReplaceCodexSkill(current, next CodexSkillSummary) bool {
	currentPriority := codexSkillSourcePriority(current.Source)
	nextPriority := codexSkillSourcePriority(next.Source)
	if nextPriority != currentPriority {
		return nextPriority > currentPriority
	}
	return codexSkillSummaryQuality(next) > codexSkillSummaryQuality(current)
}

func codexSkillSummaryQuality(skill CodexSkillSummary) int {
	score := 0
	if strings.TrimSpace(skill.DisplayName) != "" {
		score++
	}
	if strings.TrimSpace(skill.Description) != "" {
		score++
	}
	if strings.TrimSpace(skill.DefaultPrompt) != "" {
		score++
	}
	return score
}

func codexSkillSourcePriority(source CodexSkillSource) int {
	switch source {
	case CodexSkillSourceUser:
		return 3
	case CodexSkillSourceBundled:
		return 2
	default:
		return 1
	}
}

func scanCodexSkillRoot(root codexSkillRoot, logger *zap.Logger) []CodexSkillSummary {
	rootPath := strings.TrimSpace(root.Path)
	if rootPath == "" {
		return nil
	}

	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		return nil
	}

	items := make([]CodexSkillSummary, 0)
	_ = filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			logCodexSkillScanError(logger, "walk", path, walkErr)
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry == nil || !entry.IsDir() {
			return nil
		}

		skillPath := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			return nil
		}

		skill, err := loadCodexSkillSummary(path, root)
		if err != nil {
			logCodexSkillScanError(logger, "parse", path, err)
			return filepath.SkipDir
		}
		items = append(items, skill)
		return filepath.SkipDir
	})

	return items
}

func loadCodexSkillSummary(skillDir string, root codexSkillRoot) (CodexSkillSummary, error) {
	skillPath := filepath.Join(skillDir, "SKILL.md")
	rawSkill, err := os.ReadFile(skillPath)
	if err != nil {
		return CodexSkillSummary{}, fmt.Errorf("read skill file: %w", err)
	}

	frontmatter, _ := parseCodexSkillFrontmatter(rawSkill)
	agentConfig, _ := parseCodexSkillAgentConfig(filepath.Join(skillDir, "agents", "openai.yaml"))

	name := strings.TrimSpace(frontmatter.Name)
	if name == "" {
		name = filepath.Base(skillDir)
	}

	displayName := firstNonEmptyString(
		agentConfig.Interface.DisplayName,
		agentConfig.DisplayName,
		name,
	)
	description := firstNonEmptyString(
		agentConfig.Interface.ShortDescription,
		agentConfig.ShortDescription,
		frontmatter.Description,
	)
	defaultPrompt := firstNonEmptyString(
		agentConfig.Interface.DefaultPrompt,
		agentConfig.DefaultPrompt,
	)

	return CodexSkillSummary{
		Name:          name,
		DisplayName:   displayName,
		Description:   description,
		DefaultPrompt: defaultPrompt,
		Source:        detectCodexSkillSource(root, skillDir),
	}, nil
}

func parseCodexSkillFrontmatter(raw []byte) (codexSkillFrontmatter, bool) {
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") && strings.TrimSpace(normalized) != "---" {
		return codexSkillFrontmatter{}, false
	}

	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return codexSkillFrontmatter{}, false
	}

	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIndex = i
			break
		}
	}
	if endIndex <= 1 {
		return codexSkillFrontmatter{}, false
	}

	var frontmatter codexSkillFrontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:endIndex], "\n")), &frontmatter); err != nil {
		return codexSkillFrontmatter{}, false
	}

	return frontmatter, true
}

func parseCodexSkillAgentConfig(path string) (codexSkillAgentConfig, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return codexSkillAgentConfig{}, false
	}

	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(false)

	var config codexSkillAgentConfig
	if err := decoder.Decode(&config); err != nil {
		return codexSkillAgentConfig{}, false
	}

	return config, true
}

func detectCodexSkillSource(root codexSkillRoot, skillDir string) CodexSkillSource {
	if root.Source != CodexSkillSourceUser {
		return root.Source
	}

	relativePath, err := filepath.Rel(root.Path, skillDir)
	if err != nil {
		return CodexSkillSourceUser
	}

	segments := strings.Split(filepath.ToSlash(relativePath), "/")
	if len(segments) > 0 && strings.TrimSpace(segments[0]) == ".system" {
		return CodexSkillSourceSystem
	}

	return CodexSkillSourceUser
}

func resolveCodexSkillRoots() []codexSkillRoot {
	roots := make([]codexSkillRoot, 0, 1)

	if userRoot := resolveUserCodexSkillsRoot(); userRoot != "" {
		roots = append(roots, codexSkillRoot{
			Path:   userRoot,
			Source: CodexSkillSourceUser,
		})
	}

	return roots
}

func resolveUserCodexSkillsRoot() string {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return filepath.Join(codexHome, "skills")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ""
	}

	return filepath.Join(homeDir, ".codex", "skills")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func logCodexSkillScanError(logger *zap.Logger, action, path string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Debug(
		"failed to scan codex skill",
		zap.String("action", action),
		zap.String("path", path),
		zap.Error(err),
	)
}
