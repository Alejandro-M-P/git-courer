package data

import (
	"sync"
	"testing"
)

func TestGetLanguageNodes_Lazy(t *testing.T) {
	// Since init() might have already run in other tests, 
	// we should probably reset the state if we can, 
	// but better to just test concurrency and that it works.
	
	// Clear the state for this test (this is dangerous if other tests run in parallel)
	mu.Lock()
	loaded = nil
	mu.Unlock()
	// reset sync.Once - oh wait, we haven't added it yet.

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
