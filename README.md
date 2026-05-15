# mkultra

A minimal, Unix-philosophy-compliant build tool, now in Go.

## Usage

```
mkultra [target] [NAME=value ...] [-f FILE] [-j N] [-eikSnpqrst]
```

### Options

| Flag | Description |
|------|-------------|
| `-f FILE` | Read FILE as the makefile (default: Makefile, then makefile) |
| `-j N` | Run up to N recipes in parallel (default: 1) |
| `-e` | Environment variables override Makefile assignments |
| `-i` | Ignore errors from commands |
| `-k` | Keep going after errors |
| `-S` | Cancel a prior `-k` (errors stop the build) |
| `-n` | Dry run (print commands but don't execute) |
| `-p` | Print database (rules and variables) |
| `-q` | Question mode (exit 0 if up to date, 1 otherwise) |
| `-r` | Disable built-in rules (no-op, for compatibility) |
| `-s` | Silent mode (don't echo commands) |
| `-t` | Touch targets instead of running recipes |
| `-h` | Show help |
| `--version` | Show version |

Positional `NAME=value` arguments are macro overrides — they take precedence over Makefile assignments and over the environment.

Recipe lines may be prefixed with any combination of `@` (silent), `-` (ignore error), `+` (always run, even under `-n`/`-q`).

## Install

Requires [Go 1.26+](https://go.dev/). No third-party deps.

```bash
go install github.com/euxaristia/mkultra@latest
```

## Features

- **Makefile parsing**: `target: prereq1 prereq2`, tab-indented recipes, `.PHONY`
- **Variable assignment**: `=`, `:=`, `?=`, `+=` with `$(VAR)` and `${VAR}` expansion (cycle-safe)
- **Substitution references**: `$(VAR:s1=s2)` replaces the `s1` suffix with `s2` in each word of `VAR`
- **DAG construction**: Dependency graph with topological sort
- **Circular dependency detection**
- **Mtime-based staleness**: Only rebuilds when prerequisites are newer
- **Automatic variables**: `$@`, `$<`, `$^` (dedup), `$+` (keeps dups), `$?` (newer prereqs)
- **Variable functions**: `$(wildcard pattern)`, `$(shell command)`
- **Process execution**: Runs recipes via `/bin/sh`
- **Parallel jobs**: `-j N` dispatches independent recipes concurrently
- **Error handling**: Exits on first failure, `-k` to continue

## Testing

```bash
# Integration tests
(cd tests/test1 && rm -f hello hello.o && go run ../.. && go run ../..)
(cd tests/test2 && rm -f program main.o utils.o main.c utils.c 2>/dev/null && go run ../..)
(cd tests/test3 && rm -f input.txt output.txt && go run ../..)
(cd tests/test4 && go run ../.. 2>/dev/null && echo "FAIL" || echo "PASS")
```

## License

This project is released into the public domain under the terms of the [UNLICENSE](UNLICENSE).
