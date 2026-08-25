// Package runtime locates the Python runtime and manages the Client subprocess.
//
// Supported layouts:
//
//	# Development (source tree)
//	<root>/client/src/main.py            -> client source
//	<root>/client/qml                    -> QML sources
//	<root>/client/runtime/bin/python     -> interpreter (or PATH)
//	<root>/client/resources              -> static resources
//
//	# Installed (Linux deb/rpm, see packaging)
//	/opt/LianT/bin/LianT               -> this launcher binary
//	/opt/LianT/src/main.py              -> client source
//	/opt/LianT/qml                      -> QML sources
//	/opt/LianT/runtime/bin/python       -> interpreter (downloaded on first run)
//	/opt/LianT/resources                -> static resources
//
//	# Installed (Windows Inno)
//	<app>/LianT.exe                     -> this launcher binary
//	<app>/client/src/main.py            -> client source
//	<app>/client/runtime/python.exe     -> interpreter (downloaded on first run)
//
// The launcher does NOT embed client code. On every run it checks ./runtime for
// a Python interpreter; if absent it downloads a pinned standalone build there,
// then spawns a new process to run src/main.py.
package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Manager resolves paths and owns the client process lifecycle.
type Manager struct {
	// Root is the directory that contains client/src (or /opt/LianT).
	Root string
	// LibDir is the client root: <root>/client in dev, /opt/LianT when
	// installed on Linux. It also holds the Python runtime at LibDir/runtime.
	LibDir string
	// PythonEnvOverride, if set, forces a specific python interpreter.
	PythonEnvOverride string
}

// NewManager locates the launcher root and client library directory.
func NewManager() (*Manager, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}
	dir := filepath.Dir(exe)

	m := &Manager{}

	// Installed layout (Linux): binary in /opt/LianT/bin, client root is the
	// parent /opt/LianT.
	if installLib := detectInstalledLib(dir); installLib != "" {
		m.LibDir = installLib
		m.Root = filepath.Dir(dir)
		return m, nil
	}

	// macOS .app layout: LianT.app/Contents/MacOS/LianT with the bundled
	// client under LianT.app/Contents/Resources/client.
	if filepath.Base(dir) == "MacOS" {
		resDir := filepath.Join(filepath.Dir(dir), "Resources")
		if st, err := os.Stat(filepath.Join(resDir, "client", "src")); err == nil && st.IsDir() {
			m.Root = resDir
			m.LibDir = filepath.Join(resDir, "client")
			return m, nil
		}
	}

	// Dev layout: <root>/client.
	// If binary is inside desktop/launcher, the repo root is two levels up.
	if filepath.Base(dir) == "launcher" {
		dir = filepath.Dir(filepath.Dir(dir)) // -> desktop
	}
	m.Root = dir
	m.LibDir = filepath.Join(dir, "client")
	return m, nil
}

// detectInstalledLib resolves the client root from the launcher's bin
// directory. It covers several installed layouts:
//   - /opt/LianT/bin/LianT -> /opt/LianT (Linux: launcher + client + runtime)
//   - /usr/bin/LianT -> /usr/lib/LianT (legacy Linux rootfs)
//   - <app>/LianT.exe with client beside it -> <app>/client (Windows Inno)
func detectInstalledLib(binDir string) string {
	candidates := []string{
		filepath.Join(binDir, ".."),                     // /opt/LianT/bin -> /opt/LianT
		filepath.Join(binDir, "..", "lib", "LianT"),     // /usr/bin -> /usr/lib/LianT
		filepath.Join(binDir, "..", "lib64", "LianT"),
		filepath.Join(binDir, "client"),                 // <app>/LianT -> <app>/client
	}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if st, err := os.Stat(filepath.Join(c, "src")); err == nil && st.IsDir() {
			return c
		}
	}
	return ""
}

// ClientSrc returns the client source directory.
func (m *Manager) ClientSrc() string {
	for _, p := range []string{
		filepath.Join(m.LibDir, "src"),
		filepath.Join(m.LibDir, "client", "src"),
	} {
		if fileExists(filepath.Join(p, "main.py")) {
			return p
		}
	}
	return filepath.Join(m.LibDir, "src")
}

