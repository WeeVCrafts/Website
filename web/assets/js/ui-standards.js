(function () {
  "use strict";
  let errorSequence = 0;
  function clearError(field) {
    const errorID = field.dataset.uiError;
    if (!errorID) return;
    document.getElementById(errorID)?.remove();
    const descriptions = (field.getAttribute("aria-describedby") || "").split(" ").filter(id => id && id !== errorID);
    if (descriptions.length) field.setAttribute("aria-describedby", descriptions.join(" "));
    else field.removeAttribute("aria-describedby");
    field.removeAttribute("aria-invalid");
    delete field.dataset.uiError;
  }
  function validate(form) {
    let first;
    Array.from(form.elements).forEach(field => {
      if (!field.willValidate) return;
      clearError(field);
      if (field.checkValidity()) return;
      const error = document.createElement("span");
      error.id = "ui-field-error-" + (++errorSequence);
      error.className = "ui-field-error";
      error.textContent = field.validity.valueMissing ? "Complete this field to continue." : field.validity.patternMismatch ? (field.title || "Use the requested format.") : field.validationMessage;
      field.dataset.uiError = error.id;
      field.setAttribute("aria-invalid", "true");
      field.setAttribute("aria-describedby", [field.getAttribute("aria-describedby"), error.id].filter(Boolean).join(" "));
      field.insertAdjacentElement("afterend", error);
      if (!first) first = field;
    });
    if (first) {
      first.dispatchEvent(new CustomEvent('ui:invalid', {bubbles:true}));
      let disclosure = first.closest('details');
      while (disclosure) { disclosure.open = true; disclosure = disclosure.parentElement?.closest('details'); }
      first.focus(); first.scrollIntoView({block:"center",behavior:"instant"});
    }
    return !first;
  }
  function trapModalTab(event) {
    const dialog = event.currentTarget;
    if (event.key !== 'Tab' || !dialog.open) return;
    const controls = Array.from(dialog.querySelectorAll('a[href],button,input,select,textarea,[tabindex]')).filter(control => !control.matches(':disabled,input[type=hidden],[tabindex="-1"]') && control.getClientRects().length);
    const first = controls[0], last = controls[controls.length-1];
    if (!first) { event.preventDefault(); dialog.focus(); return; }
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }
  function bindModalTab(dialog) {
    if (dialog.dataset.modalTabBound) return;
    dialog.dataset.modalTabBound = 'true';
    dialog.addEventListener('keydown',trapModalTab);
  }
  function confirmAction(message, trigger, actionLabel, options = {}) {
    return new Promise(resolve => {
      const dialog = document.createElement("dialog");
      bindModalTab(dialog);
      dialog.className = "ui-confirm";
      dialog.dataset.intent = options.intent || "primary";
      dialog.setAttribute("aria-labelledby", "ui-confirm-title");
      const heading = document.createElement("h2"); heading.id = "ui-confirm-title"; heading.textContent = actionLabel || "Confirm action";
      const copy = document.createElement("p"); copy.id = "ui-confirm-copy"; copy.textContent = message; dialog.setAttribute("aria-describedby",copy.id);
      const actions = document.createElement("div"); actions.className = "ui-confirm-actions";
      const cancel = document.createElement("button"); cancel.type = "button"; cancel.textContent = options.cancelLabel || "Cancel"; cancel.autofocus = true;
      const accept = document.createElement("button"); accept.type = "button"; accept.textContent = actionLabel || "Continue"; accept.dataset.confirmAccept = "";
      actions.append(cancel);
      if(options.additionalAction){const additional=document.createElement('button');additional.type='button';additional.textContent=options.additionalAction.label;additional.addEventListener('click',()=>finish(options.additionalAction.value));actions.append(additional);}
      actions.append(accept); dialog.append(heading, copy, actions); document.body.append(dialog);
      function finish(accepted) { dialog.close(); dialog.remove(); trigger?.focus(); resolve(accepted); }
      cancel.addEventListener("click",()=>finish(false)); accept.addEventListener("click",()=>finish(true));
      dialog.addEventListener("cancel", event => { event.preventDefault(); finish(false); });
      dialog.showModal(); cancel.focus();
    });
  }
  function busy(element, active) {
    if (!element) return;
    element.classList.toggle("ui-busy", active);
    if (active) element.setAttribute("aria-busy", "true");
    else element.removeAttribute("aria-busy");
  }
  // The existing Alpine state owns opening/closing; this adapter owns modal
  // accessibility only. Closed off-canvas links must never remain tabbable.
  function initRoleNavigation(root) {
    root.querySelectorAll('#seller-sidebar, #admin-sidebar, #support-sidebar').forEach(panel => {
      if (panel.dataset.modalBound) return;
      panel.dataset.modalBound = 'true';
      const kind = panel.id.replace('-sidebar', '');
      const trigger = document.querySelector('.' + kind + '-menu-button');
      const close = panel.querySelector('.' + kind + '-sidebar-close');
      const backdrop = document.querySelector('.' + kind + '-sidebar-backdrop');
      if (!trigger || !close) return;
      let blocked = [], oldOverflow = '', open = false;
      const navigationMedia = panel.dataset.navigationMedia ? matchMedia(panel.dataset.navigationMedia) : null;
      const small = () => navigationMedia ? navigationMedia.matches : getComputedStyle(trigger).display !== 'none';
      function restore() {
        blocked.forEach(([element, inert]) => { element.inert = inert; });
        blocked = []; document.body.style.overflow = oldOverflow;
      }
      function sync() {
        const next = small() && panel.classList.contains('is-open');
        if (open && !next) { restore(); if (small()) requestAnimationFrame(() => trigger.focus({preventScroll:true})); }
        panel.inert = small() && !next;
        if (backdrop) { backdrop.tabIndex = -1; backdrop.setAttribute('aria-hidden', 'true'); }
        if (next && !open) {
          let branch = panel;
          while (branch.parentElement && branch.parentElement !== document.documentElement) {
            for (const sibling of branch.parentElement.children) {
              if (sibling === branch || sibling === backdrop || sibling.tagName === 'SCRIPT' || sibling.hasAttribute(':inert')) continue;
              blocked.push([sibling, sibling.inert]); sibling.inert = true;
            }
            branch = branch.parentElement;
          }
          oldOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden';
          close.focus({preventScroll:true});
        }
        if (next) {
          panel.setAttribute('role', 'dialog'); panel.setAttribute('aria-modal', 'true');
          panel.setAttribute('aria-label', kind[0].toUpperCase() + kind.slice(1) + ' navigation');
        } else {
          panel.removeAttribute('role'); panel.removeAttribute('aria-modal'); panel.removeAttribute('aria-label');
        }
        open = next;
      }
      panel.addEventListener('keydown', event => {
        if (!open) return;
        if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close.click(); return; }
        if (event.key !== 'Tab') return;
        event.stopPropagation();
        const items = Array.from(panel.querySelectorAll('a[href],button:not(:disabled),input:not(:disabled),select:not(:disabled)')).filter(item => item.getClientRects().length && !item.closest('[inert]'));
        const first = items[0], last = items[items.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      });
      new MutationObserver(sync).observe(panel, {attributes:true,attributeFilter:['class']});
      window.addEventListener('resize', () => { if (!small() && open) close.click(); sync(); });
      sync();
    });
  }
  // Native modal dialogs share focus, cancellation and unsaved-edit behavior.
  function initFormDialogs(root) {
    root.querySelectorAll('[data-form-dialog]').forEach(dialog => {
      if (dialog.dataset.dialogBound) return;
      dialog.dataset.dialogBound = 'true';
      let trigger;
      const close = async () => {
        const forms = Array.from(dialog.querySelectorAll('form'));
        if (forms.some(form => form.dataset.saving === 'true')) return;
        if (forms.some(form => dirtyForms.has(form)) && !(await confirmAction('Discard your unsaved changes to this form?', document.activeElement, 'Discard changes', {cancelLabel:'Keep editing',intent:'warning'}))) return;
        forms.forEach(form => {
          form.reset(); dirtyForms.delete(form); delete form.dataset.discarded;
          form.querySelector('.ui-form-error')?.remove();
          form.dispatchEvent(new CustomEvent('ui:discard', {bubbles:true}));
        });
        dialog.close();
      };
      document.querySelectorAll('[data-dialog-open]').forEach(button => {
        if (button.dataset.dialogOpen !== dialog.id) return;
        button.addEventListener('click', () => { trigger = button; dialog.showModal(); });
      });
      dialog.querySelectorAll('[data-dialog-close]').forEach(button => button.addEventListener('click', close));
      dialog.addEventListener('cancel', event => { event.preventDefault(); close(); });
      dialog.addEventListener('close', () => {
        dialog.querySelectorAll('input[autocomplete="new-password"]').forEach(input => { input.value = ''; input.type = 'password'; });
        dialog.querySelectorAll('.password-visibility').forEach(button => { button.textContent='Show password'; button.setAttribute('aria-pressed','false'); });
        (trigger || document.getElementById(dialog.dataset.dialogReturn) || document.querySelector('[data-category-search]'))?.focus({preventScroll:true});
      });
      if (dialog.hasAttribute('data-dialog-auto')) { dialog.removeAttribute('open'); dialog.showModal(); }
    });
    root.querySelectorAll('[data-user-return]').forEach(input => { input.value = location.pathname + location.search; });
  }
  function initUserDeleteDialogs(root) {
    root.querySelectorAll('[data-user-delete-trigger]').forEach(trigger => {
      if (trigger.dataset.deleteDialogBound) return;
      const dialog = document.getElementById(trigger.dataset.userDeleteDialog);
      if (!dialog) return;
      trigger.dataset.deleteDialogBound = 'true';
      const cancel = dialog.querySelector('[data-user-delete-cancel]');
      trigger.addEventListener('click', () => {
        dialog.showModal();
        requestAnimationFrame(() => cancel?.focus());
      });
      cancel?.addEventListener('click', () => dialog.close());
      dialog.addEventListener('cancel', event => { event.preventDefault(); dialog.close(); });
      dialog.addEventListener('close', () => trigger.focus({preventScroll:true}));
    });
  }
  function init(root) {
    root.querySelectorAll('[data-guided-progress]').forEach(progress=>{
      if(progress.dataset.guidedBound)return;
      progress.dataset.guidedBound='true';
      if(matchMedia('(max-width: 960px)').matches)progress.open=false;
    });
    root.querySelectorAll('[data-fulfillment-setup-action="save_invoice_profile"]').forEach(form=>{
      const registration=form.elements.namedItem('registration');
      const field=form.querySelector('[data-gstin-field]');
      const gstin=form.elements.namedItem('gstin');
      if(!registration||!field||!gstin||registration.dataset.fulfillmentBound)return;
      registration.dataset.fulfillmentBound='true';
      const update=()=>{const registered=registration.value==='registered';field.hidden=!registered;gstin.disabled=!registered;gstin.required=registered;};
      registration.addEventListener('change',update);update();
    });
    if (root.matches?.('dialog')) bindModalTab(root);
    root.querySelectorAll('dialog').forEach(bindModalTab);
    initRoleNavigation(root);
    initFormDialogs(root);
    initUserDeleteDialogs(root);
    // Persistent data-entry labels; placeholders remain examples, not labels.
    root.querySelectorAll('input,select,textarea').forEach(field=>{
      if(field.closest('label') || field.labels?.length || !field.name || field.name==='q' || ['hidden','submit','button','checkbox','radio'].includes(field.type))return;
      const names={delta:'Quantity change',reason:'Reason',note:'Note',message:'Message',amount_rupees:'Amount (INR)'};
      const label=document.createElement('label');label.className='ui-labelled-field';
      label.append(field.getAttribute('aria-label')||names[field.name]||field.name.replace(/_/g,' ').replace(/^./,letter=>letter.toUpperCase()));
      field.before(label);label.append(field);
    });
    root.querySelectorAll('.seller-search input[name="q"], .admin-search input[name="q"], .support-search input[name="q"]').forEach(field => {
      if (field.dataset.clearBound) return;
      field.dataset.clearBound = 'true';
      const clear = document.createElement('button'); clear.type = 'button'; clear.className = 'ui-search-clear';
      clear.textContent = '×'; clear.setAttribute('aria-label', 'Clear search');
      field.after(clear);
      const update = () => { clear.hidden = !field.value; };
      field.addEventListener('input', update);
      clear.addEventListener('click', () => {
        window.htmx?.trigger(field, 'htmx:abort');
        field.value = ''; field.dispatchEvent(new Event('input', {bubbles:true})); field.focus(); update();
      });
      update();
    });
    root.querySelectorAll('[data-account-navigation]').forEach(menu => {
      if (menu.dataset.navigationBound) return;
      menu.dataset.navigationBound = "true";
      const mobile = matchMedia('(max-width: 720px)');
      const update = () => { menu.open = !mobile.matches; };
      update(); mobile.addEventListener('change', update);
    });
    root.querySelectorAll('.role-user-edit-form input[type="password"], .role-user-create-form input[type="password"], [data-customer-password] input[type="password"]').forEach(field => {
      if (field.dataset.visibilityBound) return;
      field.dataset.visibilityBound = "true";
      const toggle = document.createElement("button");
      toggle.type = "button"; toggle.className = "password-visibility";
      toggle.textContent = "Show password"; toggle.setAttribute("aria-pressed", "false");
      toggle.addEventListener("click", () => {
        const visible = field.type === "password";
        field.type = visible ? "text" : "password";
        toggle.textContent = visible ? "Hide password" : "Show password";
        toggle.setAttribute("aria-pressed", String(visible));
      });
      field.insertAdjacentElement("afterend", toggle);
    });
    root.querySelectorAll("form").forEach(form => {
      if (form.dataset.uiBound) return;
      form.dataset.uiBound = "true"; form.noValidate = true;
      if (form.method.toLowerCase() === "post" && customerFormPath(new URL(form.getAttribute("action") || location.href, location.href).pathname)) form.dataset.customerRetain = "true";
      form.addEventListener("input",event => { if (event.target.dataset.uiError) clearError(event.target); });
      form.addEventListener("submit",async event => {
        if (form.dataset.wizardStep !== undefined && form.dataset.wizardStep !== 'review' && !event.submitter?.hasAttribute('data-product-save')) {
          event.preventDefault(); event.stopImmediatePropagation();
          form.dispatchEvent(new CustomEvent('ui:wizard-next')); return;
        }
        if (!event.submitter?.formNoValidate && !validate(form)) { event.preventDefault(); event.stopImmediatePropagation(); return; }
        const confirmation=event.submitter?.dataset.confirm||form.dataset.confirm;
        if (confirmation && !form.dataset.confirmed) {
          event.preventDefault(); event.stopImmediatePropagation();
          const submitter = event.submitter || form.querySelector('[type="submit"]');
          const actionPath = new URL(form.getAttribute("action") || location.href, location.href).pathname;
          const intent = /\/(delete|delete-draft)$/.test(actionPath) ? 'danger' : /\/(archive|suspend)$/.test(actionPath) ? 'warning' : 'primary';
          if (await confirmAction(confirmation, submitter, submitter?.textContent.trim(), {intent})) {
            if (actionPath.endsWith('/users/delete') && !form.querySelector('input[name="confirm"]')) {
              const confirmed = document.createElement('input'); confirmed.type = 'hidden'; confirmed.name = 'confirm'; confirmed.value = 'yes'; form.append(confirmed);
            }
            form.dataset.confirmed = "true"; form.requestSubmit(submitter); delete form.dataset.confirmed;
          }
        }
      },true);
    });
    root.querySelectorAll("textarea").forEach(field => {
      if (field.dataset.growBound) return;
      field.dataset.growBound = "true";
      const minimum=Math.max(44,parseFloat(getComputedStyle(field).minHeight)||120);
      const grow=()=> { field.style.height="auto"; field.style.height=Math.min(400, Math.max(minimum,field.scrollHeight))+"px"; };
      field.addEventListener("input", grow); grow();
    });
    root.querySelectorAll("[data-main-image]").forEach(input => {
      if(input.dataset.previewBound) return;
      input.dataset.previewBound="true";
      let previewURL;
      input.addEventListener("change",()=> {
        if(previewURL) URL.revokeObjectURL(previewURL);
        const file=input.files[0];
        input.setCustomValidity(file && (!['image/jpeg','image/png','image/webp'].includes(file.type) || file.size>25*1024*1024) ? "Choose a JPEG, PNG or WebP image up to 25 MB." : "");
        const preview=input.closest('form')?.querySelector('[data-main-image-preview]');
        if(!preview || !file || !input.checkValidity()) return;
        previewURL=URL.createObjectURL(file);
        const images = preview.querySelectorAll('[data-image-preview-image]');
        if (images.length) {
          images.forEach(img => { img.src=previewURL; img.alt="Selected product card preview"; });
        } else {
          const img=document.createElement('img'); img.src=previewURL; img.alt="Selected image preview"; img.className="maker-portrait-preview";
          preview.replaceChildren(img);
        }
      });
    });
    root.querySelectorAll('[data-main-image-preview]').forEach(preview => {
      if (preview.dataset.cardPreviewBound) return;
      preview.dataset.cardPreviewBound = 'true';
      const fit = preview.querySelector('[data-image-fit]');
      const zoom = preview.querySelector('[data-image-zoom]');
      const x = preview.querySelector('[data-image-position-x]');
      const y = preview.querySelector('[data-image-position-y]');
      const images = Array.from(preview.querySelectorAll('[data-image-preview-image]'));
      images.forEach(img => {
        const source = img.dataset.imageSource;
        if (source) {
          img.crossOrigin = 'anonymous';
          img.src = source;
        }
      });
      let isAdjusted = false;
      const apply = () => {
        const mode = fit?.value || 'cover', scale = Number(zoom?.value || 1), position = `${Number(x?.value || 50)}% ${Number(y?.value || 50)}%`;
        images.forEach(img => { img.style.objectFit = mode; img.style.objectPosition = position; img.style.transform = `scale(${scale})`; });
        const fill = mode === 'cover';
        [zoom,x,y].forEach(control => { if (control) control.disabled = false; });
        preview.dataset.imageFit = fill ? 'fill' : 'whole';
      };
      [fit,zoom,x,y].forEach(control => control?.addEventListener('input', () => { isAdjusted = true; apply(); }));
      [fit,zoom,x,y].forEach(control => control?.addEventListener('change', () => { isAdjusted = true; apply(); }));
      apply();

      const form = preview.closest('form');
      const fileInput = form?.querySelector('input[name="image_file"]');
      if (fileInput) {
        fileInput.addEventListener('change', () => { isAdjusted = true; });
      }

      if (form && !form.dataset.cropSubmitBound) {
        form.dataset.cropSubmitBound = 'true';
        form.addEventListener('submit', async (e) => {
          if (form.dataset.croppingDone === 'true') {
            delete form.dataset.croppingDone;
            return;
          }
          const activeImg = preview.querySelector('[data-image-preview-image]');
          if (!activeImg || (!fileInput?.files?.length && !isAdjusted)) return;
          if (!activeImg.complete || !activeImg.naturalWidth) return;

          e.preventDefault();
          try {
            const CW = 800, CH = 800;
            const canvas = document.createElement('canvas');
            canvas.width = CW; canvas.height = CH;
            const ctx = canvas.getContext('2d');
            ctx.fillStyle = '#fcfbf8';
            ctx.fillRect(0, 0, CW, CH);

            const mode = fit?.value || 'cover';
            const scale = Number(zoom?.value || 1);
            const posX = Number(x?.value || 50) / 100;
            const posY = Number(y?.value || 50) / 100;
            const nw = activeImg.naturalWidth, nh = activeImg.naturalHeight;

            const ratio = (mode === 'cover') ? Math.max(CW / nw, CH / nh) : Math.min(CW / nw, CH / nh);
            const baseW = nw * ratio;
            const baseH = nh * ratio;
            const drawX = (CW - baseW) * posX;
            const drawY = (CH - baseH) * posY;
            const centerX = drawX + baseW / 2;
            const centerY = drawY + baseH / 2;
            const finalW = baseW * scale;
            const finalH = baseH * scale;
            const finalX = centerX - finalW / 2;
            const finalY = centerY - finalH / 2;

            ctx.drawImage(activeImg, finalX, finalY, finalW, finalH);

            const blob = await new Promise(res => canvas.toBlob(res, 'image/webp', 0.92));
            if (blob && fileInput) {
              const file = new File([blob], 'product-cover.webp', { type: 'image/webp' });
              const dt = new DataTransfer();
              dt.items.add(file);
              fileInput.files = dt.files;
            }
          } catch (cropErr) {
            console.warn('Could not export cropped image:', cropErr);
          }
          form.dataset.croppingDone = 'true';
          const submitter = e.submitter;
          if (submitter && submitter.name) {
            const hidden = document.createElement('input');
            hidden.type = 'hidden';
            hidden.name = submitter.name;
            hidden.value = submitter.value;
            form.appendChild(hidden);
            requestAnimationFrame(() => hidden.remove());
          }
          if (form.requestSubmit) {
            form.requestSubmit(submitter);
          } else {
            form.submit();
          }
        });
      }
    });
    root.querySelectorAll('[data-parcel-preview]').forEach(preview => {
      if (preview.dataset.bound) return;
      preview.dataset.bound = 'true';
      const scope = preview.closest('fieldset');
      const field = name => scope?.querySelector(`[name="${name}"]`);
      const canvas = preview.querySelector('[data-parcel-canvas]'), label = preview.querySelector('[data-parcel-preview-label]');
      const update = () => {
        const length = Number(field('shipping_length_cm')?.value), width = Number(field('shipping_width_cm')?.value), height = Number(field('shipping_height_cm')?.value);
        if (![length,width,height].every(value => Number.isFinite(value) && value > 0)) { preview.hidden = true; return; }
        preview.hidden = false;
        const scale = 115 / Math.max(length,width,height), l = Math.max(44, length * scale), w = Math.max(36, width * scale * .72), h = Math.max(42, height * scale * .72);
        const ox = 115, oy = 160, dx = w * .65, dy = w * .35;
        const a = [ox,oy], b = [ox+l,oy], c = [ox+l+dx,oy-dy], d = [ox+dx,oy-dy], e = [ox,oy-h], f = [ox+l,oy-h], g = [ox+l+dx,oy-h-dy], q = [ox+dx,oy-h-dy];
        const point = p => p.join(',');
        const arrow = (p1,p2,text,tx,ty,anchor='middle') => `<line x1="${p1[0]}" y1="${p1[1]}" x2="${p2[0]}" y2="${p2[1]}" marker-start="url(#parcel-arrow)" marker-end="url(#parcel-arrow)"/><text x="${tx}" y="${ty}" text-anchor="${anchor}">${text}</text>`;
        canvas.innerHTML = `<svg viewBox="0 0 380 230" role="img" aria-label="Packed parcel ${length} by ${width} by ${height} centimetres"><defs><marker id="parcel-arrow" markerWidth="6" markerHeight="6" refX="3" refY="3" orient="auto"><path d="M0,0 L6,3 L0,6 z" fill="currentColor"/></marker></defs><g class="parcel-box" fill="var(--ui-surface,#fffdf8)" stroke="var(--ui-primary,#124534)" stroke-width="2"><polygon points="${[a,b,c,d].map(point).join(' ')}"/><polygon points="${[a,d,q,e].map(point).join(' ')}"/><polygon points="${[b,c,g,f].map(point).join(' ')}"/></g><g class="parcel-dimension-lines" fill="currentColor" stroke="currentColor" stroke-width="1">${arrow([a[0],a[1]+14],[b[0],b[1]+14],`${length} cm`,(a[0]+b[0])/2,oy+28)}${arrow([b[0]+14,b[1]],[c[0]+14,c[1]],`${width} cm`,c[0]+14,(b[1]+c[1])/2,'start')}${arrow([a[0]-18,a[1]],[e[0]-18,e[1]],`${height} cm`,a[0]-24,(a[1]+e[1])/2,'end')}</g><text x="190" y="220" text-anchor="middle" fill="currentColor" font-size="11">Auto-rotating packed shape preview</text></svg>`;
        label.textContent = `Packed size: ${length} × ${width} × ${height} cm. This is a proportionate preview, not a courier quote.`;
      };
      ['shipping_length_cm','shipping_width_cm','shipping_height_cm'].forEach(name => { field(name)?.addEventListener('input', update); field(name)?.addEventListener('change', update); });
      update();
    });
    root.querySelectorAll("img").forEach(img => {
      // Gallery owns loading, failure, and Retry. Replacing its source here
      // would turn a failed photograph into a successful fallback response.
      if (img.matches('[data-gallery-media],.media-lightbox img')) return;
      if (img.dataset.fallbackBound) return;
      img.dataset.fallbackBound="true";
      const fallback=()=> {
        if (img.src.endsWith("/product-unavailable.svg")) return;
        const original = new URL(img.src, location.href);
        if (original.origin===location.origin && /^\/media\/(storefront\/|products\/[1-9]\d*$)/.test(original.pathname) && !img.dataset.originalRetried) {
          img.dataset.originalRetried='true'; original.searchParams.delete('w'); original.searchParams.set('v','2');
          img.removeAttribute('srcset'); img.src=original.href; return;
        }
        img.removeAttribute("srcset"); img.src="/assets/images/product-unavailable.svg";
      };
      img.addEventListener("error",fallback); if (img.complete && !img.naturalWidth && !img.hasAttribute('data-src')) fallback();
    });
  }
  window.WVUI = {validate,confirmAction,busy,enhance:root=>init(root||document),markDirty:form=>dirtyForms.add(form)};
  document.addEventListener('keydown',()=>{document.body.dataset.keyboardInput='true';},true);
  document.addEventListener('pointerdown',()=>{delete document.body.dataset.keyboardInput;},true);
  const customerFormPath = path => /^\/account\/(profile|addresses(?:\/delete)?|notifications|reviews)$/.test(path) || /^\/support\/(create|customer-reply)$/.test(path) || /^\/privacy\/(consent|requests)$/.test(path) || /^\/orders\/[^/]+\/(cancel|return|refund)$/.test(path);
  // Protect long edits in memory. Do not persist account/product form values locally.
  const dirtyForms = new Set();
  const dirtySelector = '[data-product-builder], [data-store-profile], .admin-product-editor-form, .role-user-edit-form, [data-storefront-dirty], [data-customer-retain], [data-fulfillment-setup-action]';
  function dirtyForm() {
    for (const form of dirtyForms) {
      if (!form.isConnected) { dirtyForms.delete(form); continue; }
      if (form.dataset.saving !== 'true' && !form.dataset.discarded) return form;
    }
    return null;
  }
  document.addEventListener('input', event => {
    const form = event.target.closest(dirtySelector);
    if (form && event.isTrusted) dirtyForms.add(form);
  });
  document.addEventListener('change', event => {
    const form = event.target.closest(dirtySelector);
    if (form && event.isTrusted) dirtyForms.add(form);
  });
  let discardPending = false;
  document.addEventListener('submit',async event=>{
    const form=event.target;
    if(form.method?.toLowerCase()!=='get'||form.elements.namedItem('tab')?.value!=='setup'||form.dataset.setupDiscardConfirmed||!dirtyForm())return;
    event.preventDefault();
    if(discardPending)return;
    discardPending=true;
    const discard=await confirmAction('Discard your unsaved setup changes before changing selection?',event.submitter,'Discard changes',{cancelLabel:'Keep editing',intent:'warning'});
    discardPending=false;
    if(!discard)return;
    dirtyForms.forEach(edit=>edit.dataset.discarded='true');form.dataset.setupDiscardConfirmed='true';if(event.submitter)form.requestSubmit(event.submitter);else form.requestSubmit();
  },true);
  document.addEventListener('click', async event => {
    const link = event.target.closest('a[href]');
    const form = dirtyForm();
    if (!link || !form || event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || link.target === '_blank' || link.hasAttribute('download')) return;
    const destination = new URL(link.href, location.href);
    if (destination.pathname === location.pathname && destination.search === location.search && destination.hash) return;
    event.preventDefault(); event.stopImmediatePropagation();
    if (discardPending) return;
    discardPending = true;
    try {
      const storefront = form.matches('[data-storefront-dirty]');
      const restoreTrigger=link.hidden?document.activeElement:link;
      const result=await confirmAction('You have unsaved changes. Save them before leaving, discard them, or keep editing.', restoreTrigger, 'Discard changes', {cancelLabel:'Keep editing',intent:'warning',additionalAction:storefront?{label:'Save draft and continue',value:'save'}:undefined});
      if(result==='save') {
        const next=form.querySelector('[name=return_to]')||document.createElement('input');next.type='hidden';next.name='return_to';next.value=destination.pathname+destination.search;form.append(next);form.requestSubmit(form.querySelector('button[type=submit]:not([name=operation])'));return;
      }
      if (result === true) {
        dirtyForms.forEach(item => { item.dataset.discarded = 'true'; });
        location.assign(destination.href);
      }
    } finally { discardPending = false; }
  }, true);
  window.addEventListener('beforeunload', event => {
    if (!dirtyForm()) return;
    event.preventDefault(); event.returnValue = '';
  });
  document.addEventListener('htmx:beforeRequest', event => {
    const target = event.detail.elt;
    busy(target, true);
    if (target.matches('form')) target.querySelectorAll('button[type="submit"]').forEach(button=>busy(button,true));
  });
  document.addEventListener('htmx:afterRequest', event => {
    const target = event.detail.elt;
    busy(target, false);
    if (target.matches('form')) target.querySelectorAll('button[type="submit"]').forEach(button=>busy(button,false));
  });
  document.addEventListener('submit', async event => {
    const form = event.target.closest('[data-customer-password]');
    if (!form) return;
    event.preventDefault();
    if (form.dataset.saving || !validate(form)) return;
    const next=form.elements.namedItem('new_password'), confirmation=form.elements.namedItem('confirm_password');
    confirmation.setCustomValidity(next.value === confirmation.value ? '' : 'Passwords must match.');
    if (!validate(form)) return;
    const button=form.querySelector('[type="submit"]'), message=form.querySelector('[data-password-message]');
    form.dataset.saving='true'; button.disabled=true; busy(button,true); message.textContent='Changing your password…';
    const controller=new AbortController(), timeout=setTimeout(()=>controller.abort(),30000);
    try {
      const response=await fetch(form.action,{method:'POST',credentials:'same-origin',headers:{Accept:'application/json'},body:new URLSearchParams(new FormData(form)),signal:controller.signal});
      const result=await response.json();
      if (!response.ok) {
        if (response.status===401 && result.redirect) {location.assign(result.redirect);return;}
        throw new Error(result.message || 'Your password could not be changed.');
      }
      form.reset(); message.textContent=result.message; location.assign(result.redirect);
    } catch(error) {
      message.setAttribute('role','alert'); message.textContent=error instanceof TypeError || error.name==='AbortError' ? 'Connection interrupted. Check whether your password changed before trying again.' : error.message;
      message.tabIndex=-1; message.focus();
    } finally {clearTimeout(timeout); delete form.dataset.saving; button.disabled=false; busy(button,false);}
  });
  document.addEventListener('input',event=>{if(event.target.name==='confirm_password'||event.target.name==='new_password')event.target.form?.elements.namedItem('confirm_password')?.setCustomValidity('');});
  document.addEventListener("DOMContentLoaded",()=>init(document));
  document.addEventListener("htmx:afterSwap",()=>init(document));
  document.addEventListener('htmx:historyRestore',()=>init(document));
  // Retain typed values on a rejected save, and block repeated POSTs while saving.
  // Do not automatically retry a request whose completion is uncertain.
  document.addEventListener("submit",async event=> {
    const form=event.target;
    if(event.defaultPrevented || form.method.toLowerCase()!=="post" || form.matches('[hx-post],[data-native-submit]')) return;
    const action=new URL(event.submitter?.getAttribute("formaction") || form.getAttribute("action") || location.href,location.href);
    const customerForm = customerFormPath(action.pathname) || action.pathname === '/account/fulfillment/action';
    if(action.origin!==location.origin || (!customerForm && !/^\/(admin|seller-admin|support-portal|marketplace-admin|super-admin|finance-portal|operations-portal)\//.test(action.pathname))) return;
    event.preventDefault();
    if(form.dataset.saving) return;
    const payload=new FormData(form);
    if(event.submitter?.name) payload.append(event.submitter.name,event.submitter.value);
    const buttons=Array.from(form.querySelectorAll('button[type="submit"],input[type="submit"]')).filter(button=>!button.disabled);
    const oldError=form.querySelector('.ui-form-error'); oldError?.remove();
    form.dataset.saving="true"; form.setAttribute('aria-busy','true'); buttons.forEach(button=>{button.disabled=true;busy(button,true);});
    const controller=new AbortController(); const timer=setTimeout(()=>controller.abort(),form.hasAttribute('data-shiprocket-form')?100000:30000);
    let productInvalidField = null;
    try {
      const response=await fetch(action.href,{method:'POST',credentials:'same-origin',headers:{'X-UI-Form':'true'},body:(customerForm && form.enctype!=='multipart/form-data') || form.hasAttribute('data-shiprocket-form') ? new URLSearchParams(payload) : payload,signal:controller.signal});
      if ((customerForm || form.classList.contains('marketplace-form') || form.hasAttribute('data-store-profile') || form.closest('[data-form-dialog]')) && response.redirected && new URL(response.url).pathname === '/login') {
        const error = new Error('Your session ended. Your entries are still here. Copy your changes before signing in again.');
        error.customerSession = true; throw error;
      }
      if(response.ok && response.redirected) { location.assign(response.url); return; }
      const html=await response.text(); const doc=new DOMParser().parseFromString(html,'text/html');
      if(response.status===422 && form.dataset.fulfillmentSetupAction){
        const replacement=Array.from(doc.querySelectorAll('[data-fulfillment-setup-action]')).find(candidate=>candidate.dataset.fulfillmentSetupAction===form.dataset.fulfillmentSetupAction);
        if(replacement){
          dirtyForms.delete(form);form.replaceWith(replacement);init(replacement.parentElement);dirtyForms.add(replacement);
          (replacement.querySelector('[aria-invalid="true"]')||replacement.querySelector('.ui-error-summary'))?.focus();return;
        }
      }
      if (response.status === 422 && form.matches('[data-product-builder]')) {
        const rejectedField = response.headers.get('X-Seller-Product-Field');
        const field = rejectedField && form.elements.namedItem(rejectedField);
        if (field && field.willValidate) {
          productInvalidField = field;
          const message = doc.querySelector('.seller-form-error,.ui-error-summary p,.error-state p,main p')?.textContent?.trim();
          field.setCustomValidity(message || 'Check this value before saving.');
          field.addEventListener('input', () => field.setCustomValidity(''), {once:true});
        }
      }
      const storefrontSummary=doc.querySelector('.sf-error');
      if(storefrontSummary){const error=new Error(storefrontSummary.textContent.trim());error.storefrontSummary=storefrontSummary;throw error;}
      const message=response.headers.get('Content-Type')?.includes('text/plain') ? html.trim().slice(0,1000) : (doc.querySelector('.profile-error,.seller-form-error,.ui-error-summary p,.error-message,.error-state p') || doc.querySelector('main p'))?.textContent?.trim();
      const error = new Error(message || (response.status===403 ? 'Your session or permission changed. Sign in again before saving.' : 'The changes could not be saved. Review the fields and try again.'));
      error.orderConflict = response.status===409 && form.matches('[data-fulfillment-form], [data-shiprocket-form]');
      error.categoryConflict = response.status===409 && form.matches('[data-category-publication]');
      throw error;
    } catch(error) {
      const banner=document.createElement('div'); banner.className='ui-form-error'; banner.role='alert'; banner.tabIndex=-1;
      banner.textContent=error.name==='AbortError' || error instanceof TypeError ? 'Could not confirm whether your changes were saved. Check the record before trying again.' : error.message;
      if(error.storefrontSummary)banner.replaceChildren(...Array.from(error.storefrontSummary.childNodes).map(node=>node.cloneNode(true)));
      if (error.customerSession) {
        const signIn=document.createElement('a');signIn.href='/login?next='+encodeURIComponent(location.pathname+location.search);signIn.textContent='Sign in again';banner.append(' ',signIn);
      }
      if(error.orderConflict) {
        const reload=document.createElement('a'); reload.href=location.pathname+location.search; reload.textContent='Reload latest order';
        banner.append(' ',reload);
      }
      if(error.categoryConflict) {
        const reload=document.createElement('a');const latest=new URL(location.href);latest.searchParams.delete('new');latest.searchParams.delete('edit');reload.href=latest.pathname+latest.search;reload.textContent='Reload latest categories';banner.append(' ',reload);
      }
      form.prepend(banner); banner.focus();
      if (productInvalidField) validate(form);
      buttons.forEach(button=>{button.disabled=form.hasAttribute('data-fulfillment-stale');busy(button,false);}); delete form.dataset.saving; form.removeAttribute('aria-busy');
    } finally { clearTimeout(timer); }
  });
  document.addEventListener('click',event=>{
    const button=event.target.closest('[data-mock-delivery]');
    if (!button) return;
    const form=button.form;
    form.elements.namedItem('carrier').value='Other';
    if (form.elements.namedItem('custom_carrier')) form.elements.namedItem('custom_carrier').value='Demo courier';
    form.elements.namedItem('tracking_number').value='DEMO-'+form.elements.namedItem('order_number').value;
    form.elements.namedItem('note').value='Local delivery simulation';
    form.requestSubmit(form.querySelector('[type="submit"]'));
  });
  document.querySelectorAll('.admin-product-editor-form [data-stock-adjustment],.admin-body [data-stock-adjustment], [data-stock-adjustment]').forEach(form=>{
    if(form.closest('.seller-shell'))return;
    const preview=form.querySelector('[data-stock-preview]');if(!preview)return;
    const update=()=>{const quantity=Number(form.elements.quantity.value||0),current=Number(form.dataset.available),direction=form.elements.adjustment_type.value==='remove'?-1:1;preview.textContent='Available: '+current+'; after change: '+(current+direction*quantity)+'. Reserved units unchanged.';};form.addEventListener('input',update);form.addEventListener('change',update);update();
  });
  document.addEventListener("click",event => {
    const step=event.target.closest("[data-checkout-step]");
    if (!step) return;
    requestAnimationFrame(()=> { const panel=document.querySelector('.checkout-card[data-step="'+step.dataset.checkoutStep+'"]'); const heading=panel?.querySelector("h2"); if (heading && panel.getClientRects().length) { heading.tabIndex=-1; heading.focus();
      const reduced=matchMedia('(prefers-reduced-motion: reduce)').matches;
      if (event.detail>0 && !reduced) { panel.getAnimations().forEach(animation=>animation.cancel()); panel.animate([{opacity:0.65,transform:'translateY(4px)'},{opacity:1,transform:'translateY(0)'}],{duration:160,easing:'cubic-bezier(0.23,1,0.32,1)'}); } } });
  });
  function orderRefreshControls(region, refresh) {
    const controls = document.createElement('div'); controls.className = 'order-live-controls';
    const state = document.createElement('span'); state.role = 'status'; state.className = 'order-refresh-state';
    const pause = document.createElement('button'); pause.type = 'button'; pause.className = 'text-button'; pause.textContent = 'Pause updates';
    const retry = document.createElement('button'); retry.type = 'button'; retry.className = 'text-button'; retry.textContent = 'Retry update'; retry.hidden = true;
    controls.append(state, pause, retry);
    region.before(controls); state.textContent = 'Live updates every 15 seconds';
    const result = { state, retry, paused: false, lastSuccess: "Not refreshed yet" };
    pause.addEventListener('click', () => {
      result.paused = !result.paused;
      pause.textContent = result.paused ? 'Resume updates' : 'Pause updates';
      state.textContent = result.paused ? 'Live updates paused' : 'Live updates every 15 seconds';
      if (!result.paused) refresh();
    });
    retry.addEventListener('click', refresh);
    return result;
  }
  function refreshTime() { return 'Last updated ' + new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }); }
  function startAdminOrderRefresh() {
    let region = document.querySelector('[data-admin-orders-region]');
    if (!region) return;
    let busy = false;
    const refresh = async () => {
      if (busy || document.hidden || controls.paused) return;
      busy = true;
      try {
        const response = await fetch('/ui/admin/orders/status' + location.search, { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'text/html' }, signal: AbortSignal.timeout(12000) });
        if (!response.ok || response.redirected) throw new Error('refresh failed');
        const parsed = document.createRange().createContextualFragment(await response.text());
        const next = parsed.querySelector('[data-admin-orders-region]');
        if (!next) throw new Error('refresh failed');
        if (controls.paused || document.hidden || region.contains(document.activeElement)) return;
        region.replaceWith(next);
        region = next;
        controls.lastSuccess = refreshTime(); controls.state.textContent = controls.lastSuccess;
        controls.retry.hidden = true;
      } catch (_) {
        controls.state.textContent = region.hasAttribute('data-order-queue-unavailable')
          ? 'Order queue unavailable. Retry the update.'
          : 'Live update unavailable. ' + controls.lastSuccess + '. The last saved queue is shown.';
        controls.retry.hidden = false;
      } finally {
        busy = false;
      }
    };
    const controls = orderRefreshControls(region, refresh);
    setInterval(refresh, 15000);
    document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
  }
  startAdminOrderRefresh();
  function startPaymentConfirmationRefresh() {
    const region = document.querySelector('[data-payment-confirmation]');
    if (!region || region.dataset.refreshStarted) return;
    region.dataset.refreshStarted = 'true';
    let active = true;
    let request;
    let timer;
    let initialTimer;
    const refresh = async (manual = false) => {
      if (request || document.hidden || !active) return;
      const current = region.querySelector('[data-confirmation-content]');
      const button = current?.querySelector('[data-check-payment-status]');
      const feedback = current?.querySelector('[data-payment-refresh-feedback]');
      if (!button || !feedback || (!manual && current.dataset.paymentPending !== 'true')) return;
      if (current.querySelector('[data-retry-payment][aria-busy="true"]')) return;
      const controller = new AbortController();
      request = controller;
      const timeout = setTimeout(() => controller.abort(), 20000);
      button.setAttribute('aria-disabled', 'true');
      if (manual) feedback.textContent = 'Checking the recorded payment status…';
      try {
        const response = await fetch('/ui/orders/status?view=confirmation&order=' + encodeURIComponent(region.dataset.orderNumber), {
          credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'text/html' }, signal: controller.signal
        });
        if (!response.ok || response.redirected) throw new Error('refresh failed');
        const fragment = document.createRange().createContextualFragment(await response.text());
        const next = fragment.querySelector('[data-confirmation-content]');
        if (!next) throw new Error('refresh incomplete');
        if (request !== controller || !active || current.querySelector('[data-retry-payment][aria-busy="true"]')) return;
        const oldConfig = current.querySelector('#checkout-payment-config');
        const nextConfig = next.querySelector('#checkout-payment-config');
        if (nextConfig && !nextConfig.dataset.contactEmail && oldConfig?.dataset.contactEmail) nextConfig.dataset.contactEmail = oldConfig.dataset.contactEmail;
        const focused = document.activeElement;
        const statusChanged = current.querySelector('h1')?.textContent !== next.querySelector('h1')?.textContent
          || Boolean(oldConfig) !== Boolean(nextConfig);
        if (manual || statusChanged || !current.contains(focused)) {
          const restoringFocus = current.contains(focused);
          current.replaceWith(next);
          const nextFeedback = next.querySelector('[data-payment-refresh-feedback]');
          if (nextFeedback && (manual || statusChanged)) nextFeedback.textContent = refreshTime();
          if (restoringFocus) {
            let target;
            if (focused === button) target = next.querySelector('[data-check-payment-status]');
            else if (focused.matches('[data-retry-payment]')) target = next.querySelector('[data-retry-payment]');
            else if (focused.matches('a[href]')) target = Array.from(next.querySelectorAll('a[href]')).find(link => link.href === focused.href);
            target ||= next.querySelector('h1');
            if (target?.matches('h1')) target.tabIndex = -1;
            target?.focus({preventScroll: true});
          }
        }
      } catch (_) {
        if (request === controller && active) feedback.textContent = 'Could not verify payment status. Your last saved status remains below. Check again before paying.';
      } finally {
        clearTimeout(timeout);
        if (request === controller) {
          request = null;
          button.removeAttribute('aria-disabled');
        }
      }
    };
    region.addEventListener('click', event => {
      if (event.target.closest('[data-check-payment-status]')) refresh(true);
    });
    document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
    window.addEventListener('pagehide', () => {
      active = false;
      clearInterval(timer);
      clearTimeout(initialTimer);
      request?.abort();
      request = null;
      region.querySelector('[data-check-payment-status]')?.removeAttribute('aria-disabled');
    });
    window.addEventListener('pageshow', event => {
      if (!event.persisted || active) return;
      active = true;
      timer = setInterval(() => refresh(), 15000);
      refresh();
    });
    timer = setInterval(() => refresh(), 15000);
    initialTimer = setTimeout(() => refresh(), 1500);
  }
  startPaymentConfirmationRefresh();
  function startLiveOrderView() {
    const sellerList = location.pathname === '/seller-admin/orders';
    const sellerDetail = /^\/seller-admin\/orders\/[^/]+$/.test(location.pathname);
    if (sellerDetail && new URLSearchParams(location.search).get('couriers') === '1') return;
    const customerList = location.pathname === '/orders';
    const selector = sellerList ? '[data-seller-orders-region]' : sellerDetail ? '[data-seller-order-detail]' : customerList ? '.orders-list[data-server-orders]' : '';
    let region = selector && document.querySelector(selector);
    if (!region) return;
    let busy = false;
    // Only the seller's editable fields determine whether a refresh can discard the form.
    const fulfillmentForm = root => root.querySelector('[data-fulfillment-form]');
    const shippingForm = root => root.querySelector('[data-shiprocket-form]');
    const shippingActions = root => Array.from(shippingForm(root)?.querySelectorAll('button[name="action"]') || [], button => button.value).join(',');
    const readDraft = root => JSON.stringify([
      ...['carrier', 'custom_carrier', 'tracking_number', 'note'].map(name => fulfillmentForm(root)?.elements.namedItem(name)?.value ?? ''),
      ...['weight_grams', 'length_cm', 'width_cm', 'height_cm', 'courier_id'].map(name => shippingForm(root)?.elements.namedItem(name)?.value ?? '')
    ]);
    const nextStage = form => form?.elements.namedItem('status')?.value;
    let savedDraft = sellerDetail ? readDraft(region) : '';
    function markSellerOrderStale() {
      const forms = region.querySelectorAll('[data-fulfillment-form], [data-shiprocket-form]');
      forms.forEach(form => {
        form.setAttribute('data-fulfillment-stale', '');
        form.querySelectorAll('[type="submit"], [data-mock-delivery]').forEach(button => { button.disabled = true; });
      });
      region.querySelectorAll('[data-print-kind]').forEach(button => { button.disabled = true; });
      if (region.querySelector('[data-order-conflict]')) return false;
      const conflict = document.createElement('p');
      conflict.dataset.orderConflict = 'true';
      conflict.role = 'alert';
      conflict.className = 'ui-form-error';
      conflict.append(forms.length
        ? 'This order changed while you were working. Your entries are still here. '
        : 'This order has changed. ');
      const link = document.createElement('a');
      link.href = location.pathname + location.search;
      link.textContent = 'Reload latest order';
      conflict.append(link);
      region.prepend(conflict);
      return true;
    }
    if (sellerDetail) {
      region.parentElement.addEventListener('submit', event => {
        if (!event.target.matches('[data-fulfillment-stale]')) return;
        event.preventDefault();
        event.stopImmediatePropagation();
      }, true);
    }
    const refresh = async () => {
      if (busy || document.hidden || controls.paused) return;
      busy = true;
      try {
        const response = await fetch(location.pathname + location.search, { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'text/html' }, signal: AbortSignal.timeout(12000) });
        if (sellerDetail && (response.redirected || [401, 403, 404].includes(response.status))) markSellerOrderStale();
        if (!response.ok || response.redirected) throw new Error('refresh failed');
        const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
        const next = doc.querySelector(selector);
        if (!next) throw new Error('refresh failed');
        if (controls.paused || document.hidden) return;
        const active = document.activeElement;
        if (sellerDetail) {
          const currentForm = fulfillmentForm(region);
          const incomingForm = fulfillmentForm(next);
          const preserve = region.contains(active) || currentForm?.dataset.saving === 'true'
            || shippingForm(region)?.dataset.saving === 'true' || readDraft(region) !== savedDraft;
          if (preserve) {
            const changed = next.dataset.fulfillmentStage !== region.dataset.fulfillmentStage
              || next.dataset.fulfillmentRevision !== region.dataset.fulfillmentRevision
              || next.dataset.orderStatus !== region.dataset.orderStatus
              || next.dataset.paymentStatus !== region.dataset.paymentStatus
              || next.hasAttribute('data-payment-review') !== region.hasAttribute('data-payment-review')
              || Boolean(currentForm) !== Boolean(incomingForm)
              || nextStage(currentForm) !== nextStage(incomingForm)
              || shippingActions(region) !== shippingActions(next);
            if (changed && markSellerOrderStale()) {
              controls.state.textContent = 'Order changed. Reload for the latest details.';
            }
            return;
          }
          region.replaceWith(next);
          region = next;
          savedDraft = readDraft(region);
          init(region);
          document.dispatchEvent(new CustomEvent('ui:seller-order-refresh', { detail: region }));
        } else if (!region.contains(active)) {
          region.replaceWith(next);
          region = next;
          if (customerList) init(region);
        }
        if (customerList) {
          const counts = document.querySelector('.account-intro .tabs');
          const nextCounts = doc.querySelector('.account-intro .tabs');
          if (counts && nextCounts && !counts.contains(active)) counts.replaceWith(nextCounts);
        }
        controls.lastSuccess = refreshTime(); controls.state.textContent = controls.lastSuccess; controls.retry.hidden = true;
      } catch (_) { controls.state.textContent = 'Live update unavailable. ' + controls.lastSuccess + '. The last saved orders are shown.'; controls.retry.hidden = false; }
      finally { busy = false; }
    };
    const controls = orderRefreshControls(region, refresh);
    setInterval(refresh, 15000);
    document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
  }
  startLiveOrderView();
  // Refresh badges across all role portals without exposing conversation bodies.
  let notificationRead = null;
  let notificationPageActive = true;
  async function refreshNotifications() {
    if (!notificationPageActive || notificationRead || document.hidden || !document.querySelector('[data-notification-bell]')) return;
    const controller = new AbortController();
    notificationRead = controller;
    try {
      const response=await fetch('/ui/notifications',{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'},signal:controller.signal});
      if (!response.ok || !response.headers.get('content-type')?.includes('application/json')) return;
      const data=await response.json();
      document.querySelectorAll('[data-notification-bell]').forEach(bell=> {
        const count=bell.querySelector('[data-notification-count]');
        if (!count) return;
        count.hidden=!data.unread;count.textContent=data.unread>99?'99+':String(data.unread||'');
        bell.setAttribute('aria-label',data.unread?'Notifications, '+data.unread+' unread':'Notifications');
      });
    } catch (_) { /* The inbox remains usable when a badge refresh fails. */ }
    finally { if (notificationRead === controller) notificationRead = null; }
  }
  window.addEventListener('pagehide', () => { notificationPageActive = false; notificationRead?.abort(); });
  window.addEventListener('beforeunload', () => { notificationPageActive = false; notificationRead?.abort(); });
  const resumeNotifications=()=>{ if(!document.hidden) notificationPageActive=true; };
  window.addEventListener('pointerdown',resumeNotifications);
  window.addEventListener('keydown',resumeNotifications);
  window.addEventListener('pageshow', event => { notificationPageActive = true; if (event.persisted) refreshNotifications(); });
  refreshNotifications();
  setInterval(refreshNotifications,30000);
  document.addEventListener('visibilitychange',refreshNotifications);
  document.addEventListener('htmx:afterSettle',refreshNotifications);
  document.addEventListener('keydown',event=> {
    if (event.key==='Escape' && event.target.closest('.search-shell,.seller-search,.admin-search')) {
      event.target.blur();
      document.querySelector('.menu-toggle,.seller-menu-button,.admin-menu-button')?.focus();
    }
  });
  document.addEventListener('click',event=> {
    const button=event.target.closest('.global-search button[type="submit"],.seller-search button[type="submit"],.admin-search button[type="submit"]');
    const input=button?.form.querySelector('input[name="q"]');
    if (innerWidth<=(button?.closest('.admin-search')?960:800) && input && document.activeElement!==input && !input.value.trim()) {
      event.preventDefault();event.stopImmediatePropagation();input.focus();
    }
  },true);
})();
