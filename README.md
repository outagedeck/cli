# OutageDeck CLI

[![OutageDeck logo](https://raw.githubusercontent.com/outagedeck/mcp/main/assets/logo.png)](https://outagedeck.com?utm_source=github&utm_medium=repository&utm_campaign=cli_distribution)

[![Latest release](https://img.shields.io/github/v/release/outagedeck/cli)](https://github.com/outagedeck/cli/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/outagedeck/cli?quickstart=1)

![OutageDeck CLI checking GitHub, Anthropic, Cloudflare, and OpenAI](assets/terminal-preview.png)

Check the live status of 170+ cloud and SaaS providers from a terminal or CI script. OutageDeck normalizes each vendor's official status feed; it does not replace synthetic monitoring.

## Install

```bash
brew install outagedeck/tap/outagedeck
```

With [MacPorts](https://ports.macports.org/port/outagedeck/):

```bash
sudo port install outagedeck
```

With [mise](https://mise.jdx.dev/):

```bash
mise use -g github:outagedeck/cli@0.1.2
```

With Nix:

```bash
nix run github:outagedeck/cli -- --version
```

In a Dev Container or Codespace, use the published [Dev Containers Extra feature](https://github.com/devcontainers-extra/features/tree/main/src/outagedeck):

```json
"features": {
  "ghcr.io/devcontainers-extra/features/outagedeck:1": {}
}
```

Or launch the repository's ready-to-use configuration with [GitHub Codespaces](https://codespaces.new/outagedeck/cli?quickstart=1); it installs the current OutageDeck release automatically.

On Windows with Scoop:

```powershell
scoop bucket add outagedeck https://github.com/outagedeck/scoop-bucket
scoop install outagedeck
```

Or install from source:

```bash
go install github.com/outagedeck/cli/cmd/outagedeck@latest
```

Already use GitHub CLI? The companion extension defaults to GitHub provider and service health:

```bash
gh extension install outagedeck/gh-outagedeck
gh outagedeck
```

Release archives for macOS, Linux, and Windows are available on the [releases page](https://github.com/outagedeck/cli/releases).

## Use

```console
$ outagedeck status aws cloudflare github openai
OK AWS: Operational — All Systems Operational
OK Cloudflare: Operational — All Systems Operational
OK GitHub: Operational — All Systems Operational
!! OpenAI: Degraded — OpenAI reports an active incident
```

Find a provider slug:

```console
$ outagedeck search "Claude"
anthropic                operational        Anthropic
```

Use structured output in scripts:

```bash
outagedeck status --json --fail-on=outage aws github openai
```

The status command exits with:

- `0` when every provider is below the selected threshold;
- `1` when a request or argument fails;
- `2` when at least one provider meets the selected threshold.

`--fail-on` accepts `degraded` (default), `outage`, `major_outage`, or `never`. Set `OUTAGEDECK_API_KEY` or pass `--api-key` for a higher API quota.

## Data and trust

- Data comes from official vendor status feeds and is refreshed about every 10 minutes.
- An unreachable or malformed response is an error, never `operational`.
- The public API allows 120 requests per hour. The CLI checks up to 20 providers concurrently.
- No telemetry is added by the CLI. Requests identify the client through a standard `User-Agent` header.

Review the [API documentation](https://outagedeck.com/developers/api?utm_source=github&utm_medium=repository&utm_campaign=cli_distribution) or [configure outage alerts](https://outagedeck.com/alerts?utm_source=github&utm_medium=repository&utm_campaign=cli_distribution).

## License

MIT
