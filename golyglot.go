// Package golyglot provides Go bindings for the polyglot-sql library,
// a multi-dialect SQL parser, formatter, and transpiler.
//
// The library is loaded on first use from a path
// specified by the GOLYGLOT_LIBRARY_PATH environment variable.
// If not found, it is automatically downloaded from GitHub releases.
package golyglot

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

const (
	// Version of the polyglot-sql FFI library to download.
	Version = "1.0.17"

	// GitHub release URL pattern for downloading the FFI library.
	releaseURLPattern = "https://github.com/slingdata-io/golyglot/releases/download/v%s/polyglot-sql-ffi-%s-%s.%s"

	StatusSuccess            = 0
	StatusParseError         = 1
	StatusGenerateError      = 2
	StatusTranspileError     = 3
	StatusValidationError    = 4
	StatusInvalidArgument    = 5
	StatusSerializationError = 6
	StatusInternalError      = 99
)

var (
	initOnce  sync.Once
	initErr   error
	libHandle uintptr

	// FFI function pointers registered via purego.RegisterLibFunc.
	// polyglot_result_t is {char* data, char* error, int32 status} = 20 bytes (padded to 24).
	// On ARM64 (AAPCS), structs > 16 bytes are returned via x8 (indirect).
	// purego.RegisterLibFunc handles this transparently when the Go func returns a struct.
	ffiParse     func(sql uintptr, dialect uintptr) ffiResult
	ffiParseOne  func(sql uintptr, dialect uintptr) ffiResult
	ffiGenerate  func(astJSON uintptr, dialect uintptr) ffiResult
	ffiTranspile func(sql uintptr, fromDialect uintptr, toDialect uintptr) ffiResult
	ffiFormat    func(sql uintptr, dialect uintptr) ffiResult
	ffiTokenize  func(sql uintptr, dialect uintptr) ffiResult

	ffiValidate func(sql uintptr, dialect uintptr) ffiValidationResult

	ffiVersion      func() uintptr
	ffiDialectList  func() uintptr
	ffiDialectCount func() int32

	ffiFreeString     func(s uintptr)
	ffiFreeResult     func(result ffiResult)
	ffiFreeValidation func(result ffiValidationResult)
)

// ffiResult mirrors the C polyglot_result_t struct.
type ffiResult struct {
	Data   uintptr // char* data
	Error  uintptr // char* error
	Status int32   // status code
	_      int32   // padding
}

// ffiValidationResult mirrors the C polyglot_validation_result_t struct.
type ffiValidationResult struct {
	Valid      int32   // 1 if valid
	_          int32   // padding
	ErrorsJSON uintptr // char* errors_json
	Error      uintptr // char* error
	Status     int32   // status code
	_          int32   // padding
}

// ParseResult holds the result of parsing SQL.
type ParseResult struct {
	AST json.RawMessage // JSON array of AST expression nodes
}

// TranspileResult holds the result of transpiling SQL.
type TranspileResult struct {
	SQL string
}

// FormatResult holds the result of formatting SQL.
type FormatResult struct {
	SQL string
}

// ValidationResult holds the result of validating SQL.
type ValidationResult struct {
	Valid  bool
	Errors json.RawMessage // JSON array of validation errors
}

// TokenizeResult holds the result of tokenizing SQL.
type TokenizeResult struct {
	Tokens json.RawMessage
}

// Init loads the polyglot-sql FFI library. It is called automatically on first use,
// but can be called explicitly to trigger download/loading early.
func Init() error {
	initOnce.Do(func() {
		initErr = initLib()
	})
	return initErr
}

// ptrArg converts a byte slice to uintptr for passing to C.
func ptrArg(b []byte) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

// uintptrToString converts a uintptr C string to Go string.
func uintptrToString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	// Convert uintptr to *byte via double pointer indirection to satisfy go vet.
	// #nosec G103 -- required for FFI interop
	return goString((*byte)(*(*unsafe.Pointer)(unsafe.Pointer(&ptr))))
}

// Parse parses SQL into an AST JSON array of Expression nodes.
func Parse(sql, dialect string) (*ParseResult, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	sqlBytes := cString(sql)
	dialectBytes := cString(dialect)
	result := ffiParse(ptrArg(sqlBytes), ptrArg(dialectBytes))
	defer ffiFreeResult(result)

	if result.Status != StatusSuccess {
		return nil, fmt.Errorf("parse error: %s", uintptrToString(result.Error))
	}
	return &ParseResult{AST: json.RawMessage(uintptrToString(result.Data))}, nil
}

// ParseOne parses a single SQL statement into an AST JSON object.
func ParseOne(sql, dialect string) (*ParseResult, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	sqlBytes := cString(sql)
	dialectBytes := cString(dialect)
	result := ffiParseOne(ptrArg(sqlBytes), ptrArg(dialectBytes))
	defer ffiFreeResult(result)

	if result.Status != StatusSuccess {
		return nil, fmt.Errorf("parse error: %s", uintptrToString(result.Error))
	}
	return &ParseResult{AST: json.RawMessage(uintptrToString(result.Data))}, nil
}

