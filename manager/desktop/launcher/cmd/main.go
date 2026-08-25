// Command LianT is the LianT desktop launcher.
//
// It does NOT embed client code. On every run it:
//  1. shows a Gio splash window while checking/loading the runtime;
//  2. looks for a Python interpreter in ./runtime (next to the client);
//  3. if absent and a runtime URL is configured, downloads a pinned standalone
//     Python build and pip-installs client deps into ./runtime;
//  4. spawns a NEW process to run src/main.py, then closes the splash.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"gioui.org/app"

	"LianTLauncher/runtime"
	"LianTLauncher/ui"
	"LianTLauncher/updater"
)

// Default standalone-Python build pinned by the launcher when the runtime URL
// is neither injected at link time nor set via LIANT_RUNTIME_URL.
const (
	pbsBase = "https://github.com/astral-sh/python-build-standalone/releases/download/20260814"
	pyVer   = "3.12.14"
	pbsTag  = "20260814"
)

// platformDefaultRuntimeURL returns the python-build-standalone URL for the
// current OS/ARCH, so the launcher works out of the box when double-clicked
// without any injected link-time value or environment variable.
func platformDefaultRuntimeURL() string {
	var tag string
	switch {
	case goruntime.GOOS == "windows" && goruntime.GOARCH == "amd64":
		tag = "x86_64-pc-windows-msvc"
	case goruntime.GOOS == "darwin" && goruntime.GOARCH == "arm64":
		tag = "aarch64-apple-darwin"
	case goruntime.GOOS == "linux" && goruntime.GOARCH == "amd64":
		tag = "x86_64-unknown-linux-gnu"
	case goruntime.GOOS == "linux" && goruntime.GOARCH == "arm64":
		tag = "aarch64-unknown-linux-gnu"
	default:
		return ""
	}
	return fmt.Sprintf("%s/cpython-%s+%s-%s-install_only.tar.gz",
		pbsBase, pyVer, pbsTag, tag)
}

var (
	version = "0.0.0"
	// defaultRuntimeURL is injected at link time with
	// -X main.defaultRuntimeURL=...; it is the source for the dynamically
	// downloaded Python interpreter when ./runtime is empty.
	defaultRuntimeURL = os.Getenv("LIANT_RUNTIME_URL")
	// runtimeChecksum optionally pins the SHA-256 of the runtime bundle.
	runtimeChecksum = os.Getenv("LIANT_RUNTIME_SHA256")
	// skipSplash disables the Gio startup window (headless / debugging).
	skipSplash = os.Getenv("LIANT_NO_SPLASH") != ""
)

func main() {
	var pythonOverride string
	flag.StringVar(&pythonOverride, "python", "", "use a specific Python interpreter instead of ./runtime")
	flag.StringVar(&defaultRuntimeURL, "runtime-url", defaultRuntimeURL, "runtime bundle URL (standalone Python) or explicit interpreter path")
	flag.StringVar(&runtimeChecksum, "runtime-sha256", runtimeChecksum, "expected SHA-256 of the runtime bundle (optional)")
	flag.BoolVar(&skipSplash, "no-splash", skipSplash, "run without the startup window")
	flag.Parse()

	mgr, err := runtime.NewManager()
	if err != nil {
		fatal(err)
	}
	if pythonOverride != "" {
		mgr.PythonEnvOverride = pythonOverride
	}

	if skipSplash {
		// Headless path: identical logic, just no splash window.
		if err := doWork(mgr, nil); err != nil {
			fatal(err)
		}
		supervise()
		return
	}

	// Splash path: show the startup window and run the provisioning + client
	// boot in a background goroutine, feeding status back into the splash.
	splash := ui.NewSplash(version)
	startAt := time.Now()

	var startErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		startErr = doWork(mgr, splash)
		if startErr != nil {
			splash.SetStatus("Startup failed: " + startErr.Error())
			// Keep the error visible for a moment before closing.
			time.Sleep(3000 * time.Millisecond)
		} else if elapsed := time.Since(startAt); elapsed < 800*time.Millisecond {
			// Brief pause so the user can register the splash, then close it.
			time.Sleep(800*time.Millisecond - elapsed)
		}
		splash.Close()
	}()

	// Gio requires app.Main() to run on the main goroutine: it pumps the
	// platform event loop that delivers FrameEvent/DestroyEvent to windows.
	// The splash window processes those events in its own goroutine; app.Main()
	// returns only after the window is destroyed.
	go splash.Run()
	app.Main()
	<-done // wait for the worker goroutine to finish before deciding the outcome

	if startErr != nil {
		fatal(startErr)
	}
	supervise()
}

// supervise keeps the launcher process alive after the client has been
// started, acting as its supervisor.
func supervise() {
	select {}
}

// doWork provisions the runtime and launches the client. splash may be nil
// to run without a window (headless mode).
func doWork(mgr *runtime.Manager, splash *ui.Splash) error {
	setStatus := func(msg string) {
		if splash != nil {
			splash.SetStatus(msg)
		} else {
			fmt.Fprintln(os.Stderr, "LianT:", msg)
		}
	}

	// Step 1+2: ensure a Python runtime exists in ./runtime (download if needed).
	setStatus("Preparing runtime")
	if err := ensureRuntime(mgr); err != nil {
		return err
	}

	// Step 3: make sure the client's Python deps are present, installing any
	// missing ones and reporting each package on the splash.
	setStatus("Checking dependencies")
	if err := ensureDeps(mgr, setStatus); err != nil {
		return err
	}

	// Step 4: spawn a new process to run the client.
	setStatus("Starting LianT")
	if err := launchClient(mgr); err != nil {
		return err
	}
	return nil
}

