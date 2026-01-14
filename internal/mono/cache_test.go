package mono

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestComputeProjectID(t *testing.T) {
	id1 := ComputeProjectID("/Users/x/project1")
	id2 := ComputeProjectID("/Users/x/project1")
	id3 := ComputeProjectID("/Users/x/project2")

	if id1 != id2 {
		t.Errorf("same path should produce same ID: got %s and %s", id1, id2)
	}

	if id1 == id3 {
		t.Errorf("different paths should produce different IDs: both got %s", id1)
	}

	if len(id1) != 12 {
		t.Errorf("project ID should be 12 chars, got %d: %s", len(id1), id1)
	}
}

func TestComputeCacheKey(t *testing.T) {
	cm, err := NewCacheManager()
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	testDir := t.TempDir()
	lockfile := filepath.Join(testDir, "Cargo.lock")
	if err := os.WriteFile(lockfile, []byte("test lockfile content"), 0644); err != nil {
		t.Fatalf("failed to write lockfile: %v", err)
	}

	artifact := ArtifactConfig{
		Name:        "cargo",
		KeyFiles:    []string{"Cargo.lock"},
		KeyCommands: []string{"echo v1.0"},
		Paths:       []string{"target"},
	}

	key1, err := cm.ComputeCacheKey(artifact, testDir)
	if err != nil {
		t.Fatalf("failed to compute cache key: %v", err)
	}

	key2, err := cm.ComputeCacheKey(artifact, testDir)
	if err != nil {
		t.Fatalf("failed to compute cache key: %v", err)
	}

	if key1 != key2 {
		t.Errorf("same inputs should produce same key: got %s and %s", key1, key2)
	}

	if len(key1) != 16 {
		t.Errorf("cache key should be 16 chars, got %d: %s", len(key1), key1)
	}

	if err := os.WriteFile(lockfile, []byte("different content"), 0644); err != nil {
		t.Fatalf("failed to write lockfile: %v", err)
	}

	key3, err := cm.ComputeCacheKey(artifact, testDir)
	if err != nil {
		t.Fatalf("failed to compute cache key: %v", err)
	}

	if key1 == key3 {
		t.Errorf("different lockfile should produce different key: both got %s", key1)
	}
}

func TestComputeCacheKeyMissingKeyFile(t *testing.T) {
	cm, err := NewCacheManager()
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	testDir := t.TempDir()

	artifact := ArtifactConfig{
		Name:        "cargo",
		KeyFiles:    []string{"Cargo.lock"},
		KeyCommands: []string{"echo v1.0"},
		Paths:       []string{"target"},
	}

	key, err := cm.ComputeCacheKey(artifact, testDir)
	if err != nil {
		t.Fatalf("missing key file should not error: %v", err)
	}

	if key == "" {
		t.Error("should still produce a key from key commands")
	}
}

func TestHardlinkTree(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(src, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatalf("failed to write nested file: %v", err)
	}

	if err := HardlinkTree(src, dst); err != nil {
		t.Fatalf("HardlinkTree failed: %v", err)
	}

	srcInfo, err := os.Stat(filepath.Join(src, "file.txt"))
	if err != nil {
		t.Fatalf("failed to stat src file: %v", err)
	}
	dstInfo, err := os.Stat(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatalf("failed to stat dst file: %v", err)
	}

	srcSys := srcInfo.Sys().(*syscall.Stat_t)
	dstSys := dstInfo.Sys().(*syscall.Stat_t)

	if srcSys.Ino != dstSys.Ino {
		t.Errorf("files should share inode (hardlink): src=%d, dst=%d", srcSys.Ino, dstSys.Ino)
	}

	nestedDst := filepath.Join(dst, "subdir", "nested.txt")
	if _, err := os.Stat(nestedDst); err != nil {
		t.Errorf("nested file should exist at %s: %v", nestedDst, err)
	}
}

