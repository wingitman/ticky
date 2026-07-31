package main

import (
	"fmt"
	"os"

	"github.com/wingitman/ticky/internal/desktop"
)

func main() {
	if err := desktop.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ticky desktop:", err)
		os.Exit(1)
	}
}
