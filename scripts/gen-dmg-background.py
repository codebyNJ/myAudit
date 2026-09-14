#!/usr/bin/env python3
"""Regenerate desktop/dmg-background.png (@2x) for the macOS installer DMG.

Window size in repack-dmg.sh is 660×400; this image is 1320×800 so create-dmg
renders a retina background. Finder draws icon labels — only paint the arrow
and install hint here.
"""
from __future__ import annotations

from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont

OUT = Path(__file__).resolve().parents[1] / "desktop" / "dmg-background.png"
W, H = 1320, 800  # @2x for 660×400 Finder window


def load_font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    candidates = [
        "/System/Library/Fonts/SFNS.ttf",
        "/System/Library/Fonts/Supplemental/SF-Pro-Text-Bold.otf"
        if bold
        else "/System/Library/Fonts/Supplemental/SF-Pro-Text-Regular.otf",
        "/System/Library/Fonts/Helvetica.ttc",
        "/Library/Fonts/Arial.ttf",
    ]
    for path in candidates:
        try:
            return ImageFont.truetype(path, size)
        except OSError:
            continue
    return ImageFont.load_default()


def lerp(a: int, b: int, t: float) -> int:
    return int(a + (b - a) * t)


def vertical_gradient(size: tuple[int, int], top: tuple[int, int, int], bottom: tuple[int, int, int]) -> Image.Image:
    w, h = size
    img = Image.new("RGB", size)
    px = img.load()
    for y in range(h):
        t = y / max(h - 1, 1)
        row = (lerp(top[0], bottom[0], t), lerp(top[1], bottom[1], t), lerp(top[2], bottom[2], t))
        for x in range(w):
            px[x, y] = row
    return img


def rounded_panel(draw: ImageDraw.ImageDraw, box: tuple[int, int, int, int], radius: int, fill: str, outline: str | None = None) -> None:
    draw.rounded_rectangle(box, radius=radius, fill=fill, outline=outline, width=2 if outline else 0)


def main() -> None:
    # Dark, macOS-adjacent palette aligned with the app chrome.
    base = vertical_gradient((W, H), (18, 18, 24), (10, 10, 14))
    d = ImageDraw.Draw(base)

    margin = 56
    panel = (margin, margin, W - margin, H - margin)
    rounded_panel(d, panel, 36, "#1a1a22", "#2d2d3a")

    # Soft inner glow along the top edge of the panel.
    glow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    gd = ImageDraw.Draw(glow)
    gd.rounded_rectangle((margin + 4, margin + 4, W - margin - 4, margin + 140), radius=32, fill=(255, 255, 255, 10))
    base = Image.alpha_composite(base.convert("RGBA"), glow).convert("RGB")
    d = ImageDraw.Draw(base)

    # Icon drop zones (subtle rings — Finder icons sit on top).
    for cx in (340, 980):
        d.ellipse((cx - 118, 302, cx + 118, 538), outline="#2a2a36", width=2)

    # Arrow: myAudit (left) → Applications (right), @2x coords.
    ax, ay = W // 2, 430
    shaft_left, shaft_right = ax - 200, ax + 200
    d.rounded_rectangle((shaft_left, ay - 14, shaft_right, ay + 14), radius=12, fill="#3b3b4d")

    arrow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    ad = ImageDraw.Draw(arrow)
    tip = (ax + 210, ay)
    head = [
        tip,
        (ax + 70, ay - 52),
        (ax + 70, ay - 18),
        (ax - 30, ay - 18),
        (ax - 30, ay + 18),
        (ax + 70, ay + 18),
        (ax + 70, ay + 52),
    ]
    shadow = [(x + 6, y + 10) for x, y in head]
    ad.polygon(shadow, fill=(0, 0, 0, 70))
    ad.polygon(head, fill="#a78bfa")
    ad.polygon(head, outline="#c4b5fd", width=2)
    arrow = arrow.filter(ImageFilter.GaussianBlur(radius=1))
    base = Image.alpha_composite(base.convert("RGBA"), arrow).convert("RGB")
    d = ImageDraw.Draw(base)

    title_font = load_font(34, bold=True)
    hint_font = load_font(26)
    sub_font = load_font(22)

    title = "Install myAudit"
    hint = "Drag to the Applications folder"
    sub = "Then open from Launchpad or Spotlight"

    def center_text(text: str, y: int, font: ImageFont.ImageFont, fill: str) -> None:
        bbox = d.textbbox((0, 0), text, font=font)
        tw = bbox[2] - bbox[0]
        d.text(((W - tw) // 2, y), text, font=font, fill=fill)

    center_text(title, 108, title_font, "#f4f4f5")
    center_text(hint, 620, hint_font, "#e4e4e7")
    center_text(sub, 666, sub_font, "#a1a1aa")

    OUT.parent.mkdir(parents=True, exist_ok=True)
    base.save(OUT, optimize=True)
    print(f"wrote {OUT} ({W}×{H})")


if __name__ == "__main__":
    main()
