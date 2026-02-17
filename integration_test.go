// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

//go:build integration
// +build integration

package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/codegen"
	"go.yaml.in/yaml/v4"
)

const (
	// Maximum number of errors to display in summary
	showMaxErrors = 50

	// Default maximum concurrency for parallel test execution
	defaultMaxConcurrency = 50

	// Timeout for each spec's operations (generate, build, etc.)
	specTimeout = 5 * time.Minute

	// Maximum number of error lines to show per failure
	maxErrorLines = 15

	// Maximum length of error line before truncation
	maxErrorLineLength = 200

	// CacheFileName is the name of the cache file
	cacheFileName = ".integration-cache.json"

	// CacheTTL is how long a cached result is valid
	cacheTTL = 60 * time.Minute
)

var (
	// Specs that are known to be problematic (too large, timeout, etc.)
	// Add specs here to skip them in CI unless explicitly requested via SPEC env var
	skipSpecs = map[string]bool{
		// Example: "testdata/specs/3.0/aws/ec2.yml": true,
	}
)

//go:embed testdata/specs
var specsFS embed.FS

type testResult struct {
	name   string
	passed bool

	// "read", "generate", "write", "mod-init", "mod-tidy", "build"
	stage       string
	err         string
	tmpDir      string
	linesOfCode int
}

