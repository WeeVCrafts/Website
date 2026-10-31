(function () {
  "use strict";

  // Presentation enhancement only. Existing forms/routes remain authoritative.
  const mobileFilters = matchMedia("(max-width: 800px)");
  const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");
  let filterController = null;
  let filterSnapshot = null;
  let pendingPageRequest = null;
  let purchaseObserver = null, purchaseBar = null;
  const focusable = "button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),summary,[tabindex='0']";
  const visible = element => !element.hidden && !element.closest("[inert]") && element.getClientRects().length > 0;

  function updateSearchPlaceholder() {
    const input = document.getElementById("global-search");
    if (!input) return;
    input.dataset.desktopPlaceholder ||= input.placeholder;
    input.placeholder = mobileFilters.matches ? "Search handmade crafts..." : input.dataset.desktopPlaceholder;
  }
  mobileFilters.addEventListener("change", updateSearchPlaceholder);

  function loadMedia(element) {
    if (!element) return;
    // Install responsive candidates before src so activation cannot start an
    // unnecessary original-sized request. Inactive media has neither attribute.
    if (element.dataset.srcset) {
      element.srcset = element.dataset.srcset;
      delete element.dataset.srcset;
    }
    if (element.dataset.src) {
      element.src = element.dataset.src;
      delete element.dataset.src;
    }
    if (element.tagName === "VIDEO") {
      const source = element.querySelector("source[data-src]");
      if (source) { source.src = source.dataset.src; delete source.dataset.src; element.load(); }
    }
  }

  const heroControllers = new Map();
  function initHero(root) {
    for (const [element, controller] of heroControllers) {
      if (!element.isConnected) { controller.destroy(); heroControllers.delete(element); }
    }
    root.querySelectorAll("[data-hero]").forEach(hero => {
      if (heroControllers.has(hero)) return;
      const slides = Array.from(hero.querySelectorAll("[data-hero-slide]"));
      if (!slides.length) return;
      const controls = hero.querySelector("[data-hero-controls]");
      const selectors = Array.from(hero.querySelectorAll("[data-hero-select]"));
      const status = hero.querySelector("[data-hero-status]");
      const listeners = new AbortController();
      let active = 0, requested = 0, timer = null, paused = reducedMotion.matches, hovered = false;
      let inView = false, keyboardFocused = false, request = 0, destroyed = false, touch = null;
      let fade = null;
      const settle = () => {
        if (fade) { fade.forEach(animation => animation.cancel()); fade = null; }
        slides.forEach((slide, i) => {
          slide.hidden = i !== active; slide.inert = i !== active;
          slide.style.removeProperty('z-index');
        });
      };
      const clear = () => { clearTimeout(timer); timer = null; };
      const canPlay = () => document.body.dataset.storefrontEditing !== "true" && !paused && !hovered && !keyboardFocused && inView && !document.hidden && !destroyed && hero.isConnected;
      const sync = () => {
        clear();
        hero.dataset.playing = String(canPlay());
        if (canPlay() && slides.length > 1) timer = setTimeout(() => show(active + 1, false), 4000);
      };
      const ready = img => new Promise(resolve => {
        if (!img) return resolve(true);
        // A hidden loading=lazy image will not fetch until displayed. Activate
        // it eagerly on selection so we can keep the current slide visible.
        img.loading = "eager";
        loadMedia(img);
        let timeout;
        const done = ok => { clearTimeout(timeout); img.removeEventListener("load", loaded); img.removeEventListener("error", failed); listeners.signal.removeEventListener("abort", failed); resolve(ok); };
        const loaded = () => { if (img.decode) img.decode().then(() => done(true), () => done(img.naturalWidth > 0)); else done(true); };
        const failed = () => done(false);
        if (img.complete) { if (img.naturalWidth > 0) loaded(); else failed(); return; }
        img.addEventListener("load", loaded, {once:true}); img.addEventListener("error", failed, {once:true});
        listeners.signal.addEventListener("abort", failed, {once:true});
        timeout = setTimeout(failed, 8000);
      });
      async function show(index, manual = true) {
        clear();
        const version = ++request;
        let next = (index + slides.length) % slides.length;
        requested = next;
        let loaded = false;
        for (let attempts = 0; attempts < (manual ? 1 : slides.length); attempts++) {
          loaded = await ready(slides[next].querySelector("img"));
          if (version !== request || destroyed) return;
          if (loaded) break;
          next = (next + 1) % slides.length;
        }
        if (!loaded) {
          requested = active;
          status.textContent = "This collection image could not load. Your current collection is still available.";
          sync(); return;
        }
        // A hover/focus/visibility change during loading must not advance playback.
        if (!manual && !canPlay()) { sync(); return; }
        settle();
        const previous = active;
        active = next;
        requested = active;
        slides.forEach((slide, i) => { slide.inert = i !== active; });
        slides[active].hidden = false;
        selectors.forEach((button, i) => button.setAttribute("aria-pressed", String(i === active)));
        hero.querySelector("[data-hero-count]").textContent = (active + 1) + " / " + slides.length;
        if (manual) status.textContent = "Collection " + (active + 1) + " of " + slides.length + ": " + slides[active].querySelector("h1").textContent;
        if (previous !== active && !reducedMotion.matches) {
          slides[active].style.zIndex = '1';
          const animations = [
            slides[previous].animate([{opacity:1},{opacity:0}],{duration:450,easing:'ease-out',fill:'forwards'}),
            slides[active].animate([{opacity:0},{opacity:1}],{duration:450,easing:'ease-out',fill:'forwards'})
          ];
          fade = animations;
          Promise.all(animations.map(animation => animation.finished)).then(() => {
            if (fade === animations) settle();
          }).catch(() => {});
        } else settle();
        sync();
      }
      const on = (element, type, callback, options = {}) => element?.addEventListener(type, callback, {...options, signal:listeners.signal});
      controls.hidden = false;
      slides.forEach((slide, i) => { slide.hidden = i !== 0; slide.inert = i !== 0; });
      on(hero.querySelector("[data-hero-prev]"), "click", () => show(requested - 1));
      on(hero.querySelector("[data-hero-next]"), "click", () => show(requested + 1));
      selectors.forEach((button, i) => on(button, "click", () => show(i)));
      on(hero, "pointerenter", event => { if (event.pointerType === "mouse") { hovered = true; sync(); } });
      on(hero, "pointerleave", () => { hovered = false; sync(); });
      on(hero, "focusin", event => { keyboardFocused = event.target.matches(":focus-visible"); ++request; sync(); });
      on(hero, "focusout", event => { if (!hero.contains(event.relatedTarget)) { keyboardFocused = false; ++request; sync(); } });
      on(document, "visibilitychange", () => { ++request; sync(); });
      on(document,"weevcrafts:preview-mode",()=>{++request;settle();sync();});
      on(document,"weevcrafts:preview-slide",event=>{if(document.body.dataset.storefrontEditing==='true')show(event.detail.index);});
      on(reducedMotion, "change", () => { paused = reducedMotion.matches; ++request; settle(); sync(); });
      on(hero, "touchstart", event => { if (event.touches.length === 1) touch = {x:event.touches[0].clientX,y:event.touches[0].clientY}; }, {passive:true});
      on(hero, "touchend", event => {
        if (!touch || !event.changedTouches.length) return;
        const dx = event.changedTouches[0].clientX - touch.x, dy = event.changedTouches[0].clientY - touch.y;
        touch = null;
        if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5) show(requested + (dx < 0 ? 1 : -1));
      }, {passive:true});
      const observer = new IntersectionObserver(entries => { inView = entries[0].isIntersecting; ++request; sync(); }, {threshold:.1});
      observer.observe(hero);
      heroControllers.set(hero, {destroy(){ destroyed = true; ++request; clear(); settle(); observer.disconnect(); listeners.abort(); }});
      sync();
    });
  }

  function initProductRails(root) {
    root.querySelectorAll("[data-product-rail]").forEach(section => {
      if (section.dataset.bound) return;
      section.dataset.bound = "true";
      const rail = section.querySelector("[data-rail-track]");
      section.querySelectorAll("[data-rail-direction]").forEach(button => button.addEventListener("click", () => {
        rail.scrollBy({left: Number(button.dataset.railDirection) * rail.clientWidth * .8, behavior:reducedMotion.matches ? "instant" : "smooth"});
      }));
    });
  }

  const categoryRails = new Map();
  function initCategoryRails() {
    for (const [element, controller] of categoryRails) {
      if (!element.isConnected) { controller.destroy(); categoryRails.delete(element); }
    }
    document.querySelectorAll('[data-category-rail]').forEach(section => {
      if (categoryRails.has(section)) return;
      const track = section.querySelector('[data-category-track]');
      const buttons = Array.from(section.querySelectorAll('[data-category-direction]'));
      if (!track) return;
      const listeners = new AbortController();
      const update = () => {
        const overflow = track.scrollWidth > section.clientWidth + 2;
        section.classList.toggle('has-overflow', overflow);
        buttons.forEach(button => {
          // Do not discard keyboard focus when resizing the row to fit.
          if (!overflow && document.activeElement === button) track.querySelector('a')?.focus({preventScroll:true});
          button.hidden = !overflow;
          button.disabled = Number(button.dataset.categoryDirection) < 0
            ? track.scrollLeft <= 2 : track.scrollLeft + track.clientWidth >= track.scrollWidth - 2;
        });
      };
      buttons.forEach(button => button.addEventListener('click', () => {
        track.scrollBy({left:Number(button.dataset.categoryDirection) * track.clientWidth * .8, behavior:reducedMotion.matches ? 'instant' : 'smooth'});
      }, {signal:listeners.signal}));
      track.addEventListener('scroll', update, {passive:true, signal:listeners.signal});
      const resize = new ResizeObserver(update);
      resize.observe(track);
      const observeItems = () => { resize.disconnect(); resize.observe(track); Array.from(track.children).forEach(item => resize.observe(item)); update(); };
      const mutation = new MutationObserver(observeItems);
      mutation.observe(track, {childList:true, subtree:true});
      observeItems();
      categoryRails.set(section, {destroy(){listeners.abort();resize.disconnect();mutation.disconnect();}});
    });
  }

  function initFilters() {
    const form = document.querySelector("[data-listing-filters]");
    if (filterController?.form === form) return;
    filterController?.destroy();
    filterController = null;
    if (!form) { filterSnapshot = null; return; }
    const panel = form.querySelector("[data-filter-panel]");
    const trigger = form.querySelector("[data-filter-open]");
    const backdrop = form.querySelector(".filter-backdrop");
    // Result feedback belongs inside the active modal as well as the page.
    const sheetFeedback = document.createElement('p');
    sheetFeedback.className = 'listing-feedback'; sheetFeedback.dataset.listingFeedback = '';
    sheetFeedback.setAttribute('role','status'); sheetFeedback.setAttribute('aria-live','polite');
    panel.querySelector('.filter-done').before(sheetFeedback);
    const listeners = new AbortController();
    let blocked = [];
    let isOpen = false;
    let oldOverflow = "";
    function restoreBackground() {
      blocked.forEach(([element, inert]) => { element.inert = inert; });
      blocked = [];
      document.body.style.overflow = oldOverflow;
    }
    function close(restoreFocus = true) {
      if (isOpen) restoreBackground();
      isOpen = false;
      panel.classList.remove("is-open");
      panel.removeAttribute("aria-modal");
      panel.removeAttribute("role");
      panel.removeAttribute("aria-labelledby");
      panel.inert = mobileFilters.matches;
      trigger.setAttribute("aria-expanded", "false");
      backdrop.hidden = true;
      if (restoreFocus && trigger.isConnected) trigger.focus({preventScroll:true});
    }
    function open(moveFocus = true) {
      if (!mobileFilters.matches || isOpen) return;
      isOpen = true;
      panel.inert = false;
      panel.classList.add("is-open");
      panel.setAttribute("role", "dialog");
      panel.setAttribute("aria-modal", "true");
      panel.setAttribute("aria-labelledby", "filter-title");
      backdrop.hidden = false;
      trigger.setAttribute("aria-expanded", "true");
      // Block siblings at each level, not the form/ancestors containing the sheet.
      let branch = panel;
      while (branch.parentElement && branch.parentElement !== document.documentElement) {
        for (const sibling of branch.parentElement.children) {
          if (sibling === branch || sibling === backdrop || sibling.tagName === "SCRIPT") continue;
          blocked.push([sibling, sibling.inert]);
          sibling.inert = true;
        }
        branch = branch.parentElement;
      }
      oldOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      if (moveFocus) panel.querySelector("[data-filter-close]").focus({preventScroll:true});
    }
    function updateViewport() { close(false); panel.inert = mobileFilters.matches; }
    trigger.addEventListener("click", () => open(), {signal:listeners.signal});
    form.querySelectorAll("[data-filter-close]").forEach(button => button.addEventListener("click", () => close(), {signal:listeners.signal}));
    panel.addEventListener("keydown", event => {
      if (!isOpen) return;
      if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); close(); }
      if (event.key !== "Tab") return;
      const items = Array.from(panel.querySelectorAll(focusable)).filter(visible);
      const first = items[0], last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    }, {signal:listeners.signal});
    mobileFilters.addEventListener("change", updateViewport, {signal:listeners.signal});
    panel.inert = mobileFilters.matches;
    filterController = {form, panel, open, close, get isOpen(){return isOpen;}, destroy(){close(false); listeners.abort();}};
    if (filterSnapshot) {
      const state = filterSnapshot;
      filterSnapshot = null;
      panel.querySelectorAll("details").forEach((details,i) => { if (state.details[i] !== undefined) details.open = state.details[i]; });
      queueMicrotask(() => { for (const [id,value] of state.facets) {
        const input = document.getElementById(id);
        if (input) { input.value = value; input.dispatchEvent(new Event("input",{bubbles:true})); }
      } });
      if (state.open) { open(false); panel.scrollTop = state.scroll; }
      let next;
      if (state.focus.id) next = document.getElementById(state.focus.id);
      else if (state.focus.name) next = Array.from(form.elements).find(element => element.name === state.focus.name && element.value === state.focus.value);
      if (next && visible(next)) next.focus({preventScroll:true});
      else if (state.open) panel.querySelector("[data-filter-close]").focus({preventScroll:true});
      form.querySelectorAll('[data-listing-feedback]').forEach(feedback => { feedback.textContent = form.querySelector('[data-backend-result-count]').textContent; });
    }
  }

  function captureFilters() {
    if (!filterController) return;
    const {form,panel,isOpen} = filterController;
    const active = document.activeElement;
    filterSnapshot = {
      open:isOpen, scroll:panel.scrollTop,
      details:Array.from(panel.querySelectorAll("details")).map(element=>element.open),
      facets:Array.from(panel.querySelectorAll(".facet-search input")).map(element=>[element.id,element.value]),
      focus:form.contains(active) ? {id:active.id,name:active.name,value:active.value} : {}
    };
  }

  function initGallery(root) {
    root.querySelectorAll("[data-ui-gallery]").forEach(gallery => {
      if (gallery.dataset.bound) return;
      gallery.dataset.bound = "true";
      const media = Array.from(gallery.querySelectorAll("[data-gallery-media]"));
      const thumbnails = Array.from(gallery.querySelectorAll("[data-gallery-select]"));
      const zoom = gallery.querySelector("[data-gallery-zoom]");
      let active = 0;
      let gesture;
      const videoFeedback = document.createElement('p'); videoFeedback.className='gallery-video-feedback';videoFeedback.hidden=true;videoFeedback.setAttribute('role','status');
      const videoMessage=document.createElement('span'), retryVideo=document.createElement('button');retryVideo.type='button';retryVideo.textContent='Retry video';retryVideo.hidden=true;
      videoFeedback.append(videoMessage,retryVideo);gallery.append(videoFeedback);
      function videoState() {
        const video=media[active];videoFeedback.hidden=video.tagName!=='VIDEO';
        if(video.tagName!=='VIDEO')return;
        retryVideo.hidden=video.dataset.failed!=='true';
        videoMessage.textContent=video.dataset.failed==='true'?'Video could not load. Choose another image or retry.':video.readyState>=2?'Video ready. Use the playback controls.':'Loading product video…';
      }
      media.filter(element=>element.tagName==='VIDEO').forEach(video=>{
        const failed=()=>{video.dataset.failed='true';videoState();};
        video.addEventListener('error',failed);
        video.querySelectorAll('source').forEach(source=>source.addEventListener('error',failed));
        video.addEventListener('canplay',()=>{delete video.dataset.failed;videoState();});
      });
      retryVideo.addEventListener('click',()=>{const video=media[active];if(video.tagName!=='VIDEO')return;delete video.dataset.failed;video.load();videoState();});
      function show(index, announce = true) {
        active = (index + media.length) % media.length;
        media.forEach((element,i)=>{
          element.hidden = i !== active;
          if (element.tagName === "VIDEO" && i !== active) element.pause();
        });
        loadMedia(media[active]);
        videoState();
        thumbnails.forEach((button,i)=>{
          button.setAttribute("aria-pressed",String(i===active));
          button.setAttribute("aria-label", "Show " + (media[i]?.tagName === "VIDEO" ? "video" : "image") + " " + (i+1) + " of " + media.length);
        });
        gallery.querySelector("[data-gallery-count]").textContent = (active+1) + " / " + media.length;
        zoom.hidden = media[active].tagName === "VIDEO";
        if (announce) gallery.querySelector("[data-gallery-status]").textContent = "Showing " + (media[active].tagName === "VIDEO" ? "video" : "image") + " " + (active+1) + " of " + media.length;
      }
      gallery.querySelector("[data-gallery-controls]").hidden = media.length < 2;
      thumbnails.forEach((button,i)=>button.addEventListener("click",()=>show(i)));
      gallery.querySelector("[data-gallery-prev]").addEventListener("click",()=>show(active-1));
      gallery.querySelector("[data-gallery-next]").addEventListener("click",()=>show(active+1));
      gallery.querySelector(".thumbs").addEventListener("keydown",event=>{
        if (!["ArrowLeft","ArrowRight","Home","End"].includes(event.key)) return;
        event.preventDefault();
        show(event.key==="Home"?0:event.key==="End"?media.length-1:active+(event.key==="ArrowRight"?1:-1));
        thumbnails[active].focus({preventScroll:true});
      });
      const frame = gallery.querySelector(".gallery-main");
      frame.addEventListener("pointerdown",event=>{
        if (!event.isPrimary || event.pointerType==="mouse" || event.target.closest("button,video")) return;
        gesture={id:event.pointerId,x:event.clientX,y:event.clientY};
      });
      frame.addEventListener("pointercancel",()=>{gesture=null;});
      frame.addEventListener("pointerup",event=>{
        if (!gesture || gesture.id!==event.pointerId) return;
        const x=event.clientX-gesture.x,y=event.clientY-gesture.y; gesture=null;
        if (Math.abs(x)>60 && Math.abs(x)>Math.abs(y)*1.5) show(active+(x<0?1:-1));
      });
      zoom.addEventListener("click",()=>{
        const source=media[active];
        const dialog=document.createElement("dialog"); dialog.className="media-lightbox";
        const header=document.createElement("header"),title=document.createElement("h2"),close=document.createElement("button");
        title.id="media-lightbox-title"; title.textContent=gallery.dataset.galleryName;
        dialog.setAttribute("aria-labelledby",title.id);
        close.type="button";close.className="outline-button";close.textContent="Close image";close.autofocus=true;
        // Fetch the original only when zoom is requested; the in-page gallery
        // can use its smaller responsive candidate without reducing zoom detail.
        const image=document.createElement("img");image.src=source.src;image.alt=source.alt;
        header.append(title,close);dialog.append(header,image);document.body.append(dialog);
        function finish(){dialog.close();dialog.remove();if(zoom.isConnected)zoom.focus({preventScroll:true});}
        close.addEventListener("click",finish);
        dialog.addEventListener("cancel",event=>{event.preventDefault();finish();});
        dialog.showModal();close.focus();
      });
      show(0,false);
    });
  }

  function initSearch() {
    const input=document.getElementById("global-search");
    if (!input || input.dataset.experienceBound) return;
    input.dataset.experienceBound="true";
    const button=document.createElement("button");
    button.type="button";button.className="search-clear";button.setAttribute("aria-label","Clear product search");button.textContent="×";
    // Safari does not focus buttons on pointer activation. Keep the expanding
    // input focused until clear completes rather than collapsing on mousedown.
    button.addEventListener('mousedown',event=>event.preventDefault());
    input.after(button);
    function update(){button.hidden=!input.value;}
    input.addEventListener("input",update);
    input.addEventListener("compositionstart",()=>{input.dataset.composing="true";window.htmx?.trigger(input,"htmx:abort");});
    input.addEventListener("compositionend",()=>{delete input.dataset.composing;input.dispatchEvent(new Event("input",{bubbles:true}));});
    button.addEventListener("click",()=>{
      window.htmx?.trigger(input,"htmx:abort");
      input.value="";input.dispatchEvent(new Event("input",{bubbles:true}));
      document.getElementById("search-suggestions").hidden=true;
      input.setAttribute("aria-expanded","false");input.removeAttribute("aria-activedescendant");input.focus();update();
      if(location.pathname==='/search')input.form.requestSubmit();
    });
    update();
    const feedback=document.createElement('p');feedback.className='sr-only';feedback.dataset.searchFeedback='';feedback.setAttribute('role','status');input.closest('.search-shell').append(feedback);
  }

  function syncPurchase() {
    const price=document.querySelector("[data-mobile-price]");
    const select=document.querySelector("[data-product-variant-select]");
    if (price && select) price.textContent=select.selectedOptions[0]?.dataset.price||price.textContent;
    const bar=document.querySelector(".mobile-buy-bar");
    if (bar) document.documentElement.style.setProperty("--ui-purchase-height",bar.getBoundingClientRect().height+"px");
    else document.documentElement.style.removeProperty("--ui-purchase-height");
    document.querySelectorAll(".steps [role=listitem]").forEach((step,i)=>{
      const panel=document.querySelector('.checkout-card[data-step="'+(i+1)+'"]');
      if (panel && visible(panel)) step.setAttribute("aria-current","step");else step.removeAttribute("aria-current");
    });
  }

  function initPurchaseBar() {
    const bar=document.querySelector('.mobile-buy-bar');
    if (purchaseBar===bar) return;
    purchaseObserver?.disconnect(); purchaseBar=bar;
    delete document.body.dataset.inlinePurchaseVisible;
    if(!bar) return;
    const visibility=new Map();
    purchaseObserver=new IntersectionObserver(entries=>{
      entries.forEach(entry=>visibility.set(entry.target,entry.isIntersecting));
      if(Array.from(visibility.values()).some(Boolean)) document.body.dataset.inlinePurchaseVisible='true';
      else delete document.body.dataset.inlinePurchaseVisible;
      syncPurchase();
    },{threshold:0,rootMargin:'-72px 0px 0px 0px'});
    document.querySelectorAll('.detail-actions,.product-policy-links,.product-share').forEach(region=>purchaseObserver.observe(region));
  }

  function init(event) {
    document.body.classList.add("ui-enhanced");
    // HTMX settles incoming class/style attributes after swapping. Restore the
    // sheet only after that step so its open/focus state cannot be overwritten.
    if (!(event?.type==='htmx:afterSwap' && ['page','listing-content'].includes(event.detail.target?.id) && filterSnapshot)) initFilters();
    initHero(document);
      initCategoryRails();
    initProductRails(document);initGallery(document);initSearch();initPurchaseBar();syncPurchase();
    document.querySelectorAll(".facet-search").forEach(facet=>{
      if (facet.dataset.emptyBound) return;
      facet.dataset.emptyBound="true";
      const message=document.createElement("p");message.className="facet-empty";message.hidden=true;message.textContent="No matching options. Try another search.";message.role="status";
      facet.append(message);
      const updateEmpty=()=>{
        const options=Array.from(facet.querySelectorAll(".facet-options > div"));
        message.hidden=!options.length || options.some(visible);
      };
      const search=facet.querySelector("input");
      search.addEventListener("input",()=>requestAnimationFrame(updateEmpty));
      // Searching within a facet is local UI state. Blurring this field must
      // not trigger the parent form's server-backed filter change request.
      search.addEventListener("change",event=>event.stopPropagation());
      // Alpine x-show may settle after the input event on another engine.
      new MutationObserver(updateEmpty).observe(facet.querySelector('.facet-options'),{attributes:true,attributeFilter:['style','hidden'],subtree:true});
    });
  }

  document.addEventListener('focusin',event=>{
    if (!document.body.dataset.keyboardInput || event.target.matches('main') || event.target.closest('dialog,.mobile-buy-bar,[data-filter-panel]')) return;
    const rectangle=event.target.getBoundingClientRect();
    const header=document.querySelector('.site-header');const bar=document.querySelector('.mobile-buy-bar');
    const top=header?.getBoundingClientRect().bottom||0;
    const bottom=bar && visible(bar)?bar.getBoundingClientRect().top:innerHeight;
    if (rectangle.top<top || rectangle.bottom>bottom) event.target.scrollIntoView({block:'center',behavior:'instant'});
  });
  document.addEventListener("change",event=>{if(event.target.matches("[data-product-variant-select]"))syncPurchase();});
  document.addEventListener("click",event=>{if(event.target.closest("[data-checkout-step]"))requestAnimationFrame(syncPurchase);});
  document.addEventListener('click',async event=>{
    const button=event.target.closest('[data-product-share]');if(!button || button.disabled)return;
    const url=location.origin+location.pathname;const status=document.querySelector('[data-share-status]');
    button.disabled=true;
    try {
      if (navigator.share) await navigator.share({title:button.dataset.shareTitle,url});
      else {await navigator.clipboard.writeText(url);status.textContent='Product link copied.';}
    } catch(error) {if(error.name!=='AbortError') status.textContent='Could not share this piece. Copy the page address from your browser.';}
    finally {button.disabled=false;}
  });
  document.addEventListener("htmx:beforeRequest",event=>{
    const originatingEvent=event.detail.requestConfig?.triggeringEvent;
    if (event.detail.elt.matches('[data-listing-filters]') && originatingEvent?.type==='change' && !originatingEvent.target.name) {event.preventDefault();return;}
    if (event.detail.elt.matches("#global-search") && event.detail.elt.dataset.composing) {event.preventDefault();return;}
    if(event.detail.target?.id==='search-suggestions') document.querySelector('[data-search-feedback]').textContent='Searching products…';
    if (["page","listing-content"].includes(event.detail.target?.id)) pendingPageRequest = {xhr:event.detail.xhr,filters:Boolean(event.detail.elt.closest('[data-listing-filters]'))};
    if (!event.detail.elt.closest("[data-listing-filters]")) return;
    captureFilters();
    filterController.form.querySelectorAll('[data-listing-feedback]').forEach(feedback=>{feedback.classList.remove('is-error');feedback.textContent='Updating pieces…';});
    filterController.form.querySelector("#listing-results").setAttribute("aria-busy","true");
  });
  document.addEventListener("htmx:beforeSwap",event=>{
    if(event.detail.target?.id==='search-suggestions') {
      const query=event.detail.requestConfig?.parameters?.q;
      const current=document.getElementById('global-search');
      if(current && (current.dataset.composing || (query!==undefined && String(query)!==current.value))) {event.detail.shouldSwap=false;return;}
    }

    if (["page","listing-content"].includes(event.detail.target?.id) && event.detail.shouldSwap && filterController) {
      if (pendingPageRequest && pendingPageRequest.xhr !== event.detail.xhr) {event.detail.shouldSwap=false;return;}
      if (pendingPageRequest?.filters) captureFilters();else filterSnapshot=null;
      filterController.close(false);
    }
  });
  document.addEventListener("htmx:afterSwap",init);
  document.addEventListener("htmx:afterSettle",()=>{
    init();
    if (filterController) filterController.form.querySelector("#listing-results").removeAttribute("aria-busy");
  });
  document.addEventListener("htmx:historyRestore",()=>{filterSnapshot=null;init();});
  window.addEventListener('pageshow',()=>initHero(document));
  window.addEventListener('pagehide',()=>{
    for(const controller of heroControllers.values()) controller.destroy();
    heroControllers.clear();
    const search=document.getElementById('global-search');if(search)window.htmx?.trigger(search,'htmx:abort');
    filterController?.close(false);
  });
  document.addEventListener("htmx:afterRequest",event=>{
    if(event.detail.target?.id==='search-suggestions') {
      const feedback=document.querySelector('[data-search-feedback]');
      if(feedback) feedback.textContent=event.detail.failed?'Search suggestions could not be loaded. Press Enter to search for your query.':document.querySelectorAll('#search-suggestions [role=option]').length+' suggestions available.';
    }
    if (!filterController || !event.detail.elt.closest("[data-listing-filters]")) return;
    if (pendingPageRequest && pendingPageRequest.xhr !== event.detail.xhr) return;
    filterController.form.querySelector("#listing-results").removeAttribute("aria-busy");
    if (!event.detail.failed) return;
    filterSnapshot=null;
    filterController.form.querySelectorAll('[data-listing-feedback]').forEach(feedback=>{
      feedback.classList.add('is-error');feedback.textContent='Could not update the results. Your filters are kept. ';
      const retry=document.createElement('button');retry.type='button';retry.textContent='Retry';
      retry.addEventListener('click',()=>filterController.form.elements.namedItem('sort').dispatchEvent(new Event('change',{bubbles:true})));
      feedback.append(retry);
    });
  });
  document.addEventListener("DOMContentLoaded",()=>{
    updateSearchPlaceholder();
    init();
    const header=document.querySelector(".site-header");
    if(header) new ResizeObserver(()=>document.documentElement.style.setProperty("--ui-header-height",header.getBoundingClientRect().height+"px")).observe(header);
    if (window.visualViewport) {
      let fullHeight=innerHeight;
      const updateKeyboard=()=>{
        fullHeight=Math.max(fullHeight,innerHeight);
        const editing=document.activeElement?.matches('input:not([type=hidden]),textarea,select');
        document.body.classList.toggle('ui-keyboard-open',Boolean(editing && (innerWidth<=720 || fullHeight-visualViewport.height>150)));
        syncPurchase();
      };
      visualViewport.addEventListener('resize',updateKeyboard);
      document.addEventListener('focusin',updateKeyboard);
      document.addEventListener('focusout',()=>requestAnimationFrame(updateKeyboard));
    }
  });
})();
