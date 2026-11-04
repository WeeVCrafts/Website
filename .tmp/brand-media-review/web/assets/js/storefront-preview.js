(() => {
 'use strict';
 const draft=document.body?.dataset.storefrontPreviewId;
 if(!draft)return;
 const origin=new URL(document.baseURI).origin;
 const previewURL=path=>'/storefront/preview/'+encodeURIComponent(draft)+'?path='+encodeURIComponent(path);
 document.addEventListener('submit',event=>{event.preventDefault();event.stopImmediatePropagation()},true);
 document.addEventListener('click',event=>{
  if(event.target.closest('[data-sf-preview-edit],[data-sf-preview-protected]') || (document.body.dataset.storefrontEditing==='true' && event.target.closest('[data-sf-preview-target]')))return;
  const action=event.target.closest('[hx-post],[hx-delete],[hx-put],[data-quick-view],[data-firebase-link]');
  if(action){event.preventDefault();event.stopImmediatePropagation();return}
  const link=event.target.closest('a[href]');if(!link)return;
  const url=new URL(link.href,origin);if(url.origin!==origin){event.preventDefault();return}
  if(url.pathname.startsWith('/storefront/preview/'))return;
  event.preventDefault();event.stopImmediatePropagation();location.href=previewURL(url.pathname+url.search+url.hash);
 },true);
 document.querySelectorAll('form').forEach(form=>{form.removeAttribute('hx-get');form.removeAttribute('hx-post');form.querySelectorAll('button[type=submit],input[type=submit]').forEach(button=>{button.disabled=true;button.title='Shopping actions are disabled in draft previews'})});
 document.querySelectorAll('[hx-get]').forEach(element=>{element.removeAttribute('hx-get');element.removeAttribute('hx-trigger')});

 let editing=false,selected='',targets=[];
 const send=data=>{if(parent!==window)parent.postMessage({...data,draft},origin)};
 const style=document.createElement('style');style.textContent=`body[data-storefront-editing=true] [data-sf-preview-target]{position:relative;outline:1px dashed var(--brand-gold,#906220);outline-offset:-2px}body[data-storefront-editing=true] [data-sf-preview-target][data-sf-selected=true]{outline:3px solid var(--brand-primary,#123f32);outline-offset:-3px}.sf-preview-edit-control{position:absolute;z-index:55;top:8px;right:8px;background:var(--brand-surface,#fffdf8);color:var(--brand-primary,#123f32);border:1px solid var(--brand-primary,#123f32);border-radius:6px;min-height:44px;max-width:90%;padding:8px 12px;font:600 14px/1.4 Arial,sans-serif;cursor:pointer}.sf-preview-edit-control:focus-visible{outline:3px solid var(--brand-gold,#906220);outline-offset:3px}body:not([data-storefront-editing=true]) .sf-preview-edit-control{display:none}`;document.head.append(style);
 function annotate(){
  document.querySelectorAll('[data-sf-preview-target]').forEach(el=>{delete el.dataset.sfPreviewTarget;delete el.dataset.sfSelected});document.querySelectorAll('.sf-preview-edit-control').forEach(el=>el.remove());
  const attach=(el,id,label)=>{if(!el||!targets.some(t=>t.id===id))return;el.dataset.sfPreviewTarget=id;el.dataset.sfSelected=String(id===selected);const b=document.createElement('button');b.type='button';b.className='sf-preview-edit-control';b.textContent='Edit '+label;b.dataset.sfPreviewEdit=id;el.prepend(b)};
  document.querySelectorAll('[data-storefront-section]').forEach(el=>{const id='section:'+el.dataset.storefrontSection;attach(el,id,targets.find(t=>t.id===id)?.label||'section')});
  const slides=Array.from(document.querySelectorAll('[data-hero-slide]'));slides.forEach(el=>attach(el,'slide:'+el.dataset.storefrontSlide,'hero slide'));
  document.querySelectorAll('[data-storefront-category]').forEach(el=>attach(el,'category:'+el.dataset.storefrontCategory,'category'));
  attach(document.querySelector('.site-header'),'global:navigation','header');attach(document.querySelector('.site-footer'),'global:navigation','footer');
  document.querySelectorAll('.detail-copy,.checkout-main,.cart-items,.account-content').forEach(el=>{if(el.closest('[data-sf-preview-target]'))return;el.dataset.sfPreviewTarget='protected:commerce';const b=document.createElement('button');b.type='button';b.className='sf-preview-edit-control';b.textContent='About this protected content';b.dataset.sfPreviewProtected='';el.prepend(b)});
  const template=targets.find(t=>t.id.startsWith('template:'));if(template)attach(document.querySelector('.page-intro,.listing-banner,.seller-hero'),'template:'+template.id.slice(9),'page introduction');
  if(editing&&selected.startsWith('slide:')){const index=slides.findIndex(el=>'slide:'+el.dataset.storefrontSlide===selected);if(index>=0)document.dispatchEvent(new CustomEvent('weevcrafts:preview-slide',{detail:{index}}))}
 }
 window.addEventListener('message',event=>{if(event.source!==parent||event.origin!==origin||event.data?.type!=='weevcrafts:editor'||String(event.data.draft)!==draft)return;editing=event.data.mode==='edit';selected=String(event.data.selected||'');targets=Array.isArray(event.data.targets)?event.data.targets.filter(t=>typeof t.id==='string'&&typeof t.label==='string').slice(0,300):[];document.body.dataset.storefrontEditing=String(editing);document.dispatchEvent(new CustomEvent('weevcrafts:preview-mode'));annotate();if(Number.isFinite(event.data.scroll)&&event.data.scroll>0)window.scrollTo({top:event.data.scroll,behavior:'instant'});else if(selected){document.querySelector('[data-sf-selected=true]')?.scrollIntoView({block:'center',behavior:'instant'})}});
 document.addEventListener('click',event=>{if(editing&&event.target.closest('[data-sf-preview-protected]')){event.preventDefault();event.stopImmediatePropagation();send({type:'weevcrafts:protected'});return}const control=event.target.closest('[data-sf-preview-edit]');if(control){event.preventDefault();event.stopImmediatePropagation();send({type:'weevcrafts:select',id:control.dataset.sfPreviewEdit});return}if(!editing)return;const target=event.target.closest('[data-sf-preview-target]');if(target){event.preventDefault();event.stopImmediatePropagation();send(target.dataset.sfPreviewTarget==='protected:commerce'?{type:'weevcrafts:protected'}:{type:'weevcrafts:select',id:target.dataset.sfPreviewTarget})}},true);
 // The read-only handler above yields annotated regions to this listener.
 let scrollTimer;window.addEventListener('scroll',()=>{clearTimeout(scrollTimer);scrollTimer=setTimeout(()=>send({type:'weevcrafts:preview-scroll',scroll:scrollY}),100)},{passive:true});
 send({type:'weevcrafts:preview-ready'});
})();
