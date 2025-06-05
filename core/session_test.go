package core

import (
	"strings"
	"testing"
)

// MockSession is a minimal Session for testing BaseFilenameFromURL
// as the original method is on the Session struct.
// In a real scenario, if BaseFilenameFromURL didn't need session state,
// it could be a standalone utility function.
type MockSession struct {
	Options Options // Keep Options if BaseFilenameFromURL indirectly uses them (it doesn't directly)
}

// BaseFilenameFromURL is adapted here for testing.
// If this were a package function or if Session had an interface, this would be cleaner.
// For now, we replicate the logic or call a helper if it were refactored.
// The actual BaseFilenameFromURL method is on the real Session type.
// To test the *exact* method, we'd need to instantiate a full Session,
// or refactor BaseFilenameFromURL to be a package-level function if it doesn't need session state.

// For this test, we'll assume BaseFilenameFromURL can be tested
// by calling a similar standalone function or by creating a lightweight session.
// Let's make a helper that mimics the core logic needed for BaseFilenameFromURL
// as found in the original Session struct, to avoid full session setup.
func generateBaseFilenameForTest(pageURL string) string {
	// This helper function replicates the logic from Session.BaseFilenameFromURL
	// without needing a full Session object.
	s := &Session{} // A dummy session, not fully initialized.
	return s.BaseFilenameFromURL(pageURL)
}


func TestBaseFilenameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple http",
			url:      "http://example.com",
			expected: "http__example_com__da39a3ee5e6b4b0d", // Path="", Fragment=""
		},
		{
			name:     "simple https",
			url:      "https://example.com",
			expected: "https__example_com__da39a3ee5e6b4b0d", // Path="", Fragment=""
		},
		{
			name:     "with path",
			url:      "http://example.com/path/to/resource",
			expected: "http__example_com__0f969ad3e8721d6a", // Path="/path/to/resource", Fragment=""
		},
		{
			name:     "with path and fragment",
			url:      "http://example.com/path#section1",
			expected: "http__example_com__acc59d181f4e3bc8", // Path="/path", Fragment="section1"
		},
		{
			name:     "with port",
			url:      "http://example.com:8080/path",
			expected: "http__example_com__8080__4f26609ad3f5185f", // Path="/path", Fragment=""
		},
		{
			name:     "mixed case path", // Path is case-sensitive for hashing
			url:      "http://test.org/Some/MixedCasePath",
			expected: "http__test_org__7c796ef14121b26e", // Path="/Some/MixedCasePath", Fragment=""
		},
		{
			name:     "trailing slash",
			url:      "http://example.com/path/",
			expected: "http__example_com__eaf9d1072b828b48", // Path="/path/", Fragment=""
		},
		{
			name:     "just fragment",
			url:      "http://example.com#test",
			expected: "http__example_com__a94a8fe5ccb19ba6", // Path="", Fragment="test"
		},
		{
			name:     "domain with subdomain",
			url:      "https://sub.domain.example.com/resource",
			expected: "https__sub_domain_example_com__cec4ccb8f55dc23a", // Path="/resource", Fragment=""
		},
	}

	// Instantiate a minimal session for the method call
	// We need to use a real Session instance to call its method.
	// The BaseFilenameFromURL method doesn't rely on much of the session's state,
	// so a minimally initialized one should be fine.
	sess := &Session{
		Options: Options{}, // Provide empty options
		Out:     &Logger{}, // Provide a dummy logger
	}
	// Initialize other fields if they become necessary for BaseFilenameFromURL

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the actual method from the Session struct
			got := sess.BaseFilenameFromURL(tt.url)

			// Check if the output is lowercased
			if got != strings.ToLower(got) {
				t.Errorf("BaseFilenameFromURL(%q) output %q is not lowercased", tt.url, got)
			}

			// For simplicity in this test, we are checking against pre-calculated SHA1 hashes.
			// In a more robust test, you might not want to hardcode hashes if the hashing
			// algorithm or input processing changes. However, for unit testing a specific output,
			// it's acceptable.
			// The main purpose here is to test the structure: scheme__host__hash
			if got != tt.expected {
				t.Errorf("BaseFilenameFromURL(%q) = %q, want %q", tt.url, got, tt.expected)
			}

			// Alternative check: structural integrity (scheme__host__hash_suffix)
			parts := strings.Split(got, "__")
			if len(parts) < 3 {
				t.Errorf("BaseFilenameFromURL(%q) output %q does not have enough parts separated by '__'", tt.url, got)
				return
			}
			// scheme part: parts[0]
			// host part: parts[1] (or more if host had '__', though current replacement is specific)
			// hash part: parts[len(parts)-1] (the last part)

			// Example structural check (can be made more specific)
			if !strings.HasSuffix(got, parts[len(parts)-1]) { // Check if it ends with the hash part
				t.Errorf("BaseFilenameFromURL(%q) output %q does not seem to end with a hash", tt.url, got)
			}
			if len(parts[len(parts)-1]) != 16 {
				t.Errorf("BaseFilenameFromURL(%q) hash part %q is not 16 characters", tt.url, parts[len(parts)-1])
			}
		})
	}
}
