// Responsive design studies made from the running UI and its real data.
// This changes only a private browser DOM, never the application or database.
const { chromium } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');
const output = path.resolve('tmp/authenticated-ui-audit/designs');
const studies = [
  ['customer', 'customer01', '/account'], ['catalogue', 'customer01', '/products'],
  ['seller', 'owner01', '/seller-admin'], ['admin', 'marketplace01', '/admin'],
  ['superadmin', 'super01', '/super-admin'], ['categories', 'super01', '/super-admin/category-management'],
];
const css = `
.storefront-body .listing-banner { display:block; }
.storefront-body .site-footer .footer-grid > div > a:not(.wordmark) { min-height:32px; }
.task-guidance-action {white-space:normal;text-align:center;max-width:100%;}
.ui-wrap-action {max-width:100%;white-space:normal;text-align:center;overflow-wrap:anywhere;}
body:is(.seller-body,.admin-body) .ui-table .record-title a {text-decoration:underline;text-underline-offset:3px;}
@media(max-width:800px){.storefront-body .listing-banner{padding-top:100px;}.storefront-body .listing-banner-art{height:100px;}.storefront-body .listing-banner-copy{width:calc(100% - 32px);padding:16px 0;}}
@media(max-width:720px){.task-guidance-action{grid-column:1/-1;justify-self:stretch;}}
`;
async function main() {
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch();
  try {
    for (const [name, account, route] of studies) {
      const context = await browser.newContext({baseURL:'http://127.0.0.1:18080',reducedMotion:'reduce'});
      const login = await context.request.post('/api/v1/auth/login',{data:{email:`demo.${account}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}});
      if (!login.ok()) throw new Error(`Login failed: ${name}`);
      const page = await context.newPage(); await page.goto(route);
      await page.addStyleTag({content:css});
      await page.evaluate(name => {
        function guidance(parent, eyebrow, title, body, href, label) {
          const template = document.createElement('template');
          template.innerHTML = '<aside class="task-guidance" aria-label="Recommended next action"><span class="task-guidance-icon material-symbols-outlined" aria-hidden="true">arrow_forward</span><div class="task-guidance-copy"><p class="task-guidance-eyebrow"></p><h2></h2><p></p></div><a class="task-guidance-action"><span></span><span class="material-symbols-outlined" aria-hidden="true">arrow_forward</span></a></aside>';
          const card=template.content.firstElementChild;
          card.querySelector('.task-guidance-eyebrow').textContent=eyebrow;
          card.querySelector('h2').textContent=title;card.querySelector('.task-guidance-copy > p:last-child').textContent=body;
          card.querySelector('a').href=href;card.querySelector('a > span').textContent=label;
          parent.prepend(card);
        }
        if (name==='customer') guidance(document.querySelector('.account-content'),'YOUR NEXT STEP','Follow your latest order','View delivery progress, payment details and order help in one place.',document.querySelector('.order-history-row a,.account-order-priority a[href^="/orders/"]')?.href||'/orders','View latest order');
        if (name==='seller') guidance(document.querySelector('#seller-main'),'SELLING WORKFLOW','Keep your shop moving','Save a product draft, submit it for review, then manage stock and fulfil orders from your workspace.','/seller-admin/orders','Open fulfilment');
        if(name==='superadmin') {
          const card=[...document.querySelectorAll('.admin-panel')].find(el=>el.textContent.includes('Identity and access'));
          if(card){card.querySelector('a').textContent='Manage accounts';card.querySelector('a').href='/super-admin/users';const a=card.querySelector('a').cloneNode();a.textContent='Assign roles';a.href='/super-admin/roles';a.className='admin-button secondary';const nav=document.createElement('nav');nav.className='ui-actions';nav.setAttribute('aria-label','Identity and access');card.querySelector('a').replaceWith(nav);nav.append(card.querySelector('a')||Object.assign(document.createElement('a'),{href:'/super-admin/users',textContent:'Manage accounts',className:'admin-button primary'}),a);}
        }
        document.querySelectorAll('a.admin-button').forEach(a=>{if(a.textContent.includes('Manage category presentation')){a.textContent='Edit category presentation';a.classList.add('ui-wrap-action');}});
      },name);
      for(const width of [1440,390,320]){
        await page.setViewportSize({width,height:width>800?1000:844});await page.evaluate(()=>document.fonts.ready);
        await page.screenshot({path:path.join(output,`${name}-${width}.png`),fullPage:true});
      }
      // Readable HTML design reference using the running app's own assets.
      await page.evaluate(()=>{document.querySelectorAll('script').forEach(el=>el.remove());const base=document.createElement('base');base.href='http://127.0.0.1:18080/';document.head.prepend(base);});
      await fs.writeFile(path.join(output,`${name}.html`),await page.content());
      await context.close(); console.log(`Design study: ${name}`);
    }
  } finally { await browser.close(); }
}
main().catch(error=>{console.error(error);process.exitCode=1;});
