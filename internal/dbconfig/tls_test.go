package dbconfig

import "testing"

func TestMySQLTLSMode(t *testing.T) {
	tests := map[string]string{
		"disable":     "false",
		"prefer":      "preferred",
		"preferred":   "preferred",
		"require":     "true",
		"verify-ca":   "true",
		"verify-full": "true",
		"":            "true",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := MySQLTLSMode(input); got != want {
				t.Fatalf("MySQLTLSMode(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestMSSQLTLSMode(t *testing.T) {
	tests := []struct {
		input       string
		wantEncrypt string
		wantTrust   string
	}{
		{input: "disable", wantEncrypt: "disable"},
		{input: "prefer", wantEncrypt: "false"},
		{input: "preferred", wantEncrypt: "false"},
		{input: "require", wantEncrypt: "true", wantTrust: "true"},
		{input: "verify-ca", wantEncrypt: "true", wantTrust: "false"},
		{input: "verify-full", wantEncrypt: "true", wantTrust: "false"},
		{input: "", wantEncrypt: "true", wantTrust: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			encrypt, trust := MSSQLTLSMode(tt.input)
			if encrypt != tt.wantEncrypt || trust != tt.wantTrust {
				t.Fatalf("MSSQLTLSMode(%q) = (%q, %q), want (%q, %q)", tt.input, encrypt, trust, tt.wantEncrypt, tt.wantTrust)
			}
		})
	}
}
