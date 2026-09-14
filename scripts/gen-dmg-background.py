#!/usr/bin/env python3
"""Regenerate desktop/dmg-background.png — Docker-style DMG art.

Pure white 1080×760 @ 144 DPI. Finder draws the icons; we only paint the
centred "drag and drop" label and the U-curve scribble arrow between them.
"""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[1]
LAYOUT_PATH = ROOT / "scripts" / "dmg-layout.json"
OUT = ROOT / "desktop" / "dmg-background.png"
SCALE = 2


def load_layout() -> dict:
    with open(LAYOUT_PATH, encoding="utf-8") as f:
        return json.load(f)


def icon_box(icons: dict, side: str) -> tuple[float, float, float, float]:
    cx, cy = icons[side]
    half = icons["size"] / 2
    return cx - half, cy - half, cx + half, cy + half


def validate_layout(layout: dict) -> None:
    icons = layout["icons"]
    art = layout["art"]
    arrow = layout["arrow"]

    left_box = icon_box(icons, "left")
    right_box = icon_box(icons, "right")
    gap_left, gap_right = left_box[2], right_box[0]
    if gap_right <= gap_left:
        raise ValueError("icons overlap — no gap for label/arrow")

    text_y = art["textY"]
    icon_cy = icons["left"][1]
    if abs(text_y - icon_cy) > icons["size"] / 2:
        raise ValueError("textY should align with the icon row (Docker style)")

    label_bottom = icon_cy + icons["size"] / 2 + icons.get("textSize", 12) + 2
    if arrow["control"][1] < label_bottom:
        raise ValueError("arrow curve must dip below icon labels (Docker U-shape)")

    for pt in [arrow["start"], arrow["control"], arrow["end"]]:
        x, y = pt
        if not (gap_left <= x <= gap_right):
            raise ValueError(f"arrow ({x},{y}) must stay in the centre gap")


def hex_rgb(h: str) -> tuple[int, int, int]:
    h = h.lstrip("#")
    return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)


def s(n: float) -> int:
    return int(n * SCALE)


def load_font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    for path in (
        "/System/Library/Fonts/SFNSRounded.ttf",
        "/System/Library/Fonts/Supplemental/SF-Pro-Rounded-Medium.otf",
        "/System/Library/Fonts/Supplemental/SF-Pro-Text-Medium.otf",
        "/System/Library/Fonts/SFNS.ttf",
    ):
        try:
            return ImageFont.truetype(path, s(size))
        except OSError:
            continue
    return ImageFont.load_default()


def quad_bezier(
    p0: tuple[float, float],
    p1: tuple[float, float],
    p2: tuple[float, float],
    n: int = 100,
) -> list[tuple[float, float]]:
    pts: list[tuple[float, float]] = []
    for i in range(n + 1):
        t = i / n
        u = 1 - t
        x = u * u * p0[0] + 2 * u * t * p1[0] + t * t * p2[0]
        y = u * u * p0[1] + 2 * u * t * p1[1] + t * t * p2[1]
        pts.append((x, y))
    return pts


def stamp_stroke(
    d: ImageDraw.ImageDraw,
    path: list[tuple[float, float]],
    radius: int,
    fill: tuple[int, int, int],
) -> None:
    for x, y in path:
        r = radius
        d.ellipse((s(x) - r, s(y) - r, s(x) + r, s(y) + r), fill=fill)
    for i in range(len(path) - 1):
        d.line(
            [(s(path[i][0]), s(path[i][1])), (s(path[i + 1][0]), s(path[i + 1][1]))],
            fill=fill,
            width=radius * 2,
        )


def draw_arrow(d: ImageDraw.ImageDraw, layout: dict, navy: tuple[int, int, int]) -> None:
    a = layout["arrow"]
    stroke = s(a.get("strokePt", 3.5) / 2)
    path = quad_bezier(tuple(a["start"]), tuple(a["control"]), tuple(a["end"]))
    stamp_stroke(d, path, max(stroke, 2), navy)
    d.polygon([(s(x), s(y)) for x, y in a["head"]], fill=navy)


def draw_label(d: ImageDraw.ImageDraw, layout: dict, win_w: int) -> None:
    art = layout["art"]
    navy = hex_rgb(art["navy"])
    font = load_font(art["fontPt"])
    d.text(
        (s(win_w / 2), s(art["textY"])),
        "Drag and Drop",
        font=font,
        fill=navy,
        anchor="mm",
    )


def set_dpi_144(path: Path) -> None:
    subprocess.run(
        ["sips", "-s", "dpiWidth", "144", "-s", "dpiHeight", "144", str(path)],
        check=True,
        capture_output=True,
    )


def generate(layout: dict) -> Path:
    validate_layout(layout)
    win_w = layout["window"]["width"]
    win_h = layout["window"]["height"]
    navy = hex_rgb(layout["art"]["navy"])

    img = Image.new("RGB", (win_w * SCALE, win_h * SCALE), (255, 255, 255))
    d = ImageDraw.Draw(img)
    draw_label(d, layout, win_w)
    draw_arrow(d, layout, navy)

    OUT.parent.mkdir(parents=True, exist_ok=True)
    img.save(OUT, optimize=True)
    set_dpi_144(OUT)
    return OUT


def main() -> None:
    layout = load_layout()
    out = generate(layout)
    dpi = subprocess.run(
        ["sips", "-g", "dpiWidth", "-g", "pixelWidth", "-g", "pixelHeight", str(out)],
        capture_output=True,
        text=True,
        check=True,
    )
    win = layout["window"]
    left, right = layout["icons"]["left"], layout["icons"]["right"]
    print(f"wrote {out} ({win['width'] * SCALE}×{win['height'] * SCALE} @ 144dpi)")
    print(dpi.stdout.strip())
    print(f"icons {layout['icons']['size']}pt @ ({left[0]},{left[1]}) ({right[0]},{right[1]})")


if __name__ == "__main__":
    main()
