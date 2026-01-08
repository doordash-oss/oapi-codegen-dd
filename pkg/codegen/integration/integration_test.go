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

package integration

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

//go:embed testdata/specs
var specsFS embed.FS

type testResult struct {
	name   string
	passed bool
	stage  string // "read", "generate", "write", "mod-init", "mod-tidy", "build"
	err    string
	tmpDir string
}

var showMaxErrors = 50

func TestIntegration(t *testing.T) {
	specPath := os.Getenv("SPEC")

	// Get project root
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Clean up .sandbox directory at the start (in project root)
	sandboxDir := filepath.Join(projectRoot, ".sandbox")

	// Remove existing sandbox directory
	os.RemoveAll(sandboxDir)

	// Create fresh sandbox directory
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		t.Fatalf("Failed to create sandbox directory: %v", err)
	}

	// Collect specs to process
	specs := collectSpecs(t, specPath)
	if len(specs) == 0 {
		fmt.Fprintln(os.Stderr, "No specs to process, skipping integration test")
		return
	}

	fmt.Fprintf(os.Stderr, "\n🔍 Found %d spec(s) to process\n", len(specs))

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
		currentSpec string
		hasFailures = false
	)

	// Progress tracker
	progressTicker := make(chan struct{}, 100)
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		for range progressTicker {
			mu.Lock()
			c := completed
			spec := currentSpec
			mu.Unlock()
			if spec != "" {
				// Shorten spec name if too long
				if len(spec) > 50 {
					spec = "..." + spec[len(spec)-47:]
				}
				// Pad with spaces to clear previous line (80 chars total)
				msg := fmt.Sprintf("⏳ Progress: %d/%d - %s", c, total, spec)
				fmt.Fprintf(os.Stderr, "\r%-80s", msg)
			} else {
				fmt.Fprintf(os.Stderr, "\r%-80s", fmt.Sprintf("⏳ Progress: %d/%d completed", c, total))
			}
		}
	}()

	// Process specs in parallel
	semaphore := make(chan struct{}, 50) // Limit concurrency to 50

	for _, name := range specs {
		wg.Add(1)

		go func() {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			result := &testResult{name: name, passed: true}

			// Set current spec being processed and increment completed count
			mu.Lock()
			currentSpec = name
			completed++
			mu.Unlock()
			select {
			case progressTicker <- struct{}{}:
			default:
			}

			// Track result at the end
			defer func() {
				mu.Lock()
				results = append(results, *result)
				if !result.passed {
					hasFailures = true
				}
				mu.Unlock()
				select {
				case progressTicker <- struct{}{}:
				default:
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
			configContent := `package: integration
generate:
  client: true
  response-validators: true
client:
  name: IntegrationClient
output:
  use-single-file: true
  filename: generated.go
`

			if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
				recordFailure("setup", "failed to write config file: %s", err)
				return
			}

			// Create context with timeout for all operations
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
					recordFailure("generate", "oapi-codegen timed out after 2 minutes")
				} else {
					recordFailure("generate", "oapi-codegen failed:\n%s", string(output))
				}
				return
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "   ✅ Code generation successful\n")
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
			cmd = exec.CommandContext(ctx, "go", "mod", "edit", "-replace", fmt.Sprintf("github.com/doordash/oapi-codegen-dd/v3=%s", projectRoot))
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
					recordFailure("mod-tidy", "go mod tidy timed out after 2 minutes")
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
					recordFailure("build", "go build timed out after 2 minutes")
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

	// Close progress tracker and wait for it to finish
	close(progressTicker)
	<-progressDone
	fmt.Fprintf(os.Stderr, "\r✅ Progress: %d/%d completed\n\n", total, total)

	// Print summary
	printSummary(total, results)

	// Fail the test if there were any failures
	if hasFailures {
		t.Fail()
	}
}

func collectSpecs(t *testing.T, specPath string) []string {
	var specs []string

	if specPath != "" {
		// Check if the path exists as-is
		if _, err := os.Stat(specPath); err == nil {
			specs = append(specs, specPath)
			return specs
		}

		// If not found, try prepending testdata/specs/
		fullPath := filepath.Join("testdata", "specs", specPath)
		if _, err := os.Stat(fullPath); err == nil {
			specs = append(specs, fullPath)
			return specs
		}

		// If still not found, fail
		t.Fatalf("Spec file not found: %s (also tried %s)", specPath, fullPath)
	}

	// Walk through testdata/specs
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
			specs = append(specs, path)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk specs directory: %v", err)
	}

	return specs
}

func printSummary(total int, results []testResult) {
	var passed, failed []testResult
	failuresByStage := make(map[string]int)

	for _, r := range results {
		if r.passed {
			passed = append(passed, r)
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

		fmt.Fprintln(os.Stderr, "\n📋 FAILED SPECS (first 10):")
		if len(failed) < showMaxErrors {
			showMaxErrors = len(failed)
		}
		for i := 0; i < showMaxErrors; i++ {
			r := failed[i]
			// Shorten the spec name if it's too long
			specName := r.name
			if len(specName) > 60 {
				specName = "..." + specName[len(specName)-57:]
			}
			fmt.Fprintf(os.Stderr, "\n   %d. %s\n", i+1, specName)
			fmt.Fprintf(os.Stderr, "      Stage: %s\n", r.stage)

			// Show first line of error only
			errLines := strings.Split(r.err, "\n")
			if len(errLines) > 0 {
				errMsg := errLines[0]
				if len(errMsg) > 100 {
					errMsg = errMsg[:97] + "..."
				}
				fmt.Fprintf(os.Stderr, "      Error: %s\n", errMsg)
			}

			if r.tmpDir != "" {
				fmt.Fprintf(os.Stderr, "      Debug: %s/generated.go\n", r.tmpDir)
			}
		}

		if len(failed) > showMaxErrors {
			fmt.Fprintf(os.Stderr, "\n   ... and %d more failures (run with SPEC=<name> to test individually)\n", len(failed)-showMaxErrors)
		}

		fmt.Fprintln(os.Stderr, "\n💡 TIP: To debug a specific failure:")
		fmt.Fprintln(os.Stderr, "   SPEC=<spec-name> make test-integration")
	} else {
		fmt.Fprintln(os.Stderr, "\n🎉 ALL SPECS PASSED!")
	}

	fmt.Fprintln(os.Stderr, strings.Repeat("═", 80))
}
