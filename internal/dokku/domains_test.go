package dokku

import "testing"

func TestParseDomains(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		expectedDomains []string
	}{
		{
			name:            "single domain",
			output:          "Domains app vhosts: example.com",
			expectedDomains: []string{"example.com"},
		},
		{
			name:            "multiple domains",
			output:          "Domains app vhosts: example.com www.example.com api.example.com",
			expectedDomains: []string{"example.com", "www.example.com", "api.example.com"},
		},
		{
			name:            "no domains",
			output:          "Domains app vhosts: ",
			expectedDomains: []string{},
		},
		{
			name:            "empty output",
			output:          "",
			expectedDomains: []string{},
		},
		{
			name:            "domains in multiline output",
			output:          "Some text\nDomains app vhosts: app.com test.com\nMore text",
			expectedDomains: []string{"app.com", "test.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domains := ParseDomains(tt.output)

			if len(domains) != len(tt.expectedDomains) {
				t.Errorf("Expected %d domains, got %d", len(tt.expectedDomains), len(domains))
				return
			}

			for i, domain := range domains {
				if i < len(tt.expectedDomains) && domain != tt.expectedDomains[i] {
					t.Errorf("Expected domain[%d]=%s, got %s", i, tt.expectedDomains[i], domain)
				}
			}
		})
	}
}
