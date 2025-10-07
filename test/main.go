package main

import (
	"fmt"
	"time"

	// "temap" // replace with your module path
)

func main() {
	// Callback function to handle expired keys
	onExpire := func(key, val any) {
		fmt.Printf("[Expired] Key: %v, Value: %v\n", key, val)
	}

	// Create a TimedMap with 4 worker pool for expirations
	tm := New(onExpire, 4)

	// Set temporary keys with TTL
	tm.SetWithTTL("short_lived", "hello world", 3*time.Second)
	tm.SetWithTTL("medium_lived", 42, 5*time.Second)
    tm.SetWithTTL("tiny_lived", "test tiny", 1*time.Second)

	// Set a permanent key
	tm.SetPermanent("forever", "I never expire")

	// Set temporary key with explicit expiration time
	expiration := time.Now().Add(4 * time.Second)
	tm.SetTemporary("custom_expiry", 99, expiration)

	fmt.Println("Initial map snapshot:", tm.ToMap())

	// Wait and observe expirations
	fmt.Println("Waiting for expirations...")
	time.Sleep(6 * time.Second)

	// Check remaining keys
	fmt.Println("Map after expirations:", tm.ToMap())

	// Make a temporary key permanent
	tm.SetWithTTL("becomes_permanent", "sticky", 2*time.Second)
	tm.MakePermanent("becomes_permanent")
	fmt.Println("After making key permanent:", tm.ToMap())

	// Remove a key manually
	tm.Remove("forever")
	fmt.Println("After removing 'forever':", tm.ToMap())

	// Print stats
	stats := tm.Stats()
	fmt.Println("TimedMap stats:", stats)

	// Stop the cleaner and worker pool gracefully
	tm.StopCleaner()
}
