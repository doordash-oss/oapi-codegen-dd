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

	defaultFrameworks = []codegen.HandlerKind{codegen.HandlerKindStdHTTP}
)

//go:embed testdata/specs
var specsFS embed.FS

type testResult struct {
	name      string
	framework string // empty for default, or framework name when testing all frameworks
	passed    bool

	// "read", "generate", "write", "mod-init", "mod-tidy", "build"
	stage       string
	err         string
	tmpDir      string
	linesOfCode int
}

// All supported server frameworks for multi-framework testing
var allFrameworks = []codegen.HandlerKind{
	codegen.HandlerKindBeego,
	codegen.HandlerKindChi,
	codegen.HandlerKindEcho,
	codegen.HandlerKindEchoV5,
	codegen.HandlerKindFastHTTP,
	codegen.HandlerKindFiber,
	codegen.HandlerKindGin,
	codegen.HandlerKindGoFrame,
	codegen.HandlerKindGoZero,
	codegen.HandlerKindGorillaMux,
	codegen.HandlerKindHertz,
	codegen.HandlerKindIris,
	codegen.HandlerKindKratos,
	codegen.HandlerKindStdHTTP,
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

	// Collect frameworks to test (default: std-http only, FRAMEWORKS=all for all)
	frameworks := getFrameworks()

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

	multiFramework := len(frameworks) > 1
	if multiFramework {
		fmt.Fprintf(os.Stderr, "⚙️ Running tests in parallel (%d specs × %d frameworks)...\n\n",
			len(specs), len(frameworks))
	} else {
		fmt.Fprintf(os.Stderr, "⚙️ Running tests in parallel...\n\n")
	}

	// Track results for summary
	var (
		mu          sync.Mutex
		wg          sync.WaitGroup
		results     = make([]testResult, 0, len(specs)*len(frameworks))
		total       = len(specs) * len(frameworks)
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
			inProgressCount := len(inProgress)
			mu.Unlock()

			fmt.Fprintf(os.Stderr, "⏳ %d/%d completed, %d in progress\n", c, total, inProgressCount)
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
		if multiFramework {
			// Multi-framework mode: generate models once, then each handler
			onResult := func(r testResult) {
				mu.Lock()
				results = append(results, r)
				completed++
				if !r.passed {
					hasFailures = true
				}
				mu.Unlock()
			}
			processSpecMultiFramework(name, frameworks, binaryPath, projectRoot, sandboxDir, verbose, onResult)
			continue
		}

		// Single framework mode: parallel processing
		fw := frameworks[0]
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

				// Update cache immediately after each spec
				if cache != nil {
					if result.passed {
						cache.MarkPassed(name)
					} else {
						cache.MarkFailed(name)
					}
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

			// Create temp directory
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

			specPath, err := filepath.Abs(name)
			if err != nil {
				recordFailure("setup", "failed to get absolute path: %s", err)
				return
			}

			cfg := codegen.Configuration{
				PackageName: "integration",
				Generate: &codegen.GenerateOptions{
					Client: true,
					Validation: codegen.ValidationOptions{
						Response: true,
					},
					Handler: &codegen.HandlerOptions{
						Kind: fw,
						Name: "IntegrationHandler",
						Validation: codegen.HandlerValidation{
							Request:  true,
							Response: true,
						},
						Service: &codegen.ServiceOptions{},
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
	fmt.Fprintf(os.Stderr, "✅ Progress: %d/%d completed\n\n", total, total)

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

// processSpecMultiFramework generates models once, then each handler, builds all together
func processSpecMultiFramework(name string, frameworks []codegen.HandlerKind, binaryPath, projectRoot, sandboxDir string, verbose bool, onResult func(testResult)) {
	// Create temp directory
	safeName := strings.ReplaceAll(name, "/", "_")
	safeName = strings.ReplaceAll(safeName, "testdata_specs_", "")
	safeName = strings.TrimSuffix(safeName, ".yaml")
	safeName = strings.TrimSuffix(safeName, ".yml")
	safeName = strings.TrimSuffix(safeName, ".json")

	tmpDir := filepath.Join(sandboxDir, safeName)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		for _, fw := range frameworks {
			onResult(testResult{name: name, framework: string(fw), passed: false, stage: "setup", err: err.Error()})
		}
		return
	}

	specPath, err := filepath.Abs(name)
	if err != nil {
		for _, fw := range frameworks {
			onResult(testResult{name: name, framework: string(fw), passed: false, stage: "setup", err: err.Error()})
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), specTimeout)
	defer cancel()

	var results []testResult

	// 1. Generate models
	modelsCfg := codegen.Configuration{
		PackageName: "integration",
		Generate:    &codegen.GenerateOptions{},
		Output:      &codegen.Output{UseSingleFile: true, Filename: "models.go"},
	}
	modelsConfigFile := filepath.Join(tmpDir, "models_config.yaml")
	configContent, _ := yaml.Marshal(modelsCfg)
	if err := os.WriteFile(modelsConfigFile, configContent, 0644); err != nil {
		for _, fw := range frameworks {
			onResult(testResult{name: name, framework: string(fw), passed: false, stage: "setup", err: err.Error(), tmpDir: tmpDir})
		}
		return
	}

	cmd := exec.CommandContext(ctx, binaryPath, "-config", modelsConfigFile, specPath)
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, fw := range frameworks {
			onResult(testResult{name: name, framework: string(fw), passed: false, stage: "generate-models", err: string(output), tmpDir: tmpDir})
		}
		return
	}

	// 2. Generate handler for each framework
	for _, fw := range frameworks {
		fwDir := filepath.Join(tmpDir, string(fw))
		handlerCfg := codegen.Configuration{
			PackageName: "integration",
			Generate: &codegen.GenerateOptions{
				Models:  boolPtr(false),
				Handler: &codegen.HandlerOptions{Kind: fw, Service: &codegen.ServiceOptions{}},
			},
			Output: &codegen.Output{Directory: string(fw), UseSingleFile: true, Filename: "handler.go"},
		}
		cfgFile := filepath.Join(tmpDir, fmt.Sprintf("config_%s.yaml", fw))
		configContent, _ := yaml.Marshal(handlerCfg)
		if err := os.WriteFile(cfgFile, configContent, 0644); err != nil {
			results = append(results, testResult{name: name, framework: string(fw), passed: false, stage: "setup", err: err.Error(), tmpDir: fwDir})
			continue
		}
		cmd := exec.CommandContext(ctx, binaryPath, "-config", cfgFile, specPath)
		cmd.Dir = tmpDir
		if output, err := cmd.CombinedOutput(); err != nil {
			results = append(results, testResult{name: name, framework: string(fw), passed: false, stage: "generate", err: string(output), tmpDir: fwDir})
			continue
		}
		results = append(results, testResult{name: name, framework: string(fw), passed: true, tmpDir: fwDir})
	}

	// Helper to report all results and return
	reportAndReturn := func() {
		for _, r := range results {
			onResult(r)
		}
	}

	// 3. Initialize go module
	cmd = exec.CommandContext(ctx, "go", "mod", "init", "integration")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for i := range results {
			if results[i].passed {
				results[i].passed = false
				results[i].stage = "mod-init"
				results[i].err = string(output)
			}
		}
		reportAndReturn()
		return
	}

	cmd = exec.CommandContext(ctx, "go", "mod", "edit", "-replace", fmt.Sprintf("github.com/doordash-oss/oapi-codegen-dd/v3=%s", projectRoot))
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for i := range results {
			if results[i].passed {
				results[i].passed = false
				results[i].stage = "mod-edit"
				results[i].err = string(output)
			}
		}
		reportAndReturn()
		return
	}

	cmd = exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for i := range results {
			if results[i].passed {
				results[i].passed = false
				results[i].stage = "mod-tidy"
				results[i].err = string(output)
			}
		}
		reportAndReturn()
		return
	}

	// 4. Build each handler
	modelsFile := filepath.Join(tmpDir, "models.go")
	modelsContent, err := os.ReadFile(modelsFile)
	if err != nil {
		for i := range results {
			if results[i].passed {
				results[i].passed = false
				results[i].stage = "build"
				results[i].err = fmt.Sprintf("failed to read models.go: %s", err)
			}
		}
		reportAndReturn()
		return
	}

	var passed, failed []string
	for i := range results {
		if !results[i].passed {
			failed = append(failed, results[i].framework)
			continue
		}
		fw := results[i].framework
		fwDir := filepath.Join(tmpDir, fw)

		if err := os.WriteFile(filepath.Join(fwDir, "models.go"), modelsContent, 0644); err != nil {
			results[i].passed = false
			results[i].stage = "build"
			results[i].err = fmt.Sprintf("failed to copy models.go: %s", err)
			failed = append(failed, fw)
			continue
		}

		cmd = exec.CommandContext(ctx, "go", "build", "-o", "/dev/null", "./...")
		cmd.Dir = fwDir
		if output, err := cmd.CombinedOutput(); err != nil {
			results[i].passed = false
			results[i].stage = "build"
			results[i].err = string(output)
			failed = append(failed, fw)
			continue
		}

		// Runtime init test - catches invalid route patterns (e.g. chi panics on /**)
		initTest := routerInitTest(codegen.HandlerKind(fw))
		if err := os.WriteFile(filepath.Join(fwDir, "router_init_test.go"), []byte(initTest), 0644); err != nil {
			results[i].passed = false
			results[i].stage = "init-test"
			results[i].err = fmt.Sprintf("failed to write test: %s", err)
			failed = append(failed, fw)
			continue
		}
		cmd = exec.CommandContext(ctx, "go", "test", "-run", "TestRouterInit", "./...")
		cmd.Dir = fwDir
		if output, err := cmd.CombinedOutput(); err != nil {
			results[i].passed = false
			results[i].stage = "init-test"
			results[i].err = string(output)
			failed = append(failed, fw)
		} else {
			passed = append(passed, fw)
		}
	}

	// Compact output
	if verbose {
		fmt.Fprintf(os.Stderr, "📝 %s: ", filepath.Base(name))
		if len(failed) == 0 {
			fmt.Fprintf(os.Stderr, "✅ all %d frameworks passed\n", len(passed))
		} else {
			fmt.Fprintf(os.Stderr, "✅ %d passed, ❌ %d failed (%s)\n", len(passed), len(failed), strings.Join(failed, ", "))
		}
	}

	reportAndReturn()
}

func boolPtr(b bool) *bool { return &b }

// routerInitTest returns framework-specific test code for router initialization.
func routerInitTest(fw codegen.HandlerKind) string {
	// Frameworks where NewRouter takes framework instance first and returns nothing
	switch fw {
	case codegen.HandlerKindEcho:
		return `package integration
import ("testing"; "github.com/labstack/echo/v4")
func TestRouterInit(t *testing.T) { NewRouter(echo.New(), nil) }
`
	case codegen.HandlerKindEchoV5:
		return `package integration
import ("testing"; "github.com/labstack/echo/v5")
func TestRouterInit(t *testing.T) { NewRouter(echo.New(), nil) }
`
	case codegen.HandlerKindFiber:
		return `package integration
import ("testing"; fiber "github.com/gofiber/fiber/v3")
func TestRouterInit(t *testing.T) { NewRouter(fiber.New(), nil) }
`
	case codegen.HandlerKindGin:
		return `package integration
import ("testing"; "github.com/gin-gonic/gin")
func TestRouterInit(t *testing.T) { NewRouter(gin.New(), nil) }
`
	case codegen.HandlerKindGoFrame:
		return `package integration
import ("testing"; "github.com/gogf/gf/v2/net/ghttp")
func TestRouterInit(t *testing.T) { NewRouter(ghttp.GetServer(), nil) }
`
	case codegen.HandlerKindHertz:
		return `package integration
import ("testing"; "github.com/cloudwego/hertz/pkg/app/server")
func TestRouterInit(t *testing.T) { NewRouter(server.Default(), nil) }
`
	case codegen.HandlerKindIris:
		return `package integration
import ("testing"; "github.com/kataras/iris/v12")
func TestRouterInit(t *testing.T) { NewRouter(iris.New(), nil) }
`
	default:
		// Frameworks where NewRouter(svc, opts...) returns a router
		return `package integration
import "testing"
func TestRouterInit(t *testing.T) { _ = NewRouter(nil) }
`
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

// getFrameworks returns the list of frameworks to test based on FRAMEWORKS env var.
// Default: all frameworks. FRAMEWORKS=std-http tests only std-http.
func getFrameworks() []codegen.HandlerKind {
	env := os.Getenv("FRAMEWORKS")
	if env == "" {
		return defaultFrameworks
	}
	if env == "all" {
		return allFrameworks
	}

	// Parse comma-separated list
	var frameworks []codegen.HandlerKind
	for _, name := range strings.Split(env, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		fw := codegen.HandlerKind(name)
		if fw.IsValid() {
			frameworks = append(frameworks, fw)
		}
	}

	if len(frameworks) == 0 {
		return defaultFrameworks
	}
	return frameworks
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
			if r.framework != "" {
				fmt.Fprintf(os.Stderr, "\n   %d. %s [%s]\n", i+1, specName, r.framework)
			} else {
				fmt.Fprintf(os.Stderr, "\n   %d. %s\n", i+1, specName)
			}
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
			if r.framework != "" {
				fmt.Fprintf(os.Stderr, "  %s [%s]\n", r.name, r.framework)
			} else {
				fmt.Fprintf(os.Stderr, "  %s\n", r.name)
			}
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
