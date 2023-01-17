package main

import (
	"fmt"
	
	"github.com/edpaget/http_ioring/uring"
)

func main() {
	uring := uring.NewUring(32)
	fmt.Printf("Uring< %s >\n", uring.Inspect())
}
	
