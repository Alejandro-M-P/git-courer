package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockBaseBranchDetector is a minimal mock satisfying the BaseBranchDetector interface.
type mockBaseBranchDetector struct {
	mock.Mock
}

func (m *mockBaseBranchDetector) SymbolicRef(ref string) (string, error) {
	args := m.Called(ref)
	return args.String(0), args.Error(1)
}

func (m *mockBaseBranchDetector) ConfigGet(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

// Compile-time interface check
var _ BaseBranchDetector = (*mockBaseBranchDetector)(nil)

func TestDetectBaseBranch_SymbolicRefSucceeds(t *testing.T) {
	t.Parallel()
	m := new(mockBaseBranchDetector)
	// git symbolic-ref refs/remotes/origin/HEAD returns "refs/remotes/origin/main"
	m.On("SymbolicRef", "refs/remotes/origin/HEAD").Return("refs/remotes/origin/main", nil)

	result := DetectBaseBranch(m)
	assert.Equal(t, "main", result, "should strip refs/remotes/origin/ prefix")
	m.AssertExpectations(t)
}

func TestDetectBaseBranch_SymbolicRefFails_ConfigGetSucceeds(t *testing.T) {
	t.Parallel()
	m := new(mockBaseBranchDetector)
	// symbolic-ref fails (no remote), fallback to init.defaultBranch
	m.On("SymbolicRef", "refs/remotes/origin/HEAD").Return("", assert.AnError)
	m.On("ConfigGet", "init.defaultBranch").Return("main", nil)

	result := DetectBaseBranch(m)
	assert.Equal(t, "main", result, "should fall back to init.defaultBranch config")
	m.AssertExpectations(t)
}

func TestDetectBaseBranch_BothFail_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	m := new(mockBaseBranchDetector)
	// Both fail
	m.On("SymbolicRef", "refs/remotes/origin/HEAD").Return("", assert.AnError)
	m.On("ConfigGet", "init.defaultBranch").Return("", assert.AnError)

	result := DetectBaseBranch(m)
	assert.Equal(t, "", result, "should return empty string when both methods fail")
	m.AssertExpectations(t)
}

func TestDetectBaseBranch_SymbolicRefReturnsDevelop(t *testing.T) {
	t.Parallel()
	m := new(mockBaseBranchDetector)
	// symbolic-ref returns "refs/remotes/origin/develop"
	m.On("SymbolicRef", "refs/remotes/origin/HEAD").Return("refs/remotes/origin/develop", nil)

	result := DetectBaseBranch(m)
	assert.Equal(t, "develop", result, "should strip prefix and return 'develop'")
	m.AssertExpectations(t)
}

func TestDetectBaseBranch_SymbolicRefReturnsNoPrefix_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	m := new(mockBaseBranchDetector)
	// symbolic-ref returns just "main" without prefix — TreatPrefix should handle gracefully
	m.On("SymbolicRef", "refs/remotes/origin/HEAD").Return("main", nil)
	// Since "main" != TrimPrefix("main", "refs/remotes/origin/"), TrimPrefix returns "main"
	// which equals the input, so we fall through. But wait — our logic says branch != ref,
	// so if TrimPrefix doesn't change the string, it means no prefix was stripped.
	// In that case we DO fall through to ConfigGet.
	m.On("ConfigGet", "init.defaultBranch").Return("main", nil)

	result := DetectBaseBranch(m)
	assert.Equal(t, "main", result, "unexpected prefix format should fall through to ConfigGet")
	m.AssertExpectations(t)
}

func TestDetectBaseBranch_ConfigGetReturnsDevelop(t *testing.T) {
	t.Parallel()
	m := new(mockBaseBranchDetector)
	// symbolic-ref fails, ConfigGet returns "develop"
	m.On("SymbolicRef", "refs/remotes/origin/HEAD").Return("", assert.AnError)
	m.On("ConfigGet", "init.defaultBranch").Return("develop", nil)

	result := DetectBaseBranch(m)
	assert.Equal(t, "develop", result, "should return 'develop' from config")
	m.AssertExpectations(t)
}