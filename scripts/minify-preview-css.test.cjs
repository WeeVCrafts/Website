const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { minifyPreviewCSS } = require("./minify-preview-css.js");

test("font byte changes invalidate emitted CSS without editing maintained CSS", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "weevcrafts-font-css-"));
  try {
    const assets = path.join(root, "web", "assets");
    fs.mkdirSync(path.join(assets, "css"), { recursive: true });
    fs.mkdirSync(path.join(assets, "fonts"));
    const source = '@font-face { font-family: "Fixture"; src: url("/assets/fonts/fixture.woff2") format("woff2"); }\n';
    const sourcePath = path.join(assets, "css", "fonts.css");
    const outputPath = path.join(assets, "css", "fonts.min.css");
    const fontPath = path.join(assets, "fonts", "fixture.woff2");
    fs.writeFileSync(sourcePath, source);
    fs.writeFileSync(fontPath, "abc");

    minifyPreviewCSS(root, ["fonts.css"]);
    const first = fs.readFileSync(outputPath, "utf8");
    // SHA-256("abc") begins ba7816bf8f01cfea. This is the same first-eight-byte
    // lowercase hex contract used by web/assets.URL for template font preloads.
    assert.match(first, /\/assets\/fonts\/fixture\.woff2\?v=ba7816bf8f01cfea/);

    fs.writeFileSync(fontPath, "abcd");
    minifyPreviewCSS(root, ["fonts.css"]);
    const second = fs.readFileSync(outputPath, "utf8");
    assert.match(second, /\/assets\/fonts\/fixture\.woff2\?v=88d4266fd4e6338d/);
    assert.notEqual(second, first, "CSS asset content must change with its font dependency");
    assert.equal(fs.readFileSync(sourcePath, "utf8"), source);
    minifyPreviewCSS(root, ["fonts.css"]);
    assert.equal(fs.readFileSync(outputPath, "utf8"), second, "unchanged fonts build deterministically");
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
