# Phase 3: Cache Management

## Overview

Track cache usage in `state.db` and provide commands to view statistics and clean unused caches.

## Prerequisites

- Phase 2 completed

## Database Schema

**File**: `internal/mono/db.go`

```go
const cacheEventsSchema = `
CREATE TABLE IF NOT EXISTS cache_events (
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    event TEXT NOT NULL,
    project_id TEXT NOT NULL,
    artifact TEXT NOT NULL,
    cache_key TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cache_events_key ON cache_events(project_id, artifact, cache_key);
`

func (db *DB) RecordCacheEvent(event, projectID, artifact, cacheKey string) error {
	_, err := db.conn.Exec(
		`INSERT INTO cache_events (event, project_id, artifact, cache_key) VALUES (?, ?, ?, ?)`,
		event, projectID, artifact, cacheKey,
	)
	return err
}

type CacheUsage struct {
	ProjectID string
	Artifact  string
	CacheKey  string
	Hits      int
	LastUsed  time.Time
}

func (db *DB) GetCacheUsage() ([]CacheUsage, error) {
	rows, err := db.conn.Query(`
		SELECT project_id, artifact, cache_key,
		       COUNT(*) as hits,
		       MAX(timestamp) as last_used
		FROM cache_events
		WHERE event = 'hit'
		GROUP BY project_id, artifact, cache_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usage []CacheUsage
	for rows.Next() {
		var u CacheUsage
		if err := rows.Scan(&u.ProjectID, &u.Artifact, &u.CacheKey, &u.Hits, &u.LastUsed); err != nil {
			return nil, err
		}
		usage = append(usage, u)
	}
	return usage, rows.Err()
}
```

## Recording Events

**File**: `internal/mono/cache.go`

```go
func (cm *CacheManager) RestoreFromCache(db *DB, projectID string, artifact ArtifactConfig, envPath string) (bool, error) {
	cacheKey := cm.ComputeCacheKey(artifact, envPath)
	cachePath := filepath.Join(cm.LocalCacheDir, projectID, artifact.Name, cacheKey)

	if !dirExists(cachePath) {
		db.RecordCacheEvent("miss", projectID, artifact.Name, cacheKey)
		return false, nil
	}

	if err := cm.restoreArtifact(artifact, cachePath, envPath); err != nil {
		return false, err
	}

	db.RecordCacheEvent("hit", projectID, artifact.Name, cacheKey)
	return true, nil
}
```

## Commands

### `mono cache stats`

Shows cache entries with size, hit count, and last used time.

```
$ mono cache stats

Project           Artifact    Size      Hits   Last Used
────────────────────────────────────────────────────────────
a1b2c3d4e5f6      cargo       1.8 GB    47     2 hours ago
a1b2c3d4e5f6      npm         512 MB    12     3 days ago
f6e5d4c3b2a1      cargo       890 MB    3      2 weeks ago

Total: 3.1 GB
```

### `mono cache clean`

Interactive selection with usage context.

```
$ mono cache clean

Select caches to remove:

[ ] a1b2c3d4e5f6/cargo  1.8 GB   47 hits   2 hours ago
[ ] a1b2c3d4e5f6/npm    512 MB   12 hits   3 days ago
[x] f6e5d4c3b2a1/cargo  890 MB   3 hits    2 weeks ago

Space to toggle, Enter to confirm, Ctrl+C to cancel
```

Non-interactive mode for scripts:

```
$ mono cache clean --older-than 30d
Removed 2 entries (1.4 GB)

$ mono cache clean --all
Removed 3 entries (3.1 GB)
```

## Implementation

**File**: `internal/mono/cache.go`

```go
func (cm *CacheManager) GatherSizes() (map[string]int64, error) {
	sizes := make(map[string]int64)

	err := filepath.WalkDir(cm.LocalCacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(cm.LocalCacheDir, path)
		parts := strings.SplitN(rel, string(filepath.Separator), 3)
		if len(parts) >= 3 {
			key := parts[0] + "/" + parts[1]
			sizes[key] += info.Size()
		}
		return nil
	})

	return sizes, err
}

func (cm *CacheManager) RemoveEntry(projectID, artifact, cacheKey string) error {
	path := filepath.Join(cm.LocalCacheDir, projectID, artifact, cacheKey)
	return os.RemoveAll(path)
}

