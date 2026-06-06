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
  free API key at <https://fdc.nal.usda.gov/api-key-signup> and set it via the
  `FDC_API_KEY` environment variable or `fdc_api_key` in the config file. Without
  a key, the app runs offline-only.

## Keys

Arrows or vim `hjkl` to move; `1`–`4` switch tabs; `a` add, `d` delete, `e` edit,
`/` search, `?` help, `q` quit.

## Data & config location

`~/.config/tuitracker/` (honors `$XDG_CONFIG_HOME`).

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
