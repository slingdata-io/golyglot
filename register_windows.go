//go:build windows

package golyglot

import "github.com/ebitengine/purego"

// purego refuses struct arguments and struct returns on Windows
// (ensureStructSupportedForRegisterFunc panics). The Win64 ABI passes any
// aggregate larger than 8 bytes indirectly, so we register the same exported
// symbols with pointer signatures and wrap them to keep the struct-based call
// sites unchanged:
//
//   - a struct return becomes a hidden first argument: a pointer to
//     caller-allocated space that the callee fills and returns.
//   - a struct argument becomes a pointer to a caller-allocated copy.
//
// ffiResult and ffiValidationResult are both 24 bytes, so both rules apply.
func registerStructFuncs(lib uintptr) {
	var (
		parse     func(ret *ffiResult, sql, dialect uintptr) uintptr
		parseOne  func(ret *ffiResult, sql, dialect uintptr) uintptr
		generate  func(ret *ffiResult, astJSON, dialect uintptr) uintptr
		transpile func(ret *ffiResult, sql, from, to uintptr) uintptr
		format    func(ret *ffiResult, sql, dialect uintptr) uintptr
		tokenize  func(ret *ffiResult, sql, dialect uintptr) uintptr

		validate func(ret *ffiValidationResult, sql, dialect uintptr) uintptr

		freeResult     func(result *ffiResult)
		freeValidation func(result *ffiValidationResult)
	)

	purego.RegisterLibFunc(&parse, lib, "polyglot_parse")
	purego.RegisterLibFunc(&parseOne, lib, "polyglot_parse_one")
	purego.RegisterLibFunc(&generate, lib, "polyglot_generate")
	purego.RegisterLibFunc(&transpile, lib, "polyglot_transpile")
	purego.RegisterLibFunc(&format, lib, "polyglot_format")
	purego.RegisterLibFunc(&tokenize, lib, "polyglot_tokenize")
	purego.RegisterLibFunc(&validate, lib, "polyglot_validate")
	purego.RegisterLibFunc(&freeResult, lib, "polyglot_free_result")
	purego.RegisterLibFunc(&freeValidation, lib, "polyglot_free_validation_result")

	ffiParse = func(sql, dialect uintptr) ffiResult {
		var out ffiResult
		parse(&out, sql, dialect)
		return out
	}
	ffiParseOne = func(sql, dialect uintptr) ffiResult {
		var out ffiResult
		parseOne(&out, sql, dialect)
		return out
	}
	ffiGenerate = func(astJSON, dialect uintptr) ffiResult {
		var out ffiResult
		generate(&out, astJSON, dialect)
		return out
	}
	ffiTranspile = func(sql, from, to uintptr) ffiResult {
		var out ffiResult
		transpile(&out, sql, from, to)
		return out
	}
	ffiFormat = func(sql, dialect uintptr) ffiResult {
		var out ffiResult
		format(&out, sql, dialect)
		return out
	}
	ffiTokenize = func(sql, dialect uintptr) ffiResult {
		var out ffiResult
		tokenize(&out, sql, dialect)
		return out
	}
	ffiValidate = func(sql, dialect uintptr) ffiValidationResult {
		var out ffiValidationResult
		validate(&out, sql, dialect)
		return out
	}

	ffiFreeResult = func(result ffiResult) { freeResult(&result) }
	ffiFreeValidation = func(result ffiValidationResult) { freeValidation(&result) }
}
