package workflow

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// REQ-5: parseReleaseIntent captures prerelease version
func TestParseReleaseIntent_PrereleaseVersion(t *testing.T) {
	cases := []struct {
		name        string
		instruction string
		releases    []string
		wantTag     string
	}{
		{"prerelease rc", "release v2.0.0-rc.1", []string{"v1.0.0"}, "v2.0.0-rc.1"},
		{"prerelease alpha spanish", "sacar version 3.1.0-alpha.2", []string{"v1.0.0"}, "v3.1.0-alpha.2"},
		{"prerelease beta", "release v1.0.0-beta.3", []string{"v0.9.0"}, "v1.0.0-beta.3"},
		{"simple prerelease", "release v1.0.0-alpha", []string{"v1.0.0"}, "v1.0.0-alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseReleaseIntent(tc.instruction, tc.releases)
			if got.TagName != tc.wantTag {
				t.Errorf("parseReleaseIntent(%q) TagName = %q, want %q", tc.instruction, got.TagName, tc.wantTag)
			}
		})
	}
}

// TestParseReleaseIntent tests the NO-LLM release intent parser.
func TestParseReleaseIntent(t *testing.T) {
	cases := []struct {
		name        string
		instruction string
		releases    []string
		wantBump    string
		wantTag     string
	}{
		// User dice "major"
		{"major keyword", "release major", []string{"v1.0.0"}, "major", "v2.0.0"},
		{"major spanish", "sacar version major", []string{"v1.0.0"}, "major", "v2.0.0"},
		{"romper", "release major", []string{"v1.0.0"}, "major", "v2.0.0"},

		// User dice "minor"
		{"minor keyword", "minor release", []string{"v1.0.0"}, "minor", "v1.1.0"},
		{"minor spanish", "version minor", []string{"v1.2.3"}, "minor", "v1.3.0"},
		{"peque", "sacar version pequena", []string{"v1.0.0"}, "minor", "v1.1.0"},

		// User dice "patch"
		{"patch keyword", "patch release", []string{"v1.0.0"}, "patch", "v1.0.1"},
		{"fix keyword", "fix release", []string{"v1.0.0"}, "patch", "v1.0.1"},
		{"hotfix keyword", "hotfix release", []string{"v1.0.0"}, "patch", "v1.0.1"},
		{"bugfix keyword", "bugfix release", []string{"v1.0.0"}, "patch", "v1.0.1"},

		// Version explicita en instruction
		{"explicit version", "release v2.5.0", []string{"v1.0.0"}, "", "v2.5.0"},
		{"explicit with major", "release v3.0.0 major", []string{"v1.0.0"}, "major", "v3.0.0"},
		{"version with minor", "version 1.2.0 minor", []string{"v1.0.0"}, "minor", "v1.2.0"},

		// Sin releases - usa default patch
		{"no releases + major", "release major", []string{}, "major", ""},
		{"no releases + minor", "release minor", []string{}, "minor", ""},
		{"no releases", "release", []string{}, "", ""},

		// Instruction vacia
		{"empty", "", []string{"v1.0.0"}, "", ""},

		// Espanol variado
		{"sacar version", "sacar version", []string{"v1.0.0"}, "", ""},
		{"nueva version", "nueva version", []string{"v1.0.0"}, "", ""},
		{"crear tag", "crear tag", []string{"v1.0.0"}, "", ""},

		// Merge branch (por ahora no testeado, solo verifica que no explota)
		{"merge branch", "merge from feature/login", []string{"v1.0.0"}, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseReleaseIntent(tc.instruction, tc.releases)
			if tc.wantBump != "" && got.VersionBump != tc.wantBump {
				t.Errorf("VersionBump = %q, want %q", got.VersionBump, tc.wantBump)
			}
			if tc.wantTag != "" && got.TagName != tc.wantTag {
				t.Errorf("TagName = %q, want %q", got.TagName, tc.wantTag)
			}
		})
	}
}