func TestIntegration(t *testing.T) {
	// Collect spec paths from environment
	var specPaths []string
	if spec := os.Getenv("SPEC"); spec != "" {
		specPaths = append(specPaths, spec)
	}
	if specs := os.Getenv("SPECS"); specs != "" {
		specPaths = append(specPaths, strings.Fields(specs)...)
	}

	// Get project root (current directory since test is at root)
	projectRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Clean up sandbox directory at the start (in /tmp)
	sandboxDir := "/tmp/oapi-codegen-sandbox"

	// Remove existing sandbox directory
	os.RemoveAll(sandboxDir)

	// Create fresh sandbox directory
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		t.Fatalf("Failed to create sandbox directory: %v", err)
	}

	// Collect specs to process
	specs := collectSpecs(t, specPaths)
	if len(specs) == 0 {
		fmt.Fprintln(os.Stderr, "No specs to process, skipping integration test")
		return
	}

	// Load cache (unless disabled via INTEGRATION_NO_CACHE=1 or running single spec)
	var cache *ResultCache
	singleSpec := len(specPaths) == 1
	useCache := os.Getenv("INTEGRATION_NO_CACHE") == "" && !singleSpec
	if useCache {
		var err error
		cache, err = NewResultCache(projectRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to load cache: %v\n", err)
		} else if os.Getenv("CLEAR_CACHE") == "1" {
			if err := cache.Clear(); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Failed to clear cache: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "🗑️  Cache cleared\n")
			}
		} else {
			fmt.Fprintf(os.Stderr, "📦 Loaded cache with %d entries\n", cache.Size())
			if cache.Size() > 0 {
				originalCount := len(specs)
				specs = cache.FilterUncached(specs)
				skipped := originalCount - len(specs)
				if skipped > 0 {
					fmt.Fprintf(os.Stderr, "📦 Skipping %d cached passing specs (%d remaining)\n", skipped, len(specs))
				} else {
					fmt.Fprintf(os.Stderr, "📦 No specs matched cache (paths or hashes may differ)\n")
				}
			}
		}
	}

	if len(specs) == 0 {
		fmt.Fprintln(os.Stderr, "✅ All specs cached as passing. Use CLEAR_CACHE=1 to retest.")
		return
	}

	fmt.Fprintf(os.Stderr, "\n🔍 Found %d specs to process\n", len(specs))

	// Sort specs to start known slow ones first (LPT scheduling)
	slowSpecs := map[string]int{
		"id4i.de.yaml":                  0,
		"stripe-spec3.yaml":             1,
		"netbox.dev.yaml":               2,
		"microsoft.com/graph.1.0.1.yml": 3,
	}
	sort.SliceStable(specs, func(i, j int) bool {
		iPriority := len(slowSpecs)
		jPriority := len(slowSpecs)
		for suffix, priority := range slowSpecs {
			if strings.HasSuffix(specs[i], suffix) {
				iPriority = priority
			}
			if strings.HasSuffix(specs[j], suffix) {
				jPriority = priority
			}
		}
		return iPriority < jPriority
	})

	// Enable verbose mode for single spec
	verbose := len(specs) == 1

	// Build the oapi-codegen binary once
	fmt.Fprintf(os.Stderr, "🔨 Building oapi-codegen binary...\n")
	binaryPath := filepath.Join(os.TempDir(), "oapi-codegen-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/oapi-codegen")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build oapi-codegen:\n%s", string(output))
	}
	defer os.Remove(binaryPath)

	fmt.Fprintf(os.Stderr, "⚙️ Running tests in parallel...\n\n")

	// Track results for summary
	var (
		mu          sync.Mutex
		wg          sync.WaitGroup
		results     = make([]testResult, 0, len(specs))
		total       = len(specs)
		completed   = 0
		inProgress  = make(map[string]time.Time) // spec -> start time
		hasFailures = false
	)

	// Progress tracker with periodic refresh
	stopProgress := make(chan struct{})
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		printProgress := func() {
			mu.Lock()
			c := completed
			// Build list of in-progress specs with durations
			var running []string
			for spec, started := range inProgress {
				// Shorten spec name
				name := spec
				if len(name) > 30 {
					name = "..." + name[len(name)-27:]
				}
				running = append(running, fmt.Sprintf("%s(%.0fs)", name, time.Since(started).Seconds()))
			}
			mu.Unlock()

			// Sort for consistent output
			sort.Strings(running)

			var msg string
			if len(running) > 0 {
				if len(running) > 3 {
					msg = fmt.Sprintf("⏳ %d/%d | %s +%d more", c, total, strings.Join(running[:3], ", "), len(running)-3)
				} else {
					msg = fmt.Sprintf("⏳ %d/%d | %s", c, total, strings.Join(running, ", "))
				}
			} else {
				msg = fmt.Sprintf("⏳ %d/%d completed", c, total)
			}
			fmt.Fprintf(os.Stderr, "\r%-120s", msg)
		}

		for {
			select {
			case <-ticker.C:
				printProgress()
			case <-stopProgress:
				return
			}
		}
	}()

	// Process specs in parallel
	maxConcurrency := defaultMaxConcurrency
	if envMax := os.Getenv("INTEGRATION_MAX_CONCURRENCY"); envMax != "" {
		if parsed, err := strconv.Atoi(envMax); err == nil && parsed > 0 {
			maxConcurrency = parsed
		}
	}
	semaphore := make(chan struct{}, maxConcurrency)

	for _, name := range specs {
		wg.Add(1)

		go func() {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			result := &testResult{name: name, passed: true}

			// Track start of processing
			mu.Lock()
			inProgress[name] = time.Now()
			mu.Unlock()

			// Track result at the end
			defer func() {
				mu.Lock()
				delete(inProgress, name)
				completed++
				results = append(results, *result)
				if !result.passed {
					hasFailures = true
				}
				mu.Unlock()

				// Update cache immediately after each spec (survives timeout)
				if cache != nil {
					if result.passed {
						cache.MarkPassed(name)
					} else {
						cache.MarkFailed(name)
					}
					// Best effort, ignore errors
					_ = cache.Save()
				}
			}()

			// Helper to record failure
			recordFailure := func(stage, errMsg string, args ...any) {
				result.passed = false
				result.stage = stage
				result.err = fmt.Sprintf(errMsg, args...)
				if verbose {
					fmt.Fprintf(os.Stderr, "\n❌ FAILED at stage '%s':\n%s\n", stage, result.err)
				}
			}

			// Create temp directory for this test inside .sandbox with spec-based name
			// Convert spec path to safe directory name
			safeName := strings.ReplaceAll(name, "/", "_")
			safeName = strings.ReplaceAll(safeName, "testdata_specs_", "")
			safeName = strings.TrimSuffix(safeName, ".yaml")
			safeName = strings.TrimSuffix(safeName, ".yml")
			safeName = strings.TrimSuffix(safeName, ".json")

			tmpDir := filepath.Join(sandboxDir, safeName)
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				recordFailure("setup", "failed to create temp dir: %s", err)
				return
			}
			result.tmpDir = tmpDir

			genFile := filepath.Join(tmpDir, "generated.go")
			configFile := filepath.Join(tmpDir, "config.yaml")

			// Get absolute path to spec file
			specPath, err := filepath.Abs(name)
			if err != nil {
				recordFailure("setup", "failed to get absolute path: %s", err)
				return
			}

			// Create config file
			cfg := codegen.Configuration{
				PackageName: "integration",
				Generate: &codegen.GenerateOptions{
					Client: true,
					Validation: codegen.ValidationOptions{
						Response: true,
					},
					Handler: &codegen.HandlerOptions{
						Kind: codegen.HandlerKindStdHTTP,
						Name: "IntegrationHandler",
						Validation: codegen.HandlerValidation{
							Request:  true,
							Response: true,
						},
					},
					MCPServer: &codegen.MCPServerOptions{},
				},
				Client: &codegen.Client{
					Name: "IntegrationClient",
				},
				Output: &codegen.Output{
					UseSingleFile: true,
					Filename:      "generated.go",
				},
			}
			configContent, err := yaml.Marshal(cfg)
			if err != nil {
				recordFailure("setup", "failed to marshal config: %s", err)
				return
			}

			if err := os.WriteFile(configFile, configContent, 0644); err != nil {
				recordFailure("setup", "failed to write config file: %s", err)
				return
			}

			// Create context with timeout for all operations
			ctx, cancel := context.WithTimeout(context.Background(), specTimeout)
			defer cancel()

			if verbose {
				fmt.Fprintf(os.Stderr, "\n📝 Testing: %s\n", name)
				fmt.Fprintf(os.Stderr, "   Working directory: %s\n", tmpDir)
			}

			// Run oapi-codegen binary to generate code
			if verbose {
				fmt.Fprintf(os.Stderr, "   ⚙️  Running oapi-codegen...\n")
			}
			genCmd := exec.CommandContext(ctx, binaryPath, "-config", configFile, specPath)
			genCmd.Dir = tmpDir
			output, err := genCmd.CombinedOutput()
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					recordFailure("generate", "oapi-codegen timed out after %v", specTimeout)
				} else {
					recordFailure("generate", "oapi-codegen failed:\n%s", string(output))
				}
				return
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "   ✅ Code generation successful\n")
			}

			// Count lines of code in generated file
			if genContent, err := os.ReadFile(genFile); err == nil {
				result.linesOfCode = len(strings.Split(string(genContent), "\n"))
			}

			// Initialize go module
			if verbose {
				fmt.Fprintf(os.Stderr, "   ⚙️  Initializing go module...\n")
			}
			cmd := exec.CommandContext(ctx, "go", "mod", "init", "integration")
			cmd.Dir = tmpDir
			output, err = cmd.CombinedOutput()
			if err != nil {
				recordFailure("mod-init", "go mod init failed:\n%s", string(output))
				return
			}

			// Add replace directive to use local version of the library
			if verbose {
				fmt.Fprintf(os.Stderr, "   ⚙️  Adding replace directive...\n")
			}
			cmd = exec.CommandContext(ctx, "go", "mod", "edit", "-replace", fmt.Sprintf("github.com/doordash-oss/oapi-codegen-dd/v3=%s", projectRoot))
			cmd.Dir = tmpDir
			output, err = cmd.CombinedOutput()
			if err != nil {
				recordFailure("mod-edit", "go mod edit failed:\n%s", string(output))
				return
			}

			// Run go mod tidy
			if verbose {
				fmt.Fprintf(os.Stderr, "   ⚙️  Running go mod tidy...\n")
			}
			cmd = exec.CommandContext(ctx, "go", "mod", "tidy")
			cmd.Dir = tmpDir
			output, err = cmd.CombinedOutput()
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					recordFailure("mod-tidy", "go mod tidy timed out after %v", specTimeout)
				} else {
					recordFailure("mod-tidy", "go mod tidy failed:\n%s", string(output))
				}
				return
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "   ✅ Dependencies resolved\n")
			}

			// Build the generated code
			if verbose {
				fmt.Fprintf(os.Stderr, "   ⚙️  Building generated code...\n")
			}
			cmd = exec.CommandContext(ctx, "go", "build", "-o", "/dev/null", genFile)
			cmd.Dir = tmpDir
			output, err = cmd.CombinedOutput()
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					recordFailure("build", "go build timed out after %v", specTimeout)
				} else {
					recordFailure("build", "go build failed:\n%s", string(output))
				}
				return
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "   ✅ Build successful\n")
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Stop progress tracker and wait for it to finish
	close(stopProgress)
	<-progressDone
	fmt.Fprintf(os.Stderr, "\r✅ Progress: %d/%d completed%-80s\n\n", total, total, "")

	if cache != nil {
		fmt.Fprintf(os.Stderr, "💾 Cache has %d entries\n", cache.Size())
	}

	// Print summary
	printSummary(total, results)

	// Fail the test if there were any failures
	if hasFailures {
		t.Fail()
	}
}

