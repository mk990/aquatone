package agents

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mk990/aquatone/core"
)

type URLScreenshotter struct {
	session         *core.Session
	chromePath      string
	tempUserDirPath string
}

func NewURLScreenshotter() *URLScreenshotter {
	return &URLScreenshotter{}
}

func (a *URLScreenshotter) ID() string {
	return "agent:url_screenshotter"
}

func (a *URLScreenshotter) Register(s *core.Session) error {
	if err := s.EventBus.SubscribeAsync(core.URLResponsive, a.OnURLResponsive, false); err != nil {
		return fmt.Errorf("failed to subscribe to %s event: %w", core.URLResponsive, err)
	}
	if err := s.EventBus.SubscribeAsync(core.SessionEnd, a.OnSessionEnd, false); err != nil {
		// Not returning error here as OnSessionEnd is for cleanup,
		// main functionality might still work. Log it.
		s.Out.Error("[%s] Failed to subscribe to %s event: %v\n", a.ID(), core.SessionEnd, err)
	}
	a.session = s
	if err := a.createTempUserDir(); err != nil {
		return fmt.Errorf("failed to create temporary user directory: %w", err)
	}
	if err := a.locateChrome(); err != nil {
		return fmt.Errorf("failed to locate Chrome: %w", err)
	}

	return nil
}

func (a *URLScreenshotter) OnURLResponsive(url string) {
	a.session.Out.Debug("[%s] Received new responsive URL %s\n", a.ID(), url)
	page := a.session.GetPage(url)
	if page == nil {
		a.session.Out.Error("Unable to find page for URL: %s\n", url)
		return
	}

	a.session.WaitGroup.Add()
	go func(page *core.Page) {
		defer a.session.WaitGroup.Done()
		a.screenshotPage(page)
	}(page)
}

func (a *URLScreenshotter) OnSessionEnd() {
	a.session.Out.Debug("[%s] Received SessionEnd event\n", a.ID())
	if err := os.RemoveAll(a.tempUserDirPath); err != nil {
		a.session.Out.Error("[%s] Failed to delete temporary user directory %s: %v\n", a.ID(), a.tempUserDirPath, err)
	} else {
		a.session.Out.Debug("[%s] Deleted temporary user directory at: %s\n", a.ID(), a.tempUserDirPath)
	}
}

func (a *URLScreenshotter) createTempUserDir() error {
	dir, err := os.MkdirTemp("", "aquatone-chrome")
	if err != nil {
		return fmt.Errorf("unable to create temporary user directory for Chrome/Chromium browser: %w", err)
	}
	a.session.Out.Debug("[%s] Created temporary user directory at: %s\n", a.ID(), dir)
	a.tempUserDirPath = dir
	return nil
}

func (a *URLScreenshotter) locateChrome() error {
	if *a.session.Options.ChromePath != "" {
		if _, err := os.Stat(*a.session.Options.ChromePath); os.IsNotExist(err) {
			return fmt.Errorf("chrome path %s specified via -chrome-path does not exist: %w", *a.session.Options.ChromePath, err)
		}
		a.chromePath = *a.session.Options.ChromePath
		return nil
	}

	paths := []string{
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-beta",
		"/usr/bin/google-chrome-unstable",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"C:/Program Files (x86)/Google/Chrome/Application/chrome.exe",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		a.chromePath = path
		break // Found a path
	}

	if a.chromePath == "" {
		return fmt.Errorf("unable to locate a valid installation of Chrome. Install Google Chrome or try specifying a valid location with the -chrome-path option")
	}

	a.session.Out.Debug("[%s] Attempting to use Chrome/Chromium binary at %s\n", a.ID(), a.chromePath)

	if strings.Contains(strings.ToLower(a.chromePath), "chrome") && !strings.Contains(strings.ToLower(a.chromePath), "chromium") {
		a.session.Out.Warn("Using Google Chrome for screenshots. Install Chromium for potentially better results and stability.\n")
	}

	// Verify the found chromePath and its version
	out, err := exec.Command(a.chromePath, "--version").Output()
	if err != nil {
		return fmt.Errorf("failed to execute %s --version: %w. Ensure it is a valid Chrome/Chromium executable", a.chromePath, err)
	}
	version := string(out)
	a.session.Out.Debug("[%s] Chrome/Chromium version: %s\n", a.ID(), strings.TrimSpace(version))
	re := regexp.MustCompile(`(\d+)\.`)
	match := re.FindStringSubmatch(version)
	if len(match) <= 0 {
		a.session.Out.Warn("Unable to determine version of Chrome/Chromium from output: '%s'. Screenshotting might be unreliable.\n", version)
	} else {
		majorVersion, convErr := strconv.Atoi(match[1])
		if convErr != nil {
			a.session.Out.Warn("Unable to parse major version from '%s': %v. Screenshotting might be unreliable.\n", match[1], convErr)
		} else if majorVersion < 72 {
			a.session.Out.Warn("An older version of Chrome/Chromium (version %d) is installed. Screenshotting of HTTPS URLs might be unreliable or fail.\n", majorVersion)
		}
	}
	return nil
}

