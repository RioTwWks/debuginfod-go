// Минимальная программа для демонстрации debuginfod-go + Delve.
package main

import (
	"fmt"
	"os"
)

func add(a, b int) int {
	return a + b
}

func greet(name string) {
	answer := add(40, 2)
	fmt.Printf("Hello, %s! answer=%d\n", name, answer)
}

func main() {
	who := "debuginfod"
	if len(os.Args) > 1 {
		who = os.Args[1]
	}
	greet(who)
}
