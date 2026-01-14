# Future: Cargo Cache Improvements

These are potential edge cases that could cause cache misses or rebuilds with the current Cargo artifact caching. Documented for future reference if issues arise in practice.

## Current Implementation

We clean `.fingerprint/` directories after restore to handle absolute path mismatches. This covers the most common case.

## Potential Additional Cleanups

### 1. Incremental Compilation Directory

**Location**: `target/{debug,release}/incremental/`

**Issue**: Contains session-specific incremental compilation data that may not be portable across paths.

**Fix if needed**:
```go
incrementalDirs := []string{
    filepath.Join(targetDir, "debug", "incremental"),
    filepath.Join(targetDir, "release", "incremental"),
}
for _, dir := range incrementalDirs {
    os.RemoveAll(dir)
}
```

**Trade-off**: Slightly longer rebuilds as Cargo regenerates incremental data.

### 2. Dependency Files (.d)

**Location**: `target/{debug,release}/deps/*.d`, `target/{debug,release}/*.d`

**Issue**: Makefile-format dependency files containing absolute paths:
```
/Users/foo/project/target/debug/myapp: /Users/foo/project/src/main.rs
```

**Fix if needed**:
```go
filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
    if strings.HasSuffix(path, ".d") {
        os.Remove(path)
    }
    return nil
})
```

**Trade-off**: Cargo regenerates these quickly.

### 3. Build Script Outputs

**Location**: `target/{debug,release}/build/*/output`

**Issue**: Build scripts (`build.rs`) may write path-dependent data.

**Mitigation**: No good fix - build scripts can do anything. If a specific crate causes issues, users can exclude it from caching or rebuild.

## Potential Cache Key Additions

### 4. RUSTFLAGS

**Issue**: Environment variable affects compilation but isn't in cache key.

**Example**: Main built with `RUSTFLAGS="-C target-cpu=native"`, new env without it.

**Fix if needed**: Add to key computation:
```go
rustflags := os.Getenv("RUSTFLAGS")
h.Write([]byte(rustflags))
```

**Trade-off**: More cache entries, less sharing.

### 5. Feature Flags

**Issue**: `cargo build --features foo` produces different artifacts than `cargo build`. Only Cargo.lock is in cache key.

**Mitigation**: Hard to solve generically. Users building with different features will get incremental rebuilds (Cargo handles this correctly, just slower).

## When to Revisit

Monitor for these symptoms:
- Users reporting full rebuilds after cache restore
- Build times not improving despite cache hits
- Compilation errors after restore (ABI mismatches)

If any become common, implement the corresponding fix.
