package data

import (
	"sync"
	"testing"
)

func TestGetLanguageNodes_Lazy(t *testing.T) {
	// We test concurrency and that it works.
	
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = GetLanguageNodes("Go")
		}()
	}
	wg.Wait()

	mu.RLock()
	if loaded == nil {
		t.Error("Expected loaded map to be populated after GetLanguageNodes call")
	}
	mu.RUnlock()
}
