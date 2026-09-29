"""Build ten self-contained Weevcrafts SVG logo concepts.

Requires fonttools (`python -m pip install fonttools`). The final SVGs contain
outlined lettering and do not depend on a font being installed by the viewer.
"""

from pathlib import Path
from xml.sax.saxutils import escape

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.ttLib import TTFont


ROOT = Path(__file__).resolve().parents[2]
OUT = Path(__file__).resolve().parent
FONTS = {
    "serif": ROOT / "web/assets/fonts/playfair-display-600.ttf",
    "sans": ROOT / "web/assets/fonts/dm-sans-700.ttf",
}
INK = "#153f35"
CLAY = "#bc6048"
CREAM = "#fff9ef"
GOLD = "#d9a34d"
NAVY = "#273b52"


def lettering(font_key, color, x=131, baseline=78, size=43, tracking=0):
    font = TTFont(FONTS[font_key])
    glyphs = font.getGlyphSet()
    cmap = font.getBestCmap()
    units = font["head"].unitsPerEm
    scale = size / units
    advance = 0
    paths = []
    for char in "Weevcrafts":
        name = cmap[ord(char)]
        pen = SVGPathPen(glyphs)
        glyphs[name].draw(pen)
        paths.append(f'<path d="{pen.getCommands()}" transform="translate({advance} 0)"/>')
        advance += font["hmtx"][name][0] + tracking / scale
    font.close()
    return (f'<g fill="{color}" transform="translate({x} {baseline}) '
            f'scale({scale:.6f} {-scale:.6f})">' + "".join(paths) + "</g>")


def logo(filename, title, description, mark, font="serif", ink=INK, extras=""):
    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 480 120" role="img" aria-labelledby="title desc">
<title id="title">{escape(title)} — Weevcrafts</title>
<desc id="desc">{escape(description)}</desc>
<style>
  .pulse,.turn,.lift {{ transform-box:fill-box; transform-origin:center; transition:transform .35s ease,opacity .35s ease; }}
  svg:hover .turn {{ transform:rotate(12deg); }}
  svg:hover .lift {{ transform:translateY(-3px); }}
  svg:hover .pulse {{ transform:scale(1.08); }}
  @media (prefers-reduced-motion:reduce) {{ .pulse,.turn,.lift {{ transition:none; }} }}
</style>
{mark}
{lettering(font, ink)}
{extras}
</svg>'''
    (OUT / filename).write_text(svg, encoding="utf-8")


logo("01-woven-w.svg", "Woven W", "Interlaced threads form a W inside a soft square", f'''
<rect x="12" y="12" width="96" height="96" rx="26" fill="{INK}"/>
<g class="lift" fill="none" stroke="{CREAM}" stroke-width="7" stroke-linecap="round" stroke-linejoin="round">
 <path d="M29 38 42 83 60 52 78 83 91 38"/>
</g>
<g fill="none" stroke="{CLAY}" stroke-width="5" stroke-linecap="round">
 <path d="M38 49c7-6 13-6 20 0M62 70c7-6 13-6 20 0"/>
</g>
<circle cx="60" cy="29" r="4" fill="{GOLD}"/>
''')

logo("02-petal-wheel.svg", "Petal Wheel", "Eight petals radiate around a woven center", f'''
<g class="turn" fill="none" stroke="{CLAY}" stroke-width="3.2">
 <ellipse cx="60" cy="27" rx="10" ry="21"/>
 <ellipse cx="60" cy="27" rx="10" ry="21" transform="rotate(45 60 60)"/>
 <ellipse cx="60" cy="27" rx="10" ry="21" transform="rotate(90 60 60)"/>
 <ellipse cx="60" cy="27" rx="10" ry="21" transform="rotate(135 60 60)"/>
 <ellipse cx="60" cy="27" rx="10" ry="21" transform="rotate(180 60 60)"/>
 <ellipse cx="60" cy="27" rx="10" ry="21" transform="rotate(225 60 60)"/>
 <ellipse cx="60" cy="27" rx="10" ry="21" transform="rotate(270 60 60)"/>
 <ellipse cx="60" cy="27" rx="10" ry="21" transform="rotate(315 60 60)"/>
