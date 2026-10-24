//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
	procGetStdHandle   = kernel32.NewProc("GetStdHandle")
	procSetConsoleCP   = kernel32.NewProc("SetConsoleOutputCP")
)

const enableVirtualTerminalProcessing = 0x0004

// setupConsole makes the console render ANSI colors (the Vite output uses them)
// and UTF-8 text instead of mangled bytes.
func setupConsole() {
	procSetConsoleCP.Call(65001)

	handle, _, _ := procGetStdHandle.Call(uintptr(syscall.Stdout))
	var mode uint32
	if ret, _, _ := procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode))); ret != 0 {
		mode |= enableVirtualTerminalProcessing
		procSetConsoleMode.Call(handle, uintptr(mode))
	}
}
