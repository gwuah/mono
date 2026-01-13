# Build Cache for Mono Environments

## Problem Statement

When creating multiple mono environments (via git worktrees), each environment currently requires a full copy of build artifacts (`target/` for Rust, `node_modules/` for Node.js). This results in:

1. **Slow environment creation**: Copying multi-gigabyte directories takes minutes
2. **Wasted disk space**: Identical artifacts duplicated across environments
3. **Redundant builds**: Running `cargo build` / `npm install` for unchanged dependencies

Most environments share the same dependencies (same `Cargo.lock` / `package-lock.json`), making this duplication unnecessary.

## Goals

- Reduce environment creation time from minutes to seconds for cache hits
- Share identical build artifacts across environments to save disk space
- Support environments with different dependency versions (isolation when needed)
- Work out-of-the-box with zero configuration (sensible defaults)
- Allow power users to customize caching behavior

## Non-Goals

- Cross-project cache sharing (cache is per-project)
- Remote/distributed caching
- Caching source code (handled by git worktrees)

## Design Overview: Layered Caching

The caching system operates in three complementary layers. Each layer addresses a different part of the build process, and they work together for maximum benefit.

```
┌─────────────────────────────────────────────────────────────────┐
│  Layer 3: Artifact Cache (Hardlinks)                            │
│  - Caches entire target/ and node_modules/ directories          │
│  - Instant environment creation on cache hit                    │
│  - Keyed by lockfile hash                                       │
├─────────────────────────────────────────────────────────────────┤
│  Layer 2: Compilation Cache (sccache)                           │
│  - Caches individual compiled units (.rlib, .o files)           │
│  - Works even when lockfiles differ (shared deps hit cache)     │
│  - Content-addressed by source + flags                          │
├─────────────────────────────────────────────────────────────────┤
│  Layer 1: Download Cache (Shared registries)                    │
│  - Shared CARGO_HOME, npm cache across all environments         │
│  - Downloads happen once, never repeated                        │
│  - Always enabled, zero config                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Why three layers?**

| Scenario                             | Layer 1       | Layer 2           | Layer 3                |
| ------------------------------------ | ------------- | ----------------- | ---------------------- |
| Same lockfile, same source           | Skip download | Skip compile      | Hardlink (instant)     |
| Same lockfile, different source      | Skip download | Skip dep compile  | Hardlink + incremental |
| Different lockfile, some shared deps | Skip download | Partial cache hit | Cache miss, rebuild    |
| Completely different deps            | Skip download | Cache miss        | Cache miss             |

Layer 3 (hardlinks) is the fastest but requires identical lockfiles. Layer 2 (sccache) helps when lockfiles differ but share common dependencies. Layer 1 (downloads) always helps.

## Directory Structure

```
project/
├── .git/                         # shared by all worktrees
├── .mono/
│   ├── cargo/                    # Layer 1: shared CARGO_HOME
│   │   ├── registry/             # downloaded crates
│   │   └── git/                  # git dependencies
│   ├── npm/                      # Layer 1: shared npm cache
│   ├── sccache/                  # Layer 2: compiled unit cache
│   └── cache/                    # Layer 3: artifact cache
│       ├── cargo/
│       │   ├── <hash>/
│       │   │   └── target/       # cached build artifacts
│       │   └── <hash>/
│       │       └── target/       # different deps version
│       └── npm/
│           └── <hash>/
│               └── node_modules/
├── main/                         # primary worktree
│   └── target/                   # hardlinked from cache
└── environments/
    ├── env_1/                    # worktree
    │   └── target/               # hardlinked from cache (same hash)
    └── env_2/                    # worktree on different branch
        └── target/               # hardlinked from different cache entry
