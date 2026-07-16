package levenshtein

import "testing"

func TestDistance(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected int
	}{
		{"equal strings", "hello", "hello", 0},
		{"empty a", "", "abc", 3},
		{"empty b", "abc", "", 3},
		{"both empty", "", "", 0},
		{"one insertion", "cat", "cats", 1},
		{"one deletion", "cats", "cat", 1},
		{"one substitution", "cat", "cut", 1},
		{"multiple edits", "kitten", "sitting", 3},
		{"case sensitive", "a", "A", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Distance(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("Distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
