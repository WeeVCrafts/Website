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
    if (first) { first.focus(); first.scrollIntoView({block:"center",behavior:"instant"}); }
    return !first;
  }
  function confirmAction(message, trigger, actionLabel) {
    return new Promise(resolve => {
      const dialog = document.createElement("dialog");
      dialog.className = "ui-confirm";
      dialog.setAttribute("aria-labelledby", "ui-confirm-title");
      const heading = document.createElement("h2"); heading.id = "ui-confirm-title"; heading.textContent = actionLabel || "Confirm action";
      const copy = document.createElement("p"); copy.id = "ui-confirm-copy"; copy.textContent = message; dialog.setAttribute("aria-describedby",copy.id);
      const actions = document.createElement("div"); actions.className = "ui-confirm-actions";
      const cancel = document.createElement("button"); cancel.type = "button"; cancel.textContent = "Cancel"; cancel.autofocus = true;
      const accept = document.createElement("button"); accept.type = "button"; accept.textContent = actionLabel || "Continue"; accept.dataset.confirmAccept = "";
      actions.append(cancel, accept); dialog.append(heading, copy, actions); document.body.append(dialog);
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
  function init(root) {
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
      form.addEventListener("input",event => { if (event.target.dataset.uiError) clearError(event.target); });
      form.addEventListener("submit",async event => {
        if (!validate(form)) { event.preventDefault(); event.stopImmediatePropagation(); return; }
        if (form.dataset.confirm && !form.dataset.confirmed) {
          event.preventDefault(); event.stopImmediatePropagation();
          const submitter = event.submitter || form.querySelector('[type="submit"]');
          if (await confirmAction(form.dataset.confirm, submitter, submitter?.textContent.trim())) {
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
      img.addEventListener("error",fallback,{once:true}); if (img.complete && !img.naturalWidth) fallback();
    });
  }
  window.WVUI = {validate,confirmAction,busy};
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
  // Retain typed values on a rejected save, and block repeated POSTs while saving.
  // Do not automatically retry a request whose completion is uncertain.
  document.addEventListener("submit",async event=> {
    const form=event.target;
    if(event.defaultPrevented || form.method.toLowerCase()!=="post" || form.hasAttribute('hx-post')) return;
    const action=new URL(form.getAttribute("action") || location.href,location.href);
    if(action.origin!==location.origin || !/^\/(admin|seller-admin|support-portal|marketplace-admin|super-admin|finance-portal|operations-portal)\//.test(action.pathname)) return;
    event.preventDefault();
    if(form.dataset.saving) return;
    const payload=new FormData(form);
    if(event.submitter?.name) payload.append(event.submitter.name,event.submitter.value);
    const buttons=Array.from(form.querySelectorAll('button[type="submit"],input[type="submit"]')).filter(button=>!button.disabled);
    const oldError=form.querySelector('.ui-form-error'); oldError?.remove();
    form.dataset.saving="true"; form.setAttribute('aria-busy','true'); buttons.forEach(button=>button.disabled=true);
    const controller=new AbortController(); const timer=setTimeout(()=>controller.abort(),30000);
    try {
      const response=await fetch(action.href,{method:'POST',credentials:'same-origin',body:payload,signal:controller.signal});
      if(response.ok && response.redirected) { location.assign(response.url); return; }
      const html=await response.text(); const doc=new DOMParser().parseFromString(html,'text/html');
      const message=doc.querySelector('.error-message,.error-state p,main p')?.textContent?.trim();
      throw new Error(message || (response.status===403 ? 'Your session or permission changed. Sign in again before saving.' : 'The changes could not be saved. Review the fields and try again.'));
    } catch(error) {
      const banner=document.createElement('div'); banner.className='ui-form-error'; banner.role='alert'; banner.tabIndex=-1;
      banner.textContent=error.name==='AbortError' || error instanceof TypeError ? 'Could not confirm whether your changes were saved. Check the record before trying again.' : error.message;
      form.prepend(banner); banner.focus();
      buttons.forEach(button=>button.disabled=false); delete form.dataset.saving; form.removeAttribute('aria-busy');
    } finally { clearTimeout(timer); }
  });
  document.addEventListener('click',event=>{
    const button=event.target.closest('[data-mock-delivery]');
    if (!button) return;
    const form=button.form;
    form.elements.namedItem('carrier').value='Other';
    form.elements.namedItem('custom_carrier').value='Demo courier';
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
  // Refresh badges across all role portals without exposing conversation bodies.
  async function refreshNotifications() {
    if (document.hidden || !document.querySelector('[data-notification-bell]')) return;
    try {
      const response=await fetch('/ui/notifications',{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}});
      if (!response.ok || !response.headers.get('content-type')?.includes('application/json')) return;
      const data=await response.json();
      document.querySelectorAll('[data-notification-bell]').forEach(bell=> {
        const count=bell.querySelector('[data-notification-count]');
        if (!count) return;
        count.hidden=!data.unread;count.textContent=data.unread>99?'99+':String(data.unread||'');
        bell.setAttribute('aria-label',data.unread?'Notifications, '+data.unread+' unread':'Notifications');
      });
    } catch (_) { /* The inbox remains usable when a badge refresh fails. */ }
  }
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
