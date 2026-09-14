#!/usr/bin/env python3
"""Render desktop/dmg-layout-debug.png — alignment overlay for DMG layout QA."""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
LAYOUT_PATH = ROOT / "scripts" / "dmg-layout.json"
BG = ROOT / "desktop" / "dmg-background.png"
OUT = ROOT / "desktop" / "dmg-layout-debug.png"
SCALE = 2


def s(n: float) -> int:
    return int(n * SCALE)


def main() -> None:
    subprocess.run([sys.executable, str(ROOT / "scripts" / "gen-dmg-background.py")], check=True)

    with open(LAYOUT_PATH, encoding="utf-8") as f:
        layout = json.load(f)

    win_w = layout["window"]["width"]
    win_h = layout["window"]["height"]
    img = Image.open(BG).convert("RGBA")
    d = ImageDraw.Draw(img)

    # Icon-row band (electron-builder default y=220)
    icon_cy = layout["icons"]["left"][1]
    half = layout["icons"]["size"] / 2
    d.rectangle(
        (0, s(icon_cy - half), s(win_w), s(icon_cy + half)),
        outline=(0, 200, 0, 200),
        width=s(1),
    )

    for name, color in (("left", (255, 0, 0, 220)), ("right", (255, 0, 0, 220))):
        cx, cy = layout["icons"][name]
        r = s(6)
        d.line((s(cx) - r * 3, s(cy), s(cx) + r * 3, s(cy)), fill=color, width=s(2))
        d.line((s(cx), s(cy) - r * 3, s(cx), s(cy) + r * 3), fill=color, width=s(2))
        half = layout["icons"]["size"] / 2
        d.rectangle(
            (s(cx - half), s(cy - half), s(cx + half), s(cy + half)),
            outline=color,
            width=s(1),
        )

    a = layout["arrow"]
    for pt, label in (
        (a["start"], "S"),
        (a["control"], "C"),
        (a["end"], "E"),
    ):
        x, y = pt
        d.ellipse((s(x) - s(5), s(y) - s(5), s(x) + s(5), s(y) + s(5)), fill=(0, 100, 255, 220))
        d.text((s(x) + s(6), s(y) - s(6)), label, fill=(0, 80, 200, 255))

    ty = layout["art"]["textY"]
    d.line((0, s(ty), s(win_w), s(ty)), fill=(255, 165, 0, 180), width=s(1))

    OUT.parent.mkdir(parents=True, exist_ok=True)
    img.convert("RGB").save(OUT, optimize=True)
    set_dpi = subprocess.run(
        ["sips", "-s", "dpiWidth", "144", "-s", "dpiHeight", "144", str(OUT)],
        capture_output=True,
    )
    if set_dpi.returncode != 0:
        print("warn: could not set debug PNG dpi", file=sys.stderr)
    print(f"wrote {OUT}")


if __name__ == "__main__":
    main()
