# hiLife

High-resolution cellular automata for your terminal and browser.

Go **1.27.3**, Bubble Tea **2.1.0**, Bubbles **2.2.1**, Lip Gloss **2.0.6**, and Svelte **5.57.2**. Versions are pinned in `go.mod` and `web/pnpm-lock.yaml`.

## Run

For a fresh clone, run `make setup && make build` first.

```sh
cd ~/projects/hilife
./hilife                         # terminal; Kitty detection, Braille fallback
./hilife --renderer braille      # portable text rendering
./hilife web --open              # embedded Svelte app at http://127.0.0.1:8787
./hilife --pattern acorn --play
./hilife --pattern random --seed 42 --density .25 --trails
./hilife --pattern replicator --theme neon
./hilife path/to/pattern.rle
./hilife https://conwaylife.com/wiki/Gosper_glider_gun
./hilife --pattern glider --headless 1048576 > future.json
./hilife --list
```

`make build` creates `bin/hilife`. The `hilife` launcher runs it if present, or uses the local Go toolchain to run from source. Rebuild after source changes with `make build`.

## Build and develop

```sh
make setup               # verified latest stable Go into .tools/, Go modules, web dependencies
make build               # build Svelte assets; embed them in bin/hilife
make run                 # terminal from Go sources
make serve               # web from Go sources (requires make web first)
make dev                 # Go server :8787 + Svelte/Vite development server :5173
make check               # go vet + svelte-check
make test                # Go tests with race detector
cd web && pnpm test      # real-browser integration tests
```

Requires pnpm and Node 22.12+ for web development. The compiled Go binary needs neither Node nor pnpm. The web interface uses local assets and system fonts, with no CDN requirement. For browser tests, install Chromium with `pnpm exec playwright install chromium`, or set `HILIFE_CHROMIUM` to an existing Chromium executable. An existing Playwright Chromium cache is detected automatically.

`scripts/bootstrap-go.sh` checks the official Go download manifest, verifies the archive SHA-256, and installs within this checkout. It does not replace system Go. `go.mod` sets the minimum toolchain; the initial build uses Go 1.27.3, the latest stable release on 2026-10-21.

## Features

| Area | Implemented |
|---|---|
| Terminal graphics | Native-resolution PNG images over the Kitty graphics protocol; capability query; automatic Braille fallback; explicit renderer selection; tmux passthrough |
| Portable graphics | 2×4 Braille dots per character, age colours, fading trails, chunk-grid overlay, cached terminal-cell output through Bubble Tea |
| Browser | Svelte canvas, responsive layout, paint/erase/pan tools, scroll zoom, minimap navigation, focus mode, live statistics |
| Worlds | Sparse unbounded coordinates, optional bounded toroidal wrapping, editable dimensions |
| Simulation | Pause, step, adjustable speed, reset, 32 undo snapshots, generation/population/birth/death statistics |
| Fast-forward | Canonical quadtrees and memoized HashLife successors; jumps up to 1,048,576 generations |
| Editing | Continuous mouse strokes, erase, terminal keyboard cursor, camera pan, fit, origin and coordinate jump |
| Patterns | Twelve built-in patterns, terminal picker, browser library, RLE/plaintext/JSON import, HTTP(S) URLs and LifeWiki page discovery |
| Export | JSON preserving absolute coordinates; normalized RLE; browser PNG screenshots |
| Rules | B/S rules including Conway and HighLife; deterministic seeded random starts and adjustable density |
| Appearance | Mint, neon, amber, sunset and mono palettes; age shading; trails; minimap; focus mode |
| Automata | Life, Brian’s Brain, Langton’s Ant, Larger than Life, Faders, Forest Fire, Wildfire and an excitable cyclic reaction mode |

These features take inspiration from cli-of-life, glife, cellscape, tamiroh/clife and the original curses clife. The implementation is independent; no project source has been copied.

## Terminal controls

