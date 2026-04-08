package golyglot

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ebitengine/purego"
)

// initLib loads the polyglot-sql FFI shared library and registers all function pointers.
func initLib() error {
	libPath, err := resolveLibraryPath()
	if err != nil {
		return fmt.Errorf("could not find polyglot-sql library: %w", err)
	}

	lib, err := openLibrary(libPath)
	if err != nil {
		return fmt.Errorf("could not load polyglot-sql library at %s: %w", libPath, err)
	}
	libHandle = lib

	// Register all FFI functions. purego handles struct returns (including ARM64 x8/sret).
	purego.RegisterLibFunc(&ffiParse, lib, "polyglot_parse")
	purego.RegisterLibFunc(&ffiParseOne, lib, "polyglot_parse_one")
	purego.RegisterLibFunc(&ffiGenerate, lib, "polyglot_generate")
	purego.RegisterLibFunc(&ffiTranspile, lib, "polyglot_transpile")
	purego.RegisterLibFunc(&ffiFormat, lib, "polyglot_format")
	purego.RegisterLibFunc(&ffiValidate, lib, "polyglot_validate")
	purego.RegisterLibFunc(&ffiTokenize, lib, "polyglot_tokenize")
	purego.RegisterLibFunc(&ffiVersion, lib, "polyglot_version")
	purego.RegisterLibFunc(&ffiDialectList, lib, "polyglot_dialect_list")
	purego.RegisterLibFunc(&ffiDialectCount, lib, "polyglot_dialect_count")
	purego.RegisterLibFunc(&ffiFreeString, lib, "polyglot_free_string")
	purego.RegisterLibFunc(&ffiFreeResult, lib, "polyglot_free_result")
	purego.RegisterLibFunc(&ffiFreeValidation, lib, "polyglot_free_validation_result")

	return nil
}

// resolveLibraryPath finds the shared library, downloading if necessary.
func resolveLibraryPath() (string, error) {
	// 1. Environment variable override
	if path := os.Getenv("GOLYGLOT_LIBRARY_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("GOLYGLOT_LIBRARY_PATH set to %q but file not found", path)
	}

	// 2. Check well-known locations
	libName := libraryFileName()
	for _, dir := range searchPaths() {
		path := filepath.Join(dir, libName)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 3. Download on first use
	return downloadLibrary()
}

// libraryFileName returns the platform-specific shared library file name.
func libraryFileName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libpolyglot_sql_ffi.dylib"
	case "windows":
		return "polyglot_sql_ffi.dll"
	default:
		return "libpolyglot_sql_ffi.so"
	}
}

// searchPaths returns directories to search for the library.
func searchPaths() []string {
	paths := []string{}

	if libDir := os.Getenv("GOLYGLOT_LIBRARY_FOLDER"); libDir != "" {
		paths = append(paths, libDir)
	}

	// Current directory
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, cwd)
	}

	return paths
}

// downloadLibrary fetches the library from GitHub releases to ~/.sling/lib/.
func downloadLibrary() (string, error) {

	libDir := os.Getenv("GOLYGLOT_LIBRARY_FOLDER")
	if libDir == "" {
		return "", fmt.Errorf("need to specify env var GOLYGLOT_LIBRARY_FOLDER")
	} else if err := os.MkdirAll(libDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", libDir, err)
	}

	osName, archName, ext := releaseAssetInfo()

	// Check platform support
	if runtime.GOOS == "windows" && runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("polyglot-sql FFI not available for windows/%s", runtime.GOARCH)
	}
	if runtime.GOOS == "linux" && runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return "", fmt.Errorf("polyglot-sql FFI not available for linux/%s", runtime.GOARCH)
	}

	url := fmt.Sprintf(releaseURLPattern, Version, osName, archName, ext)
	fmt.Fprintf(os.Stderr, "Downloading polyglot-sql v%s (%s/%s)...\n", Version, osName, archName)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, url)
	}

	libName := libraryFileName()
	destPath := filepath.Join(libDir, libName)

	if ext == "zip" {
		err = extractFromZip(resp.Body, libDir, libName)
	} else {
		err = extractFromTarGz(resp.Body, libDir, libName)
	}
	if err != nil {
		return "", fmt.Errorf("extraction failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Saved to %s\n", destPath)
	return destPath, nil
}

// extractFromTarGz extracts the target library file from a .tar.gz archive.
func extractFromTarGz(r io.Reader, destDir, targetFile string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		baseName := filepath.Base(header.Name)
		if baseName != targetFile {
			continue
		}

		destPath := filepath.Join(destDir, targetFile)
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()
		return nil
	}

	return fmt.Errorf("%s not found in archive", targetFile)
}

// extractFromZip extracts the target library file from a .zip archive.
// Since zip requires seeking, we first download to a temp file.
func extractFromZip(r io.Reader, destDir, targetFile string) error {
	// Zip requires random access, write to temp file first
	tmp, err := os.CreateTemp("", "polyglot-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, r); err != nil {
		return err
	}

	stat, err := tmp.Stat()
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(tmp, stat.Size())
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		baseName := filepath.Base(f.Name)
		if baseName != targetFile {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		destPath := filepath.Join(destDir, targetFile)
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			return err
		}
		out.Close()
		return nil
	}

	return fmt.Errorf("%s not found in archive", targetFile)
}

// fixDylibRpath fixes macOS dylibs that have hardcoded rpaths from the build machine.
// The polyglot release dylibs reference a CI path that won't exist on user machines.
func fixDylibRpath(libPath string) {
	if runtime.GOOS != "darwin" {
		return
	}
	// The dylib has a hardcoded install_name from the CI build machine.
	// purego.Dlopen with an absolute path works regardless, so no fix needed.
	// If issues arise, use install_name_tool via os/exec.
	_ = strings.TrimSpace(libPath) // placeholder
}
