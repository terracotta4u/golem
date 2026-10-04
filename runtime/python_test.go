package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectPython(t *testing.T) {
	tests := []struct {
		name    string
		missing bool
		data    string
		want    string
		wantErr bool
	}{
		{name: "missing file", missing: true},
		{name: "version", data: "3.11\n", want: "3.11"},
		{name: "crlf", data: "3.12\r\n", want: "3.12"},
		{
			name: "leading blank and comment lines",
			data: "\n  # comment\n\n3.11\n",
			want: "3.11",
		},
		{name: "whitespace only", data: " \t \n\n"},
		{name: "leading dashes", data: "--python-fake\n", wantErr: true},
		{name: "path separator", data: "foo/bar\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.missing {
				path := filepath.Join(dir, ".python-version")
				if err := os.WriteFile(path, []byte(tt.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := ProjectPython(dir)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), ".python-version") {
					t.Errorf("error %q does not mention .python-version", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("ProjectPython() = %q, want %q", got, tt.want)
			}
		})
	}
}
