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
  function init(root) {
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
  window.WVUI = {validate,confirmAction};
  document.addEventListener("DOMContentLoaded",()=>init(document));
  document.addEventListener("htmx:afterSwap",()=>init(document));
  // Retain typed values on a rejected save, and block repeated POSTs while saving.
  // Do not automatically retry a request whose completion is uncertain.
  document.addEventListener("submit",async event=> {
    const form=event.target;
    if(event.defaultPrevented || form.method.toLowerCase()!=="post" || form.hasAttribute('hx-post')) return;
    const action=new URL(form.action,location.href);
    if(action.origin!==location.origin || !/^\/(seller-admin|support-portal|marketplace-admin|super-admin|finance-portal|operations-portal)\//.test(action.pathname)) return;
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
  document.addEventListener("click",event => {
    const step=event.target.closest("[data-checkout-step]");
    if (!step) return;
    requestAnimationFrame(()=> { const panel=document.querySelector('.checkout-card[data-step="'+step.dataset.checkoutStep+'"]'); const heading=panel?.querySelector("h2"); if (heading && panel.getClientRects().length) { heading.tabIndex=-1; heading.focus(); } });
  });
})();
