# Implementation notes

## Extraction

- Source: `github.com/assurrussa/goshared/pkg/logger`; keep its MIT notice.
- Own the complete logging implementation and public handler packages here,
  without a dependency on goshared's buildinfo, environment or test helpers.
- Prefer side-effect-free `New`; retain the legacy global constructor explicitly
  so consumers can migrate their imports before changing process wiring.
- Preserve consumer interface identity through aliases in goshared. The legacy
  mock generator must use package mode because source mode cannot discover an
  aliased interface.
- User asked to account for Go 1.27 slog. Versioned official docs and installed
  API files show MultiHandler arrived in Go 1.26, DiscardHandler in 1.24 and no
  public slog additions in 1.27. Use the standard handlers; retain Go 1.26 as the
  minimum and validate with Go 1.27. Replace the former samber dependencies with
  a small uniform sampling handler using concurrency-safe math/rand/v2.
- Correct inherited context slice sharing, record mutation without Clone,
  pretty grouping/LogValuer/ReplaceAttr/source handling, timestamp minutes,
  and shared close state. Pretty rendering delegates slog semantics to its
  JSONHandler, then formats the resulting record for local output.
- New output files use 0600. Existing file permissions stay unchanged. The
  compatibility facade preserves APIs/global behavior but does not restore the
  previous, more permissive file creation mode.
- Caller writers and handlers remain borrowed; owned closers transfer only on
  successful construction. Close errors are retained across concurrent callers.
- Final toolchain is Go 1.27.1; go.mod keeps the 1.26 language/API floor.
- Fresh dependency resolution initially selected x/sys v0.42.0. The vulnerability
  scan reported GO-2026-5024; the official advisory identifies x/sys/windows
  NewNTUnicodeString before v0.44.0 (the scanner's summary mislabeled it as the
  standard library). Retain source-module pins: x/sys v0.45.0, go-colorable
  v0.1.15 and go-isatty v0.0.22.

Verification: targeted tests and make check passed on Go 1.27.1 (vet, lint with
zero issues, race tests x5). gopls vulnerability check on the final dependencies
reported no findings. The published consumer probe passed for
`49b04636363424351456e65b52c9484d3e8d898d`
(`v0.0.0-20260922055603-49b046363634`) with Go 1.27.1, a fresh module cache,
GOWORK=off, no replace directives and race checking. It resolved the public
module and all handler packages without a goshared dependency. No CI workflow added.