// Generate converts AST JSON back to SQL string.
func Generate(astJSON, dialect string) (*TranspileResult, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	astBytes := cString(astJSON)
	dialectBytes := cString(dialect)
	result := ffiGenerate(ptrArg(astBytes), ptrArg(dialectBytes))
	defer ffiFreeResult(result)

	if result.Status != StatusSuccess {
		return nil, fmt.Errorf("generate error: %s", uintptrToString(result.Error))
	}
	data := uintptrToString(result.Data)
	return &TranspileResult{SQL: extractSQLFromJSON(data)}, nil
}

// Transpile converts SQL from one dialect to another.
func Transpile(sql, fromDialect, toDialect string) (*TranspileResult, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	sqlBytes := cString(sql)
	fromBytes := cString(fromDialect)
	toBytes := cString(toDialect)
	result := ffiTranspile(ptrArg(sqlBytes), ptrArg(fromBytes), ptrArg(toBytes))
	defer ffiFreeResult(result)

	if result.Status != StatusSuccess {
		return nil, fmt.Errorf("transpile error: %s", uintptrToString(result.Error))
	}
	data := uintptrToString(result.Data)
	return &TranspileResult{SQL: extractSQLFromJSON(data)}, nil
}

// Format pretty-prints SQL for a dialect.
func Format(sql, dialect string) (*FormatResult, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	sqlBytes := cString(sql)
	dialectBytes := cString(dialect)
	result := ffiFormat(ptrArg(sqlBytes), ptrArg(dialectBytes))
	defer ffiFreeResult(result)

	if result.Status != StatusSuccess {
		return nil, fmt.Errorf("format error: %s", uintptrToString(result.Error))
	}
	data := uintptrToString(result.Data)
	return &FormatResult{SQL: extractSQLFromJSON(data)}, nil
}

// Validate checks SQL syntax for a dialect.
func Validate(sql, dialect string) (*ValidationResult, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	sqlBytes := cString(sql)
	dialectBytes := cString(dialect)
	result := ffiValidate(ptrArg(sqlBytes), ptrArg(dialectBytes))

	if result.Status != StatusSuccess {
		errMsg := uintptrToString(result.Error)
		ffiFreeValidation(result)
		return nil, fmt.Errorf("validation error: %s", errMsg)
	}

	vr := &ValidationResult{Valid: result.Valid == 1}
	if result.ErrorsJSON != 0 {
		vr.Errors = json.RawMessage(uintptrToString(result.ErrorsJSON))
	}
	ffiFreeValidation(result)
	return vr, nil
}

// Tokenize splits SQL into tokens.
func Tokenize(sql, dialect string) (*TokenizeResult, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	sqlBytes := cString(sql)
	dialectBytes := cString(dialect)
	result := ffiTokenize(ptrArg(sqlBytes), ptrArg(dialectBytes))
	defer ffiFreeResult(result)

	if result.Status != StatusSuccess {
		return nil, fmt.Errorf("tokenize error: %s", uintptrToString(result.Error))
	}
	return &TokenizeResult{Tokens: json.RawMessage(uintptrToString(result.Data))}, nil
}

// LibVersion returns the version string of the loaded polyglot-sql library.
func LibVersion() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	ptr := ffiVersion()
	if ptr == 0 {
		return "", fmt.Errorf("version returned nil")
	}
	// version pointer is statically allocated, do NOT free
	return uintptrToString(ptr), nil
}

// DialectList returns the list of supported dialect names as JSON.
func DialectList() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	ptr := ffiDialectList()
	if ptr == 0 {
		return "", fmt.Errorf("dialect_list returned nil")
	}
	s := uintptrToString(ptr)
	ffiFreeString(ptr)
	return s, nil
}

// DialectCount returns the number of supported dialects.
func DialectCount() (int, error) {
	if err := Init(); err != nil {
		return 0, err
	}
	return int(ffiDialectCount()), nil
}

// extractSQLFromJSON parses the JSON result from polyglot FFI functions
// that return SQL. The format is typically a JSON array of SQL strings: ["SQL"]
func extractSQLFromJSON(data string) string {
	var arr []string
	if err := json.Unmarshal([]byte(data), &arr); err == nil && len(arr) > 0 {
		return arr[0]
	}
	return data
}

// releaseAssetInfo returns the platform-specific release asset details.
func releaseAssetInfo() (osName, archName, ext string) {
	switch runtime.GOOS {
	case "darwin":
		osName = "macos"
	case "linux":
		osName = "linux"
	case "windows":
		osName = "windows"
	default:
		osName = runtime.GOOS
	}

	switch runtime.GOARCH {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "aarch64"
	default:
		archName = runtime.GOARCH
	}

	if runtime.GOOS == "windows" {
		ext = "zip"
	} else {
		ext = "tar.gz"
	}
	return
}
