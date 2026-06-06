# caltui

A keyboard-driven calorie & macro tracker that runs entirely in your terminal —
MyFitnessPal in a TUI. Offline-first, single-user, local SQLite. Tracks calories
plus protein/carbs/fat, with daily goals, custom foods & saved meals, body-weight
tracking, and history/trend charts.

## Status

Under active construction. See the implementation plan for the phased build.

## Quick start

```bash
# Requires Go >= 1.25 (this repo was built with 1.26).
make build        # -> ./bin/caltui   (pure-Go SQLite; no CGo/Xcode needed)
go run ./cmd/caltui
```

First run launches a short setup wizard (a TDEE calculator) that suggests daily
calorie and macro targets you can edit anytime.

## Food data

- **Offline (always available):** a small USDA dataset of common whole foods is
  bundled into the binary for instant search.
- **Online (optional):** branded/packaged items via USDA FoodData Central. Get a
  free API key at <https://fdc.nal.usda.gov/api-key-signup>. Provide it any of
  these ways (highest priority first):
  1. `FDC_API_KEY` environment variable;
  2. a `.env` file in the working directory with `FDC_API_KEY=...` (auto-loaded;
     copy `.env.example`);
  3. `fdc_api_key` in the config file (`~/.config/tuitracker/config.toml`).

  Without a key the app runs offline-only.

## Deployment (macOS, Linux & Windows)

caltui is a single static binary with no CGo (pure-Go SQLite), so it
cross-compiles trivially:

```bash
make dist   # -> dist/caltui-{darwin,linux}-{arm64,amd64} and
            #    dist/caltui-windows-{amd64,arm64}.exe
```

Copy the matching binary to the target machine (or `go install ./cmd/caltui`).
The bundled food database and all features work identically on every OS. On
Windows, run the `.exe` from **Windows Terminal** (or any modern terminal) for
correct Unicode/colour rendering. For online lookups, set `FDC_API_KEY` in the
environment or `fdc_api_key` in the config file.

## Keys

Arrows or vim `hjkl` to move; `1`–`4` switch tabs; `a` add, `d` delete, `e` edit,
`/` search, `?` help, `q` quit.

## Data & config location

- macOS / Linux: `~/.config/tuitracker/` (honors `$XDG_CONFIG_HOME`)
- Windows: `%AppData%\tuitracker\` (or `$XDG_CONFIG_HOME` if set)

## Attribution

Food data: U.S. Department of Agriculture, Agricultural Research Service.
FoodData Central. <https://fdc.nal.usda.gov> (public domain, CC0).

## Development

```bash
make test           # go test ./...
make lint           # golangci-lint run
make etl            # rebuild bundled food DB from USDA downloads
make update-golden  # refresh teatest snapshots after intentional UI changes
```