func (a *URLScreenshotter) screenshotPage(page *core.Page) {
	filePath := fmt.Sprintf("screenshots/%s.png", page.BaseFilename())
	var chromeArguments = []string{
		"--headless", "--disable-gpu", "--hide-scrollbars", "--mute-audio", "--disable-notifications",
		"--no-first-run", "--disable-crash-reporter", "--ignore-certificate-errors", "--incognito",
		"--disable-infobars", "--disable-sync", "--no-default-browser-check",
		"--user-data-dir=" + a.tempUserDirPath,
		"--user-agent=" + RandomUserAgent(),
		"--window-size=" + *a.session.Options.Resolution,
		"--screenshot=" + a.session.GetFilePath(filePath),
	}

	if os.Geteuid() == 0 {
		chromeArguments = append(chromeArguments, "--no-sandbox")
	}

	if *a.session.Options.Proxy != "" {
		chromeArguments = append(chromeArguments, "--proxy-server="+*a.session.Options.Proxy)
	}

	chromeArguments = append(chromeArguments, page.URL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*a.session.Options.ScreenshotTimeout*1000)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.chromePath, chromeArguments...)
	if err := cmd.Start(); err != nil {
		a.session.Out.Debug("[%s] Error: %v\n", a.ID(), err)
		a.session.Stats.IncrementScreenshotFailed()
		a.session.Out.Error("%s: screenshot failed: %s\n", page.URL, err)
		a.killChromeProcessIfRunning(cmd)
		return
	}

	if err := cmd.Wait(); err != nil {
		a.session.Stats.IncrementScreenshotFailed()
		a.session.Out.Debug("[%s] Error: %v\n", a.ID(), err)
		if ctx.Err() == context.DeadlineExceeded {
			a.session.Out.Error("%s: screenshot timed out\n", page.URL)
			a.killChromeProcessIfRunning(cmd)
			return
		}

		a.session.Out.Error("%s: screenshot failed: %s\n", page.URL, err)
		a.killChromeProcessIfRunning(cmd)
		return
	}

	a.session.Stats.IncrementScreenshotSuccessful()
	a.session.Out.Info("%s: %s\n", page.URL, Green("screenshot successful"))
	page.ScreenshotPath = filePath
	page.HasScreenshot = true
	a.killChromeProcessIfRunning(cmd)
}

func (a *URLScreenshotter) killChromeProcessIfRunning(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := cmd.Process.Release(); err != nil {
		a.session.Out.Debug("[%s] Error releasing process: %v\n", a.ID(), err)
	}
	if err := cmd.Process.Kill(); err != nil {
		// It's common for Kill to fail if the process already exited, so log as debug.
		a.session.Out.Debug("[%s] Error killing process: %v\n", a.ID(), err)
	}
}
