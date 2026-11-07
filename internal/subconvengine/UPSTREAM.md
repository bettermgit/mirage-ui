# Vendored engine: SubConv Next Custom

Packages `model`, `renderer`, `parser`, `templatecatalog` and the `testdata/`
fixtures under this directory are vendored from **SubConv Next Custom**
(https://github.com/bettermgit/subconv-next-custom, MIT License), itself a fork
of **Earl9/subconv-next** (MIT License), at tag/commit `b63bbca` (2026-11-05).

The full MIT license text is kept in `UPSTREAM-LICENSE` next to this file.
Import paths were rewritten from `subconv-next/internal/...` to
`github.com/mhsanaei/3x-ui/v3/internal/subconvengine/...`; testdata-relative
paths were adjusted.

Fork additions (each marked `// 3x-ui fork addition` in code): the Reality
options gained an `Extra` passthrough map so non-standard `reality-opts` keys
such as `support-x25519mlkem768` survive the YAML → NodeIR → YAML round-trip
(`model/node.go`, `parser/yaml_mihomo.go`, `renderer/mihomo.go`). No other
engine behaviour was altered — the engine's deliberate output baseline (fixed
DNS/sniffer/ports enforced in `NormalizeRenderOptions`) is kept as upstream.

Integration notes: the HTTP server, auth/workspace/publication layers of the
upstream project are intentionally NOT vendored — 3x-ui provides its own
authentication and storage; only the pure conversion engine is used.
