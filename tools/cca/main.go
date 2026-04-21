package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// --- Schema types ---

type Products struct {
	Products []Product `json:"products"`
}

type Product struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	PageCount int      `json:"pageCount"`
	Versions  []string `json:"versions"`
}

type Summary struct {
	Path        string    `json:"path"`
	Product     string    `json:"product"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Headings    []Heading `json:"headings"`
	TokenEstimate int     `json:"tokenEstimate"`
	CodeExamples CodeExampleSummary `json:"codeExamples"`
}

type Heading struct {
	Text          string    `json:"text"`
	Level         int       `json:"level"`
	TokenEstimate int       `json:"tokenEstimate"`
	Slug          string    `json:"slug"`
	Children      []Heading `json:"children"`
}

type CodeExampleSummary struct {
	Total      int            `json:"total"`
	ByLanguage map[string]int `json:"byLanguage"`
}

type Section struct {
	Heading      string      `json:"heading"`
	HeadingLevel int         `json:"headingLevel"`
	Body         string      `json:"body"`
	CodeBlocks   []CodeBlock `json:"codeBlocks"`
}

type CodeBlock struct {
	Language      string `json:"language"`
	Content       string `json:"content"`
	TokenEstimate int    `json:"tokenEstimate"`
}

// --- Helpers ---

func repoRoot() string {
	// Walk up from the tool's location to find the repo root (contains products.json)
	dir, _ := os.Getwd()
	// Allow override via first non-flag argument or env
	if root := os.Getenv("CONTENT_ROOT"); root != "" {
		return root
	}
	// Try relative paths from cwd
	for _, candidate := range []string{".", "../..", "../../.."} {
		p := filepath.Join(dir, candidate, "v1", "products.json")
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(filepath.Join(dir, candidate))
			return abs
		}
	}
	return dir
}

func loadProducts(root string) ([]Product, error) {
	data, err := os.ReadFile(filepath.Join(root, "v1", "products.json"))
	if err != nil {
		return nil, err
	}
	var p Products
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return p.Products, nil
}

func pageDirectories(root string) ([]string, error) {
	v1 := filepath.Join(root, "v1")
	entries, err := os.ReadDir(v1)
	if err != nil {
		return nil, err
	}
	var pages []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		productDir := filepath.Join(v1, e.Name())
		pageDirs, err := os.ReadDir(productDir)
		if err != nil {
			continue
		}
		for _, p := range pageDirs {
			if p.IsDir() {
				pages = append(pages, filepath.Join(productDir, p.Name()))
			}
		}
	}
	return pages, nil
}

func loadSummary(pageDir string) (*Summary, error) {
	data, err := os.ReadFile(filepath.Join(pageDir, "summary.json"))
	if err != nil {
		return nil, err
	}
	var s Summary
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func countHeadings(headings []Heading) int {
	count := len(headings)
	for _, h := range headings {
		count += countHeadings(h.Children)
	}
	return count
}

func median(sorted []int) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return float64(sorted[n/2-1]+sorted[n/2]) / 2.0
	}
	return float64(sorted[n/2])
}

func mean(vals []int) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return float64(sum) / float64(len(vals))
}

func stddev(vals []int, avg float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		diff := float64(v) - avg
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(vals)))
}

func printSeparator() {
	fmt.Println(strings.Repeat("─", 60))
}

// --- report command ---

type pageStats struct {
	path          string
	product       string
	sectionCount  int
	tokenEstimate int
	codeExamples  int
}

func runReport(root string, filterProduct string) error {
	products, err := loadProducts(root)
	if err != nil {
		return fmt.Errorf("loading products: %w", err)
	}

	pageDirs, err := pageDirectories(root)
	if err != nil {
		return fmt.Errorf("listing pages: %w", err)
	}

	type productAgg struct {
		label         string
		sections      []int
		tokens        []int
		codeExamples  int
		langCounts    map[string]int
		deepestLevel  int
		pagesWithCode int
	}

	allSections := []int{}
	allTokens := []int{}
	productStats := map[string]*productAgg{}

	for _, prod := range products {
		if filterProduct != "" && prod.ID != filterProduct {
			continue
		}
		productStats[prod.ID] = &productAgg{
			label:      prod.Label,
			langCounts: map[string]int{},
		}
	}

	skipped := 0
	for _, dir := range pageDirs {
		parts := strings.Split(filepath.ToSlash(dir), "/")
		// product is the second-to-last directory component relative to v1
		// dir = root/v1/{product}/{page}
		productID := ""
		for i, p := range parts {
			if p == "v1" && i+1 < len(parts) {
				productID = parts[i+1]
				break
			}
		}
		if filterProduct != "" && productID != filterProduct {
			continue
		}
		agg, ok := productStats[productID]
		if !ok {
			skipped++
			continue
		}

		s, err := loadSummary(dir)
		if err != nil {
			skipped++
			continue
		}

		sc := countHeadings(s.Headings)
		agg.sections = append(agg.sections, sc)
		agg.tokens = append(agg.tokens, s.TokenEstimate)
		agg.codeExamples += s.CodeExamples.Total
		allSections = append(allSections, sc)
		allTokens = append(allTokens, s.TokenEstimate)

		if s.CodeExamples.Total > 0 {
			agg.pagesWithCode++
		}
		for lang, count := range s.CodeExamples.ByLanguage {
			agg.langCounts[lang] += count
		}

		// track deepest heading level
		for _, h := range s.Headings {
			if h.Level > agg.deepestLevel {
				agg.deepestLevel = h.Level
			}
			for _, ch := range h.Children {
				if ch.Level > agg.deepestLevel {
					agg.deepestLevel = ch.Level
				}
			}
		}
	}

	// Sort for median/percentile calculations
	sort.Ints(allSections)
	sort.Ints(allTokens)

	fmt.Println()
	fmt.Println("  DOCS CONTENT REPORT")
	printSeparator()

	// Overall
	fmt.Printf("  Products:          %d\n", len(productStats))
	fmt.Printf("  Total pages:       %d\n", len(allSections))
	if len(allSections) > 0 {
		avgSec := mean(allSections)
		fmt.Printf("  Sections/page:\n")
		fmt.Printf("    Min:             %d\n", allSections[0])
		fmt.Printf("    Max:             %d\n", allSections[len(allSections)-1])
		fmt.Printf("    Median:          %.1f\n", median(allSections))
		fmt.Printf("    Mean:            %.1f\n", avgSec)
		fmt.Printf("    Std dev:         %.1f\n", stddev(allSections, avgSec))
		p90idx := int(math.Ceil(0.90*float64(len(allSections)))) - 1
		fmt.Printf("    P90:             %d\n", allSections[p90idx])
	}

	totalTokens := 0
	for _, t := range allTokens {
		totalTokens += t
	}
	if len(allTokens) > 0 {
		fmt.Printf("  Tokens/page:\n")
		fmt.Printf("    Total:           %d\n", totalTokens)
		fmt.Printf("    Min:             %d\n", allTokens[0])
		fmt.Printf("    Max:             %d\n", allTokens[len(allTokens)-1])
		fmt.Printf("    Median:          %.0f\n", median(allTokens))
		fmt.Printf("    Mean:            %.0f\n", mean(allTokens))
	}

	// Per-product breakdown
	productIDs := make([]string, 0, len(productStats))
	for id := range productStats {
		productIDs = append(productIDs, id)
	}
	sort.Strings(productIDs)

	for _, id := range productIDs {
		agg := productStats[id]
		sort.Ints(agg.sections)
		sort.Ints(agg.tokens)

		fmt.Println()
		printSeparator()
		fmt.Printf("  PRODUCT: %s (%s)\n", agg.label, id)
		printSeparator()
		fmt.Printf("  Pages:             %d\n", len(agg.sections))

		if len(agg.sections) > 0 {
			avgSec := mean(agg.sections)
			fmt.Printf("  Sections/page:\n")
			fmt.Printf("    Min:             %d\n", agg.sections[0])
			fmt.Printf("    Max:             %d\n", agg.sections[len(agg.sections)-1])
			fmt.Printf("    Median:          %.1f\n", median(agg.sections))
			fmt.Printf("    Mean:            %.1f\n", avgSec)
			fmt.Printf("    Std dev:         %.1f\n", stddev(agg.sections, avgSec))
			p90idx := int(math.Ceil(0.90*float64(len(agg.sections)))) - 1
			fmt.Printf("    P90:             %d\n", agg.sections[p90idx])
		}

		productTokens := 0
		for _, t := range agg.tokens {
			productTokens += t
		}
		if len(agg.tokens) > 0 {
			fmt.Printf("  Tokens:\n")
			fmt.Printf("    Total:           %d\n", productTokens)
			fmt.Printf("    Min/page:        %d\n", agg.tokens[0])
			fmt.Printf("    Max/page:        %d\n", agg.tokens[len(agg.tokens)-1])
			fmt.Printf("    Median/page:     %.0f\n", median(agg.tokens))
		}

		fmt.Printf("  Code examples:     %d total across %d pages\n", agg.codeExamples, agg.pagesWithCode)

		if len(agg.langCounts) > 0 {
			fmt.Printf("  Languages:\n")
			langs := make([]string, 0, len(agg.langCounts))
			for l := range agg.langCounts {
				langs = append(langs, l)
			}
			sort.Slice(langs, func(i, j int) bool {
				return agg.langCounts[langs[i]] > agg.langCounts[langs[j]]
			})
			for _, l := range langs {
				fmt.Printf("    %-16s %d\n", l+":", agg.langCounts[l])
			}
		}

		if agg.deepestLevel > 0 {
			fmt.Printf("  Deepest heading:   H%d\n", agg.deepestLevel)
		}

		// Pages with zero sections
		zeroPct := 0
		for _, sc := range agg.sections {
			if sc == 0 {
				zeroPct++
			}
		}
		if zeroPct > 0 {
			fmt.Printf("  Pages w/ 0 sections: %d\n", zeroPct)
		}
	}

	fmt.Println()
	printSeparator()
	if skipped > 0 {
		fmt.Printf("  Note: %d page directories skipped (missing summary.json)\n", skipped)
	}
	fmt.Println()
	return nil
}

// --- validate command ---

func runValidate(root string, filterProduct string) error {
	pageDirs, err := pageDirectories(root)
	if err != nil {
		return fmt.Errorf("listing pages: %w", err)
	}

	type validationError struct {
		file string
		err  string
	}

	var errs []validationError
	checked := 0
	pages := 0
	navNodes := 0

	for _, dir := range pageDirs {
		parts := strings.Split(filepath.ToSlash(dir), "/")
		productID := ""
		for i, p := range parts {
			if p == "v1" && i+1 < len(parts) {
				productID = parts[i+1]
				break
			}
		}
		if filterProduct != "" && productID != filterProduct {
			continue
		}

		// Skip nav-only directories (no content files, just child dirs)
		summaryPath := filepath.Join(dir, "summary.json")
		if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
			navNodes++
			continue
		}
		pages++

		// Validate summary.json
		if data, err := os.ReadFile(summaryPath); err != nil {
			errs = append(errs, validationError{summaryPath, "cannot read: " + err.Error()})
		} else {
			checked++
			var s Summary
			if err := json.Unmarshal(data, &s); err != nil {
				errs = append(errs, validationError{summaryPath, "invalid JSON: " + err.Error()})
			} else {
				if s.Path == "" {
					errs = append(errs, validationError{summaryPath, "missing required field: path"})
				}
				if s.Product == "" {
					errs = append(errs, validationError{summaryPath, "missing required field: product"})
				}
				if s.Title == "" {
					errs = append(errs, validationError{summaryPath, "missing required field: title"})
				}
			}
		}

		// Validate full.json
		fullPath := filepath.Join(dir, "full.json")
		if data, err := os.ReadFile(fullPath); err != nil {
			errs = append(errs, validationError{fullPath, "cannot read: " + err.Error()})
		} else {
			checked++
			// full.json must be valid JSON and have path/product/title/sections
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				errs = append(errs, validationError{fullPath, "invalid JSON: " + err.Error()})
			} else {
				for _, field := range []string{"path", "product", "title", "sections"} {
					if _, ok := raw[field]; !ok {
						errs = append(errs, validationError{fullPath, "missing required field: " + field})
					}
				}
			}
		}

		// Validate section files
		sectionsDir := filepath.Join(dir, "sections")
		sectionFiles, err := os.ReadDir(sectionsDir)
		if err != nil && !os.IsNotExist(err) {
			errs = append(errs, validationError{sectionsDir, "cannot read sections dir: " + err.Error()})
			continue
		}
		for _, sf := range sectionFiles {
			if sf.IsDir() || !strings.HasSuffix(sf.Name(), ".json") {
				continue
			}
			sectionPath := filepath.Join(sectionsDir, sf.Name())
			data, err := os.ReadFile(sectionPath)
			if err != nil {
				errs = append(errs, validationError{sectionPath, "cannot read: " + err.Error()})
				continue
			}
			checked++
			var sec Section
			if err := json.Unmarshal(data, &sec); err != nil {
				errs = append(errs, validationError{sectionPath, "invalid JSON: " + err.Error()})
				continue
			}
			if sec.Heading == "" {
				errs = append(errs, validationError{sectionPath, "missing required field: heading"})
			}
			if sec.HeadingLevel == 0 {
				errs = append(errs, validationError{sectionPath, "missing or zero headingLevel"})
			}
			// body may be empty but must be present as a string (json.Unmarshal accepts "")
			for i, cb := range sec.CodeBlocks {
				if cb.Language == "" {
					errs = append(errs, validationError{sectionPath, fmt.Sprintf("codeBlock[%d] missing language", i)})
				}
			}
		}

		// Validate examples.json if present
		exPath := filepath.Join(dir, "examples.json")
		if data, err := os.ReadFile(exPath); err == nil {
			checked++
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				errs = append(errs, validationError{exPath, "invalid JSON: " + err.Error()})
			} else {
				for _, field := range []string{"path", "product", "title"} {
					if _, ok := raw[field]; !ok {
						errs = append(errs, validationError{exPath, "missing required field: " + field})
					}
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("  VALIDATION REPORT")
	printSeparator()
	fmt.Printf("  Pages checked:     %d\n", pages)
	fmt.Printf("  Files checked:     %d\n", checked)
	if navNodes > 0 {
		fmt.Printf("  Nav-only dirs:     %d (skipped, no content files)\n", navNodes)
	}

	if len(errs) == 0 {
		fmt.Println()
		fmt.Println("  All files valid.")
		fmt.Println()
		return nil
	}

	fmt.Printf("  Errors found:      %d\n", len(errs))
	fmt.Println()

	for _, e := range errs {
		// Print path relative to root for readability
		rel := e.file
		if r, err := filepath.Rel(root, e.file); err == nil {
			rel = r
		}
		fmt.Printf("  ERROR  %s\n         %s\n\n", rel, e.err)
	}

	return fmt.Errorf("%d validation error(s) found", len(errs))
}

// --- main ---

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: cca <command> [options]

Commands:
  report    Print statistical report about content
  validate  Validate all section files are well-formed

Options:
  --root <path>     Path to repo root (default: auto-detected)
  --product <id>    Filter to a specific product (e.g. atlas, node)

Environment:
  CONTENT_ROOT      Override repo root path
`)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	cmd := args[0]
	args = args[1:]

	root := ""
	filterProduct := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--root requires a value")
				os.Exit(1)
			}
			i++
			root = args[i]
		case "--product":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--product requires a value")
				os.Exit(1)
			}
			i++
			filterProduct = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if root == "" {
		root = repoRoot()
	}

	// Verify root looks right
	if _, err := os.Stat(filepath.Join(root, "v1", "products.json")); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot find v1/products.json in root %q. Use --root to specify the repo root.\n", root)
		os.Exit(1)
	}

	var err error
	switch cmd {
	case "report":
		err = runReport(root, filterProduct)
	case "validate":
		err = runValidate(root, filterProduct)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
