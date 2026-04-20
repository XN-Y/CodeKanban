package websession

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListCodexSkillsFromRootsReadsInstalledSkillsOnly(t *testing.T) {
	baseDir := t.TempDir()
	userRoot := filepath.Join(baseDir, "user-skills")

	writeTestCodexSkill(t, filepath.Join(userRoot, "codekanban-cli"), testCodexSkillFixture{
		SkillFrontmatter: "---\nname: codekanban-cli\ndescription: User skill description\n---\n# Skill\n",
		AgentYAML:        "interface:\n  display_name: User CodeKanban\n  short_description: User short description\n  default_prompt: User default prompt\n",
	})
	writeTestCodexSkill(t, filepath.Join(userRoot, ".system", "openai-docs"), testCodexSkillFixture{
		SkillFrontmatter: "---\nname: openai-docs\ndescription: System docs description\n---\n# Skill\n",
		AgentYAML:        "interface:\n  display_name: OpenAI Docs\n  short_description: Reference official OpenAI docs\n  default_prompt: Docs default prompt\n",
	})
	writeTestCodexSkill(t, filepath.Join(userRoot, "helper-skill"), testCodexSkillFixture{
		SkillFrontmatter: "# Skill without frontmatter\n",
		AgentYAML:        "display_name: Helper Skill\nshort_description: Extra installed helper\n",
	})

	items := listCodexSkillsFromRoots([]codexSkillRoot{{Path: userRoot, Source: CodexSkillSourceUser}}, nil)

	if len(items) != 3 {
		t.Fatalf("expected 3 installed skills, got %d: %#v", len(items), items)
	}

	byName := make(map[string]CodexSkillSummary, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}

	codekanban := byName["codekanban-cli"]
	if codekanban.Source != CodexSkillSourceUser {
		t.Fatalf("expected user skill to win dedupe, got source %q", codekanban.Source)
	}
	if codekanban.DisplayName != "User CodeKanban" {
		t.Fatalf("expected user display name, got %q", codekanban.DisplayName)
	}
	if codekanban.DefaultPrompt != "User default prompt" {
		t.Fatalf("expected user default prompt, got %q", codekanban.DefaultPrompt)
	}

	openAIDocs := byName["openai-docs"]
	if openAIDocs.Source != CodexSkillSourceSystem {
		t.Fatalf("expected .system skill source to be system, got %q", openAIDocs.Source)
	}
	if openAIDocs.DisplayName != "OpenAI Docs" {
		t.Fatalf("expected interface display name, got %q", openAIDocs.DisplayName)
	}

	helperSkill := byName["helper-skill"]
	if helperSkill.Source != CodexSkillSourceUser {
		t.Fatalf("expected helper skill source user, got %q", helperSkill.Source)
	}
	if helperSkill.Name != "helper-skill" {
		t.Fatalf("expected directory fallback name, got %q", helperSkill.Name)
	}
}

func TestResolveUserCodexSkillsRootUsesCodeHome(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), "custom-codex-home")
	t.Setenv("CODEX_HOME", codexHome)

	got := resolveUserCodexSkillsRoot()
	want := filepath.Join(codexHome, "skills")
	if got != want {
		t.Fatalf("resolveUserCodexSkillsRoot() = %q, want %q", got, want)
	}
}

func TestParseCodexSkillFrontmatter(t *testing.T) {
	raw := []byte("---\nname: test-skill\ndescription: Example description\n---\n# Body\n")
	got, ok := parseCodexSkillFrontmatter(raw)
	if !ok {
		t.Fatalf("expected frontmatter to parse")
	}
	if got.Name != "test-skill" {
		t.Fatalf("expected name test-skill, got %q", got.Name)
	}
	if got.Description != "Example description" {
		t.Fatalf("expected description to parse, got %q", got.Description)
	}
}

type testCodexSkillFixture struct {
	SkillFrontmatter string
	AgentYAML        string
}

func writeTestCodexSkill(t *testing.T, dir string, fixture testCodexSkillFixture) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(fixture.SkillFrontmatter), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
	if fixture.AgentYAML == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "agents", "openai.yaml"), []byte(fixture.AgentYAML), 0o644); err != nil {
		t.Fatalf("write openai.yaml failed: %v", err)
	}
}
