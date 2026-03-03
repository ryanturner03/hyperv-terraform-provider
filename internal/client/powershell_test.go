package client

import "testing"

func TestEscapePSString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "hello", "'hello'"},
		{"with single quote", "it's", "'it''s'"},
		{"with double quotes", `say "hi"`, `'say "hi"'`},
		{"with backtick", "back`tick", "'back`tick'"},
		{"empty", "", "''"},
		{"injection attempt", "'; Remove-VM -Force -Name *; '", "'''; Remove-VM -Force -Name *; '''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapePSString(tt.input)
			if got != tt.expected {
				t.Errorf("EscapePSString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