```

## Layer 1: Shared Download Cache

Mono automatically injects environment variables before running any script:

```bash
export CARGO_HOME="$MONO_ROOT/.mono/cargo"
export npm_config_cache="$MONO_ROOT/.mono/npm"
export YARN_CACHE_FOLDER="$MONO_ROOT/.mono/yarn"
export PNPM_HOME="$MONO_ROOT/.mono/pnpm"
```

**Effect**: Downloads happen once. Subsequent environments skip network entirely.

**Speed gain**: Moderate (saves download time, not compile time)

**Concurrency**: Safe. Download caches are read-heavy, writes are atomic.

**Configuration**: Always enabled. No user action required.

## Layer 2: Compilation Cache (sccache)

Mono wraps Rust compilation with sccache:

```bash
export RUSTC_WRAPPER="sccache"
export SCCACHE_DIR="$MONO_ROOT/.mono/sccache"
```

**How sccache works**:

1. Intercepts rustc invocations
2. Computes hash of: source files + compiler flags + dependency artifacts
3. On cache hit: returns cached `.rlib`/`.o` file immediately
4. On cache miss: runs rustc, stores result in cache

```
Env 1: cargo build
  └─ rustc serde@1.0.193 → hash: abc123 → MISS → compile → cache
  └─ rustc my_crate → hash: def456 → MISS → compile → cache

Env 2: cargo build (different worktree, same Cargo.lock)
  └─ rustc serde@1.0.193 → hash: abc123 → HIT → instant
  └─ rustc my_crate → hash: xyz789 → MISS → compile (different source)
```

**Effect**: Dependency compilation is shared without sharing directories. Even if two environments have slightly different lockfiles, any common dependencies get cache hits.

**Speed gain**: High for dependencies (typically 80%+ of compile time)

**Concurrency**: Safe. sccache handles its own locking internally.

**Configuration**: Enabled by default if sccache is installed. Can be disabled:

```yaml
build:
  sccache: false
```

## Layer 3: Artifact Cache (Hardlinks)

The most aggressive optimization. Caches entire `target/` and `node_modules/` directories, keyed by lockfile hash.

### Cache Key Computation

The cache key determines when artifacts can be reused. It includes all factors that affect the build output.

**Cargo cache key:**

```
sha256(
  contents(Cargo.lock) +
  rustc_version +
  build_profile (debug|release)
)
```

**npm cache key:**

```
sha256(
  contents(package-lock.json) +
  node_version
)
```

Source files are intentionally excluded. Cargo/npm handle source-level incremental compilation internally. The cache key only captures "are the dependencies the same?"

### Hardlink Mechanics

Environments with identical dependencies share the same cached artifacts via hardlinks.

```
function hardlink_tree(source_dir, target_dir):
    for each file in source_dir (recursive):
        if is_directory(file):
            mkdir(target_dir/relative_path)
        else:
            hardlink(source_dir/file, target_dir/relative_path)
```

Hardlinks share the same inode. When a program modifies a hardlinked file:

1. The filesystem breaks the link (only for that file)
2. A new copy is created with the modifications
3. Other hardlinks remain unchanged

This gives us copy-on-write behavior without explicit COW filesystem support.

**Concurrency**: Safe. Each environment gets its own hardlinked tree. When cargo/npm modifies a file, only that environment's copy changes. Other environments are unaffected.

### Cache Storage Flow

**On cache miss:**

```
1. Run build script (cargo build, npm install)
2. Compute cache key from lockfiles
3. Move artifacts to cache:
   mv env/target .mono/cache/cargo/<hash>/target
4. Hardlink back to environment:
   hardlink_tree(.mono/cache/cargo/<hash>/target, env/target)
```

**On cache hit:**

```
1. Compute cache key
2. Find matching cache entry
3. Hardlink to environment:
   hardlink_tree(.mono/cache/cargo/<hash>/target, env/target)
4. Run build script (handles source-level changes incrementally)
```

## Configuration

### Zero-Config Default (Recommended)

Mono works out-of-the-box with no cache configuration:

```yaml
scripts:
  init: |
    cargo build
    cd web && npm install

  run: |
    cargo run --bin myapp &
```

Mono automatically:

- Injects shared download cache env vars (Layer 1)
- Enables sccache if installed (Layer 2)
- Detects `Cargo.lock` / `package-lock.json` and enables artifact caching (Layer 3)

### Strategy Shorthand

For simple cases, use a single strategy option:

```yaml
build:
  strategy: layered # default: all three layers enabled
  # strategy: compile  # layers 1+2 only, no hardlinks
  # strategy: download # layer 1 only
  # strategy: none     # no caching (fully isolated)
