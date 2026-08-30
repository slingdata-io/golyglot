//go:build windows

package golyglot

import (
	"fmt"
	"syscall"
)

func openLibrary(path string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, fmt.Errorf("LoadLibrary(%s): %w", path, err)
	}
	return uintptr(handle), nil
}
