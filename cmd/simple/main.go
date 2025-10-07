package main

import (
	"fmt"
	"time"

	"github.com/majiddarvishan/temap"
    // "temap"
)

// func main() {
//     m := temap.New(time.Duration(15)*time.Second, func(val any) {
//         fmt.Printf("value %v is timeouted at: %v\n", val, time.Now())
//     })




//     time.Sleep(30*time.Second)
// }

// package main

// import (
// 	"fmt"
// 	"time"

// 	"yourmodule/temap"
// )

func main() {
	tm := temap.New(func(k, v any) {
		fmt.Printf("expired: %v -> %v\n", k, v)
	})
    tm.StartCleaner()

    // defer tm.Stop()

    TTL := time.Second * time.Duration(5)
    expiresAt := time.Now().Add(TTL)
    fmt.Printf("test-1 will be expire at: %v\n", expiresAt)
    tm.SetTemporary(1, "test-1", expiresAt)

    TTL = time.Second * time.Duration(1)
    expiresAt = time.Now().Add(TTL)
    fmt.Printf("test-2 will be expire at: %v\n", expiresAt)
    tm.SetTemporary(2, "test-2", expiresAt)

    time.Sleep(10 * time.Second)

	tm.SetWithTTL("session1", "user42", 3*time.Second)
	tm.SetPermanent("config", "always")

	time.Sleep(2 * time.Second)
	fmt.Println("Stopping cleaner temporarily...")
	tm.StopCleaner()

	tm.SetWithTTL("late", "user99", 2*time.Second)
	fmt.Println("Cleaner stopped, item won't expire yet")

	time.Sleep(3 * time.Second) // no expiration yet

	fmt.Println("Restarting cleaner...")
	tm.RestartCleaner()

	time.Sleep(2 * time.Second)
	fmt.Println("Stats:", tm.Stats())

    fmt.Println("size:", tm.Size())
}
