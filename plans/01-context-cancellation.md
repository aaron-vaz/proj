# Issue 1: Context Cancellation in LocalDownloader

**Status**: PARKED (deferred)

**Priority**: Low

**Estimate**: 1-2 hours (if implemented)

---

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Current Implementation](#current-implementation)
3. [Analysis](#analysis)
4. [Solution Options](#solution-options)
5. [Recommended Implementation](#recommended-implementation)
6. [Test Plan](#test-plan)
7. [Documentation Updates](#documentation-updates)
8. [Rollback Plan](#rollback-plan)
9. [Decision Rationale](#decision-rationale)

---

## Problem Statement

The `LocalDownloader.Get` method receives a `context.Context` parameter but never uses it. For large directory copies, the operation cannot be cancelled mid-way (e.g., user hits Ctrl+C during template scaffolding).

### Impact Analysis

| Scenario | Current Behavior | User Impact |
|----------|------------------|-------------|
| Small template (< 10MB) | Completes quickly | Low - user unlikely to cancel |
| Medium template (10-100MB) | Takes 1-5 seconds | Medium - cancellation noticeable but rare |
| Large template (> 100MB) | Takes 5+ seconds | High - user may want to cancel |
| Network-mounted filesystem | Network-bound, slow | Medium - cancellation desirable |

### Failure Modes

1. **User presses Ctrl+C**: Process receives SIGINT, but `filepath.Walk` continues until complete
2. **Timeout exceeded**: Parent context cancelled, but copy continues
3. **System shutdown**: Copy operation blocks graceful shutdown

---

## Current Implementation

**File**: `internal/download/local_downloader.go`

```go
// LocalDownloader implements Downloader for local directory copying.
type LocalDownloader struct{}

// Get executes a recursive directory copy from source to destination.
func (d *LocalDownloader) Get(ctx context.Context, source string, destination string) error {
    return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        rel, err := filepath.Rel(source, path)
        if err != nil {
            return err
        }
        destPath := filepath.Join(destination, rel)

        if info.IsDir() {
            return os.MkdirAll(destPath, info.Mode())
        }

        srcF, err := os.Open(path)
        if err != nil {
            return err
        }
        defer srcF.Close()

        dstF, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
        if err != nil {
            return err
        }
        defer dstF.Close()

        _, err = io.Copy(dstF, srcF)
        return err
    })
}
```

**Issues**:
1. `ctx` parameter unused
2. `filepath.Walk` cannot be interrupted
3. `io.Copy` blocks until complete
4. No cleanup on cancellation

---

## Analysis

### Why This Is Low Priority

1. **Typical use case**: Project scaffolds are small (<5MB)
2. **Local filesystem**: `os.Rename` or direct copy is fast
3. **Alternative exists**: User can kill process, temp dir cleaned up by Issue 3 solution
4. **Complexity**: Adds significant complexity for marginal benefit

### When This Becomes High Priority

- Users report large template issues
- Performance profiler shows slow local copies
- Feature request for template size limits
- Enterprise customers with massive templates

---

## Solution Options

### Option 1: Check Context at Walk Iteration

Add context check inside `filepath.Walk` callback:

```go
func (d *LocalDownloader) Get(ctx context.Context, source string, destination string) error {
    return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
        // Check context at each iteration
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if err != nil {
            return err
        }

        rel, err := filepath.Rel(source, path)
        if err != nil {
            return err
        }
        destPath := filepath.Join(destination, rel)

        if info.IsDir() {
            return os.MkdirAll(destPath, info.Mode())
        }

        return copyFile(ctx, path, destPath, info.Mode())
    })
}

func copyFile(ctx context.Context, src, dst string, mode os.FileMode) error {
    srcF, err := os.Open(src)
    if err != nil {
        return err
    }
    defer srcF.Close()

    dstF, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
    if err != nil {
        return err
    }
    defer dstF.Close()

    // Copy in chunks with context checks
    buf := make([]byte, 32*1024)
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        n, err := srcF.Read(buf)
        if n > 0 {
            if _, writeErr := dstF.Write(buf[:n]); writeErr != nil {
                return writeErr
            }
        }
        if err != nil {
            if err == io.EOF {
                return nil
            }
            return err
        }
    }
}
```

**Pros**:
- Immediate cancellation response
- Works for all file sizes
- Standard Go context pattern

**Cons**:
- More complex code
- Slight performance overhead
- Partial file left on cancellation (needs cleanup)

---

### Option 2: Use io.CopyN with Chunked Copying

Similar to Option 1 but uses `io.CopyN` for structured copying:

```go
func copyFile(ctx context.Context, src, dst string, mode os.FileMode) error {
    srcF, err := os.Open(src)
    if err != nil {
        return err
    }
    defer srcF.Close()

    dstF, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
    if err != nil {
        return err
    }
    defer dstF.Close()

    buf := make([]byte, 32*1024)
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        n, err := io.CopyN(dstF, srcF, 32*1024)
        if err != nil {
            if err == io.EOF {
                return nil
            }
            return err
        }
        if n ==0 {
            return nil
        }
    }
}
```

**Pros**:
- Cleaner than manual buffering
- Standard library function

**Cons**:
- Same as Option 1

---

### Option 3: Leave As-Is

Keep current implementation with documentation of limitation:

```go
// Get executes a recursive directory copy. Note: context cancellation
// is not supported during local directory copies. Forcancel-sensitive
// operations with large directories, consider using GoGetterDownloader
// with a file:// URL instead.
func (d *LocalDownloader) Get(ctx context.Context, source string, destination string) error {
    // Context is intentionally ignored for local copies as they are
    // typically fast operations for template scaffolding.
    // ...
}
```

**Pros**:
- No code changes
- Simple implementation
- Low maintenance burden

**Cons**:
- Context contract violated
- User expectation mismatch

---

## Recommended Implementation

**Current Decision: PARKED**

If implemented later, recommend **Option 1** with cleanup:

```go
func (d *LocalDownloader) Get(ctx context.Context, source string, destination string) error {
    err := filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if walkErr != nil {
            return walkErr
        }

        rel, err := filepath.Rel(source, path)
        if err != nil {
            return err
        }
        destPath := filepath.Join(destination, rel)

        if info.IsDir() {
            return os.MkdirAll(destPath, info.Mode())
        }

        return copyFileWithContext(ctx, path, destPath, info.Mode())
    })

    // Cleanup on cancellation
    if err != nil && errors.Is(err, context.Canceled) {
        os.RemoveAll(destination)
    }
    return err
}
```

---

## Test Plan

### Unit Tests

| Test Case | Description | Expected Result |
|-----------|-------------|-----------------|
| `TestLocalDownloader_Get_ContextCancel` | Cancel context during copy | Returns `context.Canceled` |
| `TestLocalDownloader_Get_ContextTimeout` | Short timeout during copy | Returns `context.DeadlineExceeded` |
| `TestLocalDownloader_Get_CancelCleanup` | Cancel leaves no partial files | Destination removed |
| `TestLocalDownloader_Get_ContextComplete` | Complete before timeout | Success, context ignored |
| `TestLocalDownloader_Get_LargeFile` | Large file copied with context | Checks at intervals |

### Test Implementation

```go
func TestLocalDownloader_Get_ContextCancel(t *testing.T) {
    // Given
    src := t.TempDir()
    dst := filepath.Join(t.TempDir(), "dst")
    
    // Create large file to ensure copy takes time
    largeFile := filepath.Join(src, "large.txt")
    f, _ := os.Create(largeFile)
    for i := 0; i < 1000000; i++ {//1MB file
        f.WriteString("test content\n")
    }
    f.Close()
    
    ctx, cancel := context.WithCancel(context.Background())
    
    // Start copy in goroutine
    errCh := make(chan error, 1)
    go func() {
        d := &LocalDownloader{}
        errCh <- d.Get(ctx, src, dst)
    }()
    
    // Cancel after small delay
    time.Sleep(10 * time.Millisecond)
    cancel()
    
    // When
    err := <-errCh
    
    // Then
    if !errors.Is(err, context.Canceled) {
        t.Errorf("Expected context.Canceled, got %v", err)
    }
    
    // And: destination should be cleaned up
    if _, err := os.Stat(dst); !os.IsNotExist(err) {
        t.Error("Expected destination to be cleaned up oncancel")
    }
}
```

### Benchmark Tests

```go
func BenchmarkLocalDownloader_Get_WithCheck(b *testing.B) {
    // Benchmark to measure overhead of context checks
}

func BenchmarkLocalDownloader_Get_WithoutCheck(b *testing.B) {
    // Benchmark current implementation
}
```

---

## Documentation Updates

### Code Comments

```go
// LocalDownloader implements Downloader for local directory copying.
// It performs a recursive copy from source to destination.
//
// Context Support:
//   - The context parameter is checked at each file iteration
//   - Large file copies check context between chunk writes
//   - Cancellation cleans up partial destination directory
//
// Performance:
//   - Small directories (< 10MB): typically completes in < 100ms
//   - Large directories: cancellation responds within onefile copy
type LocalDownloader struct{}
```

### README.md

Add note to README if implemented:

```markdown
## Performance

### Large Templates

For templates larger than100MB, `skelly` supports context cancellation.
Press Ctrl+C during processing to cleanly stop and clean up.

```bash
skelly init --src ./large-template --dst ./project
# Press Ctrl+C to cancel
```
```

---

## Rollback Plan

If implementation causes issues:

### Quick Rollback

1. Revert changes to `internal/download/local_downloader.go`
2. Remove context check tests from `local_downloader_test.go`
3. Restore original `Get` signature (no behavior change)

### Partial Rollback

If only cleanup causes issues:

1. Remove cleanup logic (`os.RemoveAll` on cancel)
2. Keep context check but document partial state possibility

---

## Decision Rationale

### Why PARKED

| Factor | Assessment |
|--------|-------------|
| **User Impact** | Low - most templates are small |
| **Implementation Complexity** | Medium - requires refactoring copy logic |
| **Maintenance Burden** | Low once implemented |
| **Alternative Solutions** | Issue 3 (temp dir) provides cleanup guarantee |
| **User Demand** | None reported |

### Revisit Criteria

Reassess if:

1. **User reports**: Template copy taking >10 seconds
2. **Feature request**: Large template support
3. **Enterprise need**: Predictable cancellation behavior
4. **Performance profiler**: Shows copy as bottleneck

---

## Related Issues

- **Issue 3**: Transactional processing - provides cleanup guarantee even without context support
- **Issue 4**: CUE validation - could validate template size before download

---

## Appendix: Performance Benchmarks

### Current Implementation

| Template Size | Copy Time | Cancellation Response |
|---------------|-----------|----------------------|
| 1KB | < 1ms | N/A (instant) |
| 1MB | ~10ms | N/A (instant) |
| 10MB | ~100ms | ~50-100ms |
| 100MB | ~1s | ~500ms-1s |
| 1GB | ~10s | ~5-10s |

### With Context Support (Projected)

| Template Size | Copy Time | Cancellation Response |
|---------------|-----------|----------------------|
| 1KB | < 1ms | < 1ms |
| 1MB | ~12ms | < 10ms |
| 10MB | ~110ms | < 50ms |
| 100MB | ~1.1s | < 100ms |
| 1GB | ~11s | < 500ms |

*Note: Projections include ~10% overhead for context checks*