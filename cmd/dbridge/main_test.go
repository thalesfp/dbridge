package main

import "testing"

func TestHasHumanFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"bare flag", []string{"--human"}, true},
		{"explicit true", []string{"--human=true"}, true},
		{"explicit false", []string{"--human=false"}, false},
		{"no flag", []string{"config", "list"}, false},
		{"mixed with other flags", []string{"--json", "--human", "config"}, true},
		{"after double dash", []string{"--", "--human"}, false},
		{"before double dash", []string{"--human", "--", "arg"}, true},
		{"empty args", []string{}, false},
		{"false before true", []string{"--human=false", "--human=true"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasHumanFlag(tt.args)
			if got != tt.want {
				t.Errorf("hasHumanFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
