package classifier

import (
	"fmt"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// BenchmarkClassify_realistic_chunk benchmarks a typical diff with ~10 AST labels.
// Target: < 1ms per call.
func BenchmarkClassify_realistic_chunk(b *testing.B) {
	c := &Classifier{}
	annotated := `📄 internal/api/handler.go
NewHandler [NEW_FUNC] internal/api/handler.go:10
HandleRequest [NEW_FUNC] internal/api/handler.go:45
validateInput [NEW_FUNC] internal/api/handler.go:80
📄 internal/api/middleware.go
AuthMiddleware [NEW_FUNC] internal/api/middleware.go:12
LoggingMiddleware [NEW_FUNC] internal/api/middleware.go:35
📄 internal/api/router.go
SetupRoutes [NEW_FUNC] internal/api/router.go:8
📄 internal/domain/user.go
User [NEW_TYPE] internal/domain/user.go:15
UserID [NEW_TYPE] internal/domain/user.go:20
📄 internal/domain/user.go
NewUser [NEW_FUNC] internal/domain/user.go:30
ValidateUser [NEW_FUNC] internal/domain/user.go:55
`
	chunk := &domain.DiffChunk{
		Files:         []string{"internal/api/handler.go", "internal/api/middleware.go"},
		Diff:          "sample diff",
		AnnotatedDiff: annotated,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Classify(chunk)
	}
}

// BenchmarkClassify_large_chunk benchmarks a diff with 50+ AST labels.
// Target: < 1ms per call.
func BenchmarkClassify_large_chunk(b *testing.B) {
	c := &Classifier{}

	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("📄 internal/pkg/file_%d.go\nFunc_%d [NEW_FUNC] internal/pkg/file_%d.go:%d\n", i, i, i, i*10+1))
	}

	chunk := &domain.DiffChunk{
		Files:         []string{"internal/pkg/large.go"},
		Diff:          "large diff",
		AnnotatedDiff: sb.String(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Classify(chunk)
	}
}
