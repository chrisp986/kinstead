//go:build !postgres

package main

import "fmt"

func main() {
	fmt.Println("Worker requires the postgres build tag: go run -tags postgres ./cmd/worker")
}
