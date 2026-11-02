(function () {
  'use strict';
  document.addEventListener('DOMContentLoaded', () => {
    const form = document.querySelector('[data-product-builder]');
    if(form?.hasAttribute('data-product-simple')) {
      form.addEventListener('input',()=>{const submit=document.querySelector('[data-review-submit] button');if(submit)submit.disabled=true;});
      form.addEventListener('ui:invalid',event=>{let details=event.target.closest('details');while(details){details.open=true;details=details.parentElement.closest('details');}});
    }

    if (form && !form.hasAttribute("data-product-simple")) {
      const keys = ['details', 'pricing', 'photos', 'review'];
      const panels = keys.map(key => form.querySelector('[data-product-step="' + key + '"]'));
      const links = Array.from(form.querySelectorAll('[data-product-step-link]'));
      const back = form.querySelector('[data-product-back]'), next = form.querySelector('[data-product-next]');
      const save = form.querySelector('[data-product-save]');
      let current = 0, completed = form.dataset.savedProduct ? 3 : 0;
      function review() {
        const target = form.querySelector('[data-product-review]');
        target.replaceChildren();
        const sections = [
          ['details', 'Details', ['name', 'category_slug', 'sku', 'description', 'delivery_label', 'materials', 'dimensions', 'care', 'technique']],
          ['pricing', 'Pricing & stock', ['price_rupees', 'compare_at_rupees', 'cost_price_rupees', 'initial_stock']],
          ['photos', 'Photos', ['image_file', 'image_url']]
        ];
        sections.forEach(([key, title, fields]) => {
          const section = document.createElement('section'), heading = document.createElement('h3');
          heading.textContent = title;
          const edit = document.createElement('button'); edit.type = 'button'; edit.className = 'seller-button outline';
          edit.textContent = 'Edit ' + title.toLowerCase(); edit.addEventListener('click', () => show(keys.indexOf(key)));
          const list = document.createElement('dl');
          fields.forEach(name => {
            const field = form.elements.namedItem(name); if (!field || field.type === 'hidden') return;
            const value = field.type === 'file' ? field.files[0]?.name : field.tagName === 'SELECT' ? field.selectedOptions[0]?.textContent : field.value;
            if (!value) return;
            const label = document.createElement('dt'), text = document.createElement('dd');
            label.textContent = field.labels?.[0]?.childNodes[0]?.textContent?.trim() || name;
            text.textContent = value; list.append(label, text);
          });
          if (key === 'photos') {
            const preview = form.querySelector('[data-main-image-preview] img');
            if (preview) section.append(preview.cloneNode());
            else { const empty = document.createElement('p'); empty.textContent = 'No cover selected yet.'; section.append(empty); }
          }
          section.append(heading, list, edit); target.append(section);
        });
      }
      function show(index, focus = true) {
        current = index; form.dataset.wizardStep = keys[index];
        panels.forEach((panel, i) => { panel.hidden = i !== index; });
        links.forEach((link, i) => {
          link.disabled = i > completed;
          if (i === index) link.setAttribute('aria-current', 'step'); else link.removeAttribute('aria-current');
        });
        back.hidden = index === 0; next.hidden = index === 3; save.hidden = index !== 3;
        if (index === 3) review();
        if (focus) panels[index].querySelector('legend,h2').focus({preventScroll:true});
      }
      function advance() {
        const fields = panels[current].querySelectorAll('input,select,textarea');
        if (!window.WVUI.validate({elements: fields})) return;
        completed = Math.max(completed, Math.min(current + 1, 3));
        show(Math.min(current + 1, 3));
      }
      form.querySelector('[data-product-steps]').hidden = false;
      next.addEventListener('click', advance); back.addEventListener('click', () => show(current - 1));
      links.forEach((link, i) => link.addEventListener('click', () => { if (i <= completed) show(i); }));
      form.addEventListener('ui:wizard-next', advance);
      form.addEventListener('ui:invalid', event => {
        const panel = event.target.closest('[data-product-step]');
        if (panel) { const index = keys.indexOf(panel.dataset.productStep); completed = Math.max(completed,index); show(index,false); }
      });
      // Submitting an older persisted draft while the editor has unsaved changes
      // would silently omit those changes from the marketplace review.
      form.addEventListener('input', () => {
        const submit = document.querySelector('[data-review-submit] button');
        if (submit && !submit.disabled) {
          submit.disabled = true;
          const hint = document.createElement('p'); hint.textContent = 'Save your changes before submitting for review.';
          hint.role = 'status'; submit.after(hint);
        }
      });
      const requested = new URLSearchParams(location.search).get('step');
      show(form.dataset.savedProduct && keys.includes(requested) ? keys.indexOf(requested) : 0, false);
      if (form.querySelector('.seller-form-error')) {
        completed = 3; show(3, false);
        const serverField = form.querySelector('[data-server-product-field]');
        const field = serverField && form.elements.namedItem(serverField.dataset.serverProductField);
        if (field) {
          field.setCustomValidity(form.querySelector('.seller-form-error').firstChild.textContent.trim());
          field.addEventListener('input', () => field.setCustomValidity(''), {once:true});
        }
        if (window.WVUI.validate(form)) form.querySelector('.seller-form-error').focus();
      }
    }
    document.querySelectorAll('[data-stock-adjustment]').forEach(adjustment => {
      const quantity = adjustment.elements.namedItem('quantity'), direction = adjustment.elements.namedItem('adjustment_type');
      const preview = adjustment.querySelector('[data-stock-preview]'), available = Number(adjustment.dataset.available);
      function update() {
        const count = Number(quantity.value), removing = direction.value === 'remove';
        quantity.setCustomValidity(removing && count > available ? 'Remove no more than the available quantity. Reserved units cannot be changed here.' : '');
        preview.textContent = quantity.value && Number.isInteger(count) && count > 0 ? 'Available after this change: ' + (available + (removing ? -count : count)) + ' (estimate until saved).' : 'Reserved units are unchanged. Enter a quantity to preview available stock.';
      }
      adjustment.addEventListener('input', update); adjustment.addEventListener('change', update); update();
    });
    const fulfillment = document.querySelector('[data-fulfillment-form]');
    if (fulfillment) {
      const carrier = fulfillment.elements.namedItem('carrier');
      const custom = fulfillment.querySelector('[data-custom-carrier]');
      if (carrier && custom) {
        const update = () => { custom.hidden = carrier.value !== 'Other'; custom.querySelector('input').required = carrier.value === 'Other'; };
        carrier.addEventListener('change',update); update();
      }
    }
  });
})();

