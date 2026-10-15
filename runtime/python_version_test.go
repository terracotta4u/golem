package runtime

import (
	"strings"
	"testing"
)

func TestParsePythonVersion(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr string
	}{
		{name: "minor", content: "3.11\n", want: "3.11"},
		{name: "patch", content: "3.11.9", want: "3.11.9"},
		{name: "crlf", content: "3.13.1\r\n", want: "3.13.1"},
		{name: "comment and blank", content: "# pin\n\n  3.10  \n", want: "3.10"},
		{name: "inline comment", content: "3.12.0rc1 # local\n", want: "3.12.0rc1"},
		{name: "implementation", content: "cpython@3.12\n", want: "cpython@3.12"},
		{name: "free threaded", content: "3.13t\n", want: "3.13t"},
		{name: "empty", content: "\n# nothing\n", want: ""},
		{name: "invalid", content: "latest\n", wantErr: `invalid Python version "latest"`},
		{name: "range", content: ">=3.11\n", wantErr: `invalid Python version ">=3.11"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePythonVersion(tt.content)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("version = %q, want %q", got, tt.want)
			}
		})
	}
}