func TestHardlinkTreeReplaceBreaksLink(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	srcFile := filepath.Join(src, "file.txt")
	if err := os.WriteFile(srcFile, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if err := HardlinkTree(src, dst); err != nil {
		t.Fatalf("HardlinkTree failed: %v", err)
	}

	dstFile := filepath.Join(dst, "file.txt")

	srcInfoBefore, _ := os.Stat(srcFile)
	dstInfoBefore, _ := os.Stat(dstFile)
	srcInodeBefore := srcInfoBefore.Sys().(*syscall.Stat_t).Ino
	dstInodeBefore := dstInfoBefore.Sys().(*syscall.Stat_t).Ino

	if srcInodeBefore != dstInodeBefore {
		t.Fatalf("inodes should match before modification")
	}

	if err := os.Remove(dstFile); err != nil {
		t.Fatalf("failed to remove dst file: %v", err)
	}
	if err := os.WriteFile(dstFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to write new dst file: %v", err)
	}

	srcInfoAfter, _ := os.Stat(srcFile)
	dstInfoAfter, _ := os.Stat(dstFile)
	srcInodeAfter := srcInfoAfter.Sys().(*syscall.Stat_t).Ino
	dstInodeAfter := dstInfoAfter.Sys().(*syscall.Stat_t).Ino

	if srcInodeAfter != srcInodeBefore {
		t.Error("src inode should be unchanged")
	}

	if dstInodeAfter == srcInodeAfter {
		t.Error("after replace, dst should have different inode")
	}

	srcContent, _ := os.ReadFile(srcFile)
	if string(srcContent) != "original" {
		t.Errorf("src file should be unchanged, got: %s", srcContent)
	}

	dstContent, _ := os.ReadFile(dstFile)
	if string(dstContent) != "modified" {
		t.Errorf("dst file should be modified, got: %s", dstContent)
	}
}

func TestStoreAndRestoreCache(t *testing.T) {
	cm, err := NewCacheManager()
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	testDir := t.TempDir()
	envPath := filepath.Join(testDir, "env")
	targetDir := filepath.Join(envPath, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}

	testFile := filepath.Join(targetDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("cached content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cacheDir := filepath.Join(testDir, "cache")
	entry := ArtifactCacheEntry{
		Name:      "cargo",
		Key:       "testkey123",
		CachePath: filepath.Join(cacheDir, "cargo", "testkey123"),
		EnvPaths:  []string{targetDir},
		Hit:       false,
	}

	if err := cm.StoreToCache(entry); err != nil {
		t.Fatalf("StoreToCache failed: %v", err)
	}

	cachedFile := filepath.Join(entry.CachePath, "target", "test.txt")
	if _, err := os.Stat(cachedFile); err != nil {
		t.Errorf("cached file should exist: %v", err)
	}

	if _, err := os.Stat(testFile); err != nil {
		t.Errorf("env file should still exist (hardlinked back): %v", err)
	}

	if err := os.RemoveAll(targetDir); err != nil {
		t.Fatalf("failed to remove target dir: %v", err)
	}

	entry.Hit = true
	if err := cm.RestoreFromCache(entry); err != nil {
		t.Fatalf("RestoreFromCache failed: %v", err)
	}

	restoredContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}

	if string(restoredContent) != "cached content" {
		t.Errorf("restored content mismatch: got %s", restoredContent)
	}
}

func TestDetectArtifacts(t *testing.T) {
	testDir := t.TempDir()

	artifacts := detectArtifacts(testDir)
	if len(artifacts) != 0 {
		t.Errorf("should detect no artifacts in empty dir, got %d", len(artifacts))
	}

	if err := os.WriteFile(filepath.Join(testDir, "Cargo.lock"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write Cargo.lock: %v", err)
	}

	artifacts = detectArtifacts(testDir)
	if len(artifacts) != 1 {
		t.Errorf("should detect 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].Name != "cargo" {
		t.Errorf("should detect cargo, got %s", artifacts[0].Name)
	}

	if err := os.WriteFile(filepath.Join(testDir, "package-lock.json"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write package-lock.json: %v", err)
	}

	artifacts = detectArtifacts(testDir)
	if len(artifacts) != 2 {
		t.Errorf("should detect 2 artifacts, got %d", len(artifacts))
	}
}

func TestCleanCargoFingerprints(t *testing.T) {
	cm, err := NewCacheManager()
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	targetDir := t.TempDir()
	debugFp := filepath.Join(targetDir, "debug", ".fingerprint")
	releaseFp := filepath.Join(targetDir, "release", ".fingerprint")

	if err := os.MkdirAll(debugFp, 0755); err != nil {
		t.Fatalf("failed to create debug fingerprint dir: %v", err)
	}
	if err := os.MkdirAll(releaseFp, 0755); err != nil {
		t.Fatalf("failed to create release fingerprint dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(debugFp, "test"), []byte("fp"), 0644); err != nil {
		t.Fatalf("failed to write fingerprint file: %v", err)
	}

	if err := cm.cleanCargoFingerprints(targetDir); err != nil {
		t.Fatalf("cleanCargoFingerprints failed: %v", err)
	}

	if _, err := os.Stat(debugFp); !os.IsNotExist(err) {
		t.Error("debug .fingerprint should be removed")
	}
	if _, err := os.Stat(releaseFp); !os.IsNotExist(err) {
		t.Error("release .fingerprint should be removed")
	}
}

func TestCleanNodeModulesBin(t *testing.T) {
	cm, err := NewCacheManager()
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	nodeModules := t.TempDir()
	binDir := filepath.Join(nodeModules, ".bin")

	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create .bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "cli"), []byte("#!/bin/bash"), 0755); err != nil {
		t.Fatalf("failed to write cli file: %v", err)
	}

	if err := cm.cleanNodeModulesBin(nodeModules); err != nil {
		t.Fatalf("cleanNodeModulesBin failed: %v", err)
	}

	if _, err := os.Stat(binDir); !os.IsNotExist(err) {
		t.Error(".bin directory should be removed")
	}
}

func TestPrepareArtifactCache(t *testing.T) {
	cm, err := NewCacheManager()
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	testDir := t.TempDir()
	envPath := filepath.Join(testDir, "env")
	if err := os.MkdirAll(envPath, 0755); err != nil {
		t.Fatalf("failed to create env dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(envPath, "Cargo.lock"), []byte("lockfile"), 0644); err != nil {
		t.Fatalf("failed to write Cargo.lock: %v", err)
	}

	artifacts := []ArtifactConfig{
		{
			Name:        "cargo",
			KeyFiles:    []string{"Cargo.lock"},
			KeyCommands: []string{"echo v1"},
			Paths:       []string{"target"},
		},
	}

	entries, err := cm.PrepareArtifactCache(artifacts, testDir, envPath)
	if err != nil {
		t.Fatalf("PrepareArtifactCache failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Name != "cargo" {
		t.Errorf("expected name 'cargo', got %s", entry.Name)
	}
	if entry.Key == "" {
		t.Error("key should not be empty")
	}
	if entry.Hit {
		t.Error("should be cache miss (cache doesn't exist)")
	}
	if len(entry.EnvPaths) != 1 {
		t.Errorf("expected 1 env path, got %d", len(entry.EnvPaths))
	}
}

func TestIntegrationCacheHitMiss(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not installed, skipping integration test")
	}

	cm, err := NewCacheManager()
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	testDir := t.TempDir()

	cmd := exec.Command("cargo", "new", "testproj", "--quiet")
	cmd.Dir = testDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create cargo project: %v", err)
	}

	envPath := filepath.Join(testDir, "testproj")

	cmd = exec.Command("cargo", "generate-lockfile", "--quiet")
	cmd.Dir = envPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to generate lockfile: %v", err)
	}

	cmd = exec.Command("cargo", "build", "--quiet")
	cmd.Dir = envPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build project: %v", err)
	}

	artifacts := detectArtifacts(envPath)
	if len(artifacts) != 1 || artifacts[0].Name != "cargo" {
		t.Fatalf("expected cargo artifact, got %v", artifacts)
	}

	entries, err := cm.PrepareArtifactCache(artifacts, testDir, envPath)
	if err != nil {
		t.Fatalf("PrepareArtifactCache failed: %v", err)
	}

	if entries[0].Hit {
		t.Error("first run should be cache miss")
	}

	if err := cm.StoreToCache(entries[0]); err != nil {
		t.Fatalf("StoreToCache failed: %v", err)
	}

	entries2, err := cm.PrepareArtifactCache(artifacts, testDir, envPath)
	if err != nil {
		t.Fatalf("PrepareArtifactCache failed: %v", err)
	}

	if !entries2[0].Hit {
		t.Error("second run should be cache hit")
	}

	if entries[0].Key != entries2[0].Key {
		t.Errorf("cache keys should match: %s vs %s", entries[0].Key, entries2[0].Key)
	}

	targetDir := filepath.Join(envPath, "target")
	if err := os.RemoveAll(targetDir); err != nil {
		t.Fatalf("failed to remove target: %v", err)
	}

	if err := cm.RestoreFromCache(entries2[0]); err != nil {
		t.Fatalf("RestoreFromCache failed: %v", err)
	}

	if _, err := os.Stat(targetDir); err != nil {
		t.Error("target should be restored")
	}

	fpDir := filepath.Join(targetDir, "debug", ".fingerprint")
	if _, err := os.Stat(fpDir); !os.IsNotExist(err) {
		t.Error("fingerprints should be cleaned after restore")
	}
}