/* Presentation enhancement: all writes remain owned by existing form handlers. */
(() => {
 'use strict';
 document.addEventListener('DOMContentLoaded', () => {
  const body=document.querySelector('.seller-body');
  if(!body)return;
  const desktop=matchMedia('(min-width:961px)'), toggle=body.querySelector('[data-seller-collapse]');
  const key='weevcrafts.seller.sidebar-collapsed';let collapsed=false;
  try{collapsed=localStorage.getItem(key)==='true';}catch{}
  const tooltip=document.createElement('div');tooltip.className='seller-rail-tooltip';tooltip.hidden=true;tooltip.role='tooltip';tooltip.id='seller-rail-tooltip';body.append(tooltip);
  function hideTip(){tooltip.hidden=true;body.querySelectorAll('[aria-describedby="seller-rail-tooltip"]').forEach(link=>link.removeAttribute('aria-describedby'));}
  function render(){body.classList.toggle('seller-rail',collapsed&&desktop.matches);hideTip();if(toggle){toggle.setAttribute('aria-expanded',String(!collapsed));toggle.setAttribute('aria-label',collapsed?'Expand sidebar':'Collapse sidebar');}window.dispatchEvent(new Event('resize'));}
  toggle?.addEventListener('click',()=>{collapsed=!collapsed;try{localStorage.setItem(key,String(collapsed));}catch{}render();});
  desktop.addEventListener('change',()=>{if(desktop.matches&&body.querySelector('.seller-sidebar.is-open'))body.querySelector('.seller-sidebar-close')?.click();render();});
  function showTip(link){if(!body.classList.contains('seller-rail'))return;hideTip();const rect=link.getBoundingClientRect();tooltip.textContent=link.dataset.tooltip;tooltip.hidden=false;tooltip.style.left=(rect.right+12)+'px';tooltip.style.top=Math.min(innerHeight-tooltip.offsetHeight-8,Math.max(8,rect.top))+'px';link.setAttribute('aria-describedby',tooltip.id);}
  body.querySelectorAll('[data-tooltip]').forEach(link=>{
   const show=()=>showTip(link);
   link.addEventListener('mouseenter',show);link.addEventListener('focus',show);link.addEventListener('mouseleave',()=>{if(document.activeElement!==link)hideTip();});link.addEventListener('blur',hideTip);
  });
  document.addEventListener('keydown',event=>{if(event.key==='Escape')hideTip();});
  window.addEventListener('scroll',()=>{const focused=body.querySelector('[data-tooltip]:focus');if(focused&&body.classList.contains('seller-rail'))showTip(focused);else hideTip();},true);render();
  body.querySelectorAll('[data-dropzone]').forEach(zone=>{
   const input=zone.querySelector('input[type=file]');if(!input)return;
   const files=document.createElement('span');files.className='seller-upload-files';files.role='status';zone.append(files);
   input.addEventListener('change',()=>{files.textContent=Array.from(input.files,file=>file.name).join(', ');});
   zone.addEventListener('dragover',event=>{event.preventDefault();zone.classList.add('is-dragging');});
   zone.addEventListener('dragleave',()=>zone.classList.remove('is-dragging'));
   zone.addEventListener('drop',event=>{event.preventDefault();zone.classList.remove('is-dragging');if(event.dataTransfer.files.length){input.files=event.dataTransfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));}});
  });
  const profile=body.querySelector('[data-store-profile]');
  if(profile){
   const set=(name,value)=>{const target=body.querySelector('[data-preview-'+name+']');if(target)target.textContent=value;};
   const update=()=>{const name=profile.elements.display_name.value;set('name',name);set('studio',name);set('location',profile.elements.public_location.value);set('story',profile.elements.public_story.value?'Added':'Not added');set('portrait',profile.elements.image_file.files.length||profile.elements.image_url.value?'Added':'Not added');};
   profile.addEventListener('input',update);profile.addEventListener('change',update);update();
  }
 });
})();
