package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

func usage() {
	fmt.Println(`SVG Animation Test Lab (YAML-driven)

Usage:
  anim-test-lab <command> [flags]

Commands:
  validate  -c <config.yaml>                     Validate YAML schema & inheritance
  list                                           List available effect families, methods & measurements
  samples   -c <config.yaml> [-out <dir>]        Generate standalone SVG samples & visual catalogue
  run       -c <config.yaml> [-test <filter>] [-out <dir>]  Run browser tests & generate reports
  report    -in <raw-results.json> [-c <resolved-config.yaml>] [-out <dir>]  Regenerate reports from saved results
  rerun     -dir <previous-run-dir> [-out <dir>] Re-run an experiment using its saved resolved-config.yaml
  serve     -dir <out-dir> [-addr :8093]         Serve visual catalogue & HTML reports`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "validate":
		fs := flag.NewFlagSet("validate", flag.ExitOnError)
		cfgPath := fs.String("c", "", "Path to YAML configuration")
		_ = fs.Parse(os.Args[2:])
		if *cfgPath == "" {
			log.Fatal("-c <config.yaml> is required")
		}
		cfg, err := animlab.LoadConfig(*cfgPath)
		if err != nil {
			log.Fatalf("INVALID: %v", err)
		}
		specs := animlab.ExpandScenarios(cfg)
		fmt.Printf("VALID: %s (%d scenarios expanded)\n", cfg.Test.Name, len(specs))

	case "list":
		reg := animlab.DefaultRegistry()
		var fams []string
		for k := range reg {
			fams = append(fams, k)
		}
		sort.Strings(fams)
		fmt.Println("Available Effect Families & Rendering Methods:")
		for _, f := range fams {
			g := reg[f]
			fmt.Printf("  - %-18s methods=%v  (%s)\n", f, g.SupportedMethods(), g.Description())
		}
		fmt.Println("\nSuggested Instance Count Presets: 50, 100, 250, 500 (or any positive integer)")
		fmt.Println("Measurements: process_cpu, frame_timing, long_tasks, rendering_activity, memory, animation_correctness")

	case "samples":
		fs := flag.NewFlagSet("samples", flag.ExitOnError)
		cfgPath := fs.String("c", "", "Path to YAML configuration")
		outDir := fs.String("out", "", "Output directory")
		_ = fs.Parse(os.Args[2:])
		if *cfgPath == "" {
			log.Fatal("-c <config.yaml> is required")
		}
		cfg, err := animlab.LoadConfig(*cfgPath)
		if err != nil {
			log.Fatalf("Config error: %v", err)
		}
		dir := resolveOutDir(*outDir, cfg)
		cfg.Output.Samples = true
		if err := animlab.WriteAllOutputs(cfg, nil, dir); err != nil {
			log.Fatalf("Generating samples failed: %v", err)
		}
		fmt.Printf("Visual samples & catalogue written to %s/catalogue/index.html\n", dir)

	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		cfgPath := fs.String("c", "", "Path to YAML configuration")
		filter := fs.String("test", "", "Optional filter substring (family/method/workload/runId)")
		outDir := fs.String("out", "", "Output directory")
		_ = fs.Parse(os.Args[2:])
		if *cfgPath == "" {
			log.Fatal("-c <config.yaml> is required")
		}
		runExperiment(*cfgPath, *outDir, *filter)

	case "rerun":
		fs := flag.NewFlagSet("rerun", flag.ExitOnError)
		prevDir := fs.String("dir", "", "Previous run directory containing resolved-config.yaml")
		outDir := fs.String("out", "", "Output directory")
		_ = fs.Parse(os.Args[2:])
		if *prevDir == "" {
			log.Fatal("-dir <previous-run-dir> is required")
		}
		runExperiment(filepath.Join(*prevDir, "resolved-config.yaml"), *outDir, "")

	case "report":
		fs := flag.NewFlagSet("report", flag.ExitOnError)
		inPath := fs.String("in", "", "Path to raw-results.json")
		cfgPath := fs.String("c", "", "Optional path to resolved-config.yaml (defaults to sibling)")
		outDir := fs.String("out", "", "Output directory (defaults to raw-results.json directory)")
		_ = fs.Parse(os.Args[2:])
		if *inPath == "" {
			log.Fatal("-in <raw-results.json> is required")
		}
		raw, err := os.ReadFile(*inPath)
		if err != nil {
			log.Fatal(err)
		}
		var res animlab.LabResults
		if err := json.Unmarshal(raw, &res); err != nil {
			log.Fatal(err)
		}
		cp := *cfgPath
		if cp == "" {
			cp = filepath.Join(filepath.Dir(*inPath), "resolved-config.yaml")
		}
		cfg, err := animlab.LoadConfig(cp)
		if err != nil {
			log.Fatalf("Loading config %s: %v", cp, err)
		}
		dir := *outDir
		if dir == "" {
			dir = filepath.Dir(*inPath)
		}
		if err := animlab.WriteAllOutputs(cfg, &res, dir); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Reports regenerated in %s\n", dir)

	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		dir := fs.String("dir", "testlab/results", "Directory to serve")
		addr := fs.String("addr", ":8093", "Listen address")
		_ = fs.Parse(os.Args[2:])
		fmt.Printf("Serving Animation Test Lab outputs from %s on http://0.0.0.0%s\n", *dir, *addr)
		log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(*dir))))

	default:
		usage()
		os.Exit(1)
	}
}

func resolveOutDir(flagOut string, cfg *animlab.Config) string {
	if flagOut != "" {
		return flagOut
	}
	if cfg.Output.Dir != "" {
		return cfg.Output.Dir
	}
	slug := strings.ReplaceAll(strings.ToLower(cfg.Test.Name), " ", "-")
	return filepath.Join("testlab", "results", fmt.Sprintf("%s-%s", slug, time.Now().Format("20060102-150405")))
}

func runExperiment(cfgPath, flagOut, filter string) {
	cfg, err := animlab.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	dir := resolveOutDir(flagOut, cfg)
	res, err := animlab.RunSuite(cfg, dir, filter)
	if err != nil {
		log.Fatalf("Run error: %v", err)
	}
	if err := animlab.WriteAllOutputs(cfg, res, dir); err != nil {
		log.Fatalf("Report error: %v", err)
	}
	fmt.Printf("\nCompleted %d scenarios -> %s\n", len(res.Scenarios), dir)
	if cfg.Output.MarkdownReport {
		fmt.Printf("  Markdown Report: %s\n", filepath.Join(dir, "report.md"))
	}
	if cfg.Output.HTMLReport {
		fmt.Printf("  HTML Report:     %s\n", filepath.Join(dir, "report.html"))
	}
	if cfg.Output.Samples {
		fmt.Printf("  Catalogue:       %s\n", filepath.Join(dir, "catalogue", "index.html"))
	}
}