func (cm *CacheManager) CleanEmptyDirs() {
	filepath.WalkDir(cm.LocalCacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == cm.LocalCacheDir {
			return nil
		}
		entries, _ := os.ReadDir(path)
		if len(entries) == 0 {
			os.Remove(path)
		}
		return nil
	})
}

func (cm *CacheManager) RemoveOlderThan(db *DB, duration time.Duration) (int, int64, error) {
	usage, err := db.GetCacheUsage()
	if err != nil {
		return 0, 0, err
	}

	sizes, err := cm.GatherSizes()
	if err != nil {
		return 0, 0, err
	}

	cutoff := time.Now().Add(-duration)
	var count int
	var totalSize int64

	for _, u := range usage {
		if u.LastUsed.Before(cutoff) {
			key := u.ProjectID + "/" + u.Artifact
			if size, ok := sizes[key]; ok {
				totalSize += size
			}
			if err := cm.RemoveEntry(u.ProjectID, u.Artifact, u.CacheKey); err != nil {
				continue
			}
			count++
		}
	}

	cm.CleanEmptyDirs()
	return count, totalSize, nil
}

func (cm *CacheManager) RemoveAll() (int, int64, error) {
	sizes, err := cm.GatherSizes()
	if err != nil {
		return 0, 0, err
	}

	var count int
	var totalSize int64
	for key, size := range sizes {
		totalSize += size
		count++
	}

	if err := os.RemoveAll(cm.LocalCacheDir); err != nil {
		return 0, 0, err
	}

	return count, totalSize, nil
}
```

**File**: `internal/cli/cache.go`

```go
func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage build cache",
	}

	cmd.AddCommand(newCacheStatsCmd())
	cmd.AddCommand(newCacheCleanCmd())

	return cmd
}

func newCacheStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show cache usage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cm, err := mono.NewCacheManager()
			if err != nil {
				return err
			}

			db, err := mono.OpenDB()
			if err != nil {
				return err
			}
			defer db.Close()

			usage, err := db.GetCacheUsage()
			if err != nil {
				return err
			}

			sizes, err := cm.GatherSizes()
			if err != nil {
				return err
			}

			printStats(usage, sizes)
			return nil
		},
	}
}

func newCacheCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove cached artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cm, err := mono.NewCacheManager()
			if err != nil {
				return err
			}

			db, err := mono.OpenDB()
			if err != nil {
				return err
			}
			defer db.Close()

			olderThan, _ := cmd.Flags().GetString("older-than")
			all, _ := cmd.Flags().GetBool("all")

			if all {
				count, size, err := cm.RemoveAll()
				if err != nil {
					return err
				}
				fmt.Printf("Removed %d entries (%s)\n", count, humanSize(size))
				return nil
			}

			if olderThan != "" {
				duration, err := parseDuration(olderThan)
				if err != nil {
					return err
				}
				count, size, err := cm.RemoveOlderThan(db, duration)
				if err != nil {
					return err
				}
				fmt.Printf("Removed %d entries (%s)\n", count, humanSize(size))
				return nil
			}

			usage, err := db.GetCacheUsage()
			if err != nil {
				return err
			}

			sizes, err := cm.GatherSizes()
			if err != nil {
				return err
			}

			entries := buildEntryList(usage, sizes)
			selected := promptSelection(entries)

			for _, entry := range selected {
				if err := cm.RemoveEntry(entry.ProjectID, entry.Artifact, entry.CacheKey); err != nil {
					fmt.Fprintf(os.Stderr, "failed to remove %s/%s: %v\n", entry.ProjectID, entry.Artifact, err)
				}
			}

			cm.CleanEmptyDirs()
			return nil
		},
	}

	cmd.Flags().String("older-than", "", "Remove entries older than duration (e.g., 30d, 2w)")
	cmd.Flags().Bool("all", false, "Remove all cached entries")

	return cmd
}

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
```

## Acceptance Criteria

- [ ] Cache hits/misses recorded to `state.db`
- [ ] `mono cache stats` shows size, hits, and last used
- [ ] `mono cache clean` provides interactive selection
- [ ] `mono cache clean --older-than` removes old entries
- [ ] `mono cache clean --all` clears everything
- [ ] Empty directories cleaned up after removal
