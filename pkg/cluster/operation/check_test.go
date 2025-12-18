// Copyright 2025 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"testing"

	"github.com/AstroProfundis/sysinfo"
	"github.com/stretchr/testify/assert"
)

// TestCompareVersion verifies the custom version comparison logic.
// It ensures that numeric segments are compared correctly (e.g., 8.10 > 8.4).
func TestCompareVersion(t *testing.T) {
	// Test Equality
	assert.Equal(t, 0, compareVersion("8.4", "8.4"))
	assert.Equal(t, 0, compareVersion("10", "10"))
	assert.Equal(t, 0, compareVersion("2.0", "2.0"))

	// Test Greater Than
	// Key scenario: "10" is numerically greater than "4", so 8.10 > 8.4
	assert.Equal(t, 1, compareVersion("8.10", "8.4"))
	assert.Equal(t, 1, compareVersion("10", "9"))
	assert.Equal(t, 1, compareVersion("2.1", "2.0"))
	assert.Equal(t, 1, compareVersion("0.0.2", "0.0.1"))
	assert.Equal(t, 1, compareVersion("8.4", "8"))

	// Test Less Than
	assert.Equal(t, -1, compareVersion("8.4", "8.10"))
	assert.Equal(t, -1, compareVersion("7", "9"))
	assert.Equal(t, -1, compareVersion("1.9", "2.0"))
	assert.Equal(t, -1, compareVersion("8", "8.4"))
}

// TestCheckOSInfo verifies the OS version validation logic.
// Note: Some OS distributions (like Ubuntu/Debian) return a Warning object even for valid versions
// because they are not officially fully tested. The test logic handles this distinction.
func TestCheckOSInfo(t *testing.T) {
	opt := &CheckOptions{}

	tests := []struct {
		name    string
		osInfo  sysinfo.OS
		wantErr bool // Expect a blocking error (Warn=false, Err!=nil)
		isWarn  bool // Expect a warning (Warn=true, Err!=nil)
	}{
		// --- CentOS tests ---
		{
			name:    "CentOS 7 (Too Old)",
			osInfo:  sysinfo.OS{Vendor: "centos", Name: "CentOS", Version: "7"},
			wantErr: true,
			isWarn:  false,
		},
		{
			name:    "CentOS 9 (OK)",
			osInfo:  sysinfo.OS{Vendor: "centos", Name: "CentOS", Version: "9"},
			wantErr: false,
			isWarn:  false,
		},

		// --- RHEL / RedHat tests ---
		{
			name:    "RHEL 8.3 (Too Old)",
			osInfo:  sysinfo.OS{Vendor: "rhel", Name: "Red Hat Enterprise Linux", Version: "8.3"},
			wantErr: true,
			isWarn:  false,
		},
		{
			name:    "RHEL 8.4 (Min Supported)",
			osInfo:  sysinfo.OS{Vendor: "rhel", Name: "Red Hat Enterprise Linux", Version: "8.4"},
			wantErr: false,
			isWarn:  false,
		},
		{
			name:    "RHEL 8.10 (Supported, verifies integer comparison)",
			osInfo:  sysinfo.OS{Vendor: "rhel", Name: "Red Hat Enterprise Linux", Version: "8.10"},
			wantErr: false,
			isWarn:  false,
		},
		{
			name:    "RHEL 9 (Not Supported in current logic)",
			osInfo:  sysinfo.OS{Vendor: "rhel", Name: "Red Hat Enterprise Linux", Version: "9"},
			wantErr: true,
			isWarn:  false,
		},

		// --- Kylin tests ---
		{
			name:    "Kylin V10 (OK)",
			osInfo:  sysinfo.OS{Vendor: "kylin", Name: "Kylin", Version: "V10"},
			wantErr: false,
			isWarn:  false,
		},
		{
			name:    "Kylin V9 (Too Old)",
			osInfo:  sysinfo.OS{Vendor: "kylin", Name: "Kylin", Version: "V9"},
			wantErr: true,
			isWarn:  false,
		},

		// --- Ubuntu tests ---
		// Ubuntu support logic always attaches an error message with Warn=true if the version is valid.
		{
			name:    "Ubuntu 20.04 (OK but Warn)",
			osInfo:  sysinfo.OS{Vendor: "ubuntu", Name: "Ubuntu", Version: "20.04"},
			wantErr: false,
			isWarn:  true, // Expect Warn=true
		},
		{
			name:    "Ubuntu 18.04 (Too Old)",
			osInfo:  sysinfo.OS{Vendor: "ubuntu", Name: "Ubuntu", Version: "18.04"},
			wantErr: true,  // Blocking error
			isWarn:  false, // Warn flag is explicitly set to false in code
		},

		// --- Debian tests ---
		// Debian logic mirrors Ubuntu: valid versions return a Warning.
		{
			name:    "Debian 10 (OK but Warn)",
			osInfo:  sysinfo.OS{Vendor: "debian", Name: "Debian", Version: "10"},
			wantErr: false,
			isWarn:  true,
		},
		{
			name:    "Debian 9 (Too Old)",
			osInfo:  sysinfo.OS{Vendor: "debian", Name: "Debian", Version: "9"},
			wantErr: true,
			isWarn:  false,
		},

		// --- Rocky Linux tests ---
		{
			name:    "Rocky 9.1 (OK)",
			osInfo:  sysinfo.OS{Vendor: "rocky", Name: "Rocky Linux", Version: "9.1"},
			wantErr: false,
			isWarn:  false,
		},
		{
			name:    "Rocky 8.5 (Too Old)",
			osInfo:  sysinfo.OS{Vendor: "rocky", Name: "Rocky Linux", Version: "8.5"},
			wantErr: true,
			isWarn:  false,
		},

		// --- Amazon Linux tests ---
		{
			name:    "Amazon Linux 2023 (OK)",
			osInfo:  sysinfo.OS{Vendor: "amzn", Name: "Amazon Linux", Version: "2023"},
			wantErr: false,
			isWarn:  false,
		},
		{
			name:    "Amazon Linux 2 (OK)",
			osInfo:  sysinfo.OS{Vendor: "amzn", Name: "Amazon Linux", Version: "2"},
			wantErr: false,
			isWarn:  false,
		},
		{
			name:    "Amazon Linux 3 (Not Supported)",
			osInfo:  sysinfo.OS{Vendor: "amzn", Name: "Amazon Linux", Version: "3"},
			wantErr: true,
			isWarn:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkOSInfo(opt, &tt.osInfo)

			if tt.isWarn {
				// Case: Warning (Soft Error)
				// The function returns an error object, but marks it as a warning.
				assert.True(t, result.Warn, "Expected warning flag to be true")
				assert.NotNil(t, result.Err, "Warning should have an error message attached")
			} else if tt.wantErr {
				// Case: Blocking Error
				// The function returns an error object, and it is NOT a warning.
				assert.False(t, result.Warn, "Expected blocking error, not warning")
				assert.NotNil(t, result.Err, "Expected error object")
			} else {
				// Case: Pass
				// No error object should be returned.
				assert.Nil(t, result.Err, "Expected success (nil error)")
			}
		})
	}
}
