package main

import (
	"fmt"
	"time"

	"github.com/majiddarvishan/temap"
)

func main() {
    m := temap.New(time.Duration(15)*time.Second, func(val any) {
        fmt.Printf("value %v is timeouted at: %v\n", val, time.Now())
    })

    TTL := time.Second * time.Duration(10)
    expiresAt := time.Now().Add(TTL)
    fmt.Printf("test-1 will be expire at: %v\n", expiresAt)
    m.SetTemporary(1, "test-1", expiresAt)


    TTL = time.Second * time.Duration(5)
    expiresAt = time.Now().Add(TTL)
    fmt.Printf("test-2 will be expire at: %v\n", expiresAt)
    m.SetTemporary(2, "test-2", expiresAt)

    time.Sleep(30*time.Second)
}