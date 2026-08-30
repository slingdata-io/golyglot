//go:build !windows

package golyglot

import "github.com/ebitengine/purego"

// registerStructFuncs registers the struct-returning FFI functions.
// purego handles struct returns directly on darwin and linux,
// including the ARM64 x8 indirect-return convention.
func registerStructFuncs(lib uintptr) {
	purego.RegisterLibFunc(&ffiParse, lib, "polyglot_parse")
	purego.RegisterLibFunc(&ffiParseOne, lib, "polyglot_parse_one")
	purego.RegisterLibFunc(&ffiGenerate, lib, "polyglot_generate")
	purego.RegisterLibFunc(&ffiTranspile, lib, "polyglot_transpile")
	purego.RegisterLibFunc(&ffiFormat, lib, "polyglot_format")
	purego.RegisterLibFunc(&ffiValidate, lib, "polyglot_validate")
	purego.RegisterLibFunc(&ffiTokenize, lib, "polyglot_tokenize")
	purego.RegisterLibFunc(&ffiFreeResult, lib, "polyglot_free_result")
	purego.RegisterLibFunc(&ffiFreeValidation, lib, "polyglot_free_validation_result")
}
