package progmgr

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name           string
		version1       string
		version2       string
		wantIsOutdated bool
		wantIsNewer    bool
		wantErr        bool
	}{
		// Equal versions
		{
			name:           "Equal versions",
			version1:       "2.6.0",
			version2:       "2.6.0",
			wantIsOutdated: false,
			wantIsNewer:    false,
			wantErr:        false,
		},
		// Major version differences
		{
			name:           "Older major version",
			version1:       "1.0.0",
			version2:       "2.0.0",
			wantIsOutdated: true,
			wantIsNewer:    false,
			wantErr:        false,
		},
		{
			name:           "Newer major version",
			version1:       "3.0.0",
			version2:       "2.0.0",
			wantIsOutdated: false,
			wantIsNewer:    true,
			wantErr:        false,
		},
		// Minor version differences
		{
			name:           "Older minor version",
			version1:       "2.5.0",
			version2:       "2.6.0",
			wantIsOutdated: true,
			wantIsNewer:    false,
			wantErr:        false,
		},
		{
			name:           "Newer minor version",
			version1:       "2.7.0",
			version2:       "2.6.0",
			wantIsOutdated: false,
			wantIsNewer:    true,
			wantErr:        false,
		},
		// Patch version differences
		{
			name:           "Older patch version",
			version1:       "2.6.1",
			version2:       "2.6.2",
			wantIsOutdated: true,
			wantIsNewer:    false,
			wantErr:        false,
		},
		{
			name:           "Newer patch version",
			version1:       "2.6.3",
			version2:       "2.6.2",
			wantIsOutdated: false,
			wantIsNewer:    true,
			wantErr:        false,
		},
		// String vs. numeric comparison tests
		{
			name:           "String vs. numeric comparison (older)",
			version1:       "2.2.0",
			version2:       "2.10.0",
			wantIsOutdated: true,
			wantIsNewer:    false,
			wantErr:        false,
		},
		{
			name:           "String vs. numeric comparison (newer)",
			version1:       "2.10.0",
			version2:       "2.2.0",
			wantIsOutdated: false,
			wantIsNewer:    true,
			wantErr:        false,
		},
		// Different version parts length
		{
			name:           "Different parts length (equal)",
			version1:       "2.6",
			version2:       "2.6.0",
			wantIsOutdated: false,
			wantIsNewer:    false,
			wantErr:        false,
		},
		{
			name:           "Different parts length (older)",
			version1:       "2.6",
			version2:       "2.6.1",
			wantIsOutdated: true,
			wantIsNewer:    false,
			wantErr:        false,
		},
		{
			name:           "Different parts length (newer)",
			version1:       "2.6.1",
			version2:       "2.6",
			wantIsOutdated: false,
			wantIsNewer:    true,
			wantErr:        false,
		},
		// Error cases
		{
			name:           "Invalid version format (version1)",
			version1:       "2.a.0",
			version2:       "2.6.0",
			wantIsOutdated: false,
			wantIsNewer:    false,
			wantErr:        true,
		},
		{
			name:           "Invalid version format (version2)",
			version1:       "2.6.0",
			version2:       "2.b.0",
			wantIsOutdated: false,
			wantIsNewer:    false,
			wantErr:        true,
		},
		{
			name:           "Empty version string",
			version1:       "",
			version2:       "2.6.0",
			wantIsOutdated: false,
			wantIsNewer:    false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIsOutdated, gotIsNewer, err := compareVersions(tt.version1, tt.version2)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("compareVersions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If error expected, don't check the other outputs
			if tt.wantErr {
				return
			}

			// Check outdated status
			if gotIsOutdated != tt.wantIsOutdated {
				t.Errorf("compareVersions() isOutdated = %v, want %v", gotIsOutdated, tt.wantIsOutdated)
			}

			// Check newer status
			if gotIsNewer != tt.wantIsNewer {
				t.Errorf("compareVersions() isNewer = %v, want %v", gotIsNewer, tt.wantIsNewer)
			}
		})
	}
}

// TestCompareVersionsManual is an easy-to-read test for checking version comparison
func TestCompareVersionsManual(t *testing.T) {
	// Test for outdated version
	isOutdated, isNewer, err := compareVersions("2.6.0", "2.7.0")
	if err != nil {
		t.Errorf("Error comparing versions: %v", err)
		return
	}

	if !isOutdated {
		t.Errorf("Expected 2.6.0 to be outdated compared to 2.7.0")
	}

	if isNewer {
		t.Errorf("Expected 2.6.0 to not be newer compared to 2.7.0")
	}

	// Test for newer version
	isOutdated, isNewer, err = compareVersions("2.8.0", "2.7.0")
	if err != nil {
		t.Errorf("Error comparing versions: %v", err)
		return
	}

	if isOutdated {
		t.Errorf("Expected 2.8.0 to not be outdated compared to 2.7.0")
	}

	if !isNewer {
		t.Errorf("Expected 2.8.0 to be newer compared to 2.7.0")
	}
}
