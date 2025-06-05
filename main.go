package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mk990/aquatone/agents"
	"github.com/mk990/aquatone/core"
	"github.com/mk990/aquatone/parsers"
)

var (
	sess *core.Session // Global session variable
	// err is no longer global as errors are handled by each function
)

// isURL checks if a string is a valid URL with a scheme.
func isURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	return u.Scheme != ""
}

// hasSupportedScheme checks if a URL has http or https scheme.
func hasSupportedScheme(s string) bool {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// handleInitialSetup performs initial setup including version printing and output directory validation.
func handleInitialSetup(currentSession *core.Session) error {
	if *currentSession.Options.Version {
		currentSession.Out.Info("%s v%s", core.Name, core.Version)
		// Indicate that the program should exit successfully after printing version.
		// This is a special case not treated as an error.
		return fmt.Errorf("version_printed")
	}

	// Output directory validation is now primarily handled by core.NewSession()
	// We still check if it's a directory here, as NewSession might not create it if it exists but isn't a dir.
	fi, err := os.Stat(*currentSession.Options.OutDir)
	if err != nil {
		return fmt.Errorf("failed to stat output directory %s: %w", *currentSession.Options.OutDir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("output destination %s is not a directory", *currentSession.Options.OutDir)
	}

	currentSession.Out.Important("%s v%s started at %s\n\n", core.Name, core.Version, currentSession.Stats.StartedAt.Format(time.RFC3339))
	return nil
}

// loadSessionAndGenerateReport loads a session from --session and generates a report if specified.
// Returns true if execution should stop, false otherwise, and an error.
func loadSessionAndGenerateReport(currentSession *core.Session) (bool, error) {
	if *currentSession.Options.SessionPath == "" {
		return false, nil // No session path provided, continue normal execution.
	}

	jsonSession, err := os.ReadFile(*currentSession.Options.SessionPath)
	if err != nil {
		return true, fmt.Errorf("unable to read session file at %s: %w", *currentSession.Options.SessionPath, err)
	}

	var parsedSession core.Session
	if err := json.Unmarshal(jsonSession, &parsedSession); err != nil {
		return true, fmt.Errorf("unable to parse session file at %s: %w", *currentSession.Options.SessionPath, err)
	}

	currentSession.Out.Important("Loaded Aquatone session at %s\n", *currentSession.Options.SessionPath)
	currentSession.Out.Important("Generating HTML report...")
	var template []byte
	if *currentSession.Options.TemplatePath != "" {
		template, err = os.ReadFile(*currentSession.Options.TemplatePath)
	} else {
		template, err = currentSession.Asset("static/report_template.html")
	}
	if err != nil {
		return true, fmt.Errorf("can't read report template file: %w", err)
	}

	report := core.NewReport(&parsedSession, string(template))
	f, err := os.OpenFile(currentSession.GetFilePath("aquatone_report.html"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return true, fmt.Errorf("error opening report file %s: %w", currentSession.GetFilePath("aquatone_report.html"), err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			currentSession.Out.Error("Error closing report file %s: %v\n", currentSession.GetFilePath("aquatone_report.html"), closeErr)
		}
	}()

	if err = report.Render(f); err != nil {
		return true, fmt.Errorf("error rendering report: %w", err)
	}
	currentSession.Out.Important(" done\n\n")
	currentSession.Out.Important("Wrote HTML report to: %s\n\n", currentSession.GetFilePath("aquatone_report.html"))
	return true, nil // Stop execution after generating report.
}

// registerAgents registers all Aquatone agents using the core.Agent interface.
func registerAgents(currentSession *core.Session) error {
	allAgents := []core.Agent{
		agents.NewTCPPortScanner(),
		agents.NewURLPublisher(),
		agents.NewURLRequester(),
		agents.NewURLHostnameResolver(),
		agents.NewURLPageTitleExtractor(),
		agents.NewURLScreenshotter(),
		agents.NewURLTechnologyFingerprinter(),
		agents.NewURLTakeoverDetector(),
	}

	for _, agent := range allAgents {
		currentSession.Out.Debug("Registering agent: %s\n", agent.ID())
		if err := agent.Register(currentSession); err != nil {
			return fmt.Errorf("failed to register agent %s: %w", agent.ID(), err)
		}
	}
	return nil
}

// readAndParseTargets reads targets from stdin and parses them.
func readAndParseTargets(currentSession *core.Session) ([]string, error) {
	reader := bufio.NewReader(os.Stdin)
	var targets []string
	var err error

	if *currentSession.Options.Nmap {
		parser := parsers.NewNmapParser()
		targets, err = parser.Parse(reader)
		if err != nil {
			return nil, fmt.Errorf("unable to parse input as Nmap/Masscan XML: %w", err)
		}
	} else {
		parser := parsers.NewRegexParser()
		targets, err = parser.Parse(reader)
		if err != nil {
			return nil, fmt.Errorf("unable to parse input: %w", err)
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets found in input")
	}
	currentSession.Out.Important("Targets    : %d\n", len(targets))
	currentSession.Out.Important("Threads    : %d\n", *currentSession.Options.Threads)
	currentSession.Out.Important("Ports      : %s\n", strings.Trim(strings.Replace(fmt.Sprint(currentSession.Ports), " ", ", ", -1), "[]"))
	currentSession.Out.Important("Output dir : %s\n\n", *currentSession.Options.OutDir)
	return targets, nil
}

// processTargets publishes Host/URL events and waits for processing.
func processTargets(currentSession *core.Session, targets []string) {
	currentSession.EventBus.Publish(core.SessionStart)
	for _, target := range targets {
		if isURL(target) {
			if hasSupportedScheme(target) {
				currentSession.EventBus.Publish(core.URL, target)
			}
		} else {
			currentSession.EventBus.Publish(core.Host, target)
		}
	}
	time.Sleep(1 * time.Second) // Allow time for initial events to be processed
	currentSession.EventBus.WaitAsync()
	currentSession.WaitGroup.Wait()

	currentSession.EventBus.Publish(core.SessionEnd)
	time.Sleep(1 * time.Second) // Allow time for final events
	currentSession.EventBus.WaitAsync()
	currentSession.WaitGroup.Wait()
}

// analyzePages calculates page structures and clusters similar pages.
func analyzePages(currentSession *core.Session) error {
	currentSession.Out.Important("Calculating page structures...")
	urlsFilePath := currentSession.GetFilePath("aquatone_urls.txt")
	urlsFile, err := os.OpenFile(urlsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Log error but don't make it fatal for the whole process,
		// as report generation might still work for already processed pages.
		currentSession.Out.Error("Failed to open %s: %v\n", urlsFilePath, err)
	} else {
		defer func() {
			if closeErr := urlsFile.Close(); closeErr != nil {
				currentSession.Out.Error("Error closing %s: %v\n", urlsFilePath, closeErr)
			}
		}()
	}

	for _, page := range currentSession.Pages {
		filename := currentSession.GetFilePath(fmt.Sprintf("html/%s.html", page.BaseFilename()))
		bodyFile, err := os.Open(filename)
		if err != nil {
			currentSession.Out.Debug("Skipping structure calculation for %s, failed to open body file: %v\n", page.URL, err)
			continue
		}

		structure, gPSErr := core.GetPageStructure(bodyFile)
		// It's important to close the file right after reading.
		if closeErr := bodyFile.Close(); closeErr != nil {
			currentSession.Out.Debug("Error closing body file for %s: %v\n", page.URL, closeErr)
		}

		if gPSErr != nil {
			currentSession.Out.Debug("Error getting page structure for %s: %v\n", page.URL, gPSErr)
			continue
		}
		page.PageStructure = structure
		if urlsFile != nil { // Ensure urlsFile was opened successfully
			if _, err := urlsFile.WriteString(page.URL + "\n"); err != nil {
				currentSession.Out.Error("Failed to write URL %s to %s: %v\n", page.URL, urlsFilePath, err)
			}
		}
	}
	currentSession.Out.Important(" done\n")

	currentSession.Out.Important("Clustering similar pages...")
	for _, page := range currentSession.Pages {
		foundCluster := false
		for clusterUUID, cluster := range currentSession.PageSimilarityClusters {
			addToCluster := true
			for _, pageURL := range cluster {
				page2 := currentSession.GetPage(pageURL)
				if page2 != nil && core.GetSimilarity(page.PageStructure, page2.PageStructure) < 0.80 {
					addToCluster = false
					break
				}
			}
			if addToCluster {
				foundCluster = true
				currentSession.PageSimilarityClusters[clusterUUID] = append(currentSession.PageSimilarityClusters[clusterUUID], page.URL)
				break
			}
		}
		if !foundCluster {
			newClusterUUID := uuid.New().String()
			currentSession.PageSimilarityClusters[newClusterUUID] = []string{page.URL}
		}
	}
	currentSession.Out.Important(" done\n")
	return nil
}

// generateHTMLReport generates the final HTML report.
func generateHTMLReport(currentSession *core.Session) error {
	currentSession.Out.Important("Generating HTML report...")
	var template []byte
	var err error
	if *currentSession.Options.TemplatePath != "" {
		template, err = os.ReadFile(*currentSession.Options.TemplatePath)
	} else {
		template, err = currentSession.Asset("static/report_template.html")
	}
	if err != nil {
		return fmt.Errorf("can't read report template file: %w", err)
	}

	report := core.NewReport(currentSession, string(template))
	reportFilePath := currentSession.GetFilePath("aquatone_report.html")
	reportFile, err := os.OpenFile(reportFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error opening HTML report file %s: %w", reportFilePath, err)
	}
	defer func() {
		if closeErr := reportFile.Close(); closeErr != nil {
			currentSession.Out.Error("Error closing HTML report file %s: %v\n", reportFilePath, closeErr)
		}
	}()

	if err = report.Render(reportFile); err != nil {
		return fmt.Errorf("error rendering HTML report: %w", err)
	}
	currentSession.Out.Important(" done\n\n")
	currentSession.Out.Important("Wrote HTML report to: %s\n\n", reportFilePath)
	return nil
}

// saveSessionAndPrintStats saves the session to JSON and prints final statistics.
func saveSessionAndPrintStats(currentSession *core.Session) error {
	currentSession.End() // Mark session end time for stats
	currentSession.Out.Important("Writing session file...")
	if err := currentSession.SaveToFile("aquatone_session.json"); err != nil {
		// Log as error but don't make it fatal for the whole process
		currentSession.Out.Error("Failed to save session file: %v\n", err)
	} else {
		currentSession.Out.Important(" done\n")
	}

	currentSession.Out.Important("Time:\n")
	currentSession.Out.Info(" - Started at  : %v\n", currentSession.Stats.StartedAt.Format(time.RFC3339))
	currentSession.Out.Info(" - Finished at : %v\n", currentSession.Stats.FinishedAt.Format(time.RFC3339))
	currentSession.Out.Info(" - Duration    : %v\n\n", currentSession.Stats.Duration().Round(time.Second))

	currentSession.Out.Important("Requests:\n")
	currentSession.Out.Info(" - Successful : %v\n", currentSession.Stats.RequestSuccessful)
	currentSession.Out.Info(" - Failed     : %v\n\n", currentSession.Stats.RequestFailed)

	currentSession.Out.Info(" - 2xx : %v\n", currentSession.Stats.ResponseCode2xx)
	currentSession.Out.Info(" - 3xx : %v\n", currentSession.Stats.ResponseCode3xx)
	currentSession.Out.Info(" - 4xx : %v\n", currentSession.Stats.ResponseCode4xx)
	currentSession.Out.Info(" - 5xx : %v\n\n", currentSession.Stats.ResponseCode5xx)

	currentSession.Out.Important("Screenshots:\n")
	currentSession.Out.Info(" - Successful : %v\n", currentSession.Stats.ScreenshotSuccessful)
	currentSession.Out.Info(" - Failed     : %v\n\n", currentSession.Stats.ScreenshotFailed)
	return nil
}

func main() {
	var err error // Local err variable for main
	sess, err = core.NewSession()
	if err != nil {
		// NewSession should ideally set up Out, but if it fails early, sess or sess.Out might be nil
		if sess != nil && sess.Out != nil {
			sess.Out.Fatal("Error initializing session: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Error initializing session: %v\n", err)
		}
		os.Exit(1)
	}

	if err = handleInitialSetup(sess); err != nil {
		if err.Error() == "version_printed" {
			os.Exit(0) // Successful exit after printing version
		}
		sess.Out.Fatal("Error during initial setup: %v\n", err)
		os.Exit(1)
	}

	stopExecution, err := loadSessionAndGenerateReport(sess)
	if err != nil {
		sess.Out.Fatal("Error loading session or generating report: %v\n", err)
		os.Exit(1)
	}
	if stopExecution {
		os.Exit(0)
	}

	if err = registerAgents(sess); err != nil {
		sess.Out.Fatal("Error registering agents: %v\n", err)
		os.Exit(1)
	}

	targets, err := readAndParseTargets(sess)
	if err != nil {
		sess.Out.Fatal("Error reading or parsing targets: %v\n", err)
		os.Exit(1)
	}

	processTargets(sess, targets)

	if err = analyzePages(sess); err != nil {
		// analyzePages currently logs errors but doesn't return fatal ones.
		// If it were to return fatal errors, they would be handled here.
		sess.Out.Error("Error during page analysis: %v\n", err)
		// Decide if this should be fatal or not, for now, continue to report generation
	}

	if err = generateHTMLReport(sess); err != nil {
		sess.Out.Fatal("Error generating HTML report: %v\n", err)
		os.Exit(1)
	}

	if err = saveSessionAndPrintStats(sess); err != nil {
		// saveSessionAndPrintStats currently logs errors but doesn't return fatal ones.
		sess.Out.Error("Error saving session or printing stats: %v\n", err)
	}
}