func collectSpecs(t *testing.T, specPaths []string) []string {
	var specs []string

	if len(specPaths) > 0 {
		// Process each provided path (can be file or directory)
		for _, specPath := range specPaths {
			collected := collectSpecsFromPath(t, specPath)
			specs = append(specs, collected...)
		}
		return specs
	}

	// No paths provided - walk through all testdata/specs
	var skipped int
	err := fs.WalkDir(specsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		fileName := d.Name()
		if fileName[0] == '-' || strings.Contains(path, "/stash/") {
			return nil
		}

		if strings.HasSuffix(fileName, ".yml") || strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".json") {
			// Skip problematic specs unless explicitly requested
			if skipSpecs[path] {
				skipped++
				return nil
			}
			specs = append(specs, path)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk specs directory: %v", err)
	}

	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "⏭️  Skipped %d known problematic specs (use SPEC=<name> to test individually)\n", skipped)
	}

	return specs
}

// collectSpecsFromPath collects specs from a single path (file or directory)
func collectSpecsFromPath(t *testing.T, specPath string) []string {
	var specs []string

	// Try as file first (check if it exists and is a file)
	if info, err := os.Stat(specPath); err == nil && !info.IsDir() {
		return []string{specPath}
	}

	// Try as directory
	if info, err := os.Stat(specPath); err == nil && info.IsDir() {
		err := filepath.Walk(specPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			fileName := info.Name()
			if fileName[0] == '-' || strings.Contains(path, "/stash/") {
				return nil
			}

			if strings.HasSuffix(fileName, ".yml") || strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".json") {
				specs = append(specs, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to walk directory %s: %v", specPath, err)
		}
		return specs
	}

	// Try prepending testdata/specs/
	testdataPath := filepath.Join("testdata", "specs", specPath)

	// Check if it's a file in testdata/specs
	if info, err := os.Stat(testdataPath); err == nil && !info.IsDir() {
		return []string{testdataPath}
	}

	// Check if it's a directory in testdata/specs
	if info, err := os.Stat(testdataPath); err == nil && info.IsDir() {
		err := filepath.Walk(testdataPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			fileName := info.Name()
			if fileName[0] == '-' || strings.Contains(path, "/stash/") {
				return nil
			}

			if strings.HasSuffix(fileName, ".yml") || strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".json") {
				specs = append(specs, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to walk directory %s: %v", testdataPath, err)
		}
		return specs
	}

	// Not found
	t.Fatalf("Spec path not found: %s (also tried %s)", specPath, testdataPath)
	return nil
}

func printSummary(total int, results []testResult) {
	var passed, failed []testResult
	failuresByStage := make(map[string]int)
	totalLOC := 0

	for _, r := range results {
		if r.passed {
			passed = append(passed, r)
			totalLOC += r.linesOfCode
		} else {
			failed = append(failed, r)
			failuresByStage[r.stage]++
		}
	}

	fmt.Println(strings.Repeat("═", 80))
	fmt.Fprintln(os.Stderr, "📊 INTEGRATION TEST SUMMARY")
	fmt.Fprintln(os.Stderr, strings.Repeat("═", 80))

	passRate := float64(len(passed)) / float64(total) * 100
	if len(failed) == 0 {
		fmt.Fprintf(os.Stderr, "✅ ALL TESTS PASSED: %d/%d (100%%)\n", len(passed), total)
	} else {
		fmt.Fprintf(os.Stderr, "📈 Results: %d passed, %d failed out of %d total (%.1f%% pass rate)\n",
			len(passed), len(failed), total, passRate)
	}

	if totalLOC > 0 {
		avgLOC := totalLOC / len(passed)
		fmt.Fprintf(os.Stderr, "📝 Total LOC generated: %s lines (avg: %s lines/spec)\n",
			formatNumber(totalLOC), formatNumber(avgLOC))
	}

	fmt.Fprintln(os.Stderr, strings.Repeat("─", 80))

	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "\n❌ FAILURES BY STAGE:")
		// Sort stages for consistent output
		stages := []string{"read", "generate", "write", "setup", "mod-init", "mod-edit", "mod-tidy", "build"}
		for _, stage := range stages {
			if count, ok := failuresByStage[stage]; ok {
				fmt.Fprintf(os.Stderr, "   • %-12s: %d\n", stage, count)
			}
		}

		fmt.Fprintf(os.Stderr, "\n📋 FAILED SPECS (first %d):\n", showMaxErrors)
		errorsToShow := showMaxErrors
		if len(failed) < errorsToShow {
			errorsToShow = len(failed)
		}
		for i := 0; i < errorsToShow; i++ {
			r := failed[i]
			// Shorten the spec name if it's too long
			specName := r.name
			if len(specName) > 60 {
				specName = "..." + specName[len(specName)-57:]
			}
			fmt.Fprintf(os.Stderr, "\n   %d. %s\n", i+1, specName)
			fmt.Fprintf(os.Stderr, "      Stage: %s\n", r.stage)

			// Show error lines for better debugging
			errLines := strings.Split(r.err, "\n")
			linesToShow := maxErrorLines
			if len(errLines) < linesToShow {
				linesToShow = len(errLines)
			}
			fmt.Fprintf(os.Stderr, "      Error:\n")
			for j := 0; j < linesToShow; j++ {
				line := errLines[j]
				if len(line) > maxErrorLineLength {
					line = line[:maxErrorLineLength-3] + "..."
				}
				fmt.Fprintf(os.Stderr, "        %s\n", line)
			}
			if len(errLines) > linesToShow {
				fmt.Fprintf(os.Stderr, "        ... (%d more lines)\n", len(errLines)-linesToShow)
			}

			if r.tmpDir != "" {
				fmt.Fprintf(os.Stderr, "      Debug: %s/generated.go\n", r.tmpDir)
			}
		}

		if len(failed) > errorsToShow {
			fmt.Fprintf(os.Stderr, "\n   ... and %d more failures (run with SPEC=<name> to test individually)\n", len(failed)-errorsToShow)
		}

		fmt.Fprintln(os.Stderr, "\n💡 TIP: To debug a specific failure:")
		fmt.Fprintln(os.Stderr, "   SPEC=<spec-name> make test-integration")
	} else {
		fmt.Fprintln(os.Stderr, "\n🎉 ALL SPECS PASSED!")
	}

	fmt.Fprintln(os.Stderr, strings.Repeat("═", 80))

	// Print simple list of all failed specs at the very end for easy copying
	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "\n📋 FAILED SPECS LIST:")
		for _, r := range failed {
			fmt.Fprintf(os.Stderr, "  %s\n", r.name)
		}
		fmt.Fprintln(os.Stderr)
	}
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// Add commas for thousands
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// cacheEntry represents a cached test result
type cacheEntry struct {
	SpecHash string    `json:"spec_hash"`
	Passed   bool      `json:"passed"`
	TestedAt time.Time `json:"tested_at"`
}

