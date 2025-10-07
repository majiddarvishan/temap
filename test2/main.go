package main

import (
	"time"
    "fmt"
)

func main() {
	// Create map with expiration callback
	ttlMap := New(func(key string, value interface{}) {
		fmt.Printf("Key '%s' expired with value: %v\n", key, value)
	})

	// Add temporary entries
	ttlMap.SetTemporary("session1", "user123", 5*time.Second)
	ttlMap.SetTemporary("cache", map[string]int{"count": 42}, 3*time.Second)
	ttlMap.SetTemporary("tiny", "tiny 1", 1*time.Second)

	// Add permanent entry
	ttlMap.SetPermanent("config", "permanent_data")

	// Get values
	if val, ok := ttlMap.Get("session1"); ok {
		fmt.Println("Found:", val)
	}

	fmt.Println("Size:", ttlMap.Size())

	// Wait for expiration
	time.Sleep(6 * time.Second)
	fmt.Println("Size after expiration:", ttlMap.Size())

	// Remove specific key
	ttlMap.Remove("config")

	// Set a key with 10 second TTL
	ttlMap.SetTemporary("session", "data", 10*time.Second)

	// Later, extend the expiration by 5 more minutes
	newExpiry := time.Now().Add(5 * time.Second)
	if ttlMap.SetExpiry("session", newExpiry) {
		fmt.Println("Expiration updated")
	} else {
		fmt.Println("Key not found")
	}

	// Or make it expire immediately
	if ttlMap.SetExpiry("session", time.Now()) {
		fmt.Println("immediately Expiration updated")
	} else {
		fmt.Println("immediately Key not found")
	}

    // Wait for expiration
	time.Sleep(10 * time.Second)

	// Clear all
	ttlMap.RemoveAll()
}
