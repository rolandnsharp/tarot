#!/usr/bin/env python3
"""Generate a procedural card back for the ember deck — warm gold mandala on dark."""

import math

W, H = 24, 40

def lerp(a, b, t):
    return tuple(int(a[i] + (b[i] - a[i]) * t) for i in range(3))

# Ember palette
BG = (10, 6, 4)          # near-black warm
DARK = (40, 20, 8)       # dark brown
MID = (140, 80, 20)      # warm amber
BRIGHT = (220, 150, 30)  # golden
HOT = (255, 200, 60)     # bright gold

def mandala_value(x, y):
    """Return 0-1 intensity for mandala pattern."""
    # Normalize to -1..1 centered
    cx = (x - W / 2 + 0.5) / (W / 2)
    cy = (y - H / 2 + 0.5) / (H / 2)

    # Radial distance
    r = math.sqrt(cx * cx + cy * cy)

    # Angle
    angle = math.atan2(cy, cx)

    # Symmetric petals (8-fold)
    petals = abs(math.sin(4 * angle))

    # Concentric rings
    rings = 0.5 + 0.5 * math.sin(r * 12)

    # Diamond lattice
    diamond = abs(math.sin(cx * 6)) * abs(math.sin(cy * 6))

    # Central glow
    glow = max(0, 1.0 - r * 1.8)

    # Border frame
    bx = abs(cx)
    by = abs(cy)
    frame = 1.0 if (bx > 0.85 or by > 0.92) else 0.0
    inner_frame = 0.6 if (bx > 0.7 and bx < 0.78) or (by > 0.82 and by < 0.88) else 0.0

    # Combine
    v = petals * 0.3 * rings + glow * 0.5 + diamond * 0.2 + frame * 0.7 + inner_frame * 0.4

    # Radial fade at edges
    fade = max(0, 1.0 - r * 0.6)
    v = v * fade + frame * 0.7 + inner_frame * 0.4

    return min(1.0, max(0.0, v))

lines = []
for y in range(H):
    row = []
    for x in range(W):
        v = mandala_value(x, y)

        if v < 0.15:
            c = lerp(BG, DARK, v / 0.15)
        elif v < 0.4:
            c = lerp(DARK, MID, (v - 0.15) / 0.25)
        elif v < 0.7:
            c = lerp(MID, BRIGHT, (v - 0.4) / 0.3)
        else:
            c = lerp(BRIGHT, HOT, (v - 0.7) / 0.3)

        row.append(f"{c[0]:02x}{c[1]:02x}{c[2]:02x}")
    lines.append(",".join(row))

with open("decks/ember/back.hex", "w") as f:
    f.write("\n".join(lines) + "\n")

print("Generated decks/ember/back.hex")