// ResultCache manages cached test results
type ResultCache struct {
	Entries map[string]cacheEntry `json:"entries"` // key is spec path
	mu      sync.RWMutex
	path    string
}

// NewResultCache creates or loads a cache from the given directory
func NewResultCache(cacheDir string) (*ResultCache, error) {
	cachePath := filepath.Join(cacheDir, cacheFileName)
	cache := &ResultCache{
		Entries: make(map[string]cacheEntry),
		path:    cachePath,
	}

	// Try to load existing cache
	data, err := os.ReadFile(cachePath)
	if err == nil {
		if err := json.Unmarshal(data, cache); err != nil {
			// Corrupted cache, start fresh
			cache.Entries = make(map[string]cacheEntry)
		}
	}

	return cache, nil
}

// hashSpec computes a hash of the spec file content
func hashSpec(specPath string) (string, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]), nil // 8 bytes = 16 hex chars
}

// IsCached checks if a spec has a valid cached passing result
func (c *ResultCache) IsCached(specPath string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.Entries[specPath]
	if !ok || !entry.Passed {
		return false
	}

	// Check if cache entry is too old
	if time.Since(entry.TestedAt) > cacheTTL {
		return false
	}

	// Verify spec hasn't changed
	currentHash, err := hashSpec(specPath)
	if err != nil {
		return false
	}

	return entry.SpecHash == currentHash
}

// MarkPassed marks a spec as passing
func (c *ResultCache) MarkPassed(specPath string) {
	hash, err := hashSpec(specPath)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.Entries[specPath] = cacheEntry{
		SpecHash: hash,
		Passed:   true,
		TestedAt: time.Now(),
	}
}

// MarkFailed removes a spec from the cache (so it will be retested)
func (c *ResultCache) MarkFailed(specPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Entries, specPath)
}

// Save persists the cache to disk
func (c *ResultCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.path, data, 0600)
}

// Clear removes all cached entries
func (c *ResultCache) Clear() error {
	c.mu.Lock()
	c.Entries = make(map[string]cacheEntry)
	c.mu.Unlock()

	// Remove the cache file
	if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Size returns the number of cached entries
func (c *ResultCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.Entries)
}

// FilterUncached returns only specs that are not cached as passing
func (c *ResultCache) FilterUncached(specs []string) []string {
	var uncached []string
	for _, spec := range specs {
		if !c.IsCached(spec) {
			uncached = append(uncached, spec)
		}
	}
	return uncached
}
