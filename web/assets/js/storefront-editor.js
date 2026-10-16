(() => {
 'use strict';
 document.querySelectorAll('[data-promotion-kind]').forEach(select=>{
  const form=select.form,input=form.querySelector('[data-discount-value]'),label=form.querySelector('[data-discount-label]'),caption=form.querySelector('[data-discount-caption]');
  const update=()=>{const fixed=select.value==='fixed',free=select.value==='free_shipping';label.hidden=free;input.disabled=free;input.min=fixed?'0.01':'1';input.max=fixed?'1000000':'100';input.step=fixed?'0.01':'1';caption.textContent=fixed?'Amount off (₹ INR)':'Percent off';};select.addEventListener('change',update);update();
 });
 const frame=document.querySelector('[data-storefront-preview]');
 document.querySelectorAll('.sf-settings form[method=post]').forEach(form=>{form.dataset.storefrontDirty='true'});
 const layout=document.querySelector('.sf-editor-layout'),paneTabs=document.querySelector('.sf-mobile-tabs');
 if(layout&&paneTabs){
  const buttons=Array.from(paneTabs.querySelectorAll('[data-editor-pane]')),narrow=matchMedia('(max-width:1100px)');
  const select=(button,focus=false)=>{layout.dataset.editorPane=button.dataset.editorPane;buttons.forEach(other=>{other.setAttribute('aria-selected',String(other===button));other.tabIndex=other===button?0:-1});if(focus)button.focus()};
  const responsive=()=>{paneTabs.hidden=!narrow.matches;for(const id of ['sf-settings-panel','sf-preview-panel']){const panel=document.getElementById(id);if(narrow.matches)panel.setAttribute('role','tabpanel');else panel.removeAttribute('role')};if(narrow.matches)select(buttons.find(button=>button.dataset.editorPane===(layout.dataset.editorPane||'edit')));else delete layout.dataset.editorPane};
  buttons.forEach((button,index)=>{button.addEventListener('click',()=>select(button));button.addEventListener('keydown',event=>{let next;if(event.key==='ArrowRight'||event.key==='ArrowLeft')next=buttons[(index+1)%buttons.length];if(event.key==='Home')next=buttons[0];if(event.key==='End')next=buttons[buttons.length-1];if(next){event.preventDefault();select(next,true)}})});narrow.addEventListener('change',responsive);responsive();
 }
 document.querySelectorAll('[data-preview-width]').forEach(button=>button.addEventListener('click',()=>{
  if(!frame)return;frame.style.width=button.dataset.previewWidth+'px';
  document.querySelectorAll('[data-preview-width]').forEach(other=>other.setAttribute('aria-pressed',String(other===button)));
 }));
 document.querySelectorAll('[data-copy-media]').forEach(input=>input.addEventListener('focus',()=>input.select()));
 const list=document.querySelector('[data-section-list]');let dragged;
 if(list){list.addEventListener('dragstart',event=>{dragged=event.target.closest('[data-section-id]');if(dragged)event.dataTransfer.setData('text/plain',dragged.dataset.sectionId)});
 list.addEventListener('dragover',event=>{if(dragged){event.preventDefault();const target=event.target.closest('[data-section-id]');list.querySelectorAll('.sf-drag-over').forEach(item=>item.classList.remove('sf-drag-over'));target?.classList.add('sf-drag-over')}});
 list.addEventListener('drop',event=>{event.preventDefault();const target=event.target.closest('[data-section-id]');if(!dragged||!target||target===dragged)return;list.insertBefore(dragged,target);const form=dragged.querySelector('form');const operation=document.createElement('input');operation.type='hidden';operation.name='operation';operation.value='reorder';form.append(operation);const order=document.createElement('input');order.type='hidden';order.name='section_order';order.value=Array.from(list.querySelectorAll('[data-section-id]')).map(item=>item.dataset.sectionId).join(',');form.append(order);form.requestSubmit()});
 list.addEventListener('dragend',()=>{dragged=null;list.querySelectorAll('.sf-drag-over').forEach(item=>item.classList.remove('sf-drag-over'))});}
})();
