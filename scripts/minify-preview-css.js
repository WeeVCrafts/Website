const fs = require("fs");
const path = require("path");
const CleanCSS = require("clean-css");
const crypto = require("node:crypto");

const root = path.resolve(__dirname, "..");
const files = [
  "fonts.css",
  "brand.css",
  "auth-gallery.css",
  "ui-standards.css",
  "theme.css",
  "utility.css",
  "customer-polish.css",
  "experience.css",
  "storefront-reference.css",
  "admin.css",
  "seller-admin.css",
  "support-portal.css",
];

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
