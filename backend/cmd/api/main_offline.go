//go:build !postgres

package main

import "fmt"

func main() {
	fmt.Println("API requires the postgres build tag: go run -tags postgres ./cmd/api")
}
