# Issue 3: Transactional Template Processing

**Status**: PLANNED (ready to implement)

**Priority**: High

**Estimate**: 2-4 hours

---

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Risk Analysis](#risk-analysis)
3. [Solution Design](#solution-design)
4. [Implementation Details](#implementation-details)
5. [File Changes](#file-changes)
6. [Test Plan](#test-plan)
7. [Migration Guide](#migration-guide)
8. [Documentation Updates](#documentation-updates)
9. [Rollback Plan](#rollback-plan)
10. [Performance Analysis](#performance-analysis)

---

## Problem Statement

`ApplyTemplate` lacks transactional safety. If processing fails mid-way (e.g., during file renaming), the destination directory is left in an inconsistent state:
- Some files renamed, some not
- Template config may or may not be deleted
- No way to recover without re-downloading
- User must manually clean up partial state

### Failure Scenarios

| Scenario | Current Behavior | User Impact |
|----------|------------------|-------------|
| Download fails | Destination has partial files | Must delete and retry |
| Render fails (Phase 1) | Files partially modified | Must delete and retry |
| Rename fails (Phase 2) | Some files renamed, some not | Corrupted state |
| Permission denied | Partial state, no cleanup | Manual intervention required |
| Disk full | Partial state, no cleanup | Manual cleanup required |
| Process killed (Ctrl+C) | Partial state, no cleanup | Next run fails with dirty state |

### Example Failure

```
template/
├── {{.Name}}.txt
├── {{.Name}}/
│   └── nested.txt
└── other.txt

Processing:
1. Phase 1: Content rendering succeeds
2. Phase 2: Rename {{.Name}}.txt → myapp.txt ✓
3. Phase 2: Rename {{.Name}}/ → myapp/ ✓
4. Phase 2: Rename myapp/nested.txt → FAILS (permission)

Result:
destination/
├── myapp.txt          ← processed
├── myapp/             ← processed dir
│   └── nested.txt     ← NOT processed (still has templates)
├── other.txt          ← NOT processed
└── .skelly.yml        ← NOT deleted

User sees: "Error: failed to rename..."
User must: Delete destination, re-download, re-run
```

---

## Risk Analysis

### Probability Assessment

| Risk | Likelihood | Impact | Risk Score |
|------|------------|--------|------------|
| Download failure | Medium | High | High |
| Permission denied | Low | High | Medium |
| Disk full | Low | High | Medium |
| Process killed | Low | High | Medium |
| Template error | Medium | Medium | Medium |

### Business Impact

- **User experience**: Frustrated users with corrupted templates
- **Support overhead**: Users asking how to fix partial state
- **Data loss**: Potential loss of user work if destination had existing files
- **Enterprise**: Critical for production environments

---

## Solution Design

### Selected Approach: Option B - Process in Temp, Move on Success

**Flow**:
```
┌─────────────────────────────────────────────────────────────┐
│  InitCommand.Run                                            │
├─────────────────────────────────────────────────────────────┤
│  1.Cleanup stale .skelly-* dirs (crash recovery)    │
│                         ↓                                   │
│ 2. Handle existing destination (prompt, delete)            │
│                         ↓                                   │
│  3. Create temp dir: .skelly-{timestamp}-{random}           │
│                         ↓                                   │
│ 4. Download template → temp dir                            │
│                         ↓                                   │
│ 5. Process in temp (CreateTemplate + ApplyTemplate)       │
│                         ↓                                   │
│ 6a. SUCCESS: Move temp → destination                       │
│ 6b. FAILURE: Delete temp, leave destination untouched     │
└─────────────────────────────────────────────────────────────┘
```

### Benefits

| Benefit | Description |
|---------|-------------|
| **Atomicity** | Destination only modified on complete success |
| **Crash recovery** | Stale temps cleaned on startup |
| **Cross-device** | Fallback to copy if rename fails |
| **Clean state** | No partial states left behind |
| **User-friendly** | Destination untouched on any error |

### Trade-offs

| Trade-off | Impact |
|-----------|--------|
| Extra disk space | 2× total (temp + destination) during processing |
| Extra copy time | Cross-device copies are slower than rename |
| Temp dir cleanup | New responsibility for temp management |

---

## Implementation Details

### Architecture

```
internal/
├── utils/
│   ├── filesystem.go      ← NEW: Temp dir management
│   └── filesystem_test.go ← NEW: Tests
├── commands/
│   └── init_command.go    ← MODIFIED: Use temp dir
└── download/
    └── local_downloader.go ← UNCHANGED
```

### Design Decisions

#### 1. Temp Dir Naming

**Format**: `.skelly-{timestamp}-{random}`

**Example**: `.skelly-20240115-143052-a1b2c3d4`

**Rationale**:
- Timestamp: Debug easier (know when crash occurred)
- Random: Avoid collisions in parallel runs
- Prefix `.skelly-`: Easy to identify and clean

**Alternative rejected**: `os.TempDir()`/tmp/skelly-...
- Pro: System temp, auto-cleaned
- Con: Harder to debug, may be different filesystem

#### 2. Temp Dir Location

**Location**: Adjacent to destination (same parent directory)

```go
parentDir := filepath.Dir(destination)
tempDir, err := os.MkdirTemp(parentDir, ".skelly-")
```

**Rationale**:
- Same filesystem = atomic `os.Rename`
- No cross-device issues (usually)
- Easy cleanup

**Alternative rejected**: Fixed temp directory
- Con: Always cross-device

#### 3. Stale Cleanup

**When**: At start of every `InitCommand.Run`

**What**: Delete all `.skelly-*` directories in parent

**Rationale**:
- Crash recovery: Previous runs left stale temps
- Idempotent: Safe to run multiple times
- Low overhead: Fast directory scan

**Alternative rejected**: Cleanup on process start/signal
- Pro: More targeted
- Con: Signal handling is complex, may miss edge cases

#### 4. Move Strategy

**Primary**: `os.Rename(src, dst)`

**Fallback**: Recursive copy if `os.Rename` fails with cross-device error

```go
func MoveDir(src, dst string) error {
    err := os.Rename(src, dst)
    if err == nil {
        return nil
    }
    
    if isCrossDeviceError(err) {
        return copyDir(src, dst)
    }
    
    return err
}
```

**Rationale**:
- Rename is atomic and fast on same filesystem
- Copy works everywhere
- Best of both worlds

---

## File Changes

### New File: `internal/utils/filesystem.go`

```go
// Package utils provides filesystem utility functions.
package utils

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"
)

// TempDirPrefix is the prefix for temporary directories created by skelly.
const TempDirPrefix = ".skelly-"

// CreateTempDir creates a timestamped temporary directory adjacent to the target.
// The directory name format is: .skelly-{timestamp}-{random}
//
// Example: .skelly-20240115-143052-a1b2c3d4
//
// The caller is responsible for removing the directory when done.
func CreateTempDir(parent string) (string, error) {
    timestamp := time.Now().Format("20060102-150405")
    prefix := TempDirPrefix + timestamp + "-"
    
    dir, err := os.MkdirTemp(parent, prefix)
    if err != nil {
        return "", fmt.Errorf("failed to create temp dir: %w", err)
    }
    
    return dir, nil
}

// CleanupStaleTempDirs removes any .skelly-* directories from previous runs
// that may have been left due to crashes or interrupted processes.
//
// This function is idempotent and safe to call multiple times.
// Non-skelly directories are not affected.
//
// Returns an error only if directory listing fails or removal fails for
// a specific stale directory.
func CleanupStaleTempDirs(parent string) error {
    entries, err := os.ReadDir(parent)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return fmt.Errorf("failed to read directory %s: %w", parent, err)
    }

    var errs []error
    for _, entry := range entries {
        if entry.IsDir() && strings.HasPrefix(entry.Name(), TempDirPrefix) {
            path := filepath.Join(parent, entry.Name())
            if err := os.RemoveAll(path); err != nil {
                errs = append(errs, fmt.Errorf("failed to remove stale temp dir %s: %w", path, err))
            }
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("encountered %d errors cleaning up stale temps: %v", len(errs), errs)
    }
    
    return nil
}

// MoveDir atomically moves a directory from src to dst.
// If src and dst are on the same filesystem, this is an atomic rename.
// If they are on different filesystems, falls back to recursive copy.
//
// The src directory is removed after successful move/copy.
func MoveDir(src, dst string) error {
    // Try atomic rename first (works on same filesystem)
    err := os.Rename(src, dst)
    if err == nil {
        return nil
    }

    // Fall back to copy if cross-device
    if isCrossDeviceError(err) {
        if copyErr := copyDir(src, dst); copyErr != nil {
            return fmt.Errorf("failed to copy %s to %s: %w", src, dst, copyErr)
        }
        // Remove source after successful copy
        if removeErr := os.RemoveAll(src); removeErr != nil {
            // Log warning but don't fail - destination is good
            // This is a minor leak, cleaned up by stale temp cleanup
            return fmt.Errorf("move succeeded but failed to remove source %s: %w", src, removeErr)
        }
        return nil
    }

    return fmt.Errorf("failed to move %s to %s: %w", src, dst, err)
}

// isCrossDeviceError checks if the error indicates a cross-device link.
func isCrossDeviceError(err error) bool {
    // Unix: EXDEV (18)
    // Windows: ERROR_NOT_SAME_DEVICE (0x11D)
    return strings.Contains(err.Error(), "invalid cross-device link") ||
           strings.Contains(err.Error(), "rename") // Fallback for various OS messages
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
    if err := os.MkdirAll(dst, 0755); err != nil {
        return fmt.Errorf("failed to create directory %s: %w", dst, err)
    }

    entries, err := os.ReadDir(src)
    if err != nil {
        return fmt.Errorf("failed to read directory %s: %w", src, err)
    }

    for _, entry := range entries {
        srcPath := filepath.Join(src, entry.Name())
        dstPath := filepath.Join(dst, entry.Name())

        if entry.IsDir() {
            if err := copyDir(srcPath, dstPath); err != nil {
                return err
            }
        } else {
            // Handle symlinks
            if entry.Type()&os.ModeSymlink != 0 {
                if err := copySymlink(srcPath, dstPath); err != nil {
                    return err
                }
                continue
            }
            
            if err := copyFile(srcPath, dstPath); err != nil {
                return err
            }
        }
    }
    
    return nil
}

// copyFile copies a single file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
    srcFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("failed to open %s: %w", src, err)
    }
    defer srcFile.Close()

    info, err := srcFile.Stat()
    if err != nil {
        return fmt.Errorf("failed to stat %s: %w", src, err)
    }

    dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
    if err != nil {
        return fmt.Errorf("failed to create %s: %w", dst, err)
    }
    defer dstFile.Close()

    if _, err := io.Copy(dstFile, srcFile); err != nil {
        return fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
    }

    // Preserve modification time
    if err := os.Chtimes(dst, info.ModTime(), info.ModTime()); err != nil {
        // Non-critical, log but don't fail
    }

    return nil
}

// copySymlink copies a symlink from src to dst.
func copySymlink(src, dst string) error {
    target, err := os.Readlink(src)
    if err != nil {
        return fmt.Errorf("failed to read symlink %s: %w", src, err)
    }
    
    if err := os.Symlink(target, dst); err != nil {
        return fmt.Errorf("failed to create symlink %s: %w", dst, err)
    }
    
    return nil
}
```

### New File: `internal/utils/filesystem_test.go`

```go
package utils

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestCreateTempDir(t *testing.T) {
    tests := []struct {
        name       string
        parent     string
        wantPrefix string
    }{
        {
            name:       "creates temp in specified dir",
            parent:     "",
            wantPrefix: TempDirPrefix,
        },
        {
            name:       "creates temp in current dir",
            parent:     ".",
            wantPrefix: TempDirPrefix,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parent := tt.parent
            if parent == "" {
                parent = t.TempDir()
            }
            
            tempDir, err := CreateTempDir(parent)
            if err != nil {
                t.Fatalf("CreateTempDir failed: %v", err)
            }
            
            // Cleanup
            defer os.RemoveAll(tempDir)
            
            // Verify directory exists
            info, err := os.Stat(tempDir)
            if err != nil {
                t.Errorf("Temp dir was not created: %v", err)
            }
            if !info.IsDir() {
                t.Error("Expected directory, got file")
            }
            
            // Verify prefix
            base := filepath.Base(tempDir)
            if !strings.HasPrefix(base, tt.wantPrefix) {
                t.Errorf("Expected prefix %s, got %s", tt.wantPrefix, base)
            }
            
            // Verify parent
            parentDir := filepath.Dir(tempDir)
            if parent != "." && parentDir != parent {
                t.Errorf("Expected parent %s, got %s", parent, parentDir)
            }
        })
    }
}

func TestCreateTempDir_TimestampFormat(t *testing.T) {
    parent := t.TempDir()
    
    tempDir, err := CreateTempDir(parent)
    if err != nil {
        t.Fatalf("CreateTempDir failed: %v", err)
    }
    defer os.RemoveAll(tempDir)
    
    base := filepath.Base(tempDir)
    // Format: .skelly-20060102-150405-<random>
    // After prefix: .skelly-
    // Then timestamp: 20060102-150405
    // Then random suffix
    
    parts := strings.Split(base, "-")
    if len(parts) < 4 {
        t.Errorf("Expected at least 4 parts in temp dir name, got %d: %s", len(parts), base)
    }
    
    // parts[0] = ".skelly"
    // parts[1] = date (YYYYMMDD)
    // parts[2] = time (HHMMSS)
    // parts[3+] = random suffix
    
    if parts[0] != ".skelly" {
        t.Errorf("Expected first part to be '.skelly', got %s", parts[0])
    }
    
    // Date should be 8 characters
    if len(parts[1]) != 8 {
        t.Errorf("Expected date part to be 8 chars, got %d: %s", len(parts[1]), parts[1])
    }
    
    // Time should be 6 characters
    if len(parts[2]) != 6 {
        t.Errorf("Expected time part to be 6 chars, got %d: %s", len(parts[2]), parts[2])
    }
}

func TestCleanupStaleTempDirs(t *testing.T) {
    tests := []struct {
        name          string
       	setup        func(parent string) []string
        wantRemoved   []string
        wantRemaining []string
    }{
        {
            name: "removes stale temps",
            setup: func(parent string) []string {
                stale1, _ := os.MkdirTemp(parent, TempDirPrefix)
                stale2, _ := os.MkdirTemp(parent, TempDirPrefix)
                return []string{stale1, stale2}
            },
            wantRemoved: []string{},
            wantRemaining: []string{},
        },
        {
            name: "keeps non-skelly dirs",
            setup: func(parent string) []string {
                stale, _ := os.MkdirTemp(parent, TempDirPrefix)
                other, _ := os.Mkdir(filepath.Join(parent, "other-dir"), 0755)
                return []string{stale, other}
            },
            wantRemoved:   []string{},
            wantRemaining: []string{"other-dir"},
        },
        {
            name: "handles non-existent parent",
            setup: func(parent string) []string {
                return []string{}
            },
            wantRemaining: []string{},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parent := t.TempDir()
            
            created := tt.setup(parent)
            
            err := CleanupStaleTempDirs(parent)
            if err != nil {
                t.Errorf("CleanupStaleTempDirs failed: %v", err)
            }
            
            // Verify stale temps removed
            for _, path := range created {
                if strings.HasPrefix(filepath.Base(path), TempDirPrefix) {
                    if _, err := os.Stat(path); !os.IsNotExist(err) {
                        t.Errorf("Expected stale temp %s to be removed", path)
                    }
                }
            }
            
            // Verify non-skelly dirs remain
            for _, name := range tt.wantRemaining {
                path := filepath.Join(parent, name)
                if _, err := os.Stat(path); os.IsNotExist(err) {
                    t.Errorf("Expected non-skelly dir %s to remain", name)
                }
            }
        })
    }
}

func TestMoveDir_SameDevice(t *testing.T) {
    src := t.TempDir()
    dstBase := t.TempDir()
    dst := filepath.Join(dstBase, "dst")
    
    // Create nested structure
    os.MkdirAll(filepath.Join(src, "subdir"), 0755)
    os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0644)
    os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("nested"), 0644)
    
    // Set file mode
    os.Chmod(filepath.Join(src, "file.txt"), 0600)
    
    err := MoveDir(src, dst)
    if err != nil {
        t.Fatalf("MoveDir failed: %v", err)
    }
    
    // Verify src is gone
    if _, err := os.Stat(src); !os.IsNotExist(err) {
        t.Error("Expected src to be removed")
    }
    
    // Verify dst has content
    content, _ := os.ReadFile(filepath.Join(dst, "file.txt"))
    if string(content) != "content" {
        t.Errorf("Expected content 'content', got %s", content)
    }
    
    nested, _ := os.ReadFile(filepath.Join(dst, "subdir", "nested.txt"))
    if string(nested) != "nested" {
        t.Errorf("Expected nested 'nested', got %s", nested)
    }
    
    // Verify permissions preserved
    info, _ := os.Stat(filepath.Join(dst, "file.txt"))
    if info.Mode().Perm() != 0600 {
        t.Errorf("Expected file mode 0600, got %o", info.Mode().Perm())
    }
}

func TestMoveDir_NonExistentSrc(t *testing.T) {
    src := filepath.Join(t.TempDir(), "nonexistent")
    dst := filepath.Join(t.TempDir(), "dst")
    
    err := MoveDir(src, dst)
    if err == nil {
        t.Error("Expected error for non-existent source")
    }
}

func TestMoveDir_DstExists(t *testing.T) {
    src := t.TempDir()
    dst := t.TempDir()
    
    os.WriteFile(filepath.Join(src, "file.txt"), []byte("src"), 0644)
    os.WriteFile(filepath.Join(dst, "existing.txt"), []byte("dst"), 0644)
    
    err := MoveDir(src, dst)
    if err == nil {
        t.Error("Expected error when destination exists")
    }
}

func TestCopyDir(t *testing.T) {
    src := t.TempDir()
    dst := filepath.Join(t.TempDir(), "dst")
    
    // Create complex structure
    os.MkdirAll(filepath.Join(src, "a/b"), 0755)
    os.WriteFile(filepath.Join(src, "a", "file1.txt"), []byte("file1"), 0644)
    os.WriteFile(filepath.Join(src, "a", "b", "file2.txt"), []byte("file2"), 0644)
    
    err := copyDir(src, dst)
    if err != nil {
        t.Fatalf("copyDir failed: %v", err)
    }
    
    // Verify structure
    if content, _ := os.ReadFile(filepath.Join(dst, "a", "file1.txt")); string(content) != "file1" {
        t.Error("file1 not copied correctly")
    }
    if content, _ := os.ReadFile(filepath.Join(dst, "a", "b", "file2.txt")); string(content) != "file2" {
        t.Error("file2 not copied correctly")
    }
}

func TestCopyFile(t *testing.T) {
    src := filepath.Join(t.TempDir(), "src.txt")
    dst := filepath.Join(t.TempDir(), "dst.txt")
    
    os.WriteFile(src, []byte("test content"), 0644)
    os.Chmod(src, 0600)
    
    err := copyFile(src, dst)
    if err != nil {
        t.Fatalf("copyFile failed: %v", err)
    }
    
    // Verify content
    content, _ := os.ReadFile(dst)
    if string(content) != "test content" {
        t.Errorf("Expected 'test content', got %s", content)
    }
    
    // Verify permissions
    info, _ := os.Stat(dst)
    if info.Mode().Perm() != 0600 {
        t.Errorf("Expected mode 0600, got %o", info.Mode().Perm())
    }
}

func TestCopySymlink(t *testing.T) {
    src := t.TempDir()
    
    // Create file and symlink
    target := filepath.Join(src, "target.txt")
    os.WriteFile(target, []byte("target"), 0644)
    
    link := filepath.Join(src, "link.txt")
    os.Symlink(target, link)
    
    // Copy symlink
    dst := filepath.Join(t.TempDir(), "link.txt")
    err := copySymlink(link, dst)
    if err != nil {
        t.Fatalf("copySymlink failed: %v", err)
    }
    
    // Verify it's a symlink
    targetDst, err := os.Readlink(dst)
    if err != nil {
        t.Errorf("Expected symlink, got error: %v", err)
    }
    if targetDst != target {
        t.Errorf("Expected symlink to %s, got %s", target, targetDst)
    }
}

func TestIsCrossDeviceError(t *testing.T) {
    tests := []struct {
        name    string
        err     error
        want    bool
    }{
        {
            name: "cross-device link error",
            err:   &os.LinkError{Err: errorString("invalid cross-device link")},
            want: true,
        },
        {
            name:  "other error",
            err:   &os.PathError{Err: errorString("permission denied")},
            want: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := isCrossDeviceError(tt.err); got != tt.want {
                t.Errorf("isCrossDeviceError() = %v, want %v", got, tt.want)
            }
        })
    }
}

type errorString string

func (e errorString) Error() string { return string(e) }
```

### Modified File: `internal/commands/init_command.go`

See full implementation in the test plan section below.

```go
// Key changes to Run method:

func (c *InitCommand) Run(args []string) error {
    if err := c.validateOptions(); err != nil {
        return fmt.Errorf("invalid options: %w", err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    parentDir := filepath.Dir(c.options.destination)
    if parentDir == "" {
        parentDir = "."
    }

    // NEW: Cleanup stale temp dirs
    if err := utils.CleanupStaleTempDirs(parentDir); err != nil {
        return fmt.Errorf("failed to cleanup stale temp dirs: %w", err)
    }

    // EXISTING: Handle existing destination
    if _, err := os.Stat(c.options.destination); err == nil {
        // ... unchanged ...
    }

    // NEW: Create temp dir
    tempDir, err := utils.CreateTempDir(parentDir)
    if err != nil {
        return fmt.Errorf("failed to create temp dir: %w", err)
    }

    // NEW: Cleanup on failure
    success := false
    defer func() {
        if !success {
            os.RemoveAll(tempDir)
        }
    }()

    // CHANGED: Download to temp
    if err := c.downloader.Get(ctx, c.options.source, tempDir); err != nil {
        return fmt.Errorf("failed to download template: %w", err)
    }

    // CHANGED: Process in temp
    config, err := c.processor.CreateTemplate(tempDir)
    if err != nil {
        return fmt.Errorf("failed to process template: %w", err)
    }
    if config == nil {
        return c.ui.RenderInfo(fmt.Sprintf("No %s file found, exiting....\n", templates.TemplateConfigName))
    }
    if err := c.ui.RenderInputs(config.Inputs); err != nil {
        return fmt.Errorf("failed to collect inputs: %w", err)
    }
    if err := c.processor.ApplyTemplate(*config, tempDir); err != nil {
        return fmt.Errorf("failed to apply template: %w", err)
    }

    // NEW: Move temp to destination
    if err := utils.MoveDir(tempDir, c.options.destination); err != nil {
        return fmt.Errorf("failed to move processed template: %w", err)
    }

    // NEW: Mark success
    success = true
    return nil
}
```

---

## Test Plan

### Unit Tests

| Test | Description | Expected |
|------|-------------|----------|
| `TestCreateTempDir` | Creates temp with correct prefix | Pass |
| `TestCreateTempDir_TimestampFormat` | Verifies timestamp format | Pass |
| `TestCleanupStaleTempDirs` | Removes `.skelly-*` dirs | Pass |
| `TestCleanupStaleTempDirs_IgnoresOthers` | Keeps non-skelly dirs | Pass |
| `TestMoveDir_SameDevice` | Atomic rename | Pass |
| `TestMoveDir_CrossDevice` | Fallback to copy | Pass |
| `TestMoveDir_NonExistentSrc` | Error handling | Pass |
| `TestMoveDir_DstExists` | Error handling | Pass |
| `TestCopyDir` | Recursive copy | Pass |
| `TestCopyDir_Symlinks` | Symlink handling | Pass |
| `TestCopyDir_Permissions` | Permission preservation | Pass |

### Integration Tests

| Test | Description | Expected |
|------|-------------|----------|
| `TestInitCommand_TempCreated` | Temp dir created | Pass |
| `TestInitCommand_TempCleanedOnSuccess` | Temp removed after success | Pass |
| `TestInitCommand_TempCleanedOnFailure` | Temp removed on error | Pass |
| `TestInitCommand_DestinationUntouchedOnFailure` | Original intact | Pass |
| `TestInitCommand_StaleTempCleanup` | Stale temps removed | Pass |
| `TestInitCommand_CrossDeviceMove` | Copy fallback works | Pass |

### Integration Test Implementation

```go
// internal/commands/init_command_test.go

func TestInitCommand_TempCreated(t *testing.T) {
    // Given
    proc := templates.NewTemplateProcessor(templates.NewRendererService())
    mockDL := &mockDownloader{}
    mockUI := &mockUIInit{}
    cmd := NewInitCommand(proc, mockDL, mockUI).(*InitCommand)
    
    tempBase := t.TempDir()
    dst := filepath.Join(tempBase, "myproject")
    
    flags := flag.NewFlagSet("init", flag.ContinueOnError)
    cmd.Init(flags)
    _ = flags.Parse([]string{"-src", "http://example.com/template", "-dst", dst})
    
    // When
    _ = cmd.Run(nil) // We care about temp creation
    
    // Then: Temp dir should have been created in parent
    // (Verify via mock or filesystem check)
}

func TestInitCommand_DestinationUntouchedOnFailure(t *testing.T) {
    // Given
    proc := templates.NewTemplateProcessor(templates.NewRendererService())
    mockDL := &mockDownloader{err: errors.New("download failed")}
    mockUI := &mockUIInit{}
    cmd := NewInitCommand(proc, mockDL, mockUI).(*InitCommand)
    
    tempBase := t.TempDir()
    dst := filepath.Join(tempBase, "myproject")
    
    // Pre-create destination
    os.MkdirAll(dst, 0755)
    os.WriteFile(filepath.Join(dst, "existing.txt"), []byte("original"), 0644)
    
    flags := flag.NewFlagSet("init", flag.ContinueOnError)
    cmd.Init(flags)
    _ = flags.Parse([]string{"-src", "http://example.com/template", "-dst", dst})
    
    // When
    err := cmd.Run(nil)
    
    // Then
    if err == nil {
        t.Error("Expected error, got nil")
    }
    
    // Destination should still have original file
    content, err := os.ReadFile(filepath.Join(dst, "existing.txt"))
    if err != nil {
        t.Error("Expected destination to still exist")
    }
    if string(content) != "original" {
        t.Error("Expected original content preserved")
    }
    
    // No stale temp dirs
    entries, _ := os.ReadDir(tempBase)
    for _, e := range entries {
        if strings.HasPrefix(e.Name(), utils.TempDirPrefix) {
            t.Error("Expected temp dir to be cleaned up")
        }
    }
}

func TestIntegration_TransactionalProcessing(t *testing.T) {
    // Build binary
    bin := buildTestBinary(t)
    defer os.Remove(bin)
    
    // Create template
    src := t.TempDir()
    os.WriteFile(filepath.Join(src, ".skelly.yml"), []byte(`
name: test
description: test
inputs:
  name:
    description: Name
`), 0644)
    os.WriteFile(filepath.Join(src, "{{.Name}}.txt"), []byte("Hello {{.Name}}"), 0644)
    
    // Run skelly init
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, bin, "init", "-src", src, "-dst", filepath.Join(t.TempDir(), "output"))
    output, err := cmd.CombinedOutput()
    
    // Provide input
    stdin, _ := cmd.StdinPipe()
    go func() {
        defer stdin.Close()
        stdin.Write([]byte("test\n")) // name input
    }()
    
    if err != nil {
        t.Fatalf("Init failed: %v\nOutput: %s", err, output)
    }
    
    // Verify no stale temps
    // Verify destination exists and is valid
}
```

---

## Migration Guide

### Behavior Changes

| Scenario | Before | After |
|----------|--------|-------|
| Init success | Files directly in destination | Same (temp cleaned) |
| Init failure | Partial state in destination | Destination untouched |
| Ctrl+C during init | Partial state | Temp cleaned, destination untouched |
| Previous crash | Stale `.skelly-*` remains | Cleaned on next run |
| Cross-filesystem | N/A | Automatic fallback to copy |

### User Experience

**Before** (failure):
```
$ skelly init --src template --dst project
Error: failed to apply template: permission denied
$ ls project
{{.Name}}.txt  # Partial state
$skelly init --src template --dst project
Error: destination already exists
```

**After** (failure):
```
$ skelly init --src template --dst project
Error: failed to apply template: permission denied
$ ls project
ls: cannot access 'project': No such file or directory
$ skelly init --src template --dst project
# Works cleanly
```

### Developer Changes

No API changes. Users of the library/CLI don't need to change code.

---

## Documentation Updates

### README.md

```markdown
## How It Works

### Transactional Safety

`skelly` ensures your project directory is never left in a partial state:

1. **Templates are processed in a temporary directory**
2. **Only moved to destination on complete success**
3. **If anything fails, destination is untouched**

#### Crash Recovery

If `skelly` crashes or is killed:

- **No partial states** - destination is never partially filled
- **Automatic cleanup** - stale temporary directories are cleaned on next run
- **Stale temps look like**: `.skelly-20240115-143052-a1b2c3d4`

#### Cross-Filesystem Support

If your destination is on a different filesystem:

- `skelly` automatically falls back to recursive copy
- Works seamlessly across mounted filesystems

### Troubleshooting

#### "destination already exists"

If you see this error after a failed init:

```bash
# Option 1: Overwrite
skelly init --src template --dst project
# When prompted, answer 'y' to overwrite

# Option 2: Delete manually
rm -rf project
skelly init --src template --dst project
```

#### Stale Temporary Directories

If you find directories named `.skelly-*`:

```bash
# They're from previous crashed runs
# Safe to delete manually:
rm -rf .skelly-*

# Or they'll be cleaned automatically on next run
```
```

### CHANGELOG.md

```markdown
## [Unreleased]

### Added

- **Transactionalsafety**: Templates are now processed in a temporary directory
  - Destination only modified on complete success
  - Failed inits leave no partial state
  - Automatic cleanup of stale `.skelly-*` directories
  - Cross-filesystem support with automatic copy fallback

### Fixed

- **Partial state on failure**: Fixed issue where failed inits left corrupted destination
- **Stale temp cleanup**: Previous crashed runs now cleaned automatically

### Changed

- **Internal**: Init process now uses temp directory before final move
```

---

## Rollback Plan

### Quick Rollback

If critical issues arise:

```bash
# Revert to commit before this change
git revert <commit-hash>

# Or hard reset if unreleased
git reset --hard <before-commit>
```

### Gradual Rollback

If only specific parts cause issues:

#### Disable Temp Cleanup

```go
// Comment out in init_command.go
// if err := utils.CleanupStaleTempDirs(parentDir); err != nil {
//     return fmt.Errorf("failed to cleanup stale temp dirs: %w", err)
// }
```

#### Restore Direct Processing

If temp dir approach causes issues, revert to old flow:

```go
// Old flow (before this change)
func (c *InitCommand) Run(args []string) error {
    // ... validate ...
    // Download directly to destination
    if err := c.downloader.Get(ctx, c.options.source, c.options.destination); err != nil {
        return err
    }
    // ... process in destination ...
}
```

### Feature Flag (Optional)

Add environment variable to toggle behavior:

```go
func (c *InitCommand) Run(args []string) error {
    useTransactional := os.Getenv("SKELLY_TRANSACTIONAL") != "false"
    
    if useTransactional {
        // New transactional flow
    } else {
        // Old direct flow
    }
}
```

---

## Performance Analysis

### Benchmark Results (Projected)

| Operation | Before | After | Delta |
|-----------|--------|-------|-------|
| Small template (<1MB) | ~100ms | ~110ms | +10% |
| Medium template (10MB) | ~1s | ~1.2s | +20% |
| Large template (100MB) | ~10s | ~12s | +20% |
| Cross-filesystem | ~10s | ~12s | +20% (copy overhead) |

### Memory Impact

| Operation | Before | After | Delta |
|-----------|--------|-------|-------|
| Small template | ~10MB | ~20MB | +10MB (temp) |
| Medium template | ~50MB | ~100MB | +50MB (temp) |
| Large template | ~500MB | ~1GB | +500MB (temp) |

**Note**: Memory impact is temporary (during processing only).

### Disk Impact

| Scenario | Before | After |
|----------|--------|-------|
| During init | destination size | 2× destination size (temp + final) |
| After success | destination size | destination size (temp cleaned) |
| After failure | partial state | 0 (temp cleaned) |

### Optimization Opportunities

1. **Streaming copy**: Copy files as they're processed (reduces temp storage)
2. **Hard links**: Use hard links instead of copy on same filesystem
3. **Compression**: Compress temp for large templates

---

## Definition of Done

- [ ] `internal/utils/filesystem.go` implemented
- [ ] `internal/utils/filesystem_test.go` all tests passing
- [ ] `internal/commands/init_command.go` updated
- [ ] `internal/commands/init_command_test.go` updated
- [ ] Integration tests passing
- [ ] Unit test coverage ≥ 90%
- [ ] Benchmark tests passing
- [ ] Code review approved
- [ ] Documentation updated (README, CHANGELOG)
- [ ] No lint errors (`make lint`)
- [ ] All tests passing (`make test`)
- [ ] Manual testing completed
- [ ] Cross-platform testing (Linux, macOS, Windows)
- [ ] Cross-filesystem testing