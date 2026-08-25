// Package updater handles dynamic download and provisioning of the Python
// runtime. It does NOT bundle the interpreter — it fetches a standalone build,
// extracts it, and installs the client's Python dependencies into it at install time.
package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Profile describes a downloadable artifact.
type Profile struct {
	Name    string // file name (zip)
	URL     string // download URL
	SHA256  string // expected checksum (optional)
	DestDir string // extraction destination
}

// Fetch downloads a file to a temp path, verifying checksum when given.
func Fetch(url, dest, expectedSHA string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if expectedSHA != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, expectedSHA) {
			os.Remove(tmp)
			return fmt.Errorf("checksum mismatch: got %s want %s", got, expectedSHA)
		}
	}
	return os.Rename(tmp, dest)
}

// Unzip extracts archive into destDir, sanitizing paths.
func Unzip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rel := filepath.Clean(f.Name)
		full := filepath.Join(destDir, rel)
		if !strings.HasPrefix(full, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe archive path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(full)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err = io.Copy(w, rc); err != nil {
			rc.Close()
			w.Close()
			return err
		}
		rc.Close()
		w.Close()
	}
	return nil
}

// DownloadDir returns the cache directory for downloads.
func DownloadDir(root string) string {
	return filepath.Join(root, "runtime", "_downloads")
}

// pythonInterpreter returns the interpreter path inside an extracted standalone
// runtime, matching python-build-standalone / venv layouts.
func pythonInterpreter(runtimeDir string) string {
	candidates := []string{
		filepath.Join(runtimeDir, "python", "python.exe"), // windows standalone
		filepath.Join(runtimeDir, "python.exe"),
		filepath.Join(runtimeDir, "bin", "python3"), // unix standalone/venv
		filepath.Join(runtimeDir, "bin", "python"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// ExtractTarGz extracts a .tar.gz archive into destDir, sanitizing paths.
// Returns the first entry directory if a top-level wrapper directory exists,
// so callers can strip it (python-build-standalone archives wrap in a dir).
func ExtractTarGz(tarPath, destDir string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var top string
	setTop := func(name string) {
		first := strings.SplitN(strings.TrimPrefix(name, "./"), "/", 2)[0]
		if top != first && first != top {
			if top == "" {
				top = first
			}
		}
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "" {
			continue
		}
		setTop(hdr.Name)
		rel := filepath.Clean(name)
		full := filepath.Join(destDir, rel)
		if !strings.HasPrefix(full, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("unsafe archive path: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			w, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return "", err
			}
			w.Close()
		}
	}
	return top, nil
}

// stripTopLevel flattens a wrapper directory created by ExtractTarGz into
// dir, so that dir/bin/python (or dir/python.exe) sits directly under dir.
func stripTopLevel(dir, top string) error {
	if top == "" || top == "." {
		return nil
	}
	wrap := filepath.Join(dir, top)
	if st, err := os.Stat(wrap); err != nil || !st.IsDir() {
		return nil
	}
	// Move contents of wrap up one level, then remove wrap.
	entries, err := os.ReadDir(wrap)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(wrap, e.Name())
		if _, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
		if err := os.Rename(from, filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return os.Remove(wrap)
}

// ProvisionRuntime downloads a standalone interpreter and pip-installs the
// client's requirements into it, all into root/runtime. It is invoked at
// install time (or on first launch) when the interpreter is found to be absent.
func ProvisionRuntime(root, url, checksum, requirements string) (string, error) {
	runtimeDir := filepath.Join(root, "runtime")
	if py := pythonInterpreter(runtimeDir); py != "" {
		// Already provisioned; ensure deps are present.
		if requirements != "" && fileExists(requirements) {
			if err := InstallRequirements(root, requirements); err != nil {
				return py, err
			}
		}
		return py, nil
	}

	dl := DownloadDir(root)
	if err := os.MkdirAll(dl, 0o755); err != nil {
		return "", err
	}
	archivePath := filepath.Join(dl, "python-bundle")
	if !fileExists(archivePath + ".tar.gz") && !fileExists(archivePath + ".zip") {
		if url == "" {
			return "", fmt.Errorf("runtime not installed and no runtime URL configured")
		}
		src, err := fetchTo(url, archivePath+".tar.gz", checksum)
		if err != nil {
			return "", err
		}
		archivePath = src
	}

	stamp := time.Now().Format("20060102150405")
	tmp := runtimeDir + "." + stamp
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}

	if strings.HasSuffix(archivePath, ".tar.gz") {
		top, err := ExtractTarGz(archivePath, tmp)
		if err != nil {
			return "", err
		}
		if err := stripTopLevel(tmp, top); err != nil {
			return "", err
		}
	} else {
		if err := Unzip(archivePath, tmp); err != nil {
			return "", err
		}
	}
	if pythonInterpreter(tmp) == "" {
		return "", fmt.Errorf("downloaded bundle has no python interpreter")
	}

	// Atomic swap.
	if _, err := os.Stat(runtimeDir); err == nil {
		if err := os.RemoveAll(runtimeDir + ".old"); err != nil {
			return "", err
		}
		if err := os.Rename(runtimeDir, runtimeDir+".old"); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, runtimeDir); err != nil {
		return "", err
	}
	_ = os.RemoveAll(runtimeDir + ".old")

	py := pythonInterpreter(runtimeDir)
	if requirements != "" && fileExists(requirements) {
		if err := InstallRequirements(root, requirements); err != nil {
			return py, err
		}
	}
	return py, nil
}

// InstallRequirements runs the runtime interpreter's pip to install the client's
// dependencies from a requirements file.
func InstallRequirements(root, requirements string) error {
	runtimeDir := filepath.Join(root, "runtime")
	py := pythonInterpreter(runtimeDir)
	if py == "" {
		return fmt.Errorf("runtime interpreter missing; reinstall runtime")
	}
	cmd := exec.Command(py, "-m", "pip", "install", "--disable-pip-version-check",
		"-q", "-r", requirements)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fetchTo(url, dest, expectedSHA string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	if expectedSHA != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, expectedSHA) {
			os.Remove(tmp)
			return "", fmt.Errorf("checksum mismatch: got %s want %s", got, expectedSHA)
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}