package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

var version = "dev"

const defaultAPIBase = "https://outagedeck.com/api/v1"

type currentStatus struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Headline string `json:"headline"`
	Summary  string `json:"summary"`
}

type source struct {
	CheckedAt string `json:"checkedAt"`
}

type counts struct {
	ActiveIncidents int `json:"activeIncidents"`
}

type provider struct {
	Slug             string        `json:"slug"`
	Name             string        `json:"name"`
	Tagline          string        `json:"tagline"`
	ShortDescription string        `json:"shortDescription"`
	CurrentStatus    currentStatus `json:"currentStatus"`
	Source           source        `json:"source"`
	Counts           counts        `json:"counts"`
}

type providerEnvelope struct {
	Data provider `json:"data"`
}

type statusEnvelope struct {
	Data struct {
		Providers []provider `json:"providers"`
	} `json:"data"`
}

type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

type result struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	Label           string `json:"label"`
	Headline        string `json:"headline,omitempty"`
	ActiveIncidents int    `json:"activeIncidents"`
	CheckedAt       string `json:"checkedAt,omitempty"`
	URL             string `json:"url"`
	Error           string `json:"error,omitempty"`
}

func apiBase() string {
	if value := strings.TrimRight(os.Getenv("OUTAGEDECK_API_BASE_URL"), "/"); value != "" {
		return value
	}
	return defaultAPIBase
}

func client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func requestJSON(ctx context.Context, httpClient *http.Client, endpoint, apiKey string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "outagedeck-cli/"+version+" (+https://github.com/outagedeck/cli)")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var payload errorEnvelope
		_ = json.NewDecoder(response.Body).Decode(&payload)
		message := payload.Error.Message
		if message == "" {
			message = payload.Message
		}
		if message == "" {
			message = response.Status
		}
		return errors.New(message)
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode OutageDeck response: %w", err)
	}
	return nil
}

func providerURL(slug string) string {
	return "https://outagedeck.com/providers/" + url.PathEscape(slug) +
		"?utm_source=cli&utm_medium=terminal&utm_campaign=cli_distribution"
}

func fetchProvider(ctx context.Context, httpClient *http.Client, slug, apiKey string) result {
	var payload providerEnvelope
	endpoint := apiBase() + "/providers/" + url.PathEscape(slug)
	if err := requestJSON(ctx, httpClient, endpoint, apiKey, &payload); err != nil {
		return result{Slug: slug, Name: slug, Status: "error", Error: err.Error(), URL: providerURL(slug)}
	}
	data := payload.Data
	if data.Name == "" || data.CurrentStatus.Code == "" {
		return result{Slug: slug, Name: slug, Status: "error", Error: "unexpected provider response", URL: providerURL(slug)}
	}
	headline := data.CurrentStatus.Headline
	if headline == "" {
		headline = data.CurrentStatus.Summary
	}
	label := data.CurrentStatus.Label
	if label == "" {
		label = data.CurrentStatus.Code
	}
	return result{
		Slug:            data.Slug,
		Name:            data.Name,
		Status:          data.CurrentStatus.Code,
		Label:           label,
		Headline:        headline,
		ActiveIncidents: data.Counts.ActiveIncidents,
		CheckedAt:       data.Source.CheckedAt,
		URL:             providerURL(data.Slug),
	}
}

func normalizeProviders(values []string) ([]string, error) {
	seen := make(map[string]bool)
	providers := make([]string, 0, len(values))
	for _, value := range values {
		for _, raw := range strings.Split(value, ",") {
			slug := strings.ToLower(strings.TrimSpace(raw))
			if slug == "" || seen[slug] {
				continue
			}
			for i, char := range slug {
				valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-'
				if !valid || (char == '-' && (i == 0 || i == len(slug)-1)) {
					return nil, fmt.Errorf("invalid provider slug: %s", slug)
				}
			}
			seen[slug] = true
			providers = append(providers, slug)
		}
	}
	if len(providers) == 0 {
		return nil, errors.New("provide at least one provider slug")
	}
	if len(providers) > 20 {
		return nil, errors.New("at most 20 providers can be checked at once")
	}
	return providers, nil
}

var statusRanks = map[string]int{
	"operational":    0,
	"maintenance":    1,
	"unknown":        1,
	"degraded":       2,
	"partial_outage": 3,
	"major_outage":   4,
}

var failureRanks = map[string]int{
	"degraded":     2,
	"outage":       3,
	"major_outage": 4,
	"never":        1 << 30,
}

func statusCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print machine-readable JSON")
	failOn := flags.String("fail-on", "degraded", "degraded, outage, major_outage, or never")
	timeout := flags.Duration("timeout", 10*time.Second, "HTTP timeout")
	apiKey := flags.String("api-key", os.Getenv("OUTAGEDECK_API_KEY"), "OutageDeck API key")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	threshold, ok := failureRanks[strings.ToLower(*failOn)]
	if !ok {
		fmt.Fprintln(stderr, "--fail-on must be degraded, outage, major_outage, or never")
		return 1
	}
	providers, err := normalizeProviders(flags.Args())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "--timeout must be positive")
		return 1
	}

	ctx := context.Background()
	results := make([]result, len(providers))
	done := make(chan int, len(providers))
	httpClient := client(*timeout)
	for index, slug := range providers {
		go func(index int, slug string) {
			results[index] = fetchProvider(ctx, httpClient, slug, strings.TrimSpace(*apiKey))
			done <- index
		}(index, slug)
	}
	for range providers {
		<-done
	}

	hasError := false
	hasFailure := false
	for _, item := range results {
		if item.Error != "" {
			hasError = true
			continue
		}
		if rank, exists := statusRanks[item.Status]; !exists || rank >= threshold {
			hasFailure = true
		}
	}

	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(results); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	} else {
		for _, item := range results {
			if item.Error != "" {
				fmt.Fprintf(stdout, "! %s: check failed: %s\n", item.Name, item.Error)
				continue
			}
			marker := "OK"
			if item.Status != "operational" {
				marker = "!!"
			}
			fmt.Fprintf(stdout, "%s %s: %s", marker, item.Name, item.Label)
			if item.Headline != "" {
				fmt.Fprintf(stdout, " — %s", item.Headline)
			}
			fmt.Fprintln(stdout)
		}
	}

	if hasError {
		return 1
	}
	if hasFailure {
		return 2
	}
	return 0
}

func searchCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("search", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print machine-readable JSON")
	limit := flags.Int("limit", 10, "maximum matches")
	timeout := flags.Duration("timeout", 10*time.Second, "HTTP timeout")
	apiKey := flags.String("api-key", os.Getenv("OUTAGEDECK_API_KEY"), "OutageDeck API key")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	query := strings.ToLower(strings.TrimSpace(strings.Join(flags.Args(), " ")))
	if query == "" {
		fmt.Fprintln(stderr, "provide a provider name or product to search for")
		return 1
	}
	if *limit < 1 || *limit > 50 {
		fmt.Fprintln(stderr, "--limit must be between 1 and 50")
		return 1
	}

	var payload statusEnvelope
	if err := requestJSON(context.Background(), client(*timeout), apiBase()+"/status", strings.TrimSpace(*apiKey), &payload); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	matches := make([]provider, 0)
	for _, item := range payload.Data.Providers {
		haystack := strings.ToLower(strings.Join([]string{item.Slug, item.Name, item.Tagline, item.ShortDescription}, " "))
		if strings.Contains(haystack, query) {
			matches = append(matches, item)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		iExact := strings.EqualFold(matches[i].Slug, query) || strings.EqualFold(matches[i].Name, query)
		jExact := strings.EqualFold(matches[j].Slug, query) || strings.EqualFold(matches[j].Name, query)
		if iExact != jExact {
			return iExact
		}
		return matches[i].Name < matches[j].Name
	})
	if len(matches) > *limit {
		matches = matches[:*limit]
	}
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(matches); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No tracked providers matched %q. Browse https://outagedeck.com/providers\n", query)
		return 0
	}
	for _, item := range matches {
		fmt.Fprintf(stdout, "%-24s %-18s %s\n", item.Slug, item.CurrentStatus.Code, item.Name)
	}
	return 0
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, `OutageDeck CLI — check cloud and SaaS status from official vendor feeds

Usage:
  outagedeck status [flags] <provider> [provider...]
  outagedeck search [flags] <query>
  outagedeck version

Examples:
  outagedeck status aws cloudflare github openai
  outagedeck status --json --fail-on=outage anthropic
  outagedeck search "Claude"

Environment:
  OUTAGEDECK_API_KEY       optional higher-quota API key
  OUTAGEDECK_API_BASE_URL  API override for testing`)
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 1
	}
	switch args[0] {
	case "status", "check":
		return statusCommand(args[1:], stdout, stderr)
	case "search":
		return searchCommand(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return 0
	case "help", "--help", "-h":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		usage(stderr)
		return 1
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
