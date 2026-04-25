package installer

import (
	"fmt"
)

// ReleaseAsset represents a GitHub release asset.
type ReleaseAsset struct {
	Name string
	URL  string
}

// Release represents a GitHub release.
type Release struct {
	TagName string
	Assets  []ReleaseAsset
}

// FetchLatestRelease fetches the latest release from GitHub (stub implementation).
func FetchLatestRelease(owner, repo string) (*Release, error) {
	return nil, fmt.Errorf("FetchLatestRelease not implemented: installer stub")
}

// FindAsset finds an asset matching the platform.
func (r *Release) FindAsset(platform *Platform) *ReleaseAsset {
	return nil
}

// DownloadAsset downloads the asset to a local path.
func (a *ReleaseAsset) DownloadAsset(dest string) error {
	return fmt.Errorf("DownloadAsset not implemented: installer stub")
}