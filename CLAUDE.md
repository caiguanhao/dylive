# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

dylive is a Go library and set of CLI tools for watching Douyin (抖音) live streams. It provides:
- A core library (`live.go`) that scrapes Douyin web pages to fetch categories, rooms, and stream URLs
- **dylive**: An interactive TUI app (built with tview/tcell) for browsing and watching live streams
- **dywatch**: A CLI monitor that polls Douyin IDs and detects when streams go live

## Build and Test

```bash
# Build
go build ./dylive/
go build ./dywatch/

# Install
go install -v github.com/caiguanhao/dylive/dylive@latest
go install -v github.com/caiguanhao/dylive/dywatch@latest

# Test core library
go test -v ./...

# Test single package
go test -v ./dylive/
```

## Architecture

The repo is a multi-module Go project with three go.mod files:

- **Root module** (`github.com/caiguanhao/dylive`): Core library with no external dependencies. Defines types (`Category`, `Room`, `User`) and functions for fetching data from Douyin (`GetCategories`, `GetRoomsByCategory`, `GetRoom`). Parses embedded JSON from HTML pages.
- **dylive/** (`github.com/caiguanhao/dylive/dylive`): TUI application. Uses a `replace` directive to reference the parent module locally. Three-pane layout (categories → subcategories → rooms). Integrates with video players (mpv, IINA, VLC) and supports multi-window grid arrangement.
- **dywatch/** (`github.com/caiguanhao/dylive/dywatch`): Standalone monitor. Polls every 5 seconds, outputs stream URLs or JSON, supports Go template-based `-run` commands.

## Key Environment Variables

- `PLAYER` — video player command (defaults: mpv, iina-cli)
- `EDITOR` — text editor for room info (defaults: vim, vi)
- `COLOR` — TUI color scheme (default: lightgreen)
- `TIME_FORMAT` — time format for templates (default: 2006-01-02-15-04-05)

## Notes

- Stream URLs come in FLV and HLS formats with quality levels: uhd, hd, sd, ld
- The core library uses a Firefox user agent and scrapes `live.douyin.com`
- Runtime config is stored at `~/.dylive.json`
