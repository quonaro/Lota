package cli

import (
	"fmt"
	"lota/config"
	"sort"
	"strings"
)

const (
	maxSuggestions         = 3
	maxSimilarityThreshold = 0.6 // Maximum allowed normalized Levenshtein distance (60%)
)

func commandNotFoundError(cfg *config.AppConfig, cliArgs []string) error {
	query := strings.TrimSpace(strings.Join(cliArgs, " "))
	if query == "" {
		query = "<empty>"
	}

	suggestions := suggestCommandPaths(cfg, cliArgs)
	if len(suggestions) == 0 {
		return fmt.Errorf("command not found: %s", query)
	}

	lines := make([]string, 0, len(suggestions)+3)
	lines = append(lines, fmt.Sprintf("command not found: %s", query), "Did you mean:")
	for _, suggestion := range suggestions {
		lines = append(lines, "  - "+suggestion)
	}

	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

func suggestCommandPaths(cfg *config.AppConfig, cliArgs []string) []string {
	query := normalizeSuggestionQuery(cliArgs)
	if query == "" {
		return nil
	}

	candidates := collectCommandPaths(cfg)
	if len(candidates) == 0 {
		return nil
	}

	type scoredPath struct {
		path  string
		score int
	}

	scored := make([]scoredPath, 0, len(candidates))
	for _, candidate := range candidates {
		scored = append(scored, scoredPath{path: candidate, score: suggestionScore(query, candidate)})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].path < scored[j].path
		}
		return scored[i].score < scored[j].score
	})

	// Filter out candidates with high scores (poor similarity)
	filtered := make([]scoredPath, 0, len(scored))
	for _, sp := range scored {
		if sp.score < 9999 {
			filtered = append(filtered, sp)
		}
	}

	limit := maxSuggestions
	if len(filtered) < limit {
		limit = len(filtered)
	}

	result := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, filtered[i].path)
	}
	return result
}

func normalizeSuggestionQuery(cliArgs []string) string {
	if len(cliArgs) == 0 {
		return ""
	}

	tokens := make([]string, 0, len(cliArgs))
	for _, token := range cliArgs {
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		tokens = append(tokens, token)
	}

	if len(tokens) == 0 {
		return ""
	}

	return strings.ToLower(strings.Join(tokens, " "))
}

func collectCommandPaths(cfg *config.AppConfig) []string {
	paths := make([]string, 0, len(cfg.Groups)+len(cfg.Commands))

	for i := range cfg.Groups {
		collectGroupPaths(&cfg.Groups[i], nil, &paths)
	}

	for i := range cfg.Commands {
		paths = append(paths, cfg.Commands[i].Name)
	}

	return paths
}

func collectGroupPaths(group *config.Group, parents []string, out *[]string) {
	path := make([]string, len(parents)+1)
	copy(path, parents)
	path[len(parents)] = group.Name
	*out = append(*out, strings.Join(path, " "))

	for i := range group.Commands {
		*out = append(*out, strings.Join(append(path, group.Commands[i].Name), " "))
	}

	for i := range group.Groups {
		collectGroupPaths(&group.Groups[i], path, out)
	}
}

func suggestionScore(query, candidate string) int {
	normCandidate := strings.ToLower(candidate)
	if normCandidate == query {
		return 0
	}
	if strings.HasPrefix(normCandidate, query) {
		return 1
	}

	// Only use Contains if the match is significant (at least 50% of query length)
	if strings.Contains(normCandidate, query) && len(query) >= 3 {
		return 2
	}

	// Use normalized Levenshtein distance to avoid suggesting completely different strings
	queryCompact := strings.ReplaceAll(query, " ", "")
	candidateCompact := strings.ReplaceAll(normCandidate, " ", "")

	dist := levenshteinDistance(queryCompact, candidateCompact)
	maxLen := max(len(queryCompact), len(candidateCompact))

	// Normalize distance: 0.0 = identical, 1.0 = completely different
	normalizedDist := float64(dist) / float64(maxLen)

	// If similarity is too low, return a high score to filter it out
	if normalizedDist > maxSimilarityThreshold {
		return 9999
	}

	return dist + 3
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			insertion := curr[j-1] + 1
			deletion := prev[j] + 1
			substitution := prev[j-1] + cost
			curr[j] = minInt(insertion, deletion, substitution)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

func minInt(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}
