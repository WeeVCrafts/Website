const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');

test('INT-014--qa-and-technical-operator', async ({ request }, testInfo) => {
  const target = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
  test.skip(process.env.WEEVCRAFTS_QA_DATABASE !== 'weev_qa_20261009_browser' || target.hostname !== '127.0.0.1' || target.port !== '18080', 'Requires the completed disposable cross-role QA sale and local SMTP mailbox.');
  const query = "SELECT json_build_object('order_number',o.order_number,'email',u.email,'carrier',sf.carrier,'tracking_number',sf.tracking_number) FROM orders o JOIN users u ON u.id=o.user_id JOIN order_items oi ON oi.order_id=o.id JOIN product_variants pv ON pv.id=oi.variant_id JOIN products p ON p.id=pv.product_id JOIN seller_fulfillments sf ON sf.order_id=o.id WHERE p.slug LIKE 'browser-qa-craft-%' ORDER BY o.id DESC LIMIT 1";
  const raw = execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'wecratfs', '-d', 'weev_qa_20261009_browser', '-tAc', query], { encoding: 'utf8' }).trim();
  expect(!!raw, 'Completed cross-role QA order fixture is required').toBe(true);
  const order = JSON.parse(raw);
  testInfo.annotations.push({ type: 'account', description: `customer=${order.email}; order_number=${order.order_number}` });
  const mailbox = await (await request.get('http://127.0.0.1:8025/api/v1/messages?limit=500')).json();
  for (const subject of ['Your Weevcrafts order has been received', 'Your Weevcrafts order has shipped']) {
    const candidates = (mailbox.messages || []).filter(message => message.Subject === subject && (message.To || []).some(recipient => recipient.Address === order.email));
    let found = false;
    for (const candidate of candidates) {
      const detail = await (await request.get(`http://127.0.0.1:8025/api/v1/message/${candidate.ID}`)).json();
      const body = `${detail.Text || ''}\n${detail.HTML || ''}`;
      if (!body.includes(order.order_number)) continue;
      expect((detail.To || candidate.To).map(recipient => recipient.Address)).toEqual([order.email]);
      expect(body).toContain(`http://127.0.0.1:18080/orders/${order.order_number}`);
      if (subject.includes('shipped')) {
        expect(body).toContain(order.carrier);
        expect(body).toContain(order.tracking_number);
      }
      testInfo.annotations.push({ type: 'account', description: `mailpit_message=${candidate.ID}; subject=${subject}` });
      found = true;
      break;
    }
    expect(found, `Missing persisted SMTP message: ${subject}`).toBe(true);
  }
});