// ensureDeps verifies the dependencies declared in the client's pyproject.toml
// are importable by the runtime interpreter and installs whatever is missing,
// one at a time, so the splash can report progress (e.g. "正在安装: PySide6").
func ensureDeps(mgr *runtime.Manager, setStatus func(string)) error {
	pp := mgr.DependencyFile()
	if pp == "" {
		return nil
	}
	py, err := mgr.FindPython()
	if err != nil {
		return err
	}
	deps, err := updater.ReadDependencies(pp)
	if err != nil {
		return fmt.Errorf("read pyproject dependencies: %w", err)
	}
	var mods []string
	for _, d := range deps {
		if name := updater.TopLevelName(d); name != "" {
			mods = append(mods, name)
		}
	}
	if len(mods) == 0 {
		return nil
	}
	missing, err := updater.MissingDeps(py, mods)
	if err != nil {
		return fmt.Errorf("dependency check: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}
	// Install only the specs whose top-level name is missing.
	var specs []string
	for _, d := range deps {
		if containsString(missing, updater.TopLevelName(d)) {
			specs = append(specs, d)
		}
	}
	setStatus("缺少依赖: " + strings.Join(missing, ", "))
	offline := mgr.LocalDepsDir()
	useOffline := updater.IsOfflineDepsDir(offline)
	for _, spec := range specs {
		name := updater.TopLevelName(spec)
		if useOffline {
			// Prefer the bundled offline directory (client/deps) so installs
			// don't hit the network; fall back to PyPI on any failure.
			setStatus("正在从本地安装: " + name)
			if err := updater.InstallModuleOffline(py, offline, spec); err == nil {
				continue
			}
			setStatus("本地无匹配，转在线安装: " + name)
		}
		setStatus("正在安装: " + name)
		if err := updater.InstallModule(py, spec); err != nil {
			return fmt.Errorf("install %s: %w", spec, err)
		}
	}
	setStatus("依赖已就绪")
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ensureRuntime checks ./runtime for a Python interpreter; if missing and a
// runtime URL is configured, it downloads the standalone build and installs
// the client's requirements into it.
func ensureRuntime(mgr *runtime.Manager) error {
	if _, err := mgr.FindPython(); err == nil {
		return nil
	}

	// Resolve the runtime source: explicit path / URL wins; otherwise fall back
	// to the platform default so the launcher also works when run directly.
	url := defaultRuntimeURL
	if url == "" {
		url = platformDefaultRuntimeURL()
	}

	if isPath(url) {
		mgr.PythonEnvOverride = url
		if err := mgr.EnsureRuntime(); err != nil {
			return fmt.Errorf("configured python not usable: %w", err)
		}
		return nil
	}
	if url == "" {
		return fmt.Errorf("no Python in %s and no runtime URL configured; set LIANT_RUNTIME_URL or -runtime-url",
			mgr.RuntimeDir())
	}
	req := mgr.DependencyFile()
	fmt.Fprintf(os.Stderr, "downloading Python runtime from %s…\n", url)
	var deps []string
	if req != "" {
		var err error
		deps, err = updater.ReadDependencies(req)
		if err != nil {
			return fmt.Errorf("read pyproject dependencies: %w", err)
		}
	}
	if _, err := updater.ProvisionRuntimeFrom(mgr.LibDir, url, runtimeChecksum, deps, mgr.LocalDepsDir()); err != nil {
		return fmt.Errorf("provision runtime: %w", err)
	}
	return mgr.EnsureRuntime()
}

// isPath reports whether s is a local filesystem path rather than a URL.
func isPath(s string) bool {
	return s != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")
}

// launchClient starts the client as a child process of the launcher.
//
// Because the launcher is built with -H windowsgui (no console window), the
// client's own stdout/stderr would otherwise be discarded, hiding the cause of
// a crash. We tee the client's output (and its interpreter's pip/import logs)
// into <libdir>/launcher.log so failures are diagnosable.
func launchClient(mgr *runtime.Manager) error {
	python, err := mgr.FindPython()
	if err != nil {
		return err
	}
	if !mgr.SourceDirExists() {
		return fmt.Errorf("client source missing at %s", mgr.ClientSrc())
	}
	cmd, err := mgr.Command(python)
	if err != nil {
		return err
	}

	logPath := filepath.Join(mgr.LibDir, "launcher.log")
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// Fall back to stderr if the log can't be opened; at least keep running.
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		defer logf.Close()
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	fmt.Fprintf(cmd.Stderr, "\n===== LianT client start (%s) =====\n", time.Now().Format(time.RFC3339))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start client: %w", err)
	}
	go func(c *exec.Cmd) {
		if err := c.Wait(); err != nil {
			// Write the failure into the same log the client's stderr used.
			fmt.Fprintf(c.Stderr, "client exited with error: %v\n", err)
		} else {
			fmt.Fprintf(c.Stderr, "client exited cleanly\n")
		}
		os.Exit(0)
	}(cmd)
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "LianT launcher:", err)
	os.Exit(1)
}