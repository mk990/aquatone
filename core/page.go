package core

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Header struct {
	Name              string `json:"name"`
	Value             string `json:"value"`
	DecreasesSecurity bool   `json:"decreasesSecurity"`
	IncreasesSecurity bool   `json:"increasesSecurity"`
}

var (
	degradingSecurityHeaders = map[string]func(value string) bool{
		"server":                      func(value string) bool { return true },
		"wpe-backend":                 func(value string) bool { return true },
		"x-powered-by":                func(value string) bool { return true },
		"x-cf-powered-by":             func(value string) bool { return true },
		"x-pingback":                  func(value string) bool { return true },
		"access-control-allow-origin": func(value string) bool { return value == "*" },
		"x-xss-protection":            func(value string) bool { return !strings.HasPrefix(value, "1") },
	}

	increasingSecurityHeaders = map[string]func(value string) bool{
		"content-security-policy":             func(value string) bool { return true },
		"content-security-policy-report-only": func(value string) bool { return true },
		"strict-transport-security":           func(value string) bool { return true },
		"x-frame-options":                     func(value string) bool { return true },
		"referrer-policy":                     func(value string) bool { return true },
		"public-key-pins":                     func(value string) bool { return true },
		"x-permitted-cross-domain-policies":   func(value string) bool { return strings.ToLower(value) == "master-only" },
		"x-content-type-options":              func(value string) bool { return strings.ToLower(value) == "nosniff" },
		"x-xss-protection":                    func(value string) bool { return strings.HasPrefix(value, "1") },
	}
)

func (h *Header) SetSecurityFlags() {
	if h.decreasesSecurity() {
		h.DecreasesSecurity = true
		h.IncreasesSecurity = false // Explicitly set other flag to false
	} else if h.increasesSecurity() {
		h.DecreasesSecurity = false // Explicitly set other flag to false
		h.IncreasesSecurity = true
	} else {
		h.DecreasesSecurity = false
		h.IncreasesSecurity = false
	}
}

func (h Header) decreasesSecurity() bool {
	lowerName := strings.ToLower(h.Name)
	if checkFunc, ok := degradingSecurityHeaders[lowerName]; ok {
		return checkFunc(h.Value)
	}
	return false
}

func (h Header) increasesSecurity() bool {
	lowerName := strings.ToLower(h.Name)
	if checkFunc, ok := increasingSecurityHeaders[lowerName]; ok {
		return checkFunc(h.Value)
	}
	return false
}

type Tag struct {
	Text string `json:"text"`
	Type string `json:"type"`
	Link string `json:"link"`
	Hash string `json:"hash"`
}

func (t Tag) HasLink() bool {
	if t.Link != "" {
		return true
	}
	return false
}

type Note struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type Page struct {
	sync.Mutex
	UUID           string   `json:"uuid"`
	URL            string   `json:"url"`
	Hostname       string   `json:"hostname"`
	Addrs          []string `json:"addrs"`
	Status         string   `json:"status"`
	PageTitle      string   `json:"pageTitle"`
	PageStructure  []string `json:"-"`
	HeadersPath    string   `json:"headersPath"`
	BodyPath       string   `json:"bodyPath"`
	ScreenshotPath string   `json:"screenshotPath"`
	HasScreenshot  bool     `json:"hasScreenshot"`
	Headers        []Header `json:"headers"`
	Tags           []Tag    `json:"tags"`
	Notes          []Note   `json:"notes"`
}

func (p *Page) AddHeader(name string, value string) {
	p.Lock()
	defer p.Unlock()
	header := Header{
		Name:  name,
		Value: value,
	}
	header.SetSecurityFlags()
	p.Headers = append(p.Headers, header)
}

func (p *Page) AddTag(text string, tagType string, link string) {
	p.Lock()
	defer p.Unlock()

	h := sha1.New()
	// Errors from io.WriteString to a sha1.New() hasher are highly unlikely
	// as it's an in-memory operation. Panicking here would be too disruptive.
	// Logging could be an option if a logger was easily available here.
	// For now, per original code, we'll ignore it. A failure would result in
	// a non-unique hash, which is not critical for this feature.
	_, _ = io.WriteString(h, text)
	_, _ = io.WriteString(h, tagType)
	_, _ = io.WriteString(h, link)

	p.Tags = append(p.Tags, Tag{
		Text: text,
		Type: tagType,
		Link: link,
		Hash: fmt.Sprintf("%x", h.Sum(nil)),
	})
}

func (p *Page) AddNote(text string, noteType string) {
	p.Lock()
	defer p.Unlock()
	p.Notes = append(p.Notes, Note{
		Text: text,
		Type: noteType,
	})
}

func (p *Page) BaseFilename() string {
	u := p.ParsedURL()
	h := sha1.New()
	// Similar to AddTag, errors here are unlikely and not critical.
	_, _ = io.WriteString(h, u.Path)
	_, _ = io.WriteString(h, u.Fragment)

	pathHash := fmt.Sprintf("%x", h.Sum(nil))[0:16]
	host := strings.Replace(u.Host, ":", "__", 1)
	filename := fmt.Sprintf("%s__%s__%s", u.Scheme, strings.Replace(host, ".", "_", -1), pathHash)
	return strings.ToLower(filename)
}

func (p *Page) ParsedURL() *url.URL {
	parsedURL, _ := url.Parse(p.URL)
	return parsedURL
}

func (p *Page) IsIPHost() bool {
	return net.ParseIP(p.ParsedURL().Hostname()) != nil
}

func NewPage(pageURL string) (*Page, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}

	return &Page{
		UUID:     uuid.New().String(),
		URL:      pageURL,
		Hostname: u.Hostname(),
	}, nil
}
