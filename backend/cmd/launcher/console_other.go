//go:build !windows

package main

// setupConsole is a no-op: Unix terminals handle ANSI colors and UTF-8 already.
func setupConsole() {}
