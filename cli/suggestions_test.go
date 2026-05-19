package cli

import (
	"lota/config"
	"strings"
	"testing"
)

func testSuggestionConfig(t *testing.T) *config.AppConfig {
	t.Helper()

	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "build", Script: "go build"},
			{Name: "test", Script: "go test ./..."},
		},
		Groups: []config.Group{
			{
				Name: "dev",
				Commands: []config.Command{
					{Name: "run", Script: "go run ."},
					{Name: "lint", Script: "go vet ./..."},
				},
				Groups: []config.Group{
					{
						Name: "docker",
						Commands: []config.Command{
							{Name: "up", Script: "docker compose up"},
						},
					},
				},
			},
		},
	}

	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	return cfg
}

func TestSuggestCommandPaths_Top3(t *testing.T) {
	cfg := testSuggestionConfig(t)

	suggestions := suggestCommandPaths(cfg, []string{"de", "rn"})
	if len(suggestions) != 3 {
		t.Fatalf("expected 3 suggestions, got %d: %v", len(suggestions), suggestions)
	}
}

func TestSuggestCommandPaths_EmptyForOnlyFlags(t *testing.T) {
	cfg := testSuggestionConfig(t)

	suggestions := suggestCommandPaths(cfg, []string{"--help"})
	if len(suggestions) != 0 {
		t.Fatalf("expected no suggestions, got %v", suggestions)
	}
}

func TestCommandNotFoundError_IncludesSuggestions(t *testing.T) {
	cfg := testSuggestionConfig(t)

	err := commandNotFoundError(cfg, []string{"de", "rn"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()

	if !strings.Contains(msg, "Did you mean:") {
		t.Fatalf("expected suggestions section, got %q", msg)
	}
	if strings.Count(msg, "  - ") != 3 {
		t.Fatalf("expected exactly 3 suggestions, got %q", msg)
	}
}

func TestCommandNotFoundError_NoSuggestionsForUnsimilarQuery(t *testing.T) {
	cfg := testSuggestionConfig(t)

	// "clear" is very different from "build", "test", "dev", "run", "lint", "docker", "up"
	err := commandNotFoundError(cfg, []string{"clear"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()

	// Should NOT include suggestions for completely different words
	if strings.Contains(msg, "Did you mean:") {
		t.Fatalf("expected no suggestions for unsimilar query, got %q", msg)
	}
}

func TestSuggestionScore_ClearVsBuild(t *testing.T) {
	// Test that "clear" vs "build" gets a high score (filtered out)
	score := suggestionScore("clear", "build")
	if score < 9999 {
		t.Fatalf("expected high score for dissimilar words, got %d", score)
	}
}

func TestSuggestionScore_ClearVsDev(t *testing.T) {
	// Test that "clear" vs "dev" gets a high score (filtered out)
	score := suggestionScore("clear", "dev")
	if score < 9999 {
		t.Fatalf("expected high score for dissimilar words, got %d", score)
	}
}
