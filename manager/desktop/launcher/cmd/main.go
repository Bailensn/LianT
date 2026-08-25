package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"
	"time"

	"gioui.org/app"

	"LianTLauncher/runtime"
	"LianTLauncher/ui"
	"LianTLauncher/updater"
)

const (
	pbsBase = "https://github.com/astral-sh/python-build-standalone/releases/download/20260814"
	pyVer   = "3.12.14"
	pbsTag  = "20260814"
)

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
	version = "0.00"
	defaultRuntimeURL = os.Getenv("LIANT_RUNTIME_URL")
	runtimeChecksum = os.Getenv("LIANT_RUNTIME_SHA256")
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
		if err := doWork(mgr, nil); err != nil {
			fatal(err)
		}
		supervise()
		return
	}

	splash := ui.NewSplash(version)
	startAt := time.Now()

	var startErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		startErr = doWork(mgr, splash)
		if startErr != nil {
			splash.SetStatus("Startup failed: " + startErr.Error())
			time.Sleep(3000 * time.Millisecond)
		} else if elapsed := time.Since(startAt); elapsed < 800*time.Millisecond {
			time.Sleep(800*time.Millisecond - elapsed)
		}
		splash.Close()
	}()

	go splash.Run()
	app.Main()
	<-done

	if startErr != nil {
		fatal(startErr)
	}
	supervise()
}

func supervise() {
	select {}
}

func doWork(mgr *runtime.Manager, splash *ui.Splash) error {
	setStatus := func(msg string) {
		if splash != nil {
			splash.SetStatus(msg)
		} else {
			fmt.Fprintln(os.Stderr, "LianT:", msg)
		}
	}

	setStatus("Preparing runtime")
	if err := ensureRuntime(mgr); err != nil {
		return err
	}

	setStatus("Starting LianT")
	if err := launchClient(mgr); err != nil {
		return err
	}
	return nil
}

func ensureRuntime(mgr *runtime.Manager) error {
	if _, err := mgr.FindPython(); err == nil {
		return nil
	}

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
	req := mgr.RequirementsFile()
	fmt.Fprintf(os.Stderr, "downloading Python runtime from %s…\n", url)
	if _, err := updater.ProvisionRuntime(mgr.LibDir, url, runtimeChecksum, req); err != nil {
		return fmt.Errorf("provision runtime: %w", err)
	}
	return mgr.EnsureRuntime()
}

func isPath(s string) bool {
	return s != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")
}

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
			if ee, ok := err.(*exec.ExitError); ok {
				os.Exit(ee.ExitCode())
			}
			os.Exit(1)
		}
		os.Exit(0)
	}(cmd)
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "LianT launcher:", err)
	os.Exit(1)
}