// RuntimeDir returns the Python runtime directory, always next to the client:
// <libdir>/runtime (i.e. ./runtime relative to the install root).
func (m *Manager) RuntimeDir() string {
	for _, p := range []string{
		filepath.Join(m.LibDir, "runtime"),
		filepath.Join(m.LibDir, "client", "runtime"),
	} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return filepath.Join(m.LibDir, "runtime")
}

// ResourcesDir returns the static resources directory.
func (m *Manager) ResourcesDir() string {
	for _, p := range []string{
		filepath.Join(m.LibDir, "resources"),
		filepath.Join(m.LibDir, "client", "resources"),
	} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return filepath.Join(m.LibDir, "resources")
}

// pythonCandidates returns the interpreter paths to probe within the runtime
// dir, covering both a venv layout (bin/Scripts) and the
// python-build-standalone layout (runtime/bin or runtime/python).
func pythonCandidates(runtimeDir string) []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(runtimeDir, "python", "python.exe"), // standalone
			filepath.Join(runtimeDir, "Scripts", "python.exe"), // venv
			filepath.Join(runtimeDir, "python.exe"),
		}
	}
	return []string{
		filepath.Join(runtimeDir, "bin", "python3"), // standalone/venv
		filepath.Join(runtimeDir, "bin", "python"),
		filepath.Join(runtimeDir, "python3"),
	}
}

// FindPython returns the interpreter for the client, in priority order:
//  1. explicit override (LIANT_PYTHON / -python)
//  2. the ./runtime directory next to the client
//
// It deliberately does NOT fall back to a PATH python: the launcher checks
// ./runtime and downloads a matching runtime if none is present.
func (m *Manager) FindPython() (string, error) {
	if m.PythonEnvOverride != "" {
		if st, err := os.Stat(m.PythonEnvOverride); err == nil && !st.IsDir() {
			return m.PythonEnvOverride, nil
		}
	}
	for _, p := range pythonCandidates(m.RuntimeDir()) {
		if fileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Python runtime found in %s", m.RuntimeDir())
}

// SourceDirExists reports whether the client entry point is present.
func (m *Manager) SourceDirExists() bool {
	return fileExists(filepath.Join(m.ClientSrc(), "main.py"))
}

// Command builds the exec.Cmd that launches the client source.
func (m *Manager) Command(python string) (*exec.Cmd, error) {
	mainPath := filepath.Join(m.ClientSrc(), "main.py")
	if !fileExists(mainPath) {
		return nil, fmt.Errorf("client entry not found: %s", mainPath)
	}
	cmd := exec.Command(python, mainPath)
	cmd.Dir = m.ClientSrc()
	cmd.Env = append(os.Environ(),
		"LIANT_CLIENT_SRC="+m.ClientSrc(),
		"LIANT_RESOURCES="+m.ResourcesDir(),
	)
	if v := os.Getenv("LIANT_VERSION"); v != "" {
		cmd.Env = append(cmd.Env, "LIANT_VERSION="+v)
	}
	return cmd, nil
}

// RequirementsFile returns the client's Python requirements file path, or ""
// when not shipped.
func (m *Manager) RequirementsFile() string {
	for _, p := range []string{
		filepath.Join(m.LibDir, "requirements.txt"),
		filepath.Join(m.ClientSrc(), "..", "requirements.txt"),
	} {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// EnsureRuntime checks interpreter + client source presence and returns a
// friendly error with remediation hints when something is missing.
func (m *Manager) EnsureRuntime() error {
	if _, err := m.FindPython(); err != nil {
		return fmt.Errorf("%w\n\nRuntime not ready at %s. The launcher will download it on first run (set LIANT_RUNTIME_URL / LIANT_PYTHON as needed).",
			err, m.RuntimeDir())
	}
	if !m.SourceDirExists() {
		return fmt.Errorf("client source missing at %s; reinstall or run updater", m.ClientSrc())
	}
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// IsWindows reports the build GOOS.
func IsWindows() bool { return runtime.GOOS == "windows" }