</g>
<circle cx="60" cy="60" r="24" fill="{CREAM}" stroke="{INK}" stroke-width="3"/>
<path d="m46 50 7 21 7-14 7 14 7-21" fill="none" stroke="{INK}" stroke-width="4" stroke-linejoin="round" stroke-linecap="round"/>
''', ink=NAVY)

logo("03-makers-seal.svg", "Maker's Seal", "A circular craft seal with a precise monogram", f'''
<circle cx="60" cy="60" r="47" fill="{INK}"/>
<circle cx="60" cy="60" r="40" fill="none" stroke="{GOLD}" stroke-width="1.6"/>
<circle cx="60" cy="60" r="33" fill="none" stroke="{CREAM}" stroke-width=".8" opacity=".8"/>
<path class="pulse" d="m35 45 11 31 14-25 14 25 11-31" fill="none" stroke="{CREAM}" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/>
<circle cx="60" cy="33" r="3" fill="{GOLD}"/><circle cx="25" cy="60" r="2" fill="{GOLD}"/><circle cx="95" cy="60" r="2" fill="{GOLD}"/>
''', ink=INK)

logo("04-endless-thread.svg", "Endless Thread", "A flowing single thread draws the initials and a needle loop", f'''
<path class="lift" d="M17 38c12-19 23 1 19 22-3 18 2 29 13 27 14-2 9-39 20-40 11-1 3 37 16 37 15 0 20-30 20-48" fill="none" stroke="{CLAY}" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"/>
<ellipse cx="105" cy="28" rx="5" ry="9" fill="none" stroke="{INK}" stroke-width="3" transform="rotate(19 105 28)"/>
<circle cx="21" cy="35" r="4" fill="{GOLD}"/>
''', font="sans", ink=NAVY)

logo("05-artisan-arch.svg", "Artisan Arch", "An architectural arch shelters a woven W", f'''
<path d="M21 101V54c0-26 17-41 39-41s39 15 39 41v47Z" fill="{CREAM}" stroke="{INK}" stroke-width="4"/>
<path d="M31 99V55c0-19 12-31 29-31s29 12 29 31v44" fill="none" stroke="{GOLD}" stroke-width="2"/>
<path class="lift" d="m35 52 11 29 14-20 14 20 11-29" fill="none" stroke="{INK}" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/>
<path d="M46 43h28M60 27v11" stroke="{CLAY}" stroke-width="3" stroke-linecap="round"/>
''')

logo("06-patchwork.svg", "Patchwork", "Four textile tiles meet in a balanced modular mark", f'''
<rect x="15" y="15" width="43" height="43" rx="8" fill="{INK}"/>
<rect x="62" y="15" width="43" height="43" rx="8" fill="{CLAY}"/>
<rect x="15" y="62" width="43" height="43" rx="8" fill="{GOLD}"/>
<rect x="62" y="62" width="43" height="43" rx="8" fill="{NAVY}"/>
<g class="pulse" fill="none" stroke="{CREAM}" stroke-width="4" stroke-linecap="round" stroke-linejoin="round">
 <path d="m25 28 9 20 8-12 7 12 5-20"/>
 <path d="M70 48 96 23M70 23l26 25"/>
 <path d="M25 84h25M37 70v28"/>
 <path d="M72 91c8-21 14-21 22 0"/>
</g>
''', font="sans", ink=INK)

logo("07-sprout-loom.svg", "Sprout Loom", "A young sprout rises from woven threads", f'''
<path d="M19 93h83" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>
<path d="m23 50 13 36 20-28 19 28 20-36" fill="none" stroke="{INK}" stroke-width="6" stroke-linejoin="round" stroke-linecap="round"/>
<path class="lift" d="M59 55V28" fill="none" stroke="{CLAY}" stroke-width="3.5" stroke-linecap="round"/>
<path d="M57 37C40 39 37 27 40 18c11-1 18 6 17 19ZM61 37c17 2 22-9 19-19-12-1-19 6-19 19Z" fill="{CLAY}"/>
<path d="M40 91c13-7 27-7 40 0" fill="none" stroke="{GOLD}" stroke-width="3"/>
''', ink=INK)

logo("08-craft-compass.svg", "Craft Compass", "A diamond compass suggests discovery of handmade goods", f'''
<path d="M60 12 108 60 60 108 12 60Z" fill="none" stroke="{INK}" stroke-width="3"/>
<path class="turn" d="M60 23 97 60 60 97 23 60Z" fill="{CREAM}" stroke="{GOLD}" stroke-width="2"/>
<path d="m60 30 7 23 23 7-23 7-7 23-7-23-23-7 23-7Z" fill="{INK}"/>
<path d="m50 55 5 14 5-9 5 9 5-14" fill="none" stroke="{CREAM}" stroke-width="3.4" stroke-linecap="round" stroke-linejoin="round"/>
<circle cx="60" cy="12" r="3" fill="{CLAY}"/><circle cx="108" cy="60" r="3" fill="{CLAY}"/><circle cx="60" cy="108" r="3" fill="{CLAY}"/><circle cx="12" cy="60" r="3" fill="{CLAY}"/>
''', font="sans", ink=NAVY)

logo("09-stitch-script.svg", "Stitch Script", "Dotted embroidery stitches trace a bold W and needle", f'''
<rect x="13" y="13" width="94" height="94" rx="47" fill="{CREAM}" stroke="{CLAY}" stroke-width="2"/>
<path class="lift" d="m26 40 11 40 23-27 22 27 12-40" fill="none" stroke="{INK}" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" stroke-dasharray="6 5"/>
<path d="m88 28 10 17" stroke="{CLAY}" stroke-width="3" stroke-linecap="round"/>
<ellipse cx="85" cy="24" rx="4" ry="7" transform="rotate(-30 85 24)" fill="none" stroke="{CLAY}" stroke-width="2.5"/>
<path d="M31 93c20 8 36 8 56 0" fill="none" stroke="{GOLD}" stroke-width="2.5" stroke-linecap="round"/>
''', font="sans", ink=INK)

logo("10-unity-knot.svg", "Unity Knot", "Linked loops form a minimal and enduring craft mark", f'''
<g class="pulse" fill="none" stroke-width="9" stroke-linecap="round" stroke-linejoin="round">
 <path d="M20 34c6 31 12 51 22 51 11 0 11-35 18-35" stroke="{INK}"/>
 <path d="M60 50c7 0 7 35 18 35 10 0 16-20 22-51" stroke="{CLAY}"/>
</g>
<path d="M54 50h12" fill="none" stroke="{GOLD}" stroke-width="5" stroke-linecap="round"/>
<circle cx="60" cy="28" r="3" fill="{INK}"/>
''', ink=NAVY)

print("Created ten outlined SVG logos in", OUT)
