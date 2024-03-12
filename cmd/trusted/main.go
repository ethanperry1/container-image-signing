package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Printf("Trusted!")
	<-time.After(time.Hour)
}