"""Generate web font artifacts from the maintained local TTF sources.

Requires fontTools and Brotli only at build time. Source fonts are never modified.
The icon subset keeps the ligatures referenced by current presentation sources.
"""
from pathlib import Path
import re
from fontTools.ttLib import TTFont
from fontTools import subset

root = Path(__file__).resolve().parents[1]
fonts = root / "web" / "assets" / "fonts"
for source in fonts.glob("*.ttf"):
    if source.name == "material-symbols-outlined.ttf":
        continue
    font = TTFont(source, recalcTimestamp=False)
    font.flavor = "woff2"
    target = source.with_suffix(".woff2")
    font.save(target)
    print(f"{source.name}: {source.stat().st_size} -> {target.stat().st_size} bytes")

# Fast path for common interface text. Full variable fonts above remain the
# fallback, preserving every original supported character outside these ranges.
latin_ranges = ((0, 0x024F), (0x1E00, 0x1EFF), (0x2000, 0x22FF))
for family in ("inter", "roboto"):
    font = TTFont(fonts / f"{family}-variable.ttf", recalcTimestamp=False)
    cmap = font.getBestCmap()
    required = {ord(character) for character in "₹0123456789"}
    if not required.issubset(cmap):
        raise RuntimeError(f"{family}: required currency/numeral coverage is missing")
    options = subset.Options()
    options.layout_features = ["*"]
    subsetter = subset.Subsetter(options=options)
    subsetter.populate(unicodes=[code for code in cmap if any(start <= code <= end for start, end in latin_ranges)])
    subsetter.subset(font)
    if not required.issubset(font.getBestCmap()):
        raise RuntimeError(f"{family}: subsetting lost currency/numerals")
    font.flavor = "woff2"
    target = fonts / f"{family}-latin.woff2"
    font.save(target)
    print(f"{family} interface fast path: {target.stat().st_size} bytes; full fallback retained")

tokens = set()
for directory in (root / "web", root / "internal" / "web"):
    for source in directory.rglob("*"):
        if source.suffix in (".templ", ".go", ".js"):
            tokens.update(re.findall(r"\b[a-z][a-z0-9_]+\b", source.read_text(encoding="utf-8-sig")))
font = TTFont(fonts / "material-symbols-outlined.ttf", recalcTimestamp=False)
cmap = font.getBestCmap()
ascii_names = {glyph: chr(code) for code, glyph in cmap.items() if 32 <= code <= 126}
glyphs = {cmap[code] for code in range(32, 127) if code in cmap}
kept = set()
for lookup in font["GSUB"].table.LookupList.Lookup:
    for table in lookup.SubTable:
        if lookup.LookupType == 7:
            table = table.ExtSubTable
        if not hasattr(table, "ligatures"):
            continue
        filtered = {}
        for first, ligatures in table.ligatures.items():
            selected = []
            for ligature in ligatures:
                name = "".join(ascii_names.get(component, "") for component in [first, *ligature.Component])
                if name in tokens:
                    selected.append(ligature)
                    glyphs.add(ligature.LigGlyph)
                    kept.add(name)
            if selected:
                filtered[first] = selected
        table.ligatures = filtered
if not kept:
    raise RuntimeError("No icon ligatures found. Do not publish an empty font.")
options = subset.Options()
options.layout_features = ["liga", "rlig"]
subsetter = subset.Subsetter(options=options)
subsetter.populate(glyphs=glyphs)
subsetter.subset(font)
font.flavor = "woff2"
target = fonts / "material-symbols-ui.woff2"
font.save(target)
print(f"Icon subset: {len(kept)} ligatures, {target.stat().st_size} bytes")
