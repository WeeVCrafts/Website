const http = require("http");
const server = http.createServer((req, res) => {
  const upstream = http.request(
    { host: "127.0.0.1", port: 8080, path: req.url, method: req.method, headers: { ...req.headers, host: "localhost:8080" } },
    (r) => {
      res.writeHead(r.statusCode, r.headers);
      r.pipe(res);
    }
  );
  upstream.on("error", () => {
    res.writeHead(502);
    res.end("upstream unavailable");
  });
  req.pipe(upstream);
});
server.listen(8081, () => console.log("preview-proxy pid=" + process.pid));
