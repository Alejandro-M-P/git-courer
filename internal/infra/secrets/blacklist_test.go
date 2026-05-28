package secrets

import (
	"testing"
)

func TestIsBlacklistedFolder(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Blacklisted folders
		{"node_modules", "node_modules/package/index.js", true},
		{"node_modules nested", "project/node_modules/pkg/dist/index.js", true},
		{".git folder", ".git/objects/pack", true},
		{"vendor go", "vendor/github.com/pkg/module", true},
		{"__pycache__", "__pycache__/cache.pyc", true},
		{".venv python", ".venv/lib/python", true},
		{"dist build", "dist/index.js", true},
		{"build folder", "build/bundle.js", true},
		{"bin folder", "bin/executable", true},
		{"out folder", "out/logs", true},
		{".env folder", ".env/secrets", true},

		// Allowed paths
		{"src folder", "src/index.js", false},
		{"internal folder", "internal/pkg/module.go", false},
		{"packages monorepo", "packages/common/utils.ts", false},
		{"env example folder", ".env.example", false},
		{"env template folder", ".env.template", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBlacklistedFolder(tt.path)
			if got != tt.expected {
				t.Errorf("IsBlacklistedFolder(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsBlacklistedName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		// Blacklisted exact names
		{".env", ".env", true},
		{".env.local", ".env.local", true},
		{".env.production", ".env.production", true},
		{".pem file", "private.pem", true},
		{".key file", "server.key", true},
		{".p12 file", "keystore.p12", true},
		{".pkcs8 file", "key.pkcs8", true},
		{".keystore file", "my.keystore", true},
		{"credentials json", "credentials.json", true},
		{"secrets yaml", "secrets.yaml", true},
		{"id_rsa key", "id_rsa", true},
		{"id_ecdsa key", "id_ecdsa", true},
		{"id_ed25519 key", "id_ed25519", true},
		{"git-courer binary", "git-courer", true},
		{"password prefix", "password.txt", true},
		{"password db prefix", "password_db.env", true},

		// Exceptions — these are allowed
		{".env.example", ".env.example", false},
		{".env.template", ".env.template", false},

		// Allowed names
		{"regular source", "index.js", false},
		{"source code", "main.go", false},
		{"config allowed", "config.yaml", false},
		{".env in subdir", "path/to/.env", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBlacklistedName(tt.filename)
			if got != tt.expected {
				t.Errorf("IsBlacklistedName(%q) = %v, want %v", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestExtractFilename(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple file", "/path/to/file.txt", "file.txt"},
		{"nested file", "src/index.js", "index.js"},
		{"file only", "index.ts", "index.ts"},
		{"hidden file", ".env", ".env"},
		{"path with dots", "v1.2.3/release.tar.gz", "release.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFilename(tt.path)
			if got != tt.expected {
				t.Errorf("ExtractFilename(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}
