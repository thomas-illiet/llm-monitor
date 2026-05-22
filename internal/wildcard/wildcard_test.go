package wildcard

import "testing"

func TestMatchSupportsStarQuestionMarkAndLiterals(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "star", pattern: "gpt-*", value: "gpt-4o", want: true},
		{name: "question", pattern: "embed-?", value: "embed-a", want: true},
		{name: "literal regexp metacharacter", pattern: "model.v1", value: "model-v1", want: false},
		{name: "anchored", pattern: "model", value: "model-large", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.pattern, tt.value); got != tt.want {
				t.Fatalf("Match(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}
