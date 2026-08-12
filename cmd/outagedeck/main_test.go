package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func providerResponse(slug, name, status string) string {
	return fmt.Sprintf(`{"data":{"slug":%q,"name":%q,"currentStatus":{"code":%q,"label":%q,"headline":"Live headline"},"source":{"checkedAt":"2026-08-05T00:00:00Z"},"counts":{"activeIncidents":1}}}`, slug, name, status, status)
}

func TestNoArgsShowsHelpAndSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(nil, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestStatusOperational(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") == "" {
			t.Fatal("expected user agent")
		}
		fmt.Fprint(writer, providerResponse("github", "GitHub", "operational"))
	}))
	defer server.Close()
	t.Setenv("OUTAGEDECK_API_BASE_URL", server.URL)

	var stdout, stderr bytes.Buffer
	exit := run([]string{"status", "github"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK GitHub: operational") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestStatusThresholdAndJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, providerResponse("openai", "OpenAI", "partial_outage"))
	}))
	defer server.Close()
	t.Setenv("OUTAGEDECK_API_BASE_URL", server.URL)

	var stdout, stderr bytes.Buffer
	exit := run([]string{"status", "--json", "--fail-on=outage", "openai"}, &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if !strings.Contains(stdout.String(), `"status": "partial_outage"`) {
		t.Fatalf("unexpected JSON: %s", stdout.String())
	}
}

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, `{"data":{"providers":[{"slug":"anthropic","name":"Anthropic","tagline":"Claude status","currentStatus":{"code":"operational"}},{"slug":"github","name":"GitHub","tagline":"Source control","currentStatus":{"code":"operational"}}]}}`)
	}))
	defer server.Close()
	t.Setenv("OUTAGEDECK_API_BASE_URL", server.URL)

	var stdout, stderr bytes.Buffer
	exit := run([]string{"search", "Claude"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "anthropic") || strings.Contains(stdout.String(), "github") {
		t.Fatalf("unexpected search output: %s", stdout.String())
	}
}

func TestStatusHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprint(writer, `{"error":{"message":"provider not found"}}`)
	}))
	defer server.Close()
	t.Setenv("OUTAGEDECK_API_BASE_URL", server.URL)

	var stdout, stderr bytes.Buffer
	exit := run([]string{"status", "missing"}, &stdout, &stderr)
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stdout.String(), "provider not found") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestNormalizeProviders(t *testing.T) {
	providers, err := normalizeProviders([]string{"AWS,github", "aws"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(providers, ",") != "aws,github" {
		t.Fatalf("providers = %v", providers)
	}
	if _, err := normalizeProviders([]string{"bad slug"}); err == nil {
		t.Fatal("expected invalid slug error")
	}
}

func TestAlertsBuildsStackSpecificAttributedURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run([]string{"alerts", "GitHub,cloudflare", "github"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	for _, expected := range []string{
		"Set up alerts for github, cloudflare:",
		"stack=github%2Ccloudflare",
		"utm_source=cli",
		"utm_medium=terminal",
		"utm_campaign=cli_distribution",
		"utm_content=alerts_command",
		"selected stack will already be filled in after sign-in",
		"Free email alerts cover up to five providers",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("missing %q in output:\n%s", expected, stdout.String())
		}
	}
}

func TestAlertsRejectsInvalidProvider(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run([]string{"alerts", "bad slug"}, &stdout, &stderr)
	if exit != 1 || !strings.Contains(stderr.String(), "invalid provider slug") {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestProviderURLIsCanonical(t *testing.T) {
	got := providerURL("github")
	if got != "https://outagedeck.com/providers/github" {
		t.Fatalf("providerURL() = %q", got)
	}
	if strings.Contains(got, "?") {
		t.Fatalf("providerURL() contains query parameters: %q", got)
	}
}
