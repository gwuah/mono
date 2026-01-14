# Doctor Command

Diagnostic command that analyzes project setup and suggests optimizations.

## Usage

```
$ mono doctor

✓ sccache installed
✗ sccache server not running
  └─ run: sccache --start-server
✗ Cargo artifact caching not configured
  └─ add to mono.yml: build.artifacts
✓ npm artifact caching configured
✓ Cache size healthy (2.3 GB)
```

## Checks

| Check | Pass | Fail |
|-------|------|------|
| sccache installed | in PATH or ~/.cargo/bin | not found |
| sccache server running | `sccache --show-stats` succeeds | server not running |
| Cargo artifact caching | Cargo.lock exists + artifacts configured | Cargo.lock exists, no config |
| npm artifact caching | package-lock.json exists + artifacts configured | lockfile exists, no config |
| yarn artifact caching | yarn.lock exists + artifacts configured | lockfile exists, no config |
| pnpm artifact caching | pnpm-lock.yaml exists + artifacts configured | lockfile exists, no config |
| Cache size | under configured max_size | exceeds limit |

## Implementation

**File**: `internal/cli/doctor.go`

```go
package cli

type Check struct {
	Name    string
	Passed  bool
	Hint    string
}

func runDoctorChecks(cfg mono.Config, projectRoot string) []Check {
	var checks []Check

	checks = append(checks, checkSccacheInstalled()...)
	checks = append(checks, checkArtifactConfigs(cfg, projectRoot)...)
	checks = append(checks, checkCacheHealth()...)

	return checks
}

func checkSccacheInstalled() []Check {
	var checks []Check

	installed := false
	if _, err := exec.LookPath("sccache"); err == nil {
		installed = true
	} else {
		home, _ := os.UserHomeDir()
		cargoBin := filepath.Join(home, ".cargo", "bin", "sccache")
		if _, err := os.Stat(cargoBin); err == nil {
			installed = true
		}
	}

	checks = append(checks, Check{
		Name:   "sccache installed",
		Passed: installed,
		Hint:   "run: cargo install sccache",
	})

	if installed {
		cmd := exec.Command("sccache", "--show-stats")
		serverRunning := cmd.Run() == nil
		checks = append(checks, Check{
			Name:   "sccache server running",
			Passed: serverRunning,
			Hint:   "run: sccache --start-server",
		})
	}

	return checks
}

func checkArtifactConfigs(cfg mono.Config, projectRoot string) []Check {
	var checks []Check

	lockfiles := map[string]string{
		"Cargo.lock":        "cargo",
		"package-lock.json": "npm",
		"yarn.lock":         "yarn",
		"pnpm-lock.yaml":    "pnpm",
	}

	configuredArtifacts := make(map[string]bool)
	for _, a := range cfg.Build.Artifacts {
		configuredArtifacts[a.Name] = true
	}

	for lockfile, name := range lockfiles {
		path := filepath.Join(projectRoot, lockfile)
		if _, err := os.Stat(path); err == nil {
			configured := configuredArtifacts[name]
			checks = append(checks, Check{
				Name:   name + " artifact caching",
				Passed: configured,
				Hint:   "add to mono.yml: build.artifacts",
			})
		}
	}

	return checks
}

func printChecks(checks []Check) {
	for _, c := range checks {
		if c.Passed {
			fmt.Printf("✓ %s\n", c.Name)
		} else {
			fmt.Printf("✗ %s\n", c.Name)
			if c.Hint != "" {
				fmt.Printf("  └─ %s\n", c.Hint)
			}
		}
	}
}
```

## Prerequisites

- Phase 2 (artifact caching config structure)
