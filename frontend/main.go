package main

import (
	"log/slog"
	"os"
)

func main() {
	if err := mainInner(); err != nil {
		slog.Error("Exit with error", "err", err)
		os.Exit(1)
	}
}

func mainInner() error {
	return nil
}
