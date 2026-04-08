package golyglot

import "unsafe"

// cString converts a Go string to a null-terminated byte slice suitable for passing to C.
func cString(s string) []byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	b[len(s)] = 0
	return b
}

// goString converts a C *byte (null-terminated) to a Go string.
// Returns empty string if ptr is nil.
func goString(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	// Walk the bytes to find the null terminator
	p := unsafe.Pointer(ptr)
	length := 0
	for {
		if *(*byte)(unsafe.Add(p, length)) == 0 {
			break
		}
		length++
	}
	if length == 0 {
		return ""
	}
	return string(unsafe.Slice(ptr, length))
}
