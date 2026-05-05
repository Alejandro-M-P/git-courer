package ports

// FileContent holds the before and after contents of a single changed file.
type FileContent struct {
	Filename string
	Before   []byte
	After    []byte
}

// ContentProvider retrieves file contents for changed files in a diff.
// It is used by the caller (e.g. commit workflow) to supply before/after
// snapshots to the annotator, because the chunker itself does not have
// access to the repository.
type ContentProvider interface {
	// GetContents returns the before and after contents for the given file paths.
	// For new files Before is empty; for deleted files After is empty.
	GetContents(files []string) ([]FileContent, error)
}