```

### Power User Configuration

For fine-grained control:

```yaml
build:
  download_cache: true # Layer 1 (default: true)
  sccache: true # Layer 2 (default: true if installed)

  artifacts: # Layer 3
    - name: cargo
      key_files: [Cargo.lock]
      key_commands: ["rustc --version"]
      paths: [target]

    - name: npm
      key_files: [web/package-lock.json]
      key_commands: ["node --version"]
      paths: [web/node_modules]

scripts:
  build: |
    cargo build
    cd web && npm install
```

## Lifecycle Phases

```
┌─────────────────────────────────────────────────────────────────┐
│                    Environment Creation                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. git worktree add                                            │
│          │                                                       │
│          ▼                                                       │
│  2. inject cache env vars (CARGO_HOME, RUSTC_WRAPPER, etc.)     │
│          │                                                       │
│          ▼                                                       │
│  3. compute artifact cache keys                                 │
│          │                                                       │
│          ▼                                                       │
│  4. for each artifact cache:                                    │
│          │                                                       │
│          ├─── cache hit ──► hardlink from cache to env          │
│          │                                                       │
│          └─── cache miss ─► (deferred to build step)            │
│                                                                  │
│          ▼                                                       │
│  5. run build script                                            │
│          │                                                       │
│          ▼                                                       │
│  6. on cache miss: store artifacts in cache, re-hardlink        │
│          │                                                       │
│          ▼                                                       │
│  7. run setup script                                            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Concurrency Safety

All caching layers are designed for concurrent access:

| Layer          | Mechanism                 | Concurrent Safety |
| -------------- | ------------------------- | ----------------- |
| Download cache | Atomic writes, read-heavy | Safe              |
| sccache        | Internal locking          | Safe              |
| Hardlinks      | COW semantics per-file    | Safe              |

Multiple environments can run `cargo build` simultaneously without conflicts:

- They share downloaded crates (Layer 1)
- They share compiled dependency units (Layer 2)
- They have isolated working copies via hardlinks (Layer 3)

**Recommendation**: Default to npm + hardlinks for compatibility. Document pnpm as the recommended approach for heavy Node.js users.

## Handling Build Profiles (Debug vs Release)

Cargo stores debug and release builds in separate subdirectories (`target/debug`, `target/release`). Two options:

**Option A: Single cache entry, both profiles**

- Cache the entire `target/` directory
- Simpler, but wastes space if only using one profile
  Recommendation: **Option A** for simplicity. Disk space is cheaper than complexity.

## Cache Management

### Automatic Invalidation

- Cache entries are keyed by content hash
- Changing `Cargo.lock` produces a new hash → new cache entry
- Old entries remain (may be used by other environments)

### Manual Commands

```bash
mono cache status            # show cache size and entries
mono cache clean             # remove all cache entries
```

## Environment Variables

Available in scripts:

| Variable               | Description                                          |
| ---------------------- | ---------------------------------------------------- |
| `MONO_CACHE_HIT`       | `true` if all artifact caches hit, `false` otherwise |
| `MONO_CACHE_DIR`       | Path to `.mono/cache/`                               |
| `MONO_CARGO_CACHE_KEY` | Current cargo artifact cache key                     |
| `MONO_NPM_CACHE_KEY`   | Current npm artifact cache key                       |
| `CARGO_HOME`           | Auto-injected shared cargo home                      |
| `SCCACHE_DIR`          | Auto-injected sccache directory                      |

## Edge Cases

**1. Corrupted cache entry**

If a cache entry is corrupted (partial write, disk error):

```
mono cache rebuild <hash>     # rebuild specific entry
mono cache verify             # check all entries for corruption
```

**2. Lockfile changes mid-session**

If a user modifies `Cargo.lock` in an environment:

