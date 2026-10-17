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
  function confirmAction(message, trigger, actionLabel, options = {}) {
    return new Promise(resolve => {
      const dialog = document.createElement("dialog");
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
      const small = () => getComputedStyle(trigger).display !== 'none';
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
  function init(root) {
    initRoleNavigation(root);
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
      if (form.method.toLowerCase() === "post" && customerFormPath(new URL(form.action, location.href).pathname)) form.dataset.customerRetain = "true";
      form.addEventListener("input",event => { if (event.target.dataset.uiError) clearError(event.target); });
      form.addEventListener("submit",async event => {
        if (form.dataset.wizardStep !== undefined && form.dataset.wizardStep !== 'review') {
          event.preventDefault(); event.stopImmediatePropagation();
          form.dispatchEvent(new CustomEvent('ui:wizard-next')); return;
        }
        if (!validate(form)) { event.preventDefault(); event.stopImmediatePropagation(); return; }
        const confirmation=event.submitter?.dataset.confirm||form.dataset.confirm;
        if (confirmation && !form.dataset.confirmed) {
          event.preventDefault(); event.stopImmediatePropagation();
          const submitter = event.submitter || form.querySelector('[type="submit"]');
          const intent = /\/delete$/.test(form.action) ? 'danger' : /\/(archive|suspend)$/.test(form.action) ? 'warning' : 'primary';
          if (await confirmAction(confirmation, submitter, submitter?.textContent.trim(), {intent})) {
            form.dataset.confirmed = "true"; form.requestSubmit(submitter); delete form.dataset.confirmed;
          }
        }
      },true);
    });
    root.querySelectorAll("textarea").forEach(field => {
      if (field.dataset.growBound) return;
      field.dataset.growBound = "true";
      const grow=()=> { field.style.height="auto"; field.style.height=Math.min(400, Math.max(120,field.scrollHeight))+"px"; };
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
        const img=document.createElement('img'); img.src=previewURL; img.alt="Selected image preview"; img.className="maker-portrait-preview";
        preview.replaceChildren(img);
      });
    });
    root.querySelectorAll("img").forEach(img => {
      if (img.dataset.fallbackBound) return;
      img.dataset.fallbackBound="true";
      const fallback=()=> { if (!img.src.endsWith("/product-unavailable.svg")) { img.removeAttribute("srcset"); img.src="/assets/images/product-unavailable.svg"; } };
      img.addEventListener("error",fallback); if (img.complete && !img.naturalWidth && !img.hasAttribute('data-src')) fallback();
    });
  }
  window.WVUI = {validate,confirmAction,busy,enhance:root=>init(root||document),markDirty:form=>dirtyForms.add(form)};
  document.addEventListener('keydown',()=>{document.body.dataset.keyboardInput='true';},true);
  document.addEventListener('pointerdown',()=>{delete document.body.dataset.keyboardInput;},true);
  const customerFormPath = path => /^\/account\/(profile|addresses(?:\/delete)?|notifications|reviews)$/.test(path) || /^\/support\/(create|customer-reply)$/.test(path) || /^\/privacy\/(consent|requests)$/.test(path) || /^\/orders\/[^/]+\/(cancel|return|refund)$/.test(path);
  // Protect long edits in memory. Do not persist account/product form values locally.
  const dirtyForms = new Set();
  const dirtySelector = '[data-product-builder], .admin-product-editor-form, .role-user-edit-form, [data-storefront-dirty], [data-customer-retain]';
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
    if(event.defaultPrevented || form.method.toLowerCase()!=="post" || form.hasAttribute('hx-post')) return;
    const action=new URL(event.submitter?.getAttribute("formaction") || form.getAttribute("action") || location.href,location.href);
    const customerForm = customerFormPath(action.pathname);
    if(action.origin!==location.origin || (!customerForm && !/^\/(admin|seller-admin|support-portal|marketplace-admin|super-admin|finance-portal|operations-portal)\//.test(action.pathname))) return;
    event.preventDefault();
    if(form.dataset.saving) return;
    const payload=new FormData(form);
    if(event.submitter?.name) payload.append(event.submitter.name,event.submitter.value);
    const buttons=Array.from(form.querySelectorAll('button[type="submit"],input[type="submit"]')).filter(button=>!button.disabled);
    const oldError=form.querySelector('.ui-form-error'); oldError?.remove();
    form.dataset.saving="true"; form.setAttribute('aria-busy','true'); buttons.forEach(button=>{button.disabled=true;busy(button,true);});
    const controller=new AbortController(); const timer=setTimeout(()=>controller.abort(),30000);
    let productInvalidField = null;
    try {
      const response=await fetch(action.href,{method:'POST',credentials:'same-origin',headers:{'X-UI-Form':'true'},body:customerForm && form.enctype!=='multipart/form-data' ? new URLSearchParams(payload) : payload,signal:controller.signal});
      if (customerForm && response.redirected && new URL(response.url).pathname === '/login') {
        const error = new Error('Your session ended. Your entries are still here. Copy your changes before signing in again.');
        error.customerSession = true; throw error;
      }
      if(response.ok && response.redirected) { location.assign(response.url); return; }
      const html=await response.text(); const doc=new DOMParser().parseFromString(html,'text/html');
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
      error.orderConflict = response.status===409 && form.matches('[data-fulfillment-form]');
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
      form.prepend(banner); banner.focus();
      if (productInvalidField) validate(form);
      buttons.forEach(button=>{button.disabled=false;busy(button,false);}); delete form.dataset.saving; form.removeAttribute('aria-busy');
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
    // Keep the controls outside the seller detail's two-column layout.
    const anchor = region.matches('[data-seller-order-detail]') ? region.parentElement : region;
    anchor.before(controls); state.textContent = 'Live updates every 15 seconds';
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
        const response = await fetch('/ui/admin/orders/status' + location.search, { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'text/html' } });
        if (!response.ok) throw new Error('refresh failed');
        const parsed = document.createRange().createContextualFragment(await response.text());
        const next = parsed.querySelector('[data-admin-orders-region]');
        if (next) {
          const active = document.activeElement;
          if (!region.contains(active)) { region.replaceWith(next); region = next; }
        } else throw new Error('refresh failed');
        controls.lastSuccess = refreshTime(); controls.state.textContent = controls.lastSuccess;
        controls.retry.hidden = true;
      } catch (_) {
        controls.state.textContent = 'Live update unavailable. ' + controls.lastSuccess + '. The last saved queue is shown.';
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
  function startLiveOrderView() {
    const sellerList = location.pathname === '/seller-admin/orders';
    const sellerDetail = /^\/seller-admin\/orders\/[^/]+$/.test(location.pathname);
    const customerList = location.pathname === '/orders';
    const selector = sellerList ? '[data-seller-orders-region]' : sellerDetail ? '[data-seller-order-detail]' : customerList ? '.orders-list[data-server-orders]' : '';
    let region = selector && document.querySelector(selector);
    if (!region) return;
    let busy = false;
    const refresh = async () => {
      if (busy || document.hidden || controls.paused) return;
      busy = true;
      try {
        const response = await fetch(location.pathname + location.search, { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'text/html' } });
        if (!response.ok || response.redirected) throw new Error('refresh failed');
        const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
        const next = doc.querySelector(selector);
        if (!next) throw new Error('refresh failed');
        const active = document.activeElement;
        // Preserve a focused button/link until the next refresh opportunity.
        if (!region.contains(active)) { region.replaceWith(next); region = next; if (customerList) init(region); }
        if (customerList) {
          const counts = document.querySelector('.account-intro .tabs');
          const nextCounts = doc.querySelector('.account-intro .tabs');
          if (counts && nextCounts && !counts.contains(active)) counts.replaceWith(nextCounts);
        }
        if (sellerDetail) {
          const form = document.querySelector('[data-fulfillment-form]');
          const latestRevision = doc.querySelector('[name="expected_revision"]')?.value;
          const currentRevision = form?.elements.namedItem('expected_revision')?.value;
          if (form && latestRevision !== undefined && latestRevision !== currentRevision && !form.querySelector('[data-order-conflict]')) {
            const conflict = document.createElement('p'); conflict.dataset.orderConflict = 'true'; conflict.role = 'alert'; conflict.className = 'ui-form-error';
            conflict.append('Another session updated this order. Your entered details are preserved. ');
            const link = document.createElement('a'); link.href = location.pathname; link.textContent = 'Reload the latest stage before saving'; conflict.append(link); form.prepend(conflict);
          }
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
