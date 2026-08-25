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
	"strings"
	"time"

	"gioui.org/app"

	"LianTLauncher/runtime"
	"LianTLauncher/ui"
	"LianTLauncher/updater"
)

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

	// Step 3: spawn a new process to run the client.
	setStatus("Starting LianT")
	if err := launchClient(mgr); err != nil {
		return err
	}
	return nil
}

// ensureRuntime checks ./runtime for a Python interpreter; if missing and a
// runtime URL is configured, it downloads the standalone build and installs
// the client's requirements into it.
func ensureRuntime(mgr *runtime.Manager) error {
	if _, err := mgr.FindPython(); err == nil {
		return nil
	}
	if isPath(defaultRuntimeURL) {
		mgr.PythonEnvOverride = defaultRuntimeURL
		if err := mgr.EnsureRuntime(); err != nil {
			return fmt.Errorf("configured python not usable: %w", err)
		}
		return nil
	}
	if defaultRuntimeURL == "" {
		return fmt.Errorf("no Python in %s and no runtime URL configured; set LIANT_RUNTIME_URL or -runtime-url",
			mgr.RuntimeDir())
	}
	req := mgr.RequirementsFile()
	fmt.Fprintf(os.Stderr, "downloading Python runtime from %s…\n", defaultRuntimeURL)
	if _, err := updater.ProvisionRuntime(mgr.LibDir, defaultRuntimeURL, runtimeChecksum, req); err != nil {
		return fmt.Errorf("provision runtime: %w", err)
	}
	return mgr.EnsureRuntime()
}

// isPath reports whether s is a local filesystem path rather than a URL.
func isPath(s string) bool {
	return s != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")
}

// launchClient starts the client as a child process of the launcher.
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start client: %w", err)
	}
	go func(c *exec.Cmd) {
		if err := c.Wait(); err != nil {
			fmt.Fprintln(os.Stderr, "client exited:", err)
		}
		os.Exit(0)
	}(cmd)
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "LianT launcher:", err)
	os.Exit(1)
}