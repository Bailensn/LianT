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
	"regexp"
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
// client's dependencies into it, all into root/runtime. deps carries the raw
// requirement specs (e.g. "PySide6>=6.6,<6.9") read from the client's
// pyproject.toml. It is invoked at install time (or on first launch) when the
// interpreter is found to be absent.
func ProvisionRuntime(root, url, checksum string, deps []string) (string, error) {
	return ProvisionRuntimeFrom(root, url, checksum, deps, "")
}

// ProvisionRuntimeFrom is ProvisionRuntime with an optional offlineDepsDir: a
// directory of pre-bundled wheels/sdists. When non-empty and usable, the
// client's dependencies are installed from it (offline) instead of PyPI.
func ProvisionRuntimeFrom(root, url, checksum string, deps []string, offlineDepsDir string) (string, error) {
	runtimeDir := filepath.Join(root, "runtime")
	if py := pythonInterpreter(runtimeDir); py != "" {
		// Already provisioned; ensure deps are present.
		if len(deps) > 0 {
			if err := installDeps(runtimeDir, deps, offlineDepsDir); err != nil {
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
	if len(deps) > 0 {
		if err := installDeps(runtimeDir, deps, offlineDepsDir); err != nil {
			return py, err
		}
	}
	return py, nil
}

// installDeps installs deps into the interpreter at runtimeDir, preferring the
// offline directory when it holds any wheels, else falling back to PyPI.
func installDeps(runtimeDir string, deps []string, offlineDepsDir string) error {
	if len(deps) == 0 {
		return nil
	}
	py := pythonInterpreter(runtimeDir)
	if py == "" {
		return fmt.Errorf("runtime interpreter missing; reinstall runtime")
	}
	if IsOfflineDepsDir(offlineDepsDir) {
		if err := InstallSpecsOffline(runtimeDir, offlineDepsDir, deps); err == nil {
			return nil
		}
		// Offline artifacts couldn't satisfy all specs; retry on PyPI.
	}
	return InstallSpecsFrom(py, deps)
}

// pipIndex returns the PyPI index the launcher uses for online installs.
// The default is TUNA (Tsinghua mirror) because reaching pypi.org is often
// very slow; override with LIANT_PIP_INDEX (e.g. https://pypi.org/simple).
func pipIndex() string {
	if v := os.Getenv("LIANT_PIP_INDEX"); v != "" {
		return v
	}
	return "https://pypi.tuna.tsinghua.edu.cn/simple"
}

// InstallSpecs runs the runtime interpreter's pip to install the client's
// dependencies from their pyproject.toml requirement specs.
func InstallSpecs(root string, deps []string) error {
	runtimeDir := filepath.Join(root, "runtime")
	py := pythonInterpreter(runtimeDir)
	if py == "" {
		return fmt.Errorf("runtime interpreter missing; reinstall runtime")
	}
	return InstallSpecsFrom(py, deps)
}

// InstallSpecsFrom pip-installs all deps into py online via the mirror index.
func InstallSpecsFrom(py string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}
	args := append([]string{"-m", "pip", "install", "--disable-pip-version-check",
		"-q", "-i", pipIndex()}, deps...)
	cmd := exec.Command(py, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// quotedValue extracts the value of a single "..." or '...' string, tolerating
// a trailing comma as found inside TOML arrays.
var quotedValue = regexp.MustCompile(`^\s*(["'])(.*?)\1\s*,?\s*(?:#.*)?$`)

// ReadDependencies parses the [project].dependencies array from a PEP 621
// pyproject.toml and returns the raw requirement specs it declares (e.g.
// "PySide6>=6.6,<6.9"), skipping comments, empty items and table markers.
func ReadDependencies(pyproject string) ([]string, error) {
	data, err := os.ReadFile(pyproject)
	if err != nil {
		return nil, err
	}
	var deps []string
	inProject := false
	inArray := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			// New table: track only [project], and leave any dependency
			// sub-tables (e.g. [project.optional-dependencies]).
			inProject = line == "[project]"
			inArray = false
			continue
		}
		if !inProject {
			continue
		}
		// Enter the dependencies = [ ... ] array (possibly spanning lines).
		if !inArray && strings.HasPrefix(line, "dependencies") && strings.Contains(line, "[") {
			inArray = true
		}
		if !inArray {
			continue
		}
		if m := quotedValue.FindStringSubmatch(line); len(m) == 3 {
			deps = append(deps, m[2])
		}
		if strings.Contains(line, "]") && !strings.Contains(line, "["+"\"") && strings.HasSuffix(line, "]") {
			inArray = false
		}
	}
	return deps, nil
}

// TopLevelName reduces a requirement spec to its top-level import/module name.
func TopLevelName(spec string) string {
	name := spec
	if i := strings.Index(name, ";"); i >= 0 { // env marker
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	if i := strings.Index(name, "["); i >= 0 { // extras
		name = name[:i]
	}
	if i := strings.IndexAny(name, "=<>!~"); i >= 0 { // version constraints
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// MissingDeps returns the subset of mods that cannot be imported by py, i.e.
// the dependencies the runtime still lacks.
func MissingDeps(py string, mods []string) ([]string, error) {
	if len(mods) == 0 {
		return nil, nil
	}
	// Single python invocation; with `python -c <script> a b c` the args start
	// at sys.argv[1].
	script := "import importlib,sys\n" +
		"missing=[]\n" +
		"for m in sys.argv[1:]:\n" +
		"  try:\n" +
		"    importlib.import_module(m)\n" +
		"  except Exception:\n" +
		"    missing.append(m)\n" +
		"if missing:\n" +
		"  print('\\n'.join(missing))\n"
	args := append([]string{"-c", script}, mods...)
	out, err := exec.Command(py, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("probe imports: %w", err)
	}
	var missing []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			missing = append(missing, l)
		}
	}
	return missing, nil
}

// InstallModule pip-installs a single dependency via the mirror index,
// preserving any version constraints carried in the raw spec.
func InstallModule(py, spec string) error {
	cmd := exec.Command(py, "-m", "pip", "install", "--disable-pip-version-check",
		"-i", pipIndex(), spec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsOfflineDepsDir reports whether dir contains at least one discoverable
// package file (wheel or source distribution) pip can install offline from.
func IsOfflineDepsDir(dir string) bool {
	if dir == "" {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if strings.HasSuffix(n, ".whl") || strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".zip") {
			return true
		}
	}
	return false
}

// InstallModuleOffline installs a single dependency from an offline directory,
// disabling PyPI so only local artifacts are used. It returns an error only if
// the installation genuinely fails (including "no matching distribution"),
// which lets the caller fall back to the network for a full-spec match.
func InstallModuleOffline(py, dir, spec string) error {
	cmd := exec.Command(py, "-m", "pip", "install",
		"--disable-pip-version-check",
		"--no-index",
		"--find-links", dir,
		spec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// InstallSpecsOffline installs all deps in one pip invocation from an offline
// directory (used during full runtime provisioning when pyproject was just read).
func InstallSpecsOffline(root, dir string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}
	runtimeDir := filepath.Join(root, "runtime")
	py := pythonInterpreter(runtimeDir)
	if py == "" {
		return fmt.Errorf("runtime interpreter missing; reinstall runtime")
	}
	args := append([]string{"-m", "pip", "install", "--disable-pip-version-check",
		"--no-index", "--find-links", dir}, deps...)
	cmd := exec.Command(py, args...)
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