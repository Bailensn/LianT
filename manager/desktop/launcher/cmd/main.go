// Command liant is the LianT desktop launcher.
//
// It does NOT embed client code. On every run it:
//  1. looks for a Python interpreter in ./runtime (next to the client);
//  2. if absent and a runtime URL is configured, downloads a pinned standalone
//     Python build and pip-installs client deps into ./runtime;
//  3. spawns a NEW process to run src/main.py and waits for it.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"liantlauncher/runtime"
	"liantlauncher/updater"
)

var (
	version = "0.0.0"
	// defaultRuntimeURL is injected at link time with
	// -X main.defaultRuntimeURL=...; it is the source for the dynamically
	// downloaded Python interpreter when ./runtime is empty.
	defaultRuntimeURL = os.Getenv("LIANT_RUNTIME_URL")
	// runtimeChecksum optionally pins the SHA-256 of the runtime bundle.
	runtimeChecksum = os.Getenv("LIANT_RUNTIME_SHA256")
)

func main() {
	var pythonOverride string
	flag.StringVar(&pythonOverride, "python", "", "use a specific Python interpreter instead of ./runtime")
	flag.StringVar(&defaultRuntimeURL, "runtime-url", defaultRuntimeURL, "runtime bundle URL (standalone Python) or explicit interpreter path")
	flag.StringVar(&runtimeChecksum, "runtime-sha256", runtimeChecksum, "expected SHA-256 of the runtime bundle (optional)")
	flag.Parse()

	mgr, err := runtime.NewManager()
	if err != nil {
		fatal(err)
	}
	if pythonOverride != "" {
		mgr.PythonEnvOverride = pythonOverride
	}

	// Step 1+2: ensure a Python runtime exists in ./runtime (download if needed).
	if err := ensureRuntime(mgr); err != nil {
		fatal(err)
	}

	// Step 3: spawn a new process to run the client.
	if err := launchClient(mgr); err != nil {
		fatal(err)
	}

	// Wait until the client exits, keeping this launcher as its supervisor.
	select {}
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