// TestCalculateBumpFromRealCommits usa mensajes QUE EL LLM GENERA реально.
// Estos son ejemplos típicos de convencionales commits que el sistema genera.
func TestCalculateBumpFromRealCommits(t *testing.T) {
	cases := []struct {
		name     string
		lastTag  string
		commits  []string
		wantBump string
		wantTag  string
	}{
		// --- mensajes reales que el LLM genera ---

		// Solo feat (minor)
		{
			"LLM: feat only",
			"v1.0.0",
			[]string{
				"feat: add user login",
			},
			"minor",
			"v1.1.0",
		},
		// feat + fix (minor - feat wins)
		{
			"LLM: feat + fix",
			"v1.0.0",
			[]string{
				"feat: add user login",
				"fix: resolve auth bug",
			},
			"minor",
			"v1.1.0",
		},
		// feat + fix + docs (minor)
		{
			"LLM: feat + fix + docs",
			"v1.0.0",
			[]string{
				"feat: add user login",
				"fix: resolve auth bug",
				"docs: update README",
			},
			"minor",
			"v1.1.0",
		},
		// feat! breaking = major
		{
			"LLM: feat! breaking",
			"v1.0.0",
			[]string{
				"feat!: change API for v2",
			},
			"major",
			"v2.0.0",
		},
		// BREAKING CHANGE en body = major
		{
			"LLM: BREAKING CHANGE body",
			"v1.0.0",
			[]string{
				"feat: add new endpoint\n\nBREAKING CHANGE: removed old method",
			},
			"major",
			"v2.0.0",
		},
		// Solo fix = patch
		{
			"LLM: fix only",
			"v1.2.0",
			[]string{
				"fix: null pointer in auth",
			},
			"patch",
			"v1.2.1",
		},
		// Mixto: feat + fix + refactor (minor)
		{
			"LLM: feat + fix + refactor",
			"v1.0.0",
			[]string{
				"feat: add new feature",
				"fix: resolve bug",
				"refactor: clean code",
			},
			"minor",
			"v1.1.0",
		},
		// chore/ci/build = patch
		{
			"LLM: chore + ci",
			"v1.0.0",
			[]string{
				"chore: update dependencies",
				"ci: update pipeline",
				"build: compile",
			},
			"patch",
			"v1.0.1",
		},
		// test = patch
		{
			"LLM: test only",
			"v1.0.0",
			[]string{
				"test: add unit tests",
			},
			"patch",
			"v1.0.1",
		},
		// style = patch
		{
			"LLM: style",
			"v1.0.0",
			[]string{
				"style: format code",
			},
			"patch",
			"v1.0.1",
		},
		// perf = minor (como feat) - AHORA IMPLEMENTADO
		{
			"LLM: perf",
			"v1.0.0",
			[]string{
				"perf: optimize query",
			},
			"minor",
			"v1.1.0",
		},
		// revert = patch
		{
			"LLM: revert",
			"v1.0.0",
			[]string{
				"revert: abc123",
			},
			"patch",
			"v1.0.1",
		},
		// merge = patch
		{
			"LLM: merge",
			"v1.0.0",
			[]string{
				"merge feature/login into main",
			},
			"patch",
			"v1.0.1",
		},
		// varios feat = minor
		{
			"LLM: multiple feat",
			"v2.0.0",
			[]string{
				"feat: add login",
				"feat: add logout",
				"feat: add register",
			},
			"minor",
			"v2.1.0",
		},
		// major trumps all
		{
			"LLM: major trumps all",
			"v1.0.0",
			[]string{
				"feat: add feature",
				"fix: bug",
				"feat!: break everything",
				"chore: cleanup",
			},
			"major",
			"v2.0.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// CalculateBump desde domain
			bump := domain.CalculateBump(tc.commits)
			if bump != tc.wantBump {
				t.Errorf("CalculateBump() = %q, want %q", bump, tc.wantBump)
			}

			// BumpVersion
			newTag, err := domain.BumpVersion(tc.lastTag, bump)
			if err != nil {
				t.Fatalf("BumpVersion error: %v", err)
			}
			if newTag != tc.wantTag {
				t.Errorf("BumpVersion(%q, %q) = %q, want %q", tc.lastTag, bump, newTag, tc.wantTag)
			}
		})
	}
}

func TestFullReleaseFlow_NoLLM(t *testing.T) {
	// Test: instruction → parseIntent → commits → bump → new tag
	// Simula el flujo completo sin LLM

	instruction := "release minor version"
	releasesList := []string{"v1.0.0", "v1.1.0"}
	commits := []string{
		"feat: add user profile",
		"fix: typo in name",
		"chore: update deps",
	}

	// Step 1: parse instruction (sin LLM)
	intent := parseReleaseIntent(instruction, releasesList)
	if intent.VersionBump != "minor" {
		t.Errorf("parseReleaseIntent bump = %q, want %q", intent.VersionBump, "minor")
	}

	// Step 2: CalculateBump from commits
	bump := domain.CalculateBump(commits)
	if bump != "minor" {
		t.Errorf("CalculateBump = %q, want %q", bump, "minor")
	}

	// Step 3: User choice override - user dijo "minor", usar ese
	actualBump := intent.VersionBump
	if actualBump == "" {
		// fallback a lo que calculó Go
		actualBump = bump
	}

	// Step 4: Calcular siguiente tag
	lastTag := releasesList[len(releasesList)-1] // v1.1.0 (ya ordered)
	newTag, err := domain.BumpVersion(lastTag, actualBump)
	if err != nil {
		t.Fatalf("BumpVersion error: %v", err)
	}
	if newTag != "v1.2.0" {
		t.Errorf("Final tag = %q, want %q", newTag, "v1.2.0")
	}
}
