// Local protocol fixture. No payment credentials or external network calls.
const http = require('node:http');
let sequence = 0;
http.createServer(async (request, response) => {
  let body = '';
  for await (const chunk of request) body += chunk;
  response.setHeader('Content-Type','application/json');
  if (request.method === 'POST' && request.url === '/v1/orders') {
    const input = JSON.parse(body);
    response.end(JSON.stringify({id:'order_discovery_'+(++sequence),amount:input.amount,currency:input.currency,status:'created'}));
  } else { response.statusCode=404; response.end('{}'); }
}).listen(18088,'127.0.0.1');
