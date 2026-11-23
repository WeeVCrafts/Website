const fs = require("fs");
const path = require("path");
const CleanCSS = require("clean-css");
const crypto = require("node:crypto");

const previewFiles = [
  "fonts.css",
  "brand.css",
  "auth-gallery.css",
  "customer-header.css",
  "ui-standards.css",
  "theme.css",
  "utility.css",
  "customer-polish.css",
  "experience.css",
  "storefront-reference.css",
  "homepage-reference.css",
  "admin.css",
  "superadmin.css",
  "fulfillment-guided.css",
  "storefront-editor.css",
  "seller-admin.css",
  "seller-reference.css",
  "support-portal.css",
];

const storefrontBundleFiles = [
  "theme.css",
  "utility.css",
  "customer-polish.css",
  "ui-standards.css",
  "experience.css",
  "storefront-reference.css",
  "brand.css",
  "auth-gallery.css",
  "customer-header.css",
];

function minifyPreviewCSS(root = path.resolve(__dirname, ".."), files = previewFiles) {
  for (const file of files) {
    const inputPath = path.join(root, "web", "assets", "css", file);
    const outputPath = path.join(root, "web", "assets", "css", file.replace(/\.css$/, ".min.css"));
    const source = fs.readFileSync(inputPath, "utf8").replace(/url\(["']?(\/assets\/fonts\/[^"')]+\.woff2)["']?\)/g, (_, url) => {
      const bytes=fs.readFileSync(path.join(root,"web","assets",url.slice('/assets/'.length)));
      const version=crypto.createHash('sha256').update(bytes).digest('hex').slice(0,16);
      return `url("${url}?v=${version}")`;
    });
    const result = new CleanCSS({ level: 2 }).minify(source);
    if (result.errors.length > 0) {
      throw new Error(`${file}: ${result.errors.join("; ")}`);
    }
    const minified = `${result.styles}\n`;
    if (!fs.existsSync(outputPath) || fs.readFileSync(outputPath, "utf8") !== minified) fs.writeFileSync(outputPath, minified);
  }

  // Generate consolidated storefront CSS bundle
  let bundleSource = "";
  for (const file of storefrontBundleFiles) {
    const inputPath = path.join(root, "web", "assets", "css", file);
    if (fs.existsSync(inputPath)) {
      bundleSource += fs.readFileSync(inputPath, "utf8") + "\n";
    }
  }
  const bundleResult = new CleanCSS({ level: 2 }).minify(bundleSource);
  if (bundleResult.errors.length > 0) {
    throw new Error(`storefront-bundle: ${bundleResult.errors.join("; ")}`);
  }
  const bundleOutputPath = path.join(root, "web", "assets", "css", "storefront-bundle.min.css");
  const bundleMinified = `${bundleResult.styles}\n`;
  if (!fs.existsSync(bundleOutputPath) || fs.readFileSync(bundleOutputPath, "utf8") !== bundleMinified) {
    fs.writeFileSync(bundleOutputPath, bundleMinified);
  }
}

if (require.main === module) minifyPreviewCSS();

module.exports = { minifyPreviewCSS };
