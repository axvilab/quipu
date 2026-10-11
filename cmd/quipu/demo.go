package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/axvilab/quipu/internal/demo"
)

func runDemo(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: quipu demo <empty dir>")
	}
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	if err := demo.Create(dir, time.Now()); err != nil {
		return err
	}
	fmt.Printf("Demo home created in %s. Try it without touching your real ~/.claude:\n\n  HOME=%s quipu ui\n", dir, dir)
	return nil
}
