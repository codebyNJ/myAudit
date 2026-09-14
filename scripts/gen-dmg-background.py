#!/usr/bin/env python3
"""Regenerate desktop/dmg-background.png for the macOS installer DMG."""
from pathlib import Path

from PIL import Image, ImageDraw

OUT = Path(__file__).resolve().parents[1] / "desktop" / "dmg-background.png"
W, H = 660, 400

img = Image.new("RGB", (W, H), "#14141a")
d = ImageDraw.Draw(img)
d.rounded_rectangle([20, 20, W - 20, H - 20], radius=16, fill="#1c1c24", outline="#2e2e3a")
ax, ay = 330, 210
# Arrow points left → right (myAudit → Applications), matching icon positions.
d.polygon(
    [
        (ax + 90, ay),
        (ax - 40, ay - 28),
        (ax - 40, ay - 10),
        (ax - 110, ay - 10),
        (ax - 110, ay + 10),
        (ax - 40, ay + 10),
        (ax - 40, ay + 28),
    ],
    fill="#8b5cf6",
)
for text, x, y in [("myAudit", 170, 320), ("Applications", 490, 320)]:
    d.text((x - 8 * len(text) // 2, y), text, fill="#d4d4d8")
d.text((330 - 95, 360), "Drag to install", fill="#a1a1aa")
OUT.parent.mkdir(parents=True, exist_ok=True)
img.save(OUT)
print(f"wrote {OUT}")