| Key | Action |
|---|---|
| Space | Run / pause |
| T / N | Advance one generation |
| U | Undo; also pauses |
| R / Shift R | Reset / regenerate the seeded random world |
| WASD / Shift WASD | Pan / pan farther |
| Arrow keys | Move editing cursor |
| X / Enter | Toggle cell under cursor |
| Left mouse drag / right mouse drag | Paint / erase |
| Mouse wheel / + / − | Zoom |
| 0 / H | Fit pattern / go to world origin |
| P | Pattern library |
| O | Import file or URL |
| Ctrl S | Save JSON or RLE |
| E | Edit B/S rule |
| J | Jump to coordinates |
| G / M / L | Grid / minimap / trails |
| Y / B | Cycle theme / switch Kitty ↔ Braille |
| F / ? | Focus / help |
| : | Command prompt |
| Q / Esc / Ctrl C | Quit |

The minimap supports clicking to recenter. Pixel-coordinate mouse events allow individual-dot editing on compatible terminals; keyboard editing is available everywhere. Kitty images and pixel mouse reporting are deleted/reset on exit.

Commands include:

```text
pattern gosper
load ~/patterns/acorn.rle
load https://conwaylife.com/wiki/Acorn
save scene.json
save scene.rle
rule B36/S23
mode brians-brain
advance 1048576
speed 60
seed 42
density 0.28
random
wrap
bounds 160x100
jump -200,500
theme amber
```

## Rendering and engine notes

The terminal starts in Braille and queries Kitty support rather than trusting the terminal name. A positive reply enables a persistent image placement. PNG data is base64-encoded in protocol-sized chunks, emitted through Bubble Tea’s serialized raw-output path. The viewport stays separate from the text controls. Inside tmux, graphics sequences are wrapped in DCS passthrough; tmux must allow passthrough and the outer terminal must support Kitty graphics. If detection fails, Braille remains active. Sixel and iTerm2’s proprietary image protocol are not implemented.

Braille rasterization visits occupied world cells, then assembles terminal glyphs; it does not perform eight world lookups per character. Kitty output uses reported terminal cell pixel dimensions when available, with a 10×20 fallback and a maximum frame of 2400×1600 pixels. At distant zoom levels both renderers aggregate occupancy. Terminal Braille dots share a foreground colour; Kitty and canvas pixels can have separate colours.

Ordinary evolution maintains cell age, transient states and optional trails in a sparse map. The displayed chunk count is the number of occupied 64×64 regions; it is not a claim that ordinary evolution uses bit-packed chunks. Fast-forward builds canonical quadtrees with shared identical nodes and cached successors, and tests compare its results with ordinary stepping. Life rules without B0 are supported. B0 and multistate RLE are rejected explicitly.

HashLife applies to **Life mode without trails or wrapping**. Other modes can advance up to 4,096 generations per command. A jump reports births/deaths between its initial and final snapshots, and restarts visible cell ages; it does not reconstruct every intermediate birth or trail. The cache is reset between large jobs when its retained successor count exceeds 200,000. Very large chaotic worlds can still require substantial memory and time.

Brian’s Brain has firing/refractory states. Larger than Life uses a radius-2 square neighbourhood with B6–9/S5–11. Faders adds decay states to Life. The fire modes use the configured bounded area, deterministic seeded stochastic regrowth and neighbour-driven spread, with rare lightning and a higher ignition probability in Wildfire. `belousov` is an **excitable cyclic cellular automaton**, not a quantitative chemical reaction solver. Langton’s Ant uses the standard right-on-white/left-on-black rule.

Web mode runs one shared local world. Browser tabs connected to the same process share it. Simulation timing is independent of the 20 Hz event stream and 60 Hz terminal display. The server binds to loopback by default; web imports accept uploaded text or HTTP(S) URLs, while local filesystem imports are terminal-only. The server does not need a database or external service.

## Verification

Go tests cover known oscillators, negative coordinates, wrapping, pattern round trips, invalid imports, seeded replay, undo/reset, API actions, cross-origin rejection, Braille bit layout, valid Kitty PNG chunks, and HashLife equivalence including a million-generation glider. Playwright checks playback, drawing/erasing, imports, exports, themes, all automata and mobile overflow.

Kitty protocol responses can be tested in a pseudo-terminal with `python3 scripts/test-terminal.py`. Actual appearance depends on your terminal and font; protocol tests do not replace a visual check in a Kitty-capable terminal.