- Current environment continues using its hardlinked artifacts
- Next `mono sync` or `mono build` recomputes the hash
- If new hash exists in cache: re-hardlink
- If not: build and cache

**3. Multiple environments modifying same cached files**

Not possible. Hardlinks provide isolation:

- Env A and Env B both link to cached file X
- Env A's cargo build modifies X
- Filesystem creates a new file for Env A, breaks the hardlink
- Env B still sees original X

**4. Filesystem doesn't support hardlinks**

Fall back to regular copy. Log a warning:

```
warning: hardlinks not supported on this filesystem, falling back to copy
```

**5. Cache directory on different filesystem**

Hardlinks don't work across filesystems. Options:

- Error with helpful message
- Fall back to copy
- Recommend moving project to same filesystem as home directory

**6. sccache not installed**

Layer 2 is skipped. Layers 1 and 3 still provide significant benefits. Log info:

```
info: sccache not found, compilation caching disabled
hint: install sccache for faster builds: cargo install sccache
```

## Performance Characteristics

| Operation      | No Caching | Layer 1 Only | Layers 1+2 | All Layers (hit) |
| -------------- | ---------- | ------------ | ---------- | ---------------- |
| Download deps  | Full       | Skip         | Skip       | Skip             |
| Compile deps   | Full       | Full         | Skip       | Skip             |
| Link artifacts | N/A        | N/A          | N/A        | Instant          |
| Compile source | Full       | Full         | Full       | Incremental      |

Typical improvements:

- Environment creation: **minutes → seconds** (on full cache hit)
- Disk usage: **~80% reduction** (for similar environments)
- Build time: **60-90% reduction** (via sccache, even on artifact cache miss)

## Implementation Phases

### Phase 1: Shared Caches (Layer 1 + 2)

Low effort, good gains.

- Auto-inject `CARGO_HOME`, `npm_config_cache` env vars
- Detect sccache installation, set `RUSTC_WRAPPER` if available
- Update `.mono/` gitignore patterns

### Phase 2: Artifact Cache (Layer 3)

Medium effort, great gains.

- Implement cache key computation
- Implement hardlink_tree function
- Basic cache storage/retrieval
- Integration with `mono env create`
- Auto-detect Cargo.lock / package-lock.json

### Phase 3: Cache Management

- `mono cache` CLI commands
- Cache size reporting
- Manual cleanup commands

### Phase 4: Polish

- Detect lockfile identity across envs for stats
- Cache hit/miss reporting: "Saved 3m42s via shared cache"
- Optional cache size limits with LRU eviction

## Open Questions

1. **Should `build` script always run after cache restore?**

   - Yes: Ensures source changes are compiled
   - No: Faster, but user must run manually
   - Recommendation: Yes, but make it skippable with `--no-build`

2. **How to handle environments that intentionally diverge?**

   - User modifies deps, wants isolation
   - Current design handles this: new lockfile → new cache key
   - Consider: `mono env detach` to explicitly break cache link

3. **Cache size limits?**

   - No limit by default
   - Optional config: `cache.max_size: 10GB`
   - LRU eviction when limit reached

4. **sccache dependency management?**
   - Option A: Expect users to install sccache themselves
   - Option B: Bundle sccache with mono
   - Option C: Offer to install on first run
   - Recommendation: Option A with helpful messaging

## Alternatives Considered

### Shared CARGO_TARGET_DIR

All worktrees share a single `target/` directory via `CARGO_TARGET_DIR` environment variable.

**Rejected because:**

- Cargo's incremental compilation thrashes when switching between different source states
- No isolation between environments
- Confusing behavior when running multiple environments simultaneously

### Copy-on-Write filesystem (APFS clones)

Use `cp -c` on macOS for instant COW copies.

**Rejected because:**

- Platform-specific (macOS only)
- Requires same filesystem
- Hardlinks achieve similar benefits more portably
- Could be added as optional optimization for macOS users later

### Symlinks instead of hardlinks

Symlink entire `target/` directory to cache.

**Rejected because:**

- All environments would share the exact same directory
- No isolation: one environment's build affects others
- Can't support environments with different dependency versions
