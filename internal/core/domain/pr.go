// Package domain holds the core business types.
package domain

// PRCommit represents a single commit within a Pull Request.
type PRCommit struct {
	SHA     string
	Message string
	Author  string
}
