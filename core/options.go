package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type Options struct {
	Threads           *int
	OutDir            *string
	SessionPath       *string
	TemplatePath      *string
	Proxy             *string
	ChromePath        *string
	Resolution        *string
	Ports             *string
	ScanTimeout       *int
	HTTPTimeout       *int
	ScreenshotTimeout *int
	Nmap              *bool
	SaveBody          *bool
	Silent            *bool
	Debug             *bool
	Version           *bool
}

func ParseOptions() (Options, error) {
	var (
		threads           int
		outDir            string
		sessionPath       string
		templatePath      string
		proxy             string
		chromePath        string
		resolution        string
		ports             string
		scanTimeout       int
		httpTimeout       int
		screenshotTimeout int
		nmap              bool
		saveBody          bool
		silent            bool
		debug             bool
		version           bool
	)

	rootCmd := &cobra.Command{
		Use:   "aquatone",
		Short: "Discover and report on HTTP services",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}

	flags := rootCmd.PersistentFlags()

	flags.IntVarP(&threads, "threads", "t", 0, "Number of concurrent threads")
	flags.StringVarP(&outDir, "out", "o", ".", "Directory to write files to")
	flags.StringVarP(&sessionPath, "session", "s", "", "Load Aquatone session file and generate HTML report")
	flags.StringVarP(&templatePath, "template-path", "T", "", "Path to HTML template to use for report")

	defaultPorts := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(MediumPortList)), ","), "[]")
	flags.StringVarP(&ports, "ports", "p", defaultPorts, "Ports to scan on hosts (alias list: small, medium, large, xlarge)")
	flags.StringVarP(&proxy, "proxy", "x", "", "Proxy to use for HTTP requests (like curl -x)")
	flags.StringVarP(&chromePath, "chrome-path", "c", "", "Full path to Chrome/Chromium executable")
	flags.StringVarP(&resolution, "resolution", "r", "1440,900", "Screenshot resolution")

	flags.IntVarP(&scanTimeout, "scan-timeout", "S", 100, "Timeout in milliseconds for port scans")
	flags.IntVarP(&httpTimeout, "http-timeout", "H", 3000, "Timeout in milliseconds for HTTP requests")
	flags.IntVarP(&screenshotTimeout, "screenshot-timeout", "z", 30000, "Timeout in milliseconds for screenshots")

	flags.BoolVarP(&nmap, "nmap", "m", false, "Parse input as Nmap/Masscan XML")

	flags.BoolVarP(&saveBody, "save-body", "b", true, "Save response bodies to files")
	flags.BoolVarP(&silent, "silent", "q", false, "Suppress all output except for errors")
	flags.BoolVarP(&debug, "debug", "d", false, "Print debugging information")
	flags.BoolVarP(&version, "version", "v", false, "Print current Aquatone version")

	// Use ExecuteC to capture help invocation
	// Execute and handle help
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		os.Exit(1)
	}
	if cmd.Flags().Changed("help") {
		os.Exit(0)
	}

	return Options{
		Threads:           &threads,
		OutDir:            &outDir,
		SessionPath:       &sessionPath,
		TemplatePath:      &templatePath,
		Proxy:             &proxy,
		ChromePath:        &chromePath,
		Resolution:        &resolution,
		Ports:             &ports,
		ScanTimeout:       &scanTimeout,
		HTTPTimeout:       &httpTimeout,
		ScreenshotTimeout: &screenshotTimeout,
		Nmap:              &nmap,
		SaveBody:          &saveBody,
		Silent:            &silent,
		Debug:             &debug,
		Version:           &version,
	}, nil
}
