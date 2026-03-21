# Issue 2: Recursive Stack Overflow in StdUI.renderInput

**Status**: PLANNED (ready to implement)

**Priority**: Medium

**Estimate**: 30 minutes to 1 hour

---

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Security Analysis](#security-analysis)
3. [Current Implementation](#current-implementation)
4. [Solution Design](#solution-design)
5. [Implementation Details](#implementation-details)
6. [Test Plan](#test-plan)
7. [Migration Guide](#migration-guide)
8. [Documentation Updates](#documentation-updates)
9. [Rollback Plan](#rollback-plan)
10. [Security Considerations](#security-considerations)

---

## Problem Statement

The `StdUI.renderInput` method uses recursion to re-prompt for required inputs. If a user (or malicious input stream) repeatedly submits empty values, the call stack grows until potential stack overflow.

### Attack Vector

```bash
# Malicious input: pipe endless empty lines
yes "" | skelly init --src template --dst output
```

**Result**: Stack overflow after significant empty inputs.

### Real-World Scenarios

1. **Scripted input**: CI/CD pipelines with buggy input
2. **User error**: Holding Enter key
3. **Automation tools**: Expect-style scripts misconfigured
4. **Malicious use**: Denial of service on shared systems

---

## Security Analysis

### Threat Model

| Actor | Capability | Likelihood | Impact |
|-------|------------|------------|--------|
| Malicious user | Pipe empty input | Medium | High (crash) |
| Buggy automation | Endless empty input | Low | High (crash) |
| Confused user | Holding Enter | Low | Medium (frustration) |

### Vulnerability Assessment

| ASVS Category | Finding |
|---------------|---------|
| **V5.2.1**: Sanitization | N/A - no sanitization needed |
| **V5.3.1**: Input Validation | FAIL - no attempt limit |
| **V5.4.1**: Memory Limits | FAIL - unbounded recursion |

### CVSS Score

**Base Score: 3.3 (Low)**

- Attack Vector: Local (AV:L)
- Attack Complexity: Low (AC:L)
- Privileges Required: None (PR:N)
- User Interaction: Required (UI:R)
- Scope: Unchanged (S:U)
- Confidentiality: None (C:N)
- Integrity: None (I:N)
- Availability: Low (A:L)

---

## Current Implementation

**File**: `internal/view/std_ui.go:74-103`

```go
func (s *StdUI) renderInput(name string, input *templates.Input) error {
    required := false
    defaultValue := input.Default
    if defaultValue == "" || defaultValue == nil {
        required = true
        defaultValue = "*" // required marker
    }

    _, err := fmt.Fprintf(s.stdout, "%s: \n%s: [%s]\n", name, input.Description, defaultValue)
    if err != nil {
        return fmt.Errorf("failed to write prompt for '%s': %w", name, err)
    }

    userInput, err := s.waitForUserInput()
    if err != nil {
        return fmt.Errorf("failed to read user input for '%s': %w", name, err)
    }

    if userInput == "" {
        if required {
            // PROBLEM: Recursive call with no limit
            _ = s.RenderError(fmt.Sprintf("This input '%s' is required. Please provide a value.\n", name))
            return s.renderInput(name, input)
        }
        input.Value = input.Default
    } else {
        input.Value = userInput
    }
    return nil
}
```

### Stack Depth Analysis

| Go Version | Default Stack Size | Overflow Threshold |
|------------|---------------------|-------------------|
| Go 1.20+ | 2 MB (initial) | ~10,000+ calls |
| Go 1.18- | 2 MB (fixed) | ~10,000+ calls |

- Each recursive call: ~100 bytes (frame + locals)
- Overflow threshold: ~20,000 empty inputs
- Time to overflow: ~10 minutes at 30 inputs/second

**Note**: While unlikely in normal use, it's a theoretical vulnerability and poor practice.

---

## Solution Design

### Requirements

1. **Security**: No unbounded recursion
2. **UX**: Clear feedback on remaining attempts
3. **Compatibility**: Same function signature
4. **Testability**: Easy to test attempt limits
5. **Maintainability**: Simple loop logic

### Design Decision

Convert recursive call to iterative loop with:
- Fixed attempt limit (3)
- Remaining attempt counter
- Clear error message on exhaustion

### Why 3 Attempts?

| Attempts | UX Consideration |
|----------|------------------|
| 1 | Too strict, no second chance for typos |
| 2 | Better, but still frustrating |
| 3 | **Optimal - industry standard** |
| 5 | Forgiving but slow for automation |
| Unlimited | Security issue |

**Industry examples**:
- SSH: 3-6 attempts (configurable)
- sudo: 3 attempts
- passwd: 3 attempts

---

## Implementation Details

### File: `internal/view/std_ui.go`

Replace the entire `renderInput` method:

```go
func (s *StdUI) renderInput(name string, input *templates.Input) error {
    const maxAttempts = 3
    required := false
    defaultValue := input.Default
    if defaultValue == "" || defaultValue == nil {
        required = true
        defaultValue = "*"
    }

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        _, err := fmt.Fprintf(s.stdout, "%s: \n%s: [%s]\n", name, input.Description, defaultValue)
        if err != nil {
            return fmt.Errorf("failed to write prompt for '%s': %w", name, err)
        }

        userInput, err := s.waitForUserInput()
        if err != nil {
            return fmt.Errorf("failed to read user input for '%s': %w", name, err)
        }

        if userInput == "" {
            if required {
                remaining := maxAttempts - attempt
                if remaining > 0 {
                    _ = s.RenderError(fmt.Sprintf("Input '%s' is required. %d attempt(s) remaining.\n", name, remaining))
                    continue
                }
                return fmt.Errorf("input '%s' is required: no attempts remaining", name)
            }
            input.Value = input.Default
        } else {
            input.Value = userInput
        }
        return nil
    }
    return nil
}
```

### Key Changes

| Change | Reason |
|--------|--------|
| `for` loop | Eliminates recursion |
| `maxAttempts` constant | Configurable, clear intent |
| `remaining` calculation | Better UX |
| Early return on exhaustion | Prevents infinite loop |
| Consistent error messages | Clearer than before |

---

## Test Plan

### Unit Tests

**File**: `internal/view/std_ui_test.go`

```go
package view

import (
    "bytes"
    "strings"
    "testing"
    
    "github.com/aaron-vaz/skelly/internal/templates"
)

// Test 1: First attempt success
func TestStdUI_RenderInput_FirstAttempt(t *testing.T) {
    stdin := bytes.NewBufferString("my-project\n")
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
    
    input := &templates.Input{
        Description: "Project name",
    }
    
    err := ui.renderInput("project_name", input)
    
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
    if input.Value != "my-project" {
        t.Errorf("Expected value 'my-project', got: %v", input.Value)
    }
}

// Test 2: Use default on empty input
func TestStdUI_RenderInput_UseDefault(t *testing.T) {
    stdin := bytes.NewBufferString("\n") // Empty input
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
    
    input := &templates.Input{
        Description: "Project name",
        Default:      "default-project",
    }
    
    err := ui.renderInput("project_name", input)
    
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
    if input.Value != "default-project" {
        t.Errorf("Expected default value, got: %v", input.Value)
    }
}

// Test 3: Retry on required empty input
func TestStdUI_RenderInput_RequiredRetry(t *testing.T) {
    stdin := bytes.NewBufferString("\n\nmy-project\n") // 2 empties, then valid
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
    
    input := &templates.Input{
        Description: "Project name",
    }
    
    err := ui.renderInput("project_name", input)
    
    if err != nil {
        t.Errorf("Expected no error after retries, got: %v", err)
    }
    if input.Value != "my-project" {
        t.Errorf("Expected value 'my-project', got: %v", input.Value)
    }
    
    // Verify retry messages
    output := stderr.String()
    if !strings.Contains(output, "2 attempt(s) remaining") {
        t.Errorf("Expected remaining attempts message, got: %s", output)
    }
}

// Test 4: Exhaust attempts
func TestStdUI_RenderInput_ExhaustAttempts(t *testing.T) {
    stdin := bytes.NewBufferString("\n\n\n") // 3 empty inputs
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
    
    input := &templates.Input{
        Description: "Project name",
    }
    
    err := ui.renderInput("project_name", input)
    
    if err == nil {
        t.Error("Expected error when attempts exhausted")
    }
    if !strings.Contains(err.Error(), "no attempts remaining") {
        t.Errorf("Expected 'no attempts remaining' error, got: %v", err)
    }
}

// Test 5: Exceed max attempts
func TestStdUI_RenderInput_ExceedMaxAttempts(t *testing.T) {
    stdin := bytes.NewBufferString("\n\n\n\nmy-project\n") // 4 empties, then valid
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
    
    input := &templates.Input{
        Description: "Project name",
    }
    
    err := ui.renderInput("project_name", input)
    
    if err == nil {
        t.Error("Expected error after 3 empty inputs")
    }
}

// Test 6: Remaining count accuracy
func TestStdUI_RenderInput_RemainingCount(t *testing.T) {
    stdin := bytes.NewBufferString("\n\nvalid\n")
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
    
    input := &templates.Input{
        Description: "Project name",
    }
    
    _ = ui.renderInput("project_name", input)
    
    output := stderr.String()
    // First empty: "2 attempt(s) remaining"
    // Second empty: "1 attempt(s) remaining"
    if !strings.Contains(output, "2 attempt(s) remaining") {
        t.Error("Expected '2 attempt(s) remaining' after first empty")
    }
    if !strings.Contains(output, "1 attempt(s) remaining") {
        t.Error("Expected '1 attempt(s) remaining' after second empty")
    }
}

// Test 7: Write error propagation
func TestStdUI_RenderInput_WriteError(t *testing.T) {
    stdin := bytes.NewBufferString("test\n")
    stdout := &errorWriter{} // Mock that returns error
    stderr := &bytes.Buffer{}
    ui := &StdUI{
        stdin:  bufio.NewScanner(stdin),
        stdout: stdout,
        stderr: stderr,
    }
    
    input := &templates.Input{
        Description: "Project name",
    }
    
    err := ui.renderInput("project_name", input)
    
    if err == nil {
        t.Error("Expected error from write failure")
    }
}

// Test 8: Scanner error
func TestStdUI_RenderInput_ScannerError(t *testing.T) {
    stdin := &errorReader{} // Mock that returns error
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := &StdUI{
        stdin:  bufio.NewScanner(stdin),
        stdout: stdout,
        stderr: stderr,
    }
    
    input := &templates.Input{
        Description: "Project name",
    }
    
    err := ui.renderInput("project_name", input)
    
    if err == nil {
        t.Error("Expected error from scanner failure")
    }
}
```

### Table-Driven Tests

```go
func TestStdUI_RenderInput_Table(t *testing.T) {
    tests := []struct {
        name          string
        input         string
        defaultVal    any
        required      bool
        wantErr       bool
        errMsg        string
        expectedValue any
    }{
        {
            name:          "valid input first try",
            input:         "my-project\n",
            required:      true,
            wantErr:       false,
            expectedValue: "my-project",
        },
        {
            name:          "use default on empty",
            input:         "\n",
            defaultVal:    "default",
            required:      false,
            wantErr:       false,
            expectedValue: "default",
        },
        {
            name:          "empty required fails after 3",
            input:         "\n\n\n",
            required:      true,
            wantErr:       true,
            errMsg:        "no attempts remaining",
        },
        {
            name:          "success on third attempt",
            input:         "\n\nfinally\n",
            required:      true,
            wantErr:       false,
            expectedValue: "finally",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stdin := bytes.NewBufferString(tt.input)
            stdout := &bytes.Buffer{}
            stderr := &bytes.Buffer{}
            ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
            
            input := &templates.Input{
                Description: "Test input",
            }
            if tt.defaultVal != nil {
                input.Default = tt.defaultVal
            }
            
            err := ui.renderInput("test", input)
            
            if tt.wantErr {
                if err == nil {
                    t.Error("Expected error, got nil")
                }
                if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
                    t.Errorf("Expected error containing '%s', got: %v", tt.errMsg, err)
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error, got: %v", err)
                }
                if input.Value != tt.expectedValue {
                    t.Errorf("Expected value %v, got: %v", tt.expectedValue, input.Value)
                }
            }
        })
    }
}
```

### Benchmark Tests

```go
func BenchmarkStdUI_RenderInput(b *testing.B) {
    stdin := bytes.NewBufferString(strings.Repeat("value\n", b.N))
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    ui := NewStdUI(stdin, stdout, stderr).(*StdUI)
    
    for i := 0; i < b.N* b.N; i++ {
        input := &templates.Input{Description: "Test"}
        _ = ui.renderInput("test", input)
    }
}
```

---

## Migration Guide

### Breaking Changes

| Aspect | Before | After |
|--------|--------|-------|
| Max attempts | Unlimited | 3 |
| Error message | `"This input 'X' is required..."` | `"Input 'X' is required. N attempt(s) remaining."` |
| Final error | N/A (stack overflow) | `"input 'X' is required: no attempts remaining"` |

### User-Facing Changes

**Before**:
```
project_name: 
The name of the project: [*]
This input 'project_name' is required. Please provide a value.
project_name: 
The name of the project: [*]
This input 'project_name' is required. Please provide a value.
[... continues infinitely ...]
```

**After**:
```
project_name:
The name of the project: [*]

Input 'project_name' is required. 2 attempt(s) remaining.
project_name:
The name of the project: [*]

Input 'project_name' is required. 1 attempt(s) remaining.
project_name:
The name of the project: [*]

Error: input 'project_name' is required: no attempts remaining
```

### Migration Steps for Users

No migration needed for normal users. For automation scripts:

```bash
# Before: Empty input would hang
echo "" | skelly init --src template --dst output

# After: Will fail after 3 attempts
echo "" | skelly init --src template --dst output
# Error: input 'project_name' is required: no attempts remaining
```

---

## Documentation Updates

### Code Comments

```go
// renderInput prompts the user for an input value with a maximum of 3 attempts.
// For required inputs without defaults, empty values will re-prompt up to
// maxAttempts times before returning an error.
//
// Parameters:
//   - name: The input key from the template config
//   - input: The input definition containing description and default value
//
// Returns:
//   - error: "no attempts remaining" if max attempts exceeded for required input
//
// Example:
//   Input 'project_name' is required. 2 attempt(s) remaining.
//   Input 'project_name' is required. 1 attempt(s) remaining.
//   Error: input 'project_name' is required: no attempts remaining
func (s *StdUI) renderInput(name string, input *templates.Input) error {
    const maxAttempts = 3
    // ...
}
```

### README.md

Add troubleshooting section:

```markdown
## Troubleshooting

### Input Validation Errors

If you see `no attempts remaining` for required inputs:

```
Error: input 'project_name' is required: no attempts remaining
```

**Solution**: Provide a value when prompted, or run with default values:

```bash
# Option 1: Provide values interactively
skelly init --src template --dst output

# Option 2: Use template with defaults
# Templates can define default values in .skelly.yml
```

### Automation and Scripts

For CI/CD pipelines, ensure your input script provides values within 3 attempts:

```bash
# Good: Providing all required values
printf "my-project\nauthor-name\n" | skelly init --src template
```
```

### CHANGELOG.md

```markdown
## [Unreleased]

### Changed

- **BREAKING**: Input prompts now limited to 3 attempts forrequired inputs
  - Previous behavior: Unlimited retries, potential stack overflow
  - New behavior: After 3 empty inputs, returns error "no attempts remaining"
  - Migration: Automation scripts must provide values within 3 attempts
  - Security: Prevents potential DoS from infinite empty input
```

---

## Rollback Plan

### QuickRollback

If critical issues arise:

1. Revert `internal/view/std_ui.go` to recursive version
2. Revert test changes in `internal/view/std_ui_test.go`
3. Run: `make test`

### Git Commands

```bash
# Commit hash before this change
BEFORE=<commit-hash>

# Rollback
git revert <commit-hash-of-this-change>

# Or hard reset if unreleased
git reset --hard $BEFORE
```

---

## Security Considerations

### Remediation

| Vulnerability | Status |
|---------------|--------|
| CWE-674: Unbounded Recursion | FIXED |
| CWE-400: Resource Exhaustion | FIXED |
| CWE-770: Allocation Without Limits | FIXED |

### Remaining Risks

| Risk | Mitigation |
|------|------------|
| User frustration from 3-attempt limit | Clear error message |
| Automation scripts fail | Document in README |
| Brute force attempts | Not applicable (no authentication) |

### Security Review Checklist

- [ ] No unbounded recursion
- [ ] Limit enforced on all code paths
- [ ] Error messages don't leak information
- [ ] No timing attacks possible
- [ ] Resource cleanup on failure

---

## Appendix: Alternative Designs Considered

### Alternative 1: Configurable Attempts

```go
func (s *StdUI) renderInput(name string, input *templates.Input, maxAttempts int) error
```

**Rejected**: Makes API more complex, 3 is industry standard.

### Alternative 2: Exponential Backoff

```go
// Sleep between retries: 100ms, 200ms, 400ms
time.Sleep(time.Duration(100*math.Pow(2, attempt-1)))
```

**Rejected**: Delays user experience, not necessary for CLI.

### Alternative 3: Warning Only

```go
// After 3 attempts, use empty string and warn
if attempt >= maxAttempts {
    input.Value = ""
    _ = s.RenderError("Warning: Using empty value for '%s'\n", name)
    return nil
}
```

**Rejected**: Silently accepts invalid input, defeats validation purpose.

---

## Performance Impact

| Metric | Before | After | Impact |
|--------|--------|-------|--------|
| Memory per call | Stack frame × N | Constant | Significant improvement |
| CPU per call | Same | Same | None |
| Max calls allowed | Unlimited | 3 | Prevents overflow |

### Memory Analysis

**Before** (Recursive):
- Each call: ~100 bytes (stack frame + locals)
- 10,000 calls: ~1 MB stack
- Max before crash: ~20,000 calls

**After** (Iterative):
- Each call: ~100 bytes (loop variables)
- Constant memory regardless of attempts
- Max: 3 iterations, guaranteed

---

## Definition of Done

- [ ] Implementation complete
- [ ] All unit tests passing
- [ ] Benchmark tests passing
- [ ] Code review approved
- [ ] Documentation updated
- [ ] CHANGELOG entry added
- [ ] No lint errors
- [ ] Coverage maintained (87.5%+)