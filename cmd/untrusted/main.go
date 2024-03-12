package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Printf("Untrusted :(")
	<-time.After(time.Hour)
}