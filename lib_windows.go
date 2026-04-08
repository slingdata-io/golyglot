//go:build windows

package golyglot

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

func openLibrary(path string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, fmt.Errorf("LoadLibrary(%s): %w", path, err)
	}
	return uintptr(handle), nil
}

// On Windows, purego.Dlsym won't work with syscall handles.
// We override it in init.go's initLib by using GetProcAddress.
func init() {
	_ = unsafe.Pointer(nil) // ensure import
	_ = purego.SyscallN     // ensure import
}
