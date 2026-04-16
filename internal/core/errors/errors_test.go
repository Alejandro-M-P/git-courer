package errors

import (
	stderrors "errors"
	"testing"
)

func TestIsDomainError_AllSentinels(t *testing.T) {
	sentinels := []error{
		ErrNotAGitRepo,
		ErrNothingToCommit,
		ErrNoRemoteConfigured,
		ErrOllamaNotAvailable,
		ErrModelNotReady,
		ErrSecretsDetected,
		ErrInvalidJSON,
		ErrOperationCancelled,
		ErrSecurityHalt,
		ErrBlacklistedFile,
		ErrBinaryFile,
		ErrSelfBinary,
	}
	for _, sentinel := range sentinels {
		if !IsDomainError(sentinel) {
			t.Errorf("IsDomainError(%v) = false, want true", sentinel)
		}
	}
}

func TestIsDomainError_WrappedSentinel(t *testing.T) {
	wrapped := stderrors.Join(ErrNotAGitRepo, stderrors.New("extra context"))
	// errors.Is traverses the chain — but Join returns a multi-error,
	// so IsDomainError should still find ErrNotAGitRepo.
	if !IsDomainError(wrapped) {
		t.Error("IsDomainError should return true for wrapped domain error")
	}
}

func TestIsDomainError_NonDomainError(t *testing.T) {
	nonDomain := stderrors.New("some random error")
	if IsDomainError(nonDomain) {
		t.Error("IsDomainError(non-domain error) = true, want false")
	}
}

func TestIsDomainError_Nil(t *testing.T) {
	// errors.Is(nil, X) returns false for all our sentinels, so this should be false
	if IsDomainError(nil) {
		t.Error("IsDomainError(nil) = true, want false")
	}
}

func TestSentinelMessages(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{ErrNotAGitRepo, "not a git repository"},
		{ErrNothingToCommit, "nothing to commit"},
		{ErrSecretsDetected, "secrets detected in files"},
		{ErrSecurityHalt, "SECURITY_HALT"},
	}
	for _, tc := range cases {
		if tc.err.Error() == "" {
			t.Errorf("error %T has empty message", tc.err)
		}
		if tc.want != "" && !contains(tc.err.Error(), tc.want) {
			t.Errorf("error %q does not contain %q", tc.err.Error(), tc.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || find(s, sub))
}

func find(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
