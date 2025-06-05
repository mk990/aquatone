package core

import (
	"testing"
)

func TestHeaderSecurityFlags(t *testing.T) {
	tests := []struct {
		name              string
		headerName        string
		headerValue       string
		decreasesExpected bool
		increasesExpected bool
	}{
		// Increasing Security Headers
		{"Strict-Transport-Security", "Strict-Transport-Security", "max-age=31536000", false, true},
		{"X-Frame-Options", "X-Frame-Options", "DENY", false, true},
		{"X-XSS-Protection_mode_block", "X-XSS-Protection", "1; mode=block", false, true},
		{"Content-Security-Policy", "Content-Security-Policy", "default-src 'self'", false, true},
		{"Referrer-Policy", "Referrer-Policy", "no-referrer", false, true},
		{"Public-Key-Pins", "Public-Key-Pins", `pin-sha256="abc"; max-age=5184000`, false, true},
		{"X-Permitted-Cross-Domain-Policies_master-only", "X-Permitted-Cross-Domain-Policies", "master-only", false, true},
		{"X-Content-Type-Options_nosniff", "X-Content-Type-Options", "nosniff", false, true},

		// Decreasing Security Headers
		{"Server_Apache", "Server", "Apache/2.4.1 (Unix)", true, false},
		{"X-Powered-By_PHP", "X-Powered-By", "PHP/7.0.0", true, false},
		{"Access-Control-Allow-Origin_Wildcard", "Access-Control-Allow-Origin", "*", true, false},
		{"X-XSS-Protection_disabled", "X-XSS-Protection", "0", true, false},
		{"WPE-Backend", "WPE-Backend", "apache", true, false},
		{"X-CF-Powered-By", "X-CF-Powered-By", "WordPress", true, false},
		{"X-Pingback", "X-Pingback", "http://example.com/xmlrpc.php", true, false},


		// Neutral Headers (neither increasing nor decreasing)
		{"Cache-Control", "Cache-Control", "no-cache", false, false},
		{"Content-Type", "Content-Type", "text/html; charset=utf-8", false, false},
		{"Date", "Date", "Tue, 15 Nov 1994 08:12:31 GMT", false, false},
		{"X-Custom-Header", "X-Custom-Header", "SomeValue", false, false},
		{"X-Permitted-Cross-Domain-Policies_none", "X-Permitted-Cross-Domain-Policies", "none", false, false}, // 'none' is not 'master-only'
		{"Access-Control-Allow-Origin_Specific", "Access-Control-Allow-Origin", "https://example.com", false, false}, // Not wildcard
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Header{Name: tt.headerName, Value: tt.headerValue}

			// Test decreasesSecurity()
			if got := h.decreasesSecurity(); got != tt.decreasesExpected {
				t.Errorf("Header{%q, %q}.decreasesSecurity() = %v, want %v", tt.headerName, tt.headerValue, got, tt.decreasesExpected)
			}

			// Test increasesSecurity()
			if got := h.increasesSecurity(); got != tt.increasesExpected {
				t.Errorf("Header{%q, %q}.increasesSecurity() = %v, want %v", tt.headerName, tt.headerValue, got, tt.increasesExpected)
			}

			// Test SetSecurityFlags()
			h.SetSecurityFlags()
			if h.DecreasesSecurity != tt.decreasesExpected {
				t.Errorf("After SetSecurityFlags(), Header.DecreasesSecurity = %v, want %v for Header{%q, %q}", h.DecreasesSecurity, tt.decreasesExpected, tt.headerName, tt.headerValue)
			}
			if h.IncreasesSecurity != tt.increasesExpected {
				t.Errorf("After SetSecurityFlags(), Header.IncreasesSecurity = %v, want %v for Header{%q, %q}", h.IncreasesSecurity, tt.increasesExpected, tt.headerName, tt.headerValue)
			}

			// Test mutual exclusivity after SetSecurityFlags()
			if h.DecreasesSecurity && h.IncreasesSecurity {
				t.Errorf("After SetSecurityFlags(), both DecreasesSecurity and IncreasesSecurity are true for Header{%q, %q}", tt.headerName, tt.headerValue)
			}
		})
	}
}
