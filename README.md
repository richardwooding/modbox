# modbox

Classic tracker music (MOD / S3M / XM / IT) playing in your browser, with a
FastTracker-flavored visualizer — scrolling pattern grid, per-channel
oscilloscopes, VU meters. **100% Go**, compiled to WebAssembly.

**▶ Play it: https://richardwooding.github.io/modbox/play/**

Drag any module file onto the page, or start with a bundled demo song.

## How it works

- [gotracker/playback](https://github.com/gotracker/playback) renders the
  module tick by tick (~20 ms of audio at a time). Its premix output carries
  per-channel amplitude buffers and the machine's order/row position, which
  is exactly what the oscilloscopes, VU meters, and pattern scroller need —
  no forked engine, no double bookkeeping.
- [Ebitengine](https://ebitengine.org) draws the UI on a canvas and handles
  browser audio output (AudioWorklet under the hood — no special headers, so
  it runs happily on GitHub Pages).
- A backpressured PCM ring buffer bridges the render loop to the audio
  device, and every UI state event is keyed by **sample position**, so the
  highlighted row flips when you *hear* the row, not when it was rendered.

## Controls

| Key       | Action                    |
| --------- | ------------------------- |
| `space`   | pause / resume            |
| `←` / `→` | seek one order back/ahead |
| `+` / `-` | volume                    |
| `esc`     | back to the song picker   |
| `d`       | debug overlay             |

Deep-link a demo with `?demo=0`, `?demo=1`, …

## Development

```sh
go run .                 # native window (macOS/Linux/Windows)
go test ./...            # headless engine tests, no display needed
GOOS=js GOARCH=wasm go build -o docs/play/modbox.wasm .   # the real thing
```

The `docs/` directory is the GitHub Pages site; CI builds the wasm on every
push to `main`.

Demo music by **Drozerix** (Public Domain) — see [NOTICE.md](NOTICE.md).
