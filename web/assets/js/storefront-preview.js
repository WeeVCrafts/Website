(() => {
 'use strict';
 const draft=document.body?.dataset.storefrontPreviewId;
 if(!draft)return;
 const previewURL=path=>'/storefront/preview/'+encodeURIComponent(draft)+'?path='+encodeURIComponent(path);
 document.addEventListener('submit',event=>{event.preventDefault();event.stopImmediatePropagation()},true);
 document.addEventListener('click',event=>{
  const action=event.target.closest('[hx-post],[hx-delete],[hx-put],[data-quick-view],[data-firebase-link]');
  if(action){event.preventDefault();event.stopImmediatePropagation();return}
  const link=event.target.closest('a[href]');if(!link)return;
  const url=new URL(link.href,location.origin);if(url.origin!==location.origin){event.preventDefault();return}
  if(url.pathname.startsWith('/storefront/preview/'))return;
  event.preventDefault();event.stopImmediatePropagation();location.href=previewURL(url.pathname+url.search+url.hash);
 },true);
 document.querySelectorAll('form').forEach(form=>{form.removeAttribute('hx-get');form.removeAttribute('hx-post');form.querySelectorAll('button[type=submit],input[type=submit]').forEach(button=>{button.disabled=true;button.title='Shopping actions are disabled in draft previews'})});
 document.querySelectorAll('[hx-get]').forEach(element=>{element.removeAttribute('hx-get');element.removeAttribute('hx-trigger')});
})();
