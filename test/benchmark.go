package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

)

func main() {
	const numKeys = 100_000
	const maxTTL = 10 * time.Second

	var wg sync.WaitGroup
	wg.Add(numKeys) // Track all expirations

	expiredCount := 0
	expiredLock := sync.Mutex{}

	onExpire := func(key, val any) {
		// Track total expired keys
		expiredLock.Lock()
		expiredCount++
		if expiredCount%10000 == 0 {
			fmt.Printf("[Callback] Expired %d keys so far\n", expiredCount)
		}
		expiredLock.Unlock()
		wg.Done() // Mark one key expired
	}

	// Create TimedMap with 8 worker pool
	tm := New(onExpire, 8)

	fmt.Println("Inserting keys...")

	// Insert 100k keys with random TTLs
	for i := 0; i < numKeys; i++ {
		ttl := time.Duration(rand.Intn(int(maxTTL.Milliseconds()))) * time.Millisecond
		tm.SetWithTTL(fmt.Sprintf("key_%d", i), i, ttl)
	}

	fmt.Printf("Inserted %d keys\n", tm.Size())

	// Wait until all expirations are processed
	start := time.Now()
	wg.Wait()
	duration := time.Since(start)

	fmt.Println("All keys expired")
	fmt.Printf("Total expired keys: %d\n", expiredCount)
	fmt.Printf("Time taken: %s\n", duration)
	fmt.Printf("Final stats: %+v\n", tm.Stats())

	tm.StopCleaner()
}
