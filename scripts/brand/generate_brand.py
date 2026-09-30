#!/usr/bin/env python3
"""Generates the Yuheng brand assets into website-docs/public/brand/.

    pip install fonttools
    curl -LO https://github.com/notofonts/noto-cjk/raw/main/Serif/OTF/SimplifiedChinese/NotoSerifCJKsc-SemiBold.otf
    YUHENG_BRAND_FONT=NotoSerifCJKsc-SemiBold.otf python3 scripts/brand/generate_brand.py

The lettering is Noto Serif CJK SC SemiBold (SIL Open Font License 1.1),
converted to outlines: the logo reads the same on every system, and the font
itself is not shipped.

玉衡 (Alioth) is the fifth star of the Big Dipper, where the bowl meets the
handle; 璇玑玉衡 is also the ancient instrument for measuring the heavens. The
mark draws the Dipper with 玉衡 as a jade four-pointed star at its heart:
stars joined by lines (knowledge connected), and the star that weighs and
measures (knowledge kept in balance).
"""
import os
import random
import sys

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, "..", "..", "website-docs", "public", "brand")
font = TTFont(os.environ.get("YUHENG_BRAND_FONT", "NotoSerifCJKsc-SemiBold.otf"))
gs = font.getGlyphSet()
cmap = font.getBestCmap()
upm = font["head"].unitsPerEm


def text_path(text, size, x, baseline, tracking=0.0):
    """Path data for text at font size `size`, from x on the baseline, with
    `tracking` em of extra space between letters. Returns (d, width)."""
    scale = size / upm
    pen = SVGPathPen(gs)
    cursor = 0.0
    for ch in text:
        g = gs[cmap[ord(ch)]]
        g.draw(TransformPen(pen, (scale, 0, 0, -scale, x + cursor, baseline)))
        cursor += g.width * scale + tracking * size
    return pen.getCommands(), cursor - tracking * size

os.makedirs(OUT, exist_ok=True)

NIGHT_A, NIGHT_B = "#0E1E3C", "#1B3A6A"
JADE_A, JADE_B = "#8AF5DA", "#1DB592"
STAR = "#EEF3FB"
LINE = "#8FB3E6"
GOLD = "#E5BE6B"

# The Dipper in a 256 box: bowl on the left, handle to the right.
STARS = {
    "dubhe": (52, 86, 6.6), "merak": (58, 146, 6.0), "phecda": (108, 160, 6.0),
    "megrez": (114, 106, 4.6), "alioth": (156, 96, 0), "mizar": (190, 106, 6.0), "alkaid": (216, 146, 6.2),
}

def dipper(scale=1.0, dx=0.0, dy=0.0, hero=24.0, line_w=2.6, uid="m"):
    P = {k: (x * scale + dx, y * scale + dy, r * scale) for k, (x, y, r) in STARS.items()}
    def pt(k): return f"{P[k][0]:.1f} {P[k][1]:.1f}"
    lines = (f'<path d="M{pt("dubhe")} L{pt("merak")} L{pt("phecda")} L{pt("megrez")} Z '
             f'M{pt("megrez")} L{pt("alioth")} L{pt("mizar")} L{pt("alkaid")}" fill="none" '
             f'stroke="{LINE}" stroke-opacity="0.5" stroke-width="{line_w*scale:.2f}" '
             f'stroke-linejoin="round" stroke-linecap="round"/>')
    dots = "".join(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="{r:.2f}" fill="{STAR}"/>'
                   for k, (x, y, r) in P.items() if k != "alioth")
    cx, cy, _ = P["alioth"]
    R = hero * scale
    sparkle = (f"M{cx:.1f} {cy-R:.1f} Q{cx:.1f} {cy:.1f} {cx+R:.1f} {cy:.1f} "
               f"Q{cx:.1f} {cy:.1f} {cx:.1f} {cy+R:.1f} Q{cx:.1f} {cy:.1f} {cx-R:.1f} {cy:.1f} "
               f"Q{cx:.1f} {cy:.1f} {cx:.1f} {cy-R:.1f}Z")
    defs = (f'<radialGradient id="{uid}glow" cx="50%" cy="50%" r="50%">'
            f'<stop offset="0" stop-color="{JADE_A}" stop-opacity="0.55"/>'
            f'<stop offset="1" stop-color="{JADE_A}" stop-opacity="0"/></radialGradient>'
            f'<linearGradient id="{uid}jade" x1="0" y1="0" x2="1" y2="1">'
            f'<stop offset="0" stop-color="{JADE_A}"/><stop offset="1" stop-color="{JADE_B}"/></linearGradient>')
    hero_svg = (f'<circle cx="{cx:.1f}" cy="{cy:.1f}" r="{R*1.45:.1f}" fill="url(#{uid}glow)"/>'
                f'<path d="{sparkle}" fill="url(#{uid}jade)"/>'
                f'<circle cx="{cx:.1f}" cy="{cy:.1f}" r="{R*0.13:.2f}" fill="#FFFFFF"/>')
    return defs, lines + dots + hero_svg

def tile(size=256, uid="m"):
    defs, body = dipper(uid=uid, hero=30)
    return (f'<defs>{defs}<linearGradient id="{uid}night" x1="0" y1="0" x2="1" y2="1">'
            f'<stop offset="0" stop-color="{NIGHT_A}"/><stop offset="1" stop-color="{NIGHT_B}"/></linearGradient></defs>'
            f'<rect width="256" height="256" rx="58" fill="url(#{uid}night)"/>'
            # the armillary ring of 璇玑: a whisper, not a drawing
            f'<circle cx="134" cy="124" r="98" fill="none" stroke="#FFFFFF" stroke-opacity="0.07" stroke-width="2"/>'
            f'{body}')

