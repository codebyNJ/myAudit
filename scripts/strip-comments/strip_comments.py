#!/usr/bin/env python3
"""Remove comments from source files across the repository."""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
GO_STRIPPER = Path(__file__).resolve().parent

SKIP_DIRS = {
    ".git",
    "node_modules",
    "dist",
    "target",
    ".vite",
    "binaries",
    "runs",
    "bin",
}

CODE_EXTENSIONS = {".go", ".ts", ".tsx", ".rs", ".sh"}


def iter_source_files(root: Path):
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for name in filenames:
            path = Path(dirpath) / name
            if path.suffix in CODE_EXTENSIONS:
                yield path


def strip_c_style(source: str, *, jsx: bool = False, shell_hash: bool = False) -> str:
    """Remove //, /* */, optional JSX {/* */}, and optional # line comments."""
    out: list[str] = []
    i = 0
    n = len(source)

    while i < n:
        if jsx and i + 3 <= n and source[i : i + 3] == "{/*":
            end = source.find("*/}", i + 3)
            if end != -1:
                i = end + 3
                continue

        if shell_hash and source[i] == "#":
            if i == 0 or source[i - 1] == "\n":
                while i < n and source[i] != "\n":
                    i += 1
                continue

        if i + 1 < n and source[i : i + 2] == "//":
            while i < n and source[i] != "\n":
                i += 1
            continue

        if i + 1 < n and source[i : i + 2] == "/*":
            end = source.find("*/", i + 2)
            i = end + 2 if end != -1 else n
            continue

        ch = source[i]

        if ch == "`":
            out.append(ch)
            i += 1
            while i < n:
                if source[i] == "\\":
                    out.append(source[i : i + 2])
                    i += 2
                elif source[i] == "`":
                    out.append(source[i])
                    i += 1
                    break
                else:
                    out.append(source[i])
                    i += 1
            continue

        if ch in "\"'":
            quote = ch
            out.append(ch)
            i += 1
            while i < n:
                if source[i] == "\\":
                    out.append(source[i : i + 2])
                    i += 2
                elif source[i] == quote:
                    out.append(source[i])
                    i += 1
                    break
                else:
                    out.append(source[i])
                    i += 1
            continue

        if ch == "r" and i + 1 < n and source[i + 1] == "#":
            out.append("r")
            i += 1
            hashes = 0
            while i < n and source[i] == "#":
                hashes += 1
                out.append("#")
                i += 1
            if i < n and source[i] in "\"'":
                quote = source[i]
                out.append(quote)
                i += 1
                while i < n:
                    if source[i] == quote:
                        count = 0
                        j = i + 1
                        while j < n and source[j] == "#":
                            count += 1
                            j += 1
                        if count == hashes:
                            out.append(source[i:j])
                            i = j
                            break
                    out.append(source[i])
                    i += 1
            continue

        out.append(ch)
        i += 1

    return "".join(out)


def collapse_blank_runs(text: str) -> str:
    lines = text.splitlines(keepends=True)
    cleaned: list[str] = []
    blank_run = 0
    for line in lines:
        if line.strip() == "":
            blank_run += 1
            if blank_run <= 2:
                cleaned.append(line)
        else:
            blank_run = 0
            cleaned.append(line)
    result = "".join(cleaned)
    if result and not result.endswith("\n"):
        result += "\n"
    return result


def strip_non_go(path: Path) -> None:
    source = path.read_text(encoding="utf-8")
    jsx = path.suffix == ".tsx"
    shell_hash = path.suffix == ".sh"
    stripped = strip_c_style(source, jsx=jsx, shell_hash=shell_hash)
    stripped = collapse_blank_runs(stripped)
    path.write_text(stripped, encoding="utf-8")


def strip_go_files(paths: list[Path]) -> None:
    if not paths:
        return
    cmd = ["go", "run", f"./{GO_STRIPPER.relative_to(ROOT)}"] + [str(p) for p in paths]
    subprocess.run(cmd, cwd=ROOT, check=True)


def main() -> int:
    files = sorted(iter_source_files(ROOT))
    go_files = [p for p in files if p.suffix == ".go"]
    other_files = [p for p in files if p.suffix != ".go"]

    if go_files:
        strip_go_files(go_files)
    for path in other_files:
        strip_non_go(path)

    print(f"Stripped comments from {len(files)} files")
    return 0


if __name__ == "__main__":
    sys.exit(main())
