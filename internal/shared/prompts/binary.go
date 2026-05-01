package prompts

// IsBinary returns true if data contains any null byte within the first
// 8KB of content. The scan is bounded for performance on large files.
func IsBinary(data []byte) bool {
	limit := len(data)
	const maxScan = 8192
	if limit > maxScan {
		limit = maxScan
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