def svg(w, h, inner, label):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" '
            f'role="img" aria-label="{label}"><title>{label}</title>{inner}</svg>\n')

# 1. The mark.
mark = svg(256, 256, tile(), "玉衡 Yuheng")
open(f"{OUT}/yuheng-mark.svg", "w").write(mark)

# 1b. The favicon: at 16-32 px seven stars are a smudge, so the icon keeps
# only what reads at that size — 玉衡 itself, and two stars of the bowl.
fav = (f'<defs><linearGradient id="fnight" x1="0" y1="0" x2="1" y2="1">'
       f'<stop offset="0" stop-color="{NIGHT_A}"/><stop offset="1" stop-color="{NIGHT_B}"/></linearGradient>'
       f'<linearGradient id="fjade" x1="0" y1="0" x2="1" y2="1">'
       f'<stop offset="0" stop-color="{JADE_A}"/><stop offset="1" stop-color="{JADE_B}"/></linearGradient></defs>'
       f'<rect width="64" height="64" rx="14" fill="url(#fnight)"/>'
       f'<path d="M14 22 L28 34" stroke="{LINE}" stroke-opacity="0.6" stroke-width="2" stroke-linecap="round"/>'
       f'<circle cx="14" cy="22" r="3.2" fill="{STAR}"/><circle cx="16" cy="44" r="2.8" fill="{STAR}"/>'
       f'<path d="M38 12 Q38 34 60 34 Q38 34 38 56 Q38 34 16 34 Q38 34 38 12Z" fill="url(#fjade)" '
       f'transform="translate(-2 0) scale(1)"/>'
       f'<circle cx="36" cy="34" r="2.6" fill="#FFFFFF"/>')
open(f"{OUT}/yuheng-favicon.svg", "w").write(svg(64, 64, fav, "玉衡 Yuheng"))

# 2. Horizontal logos: mark + 玉衡 over YUHENG.
def logo(dark):
    ink = "#F3F6FB" if dark else "#0F2140"
    sub = "#9FB0C8" if dark else "#5A6B85"
    zh, zw = text_path("玉衡", 64, 124, 64, 0.06)
    en, ew = text_path("YUHENG", 19, 127, 100, 0.34)
    width = int(124 + max(zw, ew) + 8)
    inner = (f'<g transform="scale(0.4375)">{tile(uid="l")}</g>'
             f'<path d="{zh}" fill="{ink}"/><path d="{en}" fill="{sub}"/>')
    return svg(width, 112, inner, "玉衡 Yuheng")
open(f"{OUT}/yuheng-logo.svg", "w").write(logo(False))
open(f"{OUT}/yuheng-logo-dark.svg", "w").write(logo(True))

# 3. The banner, 1280x640: a night sky, the Dipper large on the right.
rnd = random.Random(1937)
sky = "".join(
    f'<circle cx="{rnd.uniform(0,1280):.0f}" cy="{rnd.uniform(0,640):.0f}" r="{rnd.choice([0.8,1,1.2,1.6]):.1f}" '
    f'fill="#FFFFFF" fill-opacity="{rnd.uniform(0.12,0.45):.2f}"/>' for _ in range(140))
bdefs, bdipper = dipper(scale=2.55, dx=560, dy=40, hero=26, line_w=1.6, uid="b")
zh, zw = text_path("玉衡", 132, 110, 300, 0.08)
en, ew = text_path("YUHENG", 30, 116, 356, 0.42)
t1, _ = text_path("AI 智能体时代的知识平台", 34, 112, 446, 0.04)
t2, _ = text_path("The knowledge layer for AI agents", 24, 113, 492, 0.02)
banner_inner = (
    f'<defs>{bdefs}<linearGradient id="bnight" x1="0" y1="0" x2="1" y2="1">'
    f'<stop offset="0" stop-color="#0A1630"/><stop offset="0.6" stop-color="#12284D"/>'
    f'<stop offset="1" stop-color="#1B3A6A"/></linearGradient></defs>'
    f'<rect width="1280" height="640" fill="url(#bnight)"/>{sky}'
    f'<circle cx="905" cy="320" r="265" fill="none" stroke="#FFFFFF" stroke-opacity="0.06" stroke-width="2"/>'
    f'<circle cx="905" cy="320" r="205" fill="none" stroke="#FFFFFF" stroke-opacity="0.04" stroke-width="1.5"/>'
    f'<ellipse cx="905" cy="320" rx="265" ry="92" fill="none" stroke="{GOLD}" stroke-opacity="0.18" '
    f'stroke-width="1.5" transform="rotate(-18 905 320)"/>'
    f'{bdipper}'
    f'<path d="{zh}" fill="#F3F6FB"/><path d="{en}" fill="{JADE_A}" fill-opacity="0.9"/>'
    f'<rect x="114" y="394" width="64" height="3" rx="1.5" fill="{GOLD}"/>'
    f'<path d="{t1}" fill="#DCE5F2"/><path d="{t2}" fill="#9FB0C8"/>')
open(f"{OUT}/yuheng-banner.svg", "w").write(svg(1280, 640, banner_inner, "玉衡 Yuheng — AI 智能体时代的知识平台"))
print("ok")
