(function (global) {
  "use strict";

  const { create } = global.WeevCraftsBackend;
  const SHOPPING_CHECKOUT_KEY = "weevcrafts.checkout.idempotency.v1";
  const checkoutIntent=document.querySelector('[data-checkout-intent]')?.dataset.checkoutIntent||'';
  const CHECKOUT_KEY=SHOPPING_CHECKOUT_KEY+(checkoutIntent?':'+checkoutIntent:'');
  let backend = null;
  let backendError = null;
  let authenticated = Boolean(document.querySelector('form[action="/logout"]'));
  let razorpayLoader = null;
  let drawerRestoreFocus = null;
  let checkoutInFlight = false;

  function promptLogin(destination, trigger) {
    const dialog = document.getElementById("guest-login-dialog");
    const next = typeof destination === "string" && destination.startsWith("/") && !destination.startsWith("//") && !destination.includes("\\") ? destination : "/products";
    if (!dialog) { global.location.assign("/login?next=" + encodeURIComponent(next)); return; }
    if (!dialog.dataset.authBound) {
      dialog.dataset.authBound = "true";
      dialog.querySelectorAll('[data-auth-close]').forEach(button=>button.addEventListener('click',()=>dialog.close()));
      dialog.addEventListener('keydown',event=>{
        if(event.key!=='Tab')return;
        const controls=Array.from(dialog.querySelectorAll('a[href],button:not(:disabled),input:not(:disabled),[tabindex="0"]')).filter(element=>element.getClientRects().length && !element.closest('[hidden]'));
        const first=controls[0],last=controls[controls.length-1];
        if(event.shiftKey && document.activeElement===first){event.preventDefault();last?.focus();}
        else if(!event.shiftKey && document.activeElement===last){event.preventDefault();first?.focus();}
      });
      // Native Escape/backdrop dismissal never performs a protected action.
      dialog.addEventListener('click',event=>{if(event.target===dialog)dialog.close();});
      dialog.addEventListener('close',()=>{
        // Dismissed credentials and linking proof never linger in a hidden modal.
        dialog.querySelectorAll('form').forEach(form=>form.reset());
        dialog.querySelectorAll('[data-password-toggle]').forEach(button=>{
          const input=document.getElementById(button.getAttribute('aria-controls'));
          if(input)input.type='password';
          button.setAttribute('aria-pressed','false');button.setAttribute('aria-label','Show '+(button.dataset.passwordLabel||'password'));
          button.querySelector('.material-symbols-outlined').textContent='visibility';
        });
        dialog.querySelectorAll('[data-backend-form-message],[data-firebase-status],[data-firebase-link]').forEach(element=>{element.hidden=true;});
        dialog.querySelectorAll('[data-ui-error]').forEach(field=>{
          const id=field.dataset.uiError;document.getElementById(id)?.remove();
          const descriptions=(field.getAttribute('aria-describedby')||'').split(' ').filter(value=>value && value!==id);
          if(descriptions.length)field.setAttribute('aria-describedby',descriptions.join(' '));else field.removeAttribute('aria-describedby');
          field.removeAttribute('aria-invalid');delete field.dataset.uiError;
        });
        const consent=dialog.querySelector('[data-google-terms]');
        if(consent){consent.checked=false;consent.removeAttribute('aria-invalid');}
        global.WeevCraftsFirebase?.cancelLink(dialog);
        const origin=dialog._restoreFocus;
        if(origin?.isConnected && origin.getClientRects().length && !origin.closest('[inert]'))origin.focus({preventScroll:true});
        else document.querySelector('.menu-toggle')?.focus({preventScroll:true});
      });
    }
    dialog._restoreFocus=trigger || document.activeElement;
    dialog.querySelector('[name="next"]').value=next;
    dialog.querySelector('[data-firebase-google-panel]').dataset.authNext=next;
    dialog.querySelectorAll('[data-auth-route]').forEach(link=>{link.href=link.dataset.authRoute+'?next='+encodeURIComponent(next);});
    const menu = document.getElementById("mobile-menu");
    if (menu?.open) menu.close();
    if (!dialog.open) dialog.showModal();
    global.WeevCraftsFirebase?.enhanceGoogle(dialog);
    dialog.querySelector('[aria-label="Close sign-in prompt"]').focus();
  }

  const esc = (value) => String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;"
  })[char]);
  const money = (cents) => "INR " + (Number(cents || 0) / 100).toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  const unwrap = (value) => value && Object.prototype.hasOwnProperty.call(value, "data") ? value.data : value;

  function notice(message, error) {
    let target = document.querySelector("[data-backend-message]");
    if (!target || target.getClientRects().length === 0) target = document.getElementById("ui-toast");
    if (!target) {
      target = document.createElement("div");
      target.id = "ui-toast";
      target.setAttribute("role", "status");
      target.setAttribute("aria-live", "polite");
      document.body.appendChild(target);
    }
    target.textContent = message;
    target.classList.toggle("is-error", Boolean(error));
  }

  function ensureBackend() {
    if (backend) return backend;
    if (backendError) throw backendError;
    throw new Error("The storefront backend has not been configured.");
  }

  function markReady(element) {
    if (element) element.dataset.backendReady = "true";
  }

  function priceMarkup(product) {
    return "<div class=\"price\"><strong>" + money(product.priceCents) + "</strong>" +
      (product.compareAtCents > product.priceCents ? "<del>" + money(product.compareAtCents) + "</del>" : "") + "</div>";
  }


  function savedMarkup(product) {
    const slug = esc(product.slug);
    return "<article class=\"saved-card\" data-product=\"" + slug + "\"><div class=\"saved-media\"><a href=\"/products/" + slug + "\" tabindex=\"-1\" aria-hidden=\"true\"><img src=\"" + esc(product.image) + "\" alt=\"" + esc(product.name) + "\" width=\"320\" height=\"346\"/></a><button class=\"heart is-active\" aria-pressed=\"true\" type=\"button\" data-backend-action=\"wishlist-remove\" data-product-slug=\"" + slug + "\" aria-label=\"Remove from wishlist\"><svg class=\"wishlist-heart-icon\" viewBox=\"0 0 24 24\" width=\"24\" height=\"24\" aria-hidden=\"true\" focusable=\"false\"><path d=\"M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z\"></path></svg></button></div><div class=\"saved-copy\"><span class=\"badge\">" + (product.available > 0 ? "In stock" : "Sold out") + "</span><h3><a href=\"/products/" + slug + "\">" + esc(product.name) + "</a></h3><p>By " + esc(product.seller) + "</p>" + priceMarkup(product) + "<small class=\"availability\">" + (product.available > 0 ? "In stock" : "Currently unavailable") + "</small><div><a class=\"primary-button\" href=\"/products/" + slug + "\">View product</a><button class=\"text-button\" type=\"button\" data-backend-action=\"wishlist-remove\" data-product-slug=\"" + slug + "\" aria-label=\"Remove from wishlist\">Remove</button></div></div></article>";
  }



  function orderDate(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "Date unavailable" : date.toLocaleDateString("en-IN", { year: "numeric", month: "short", day: "numeric" });
  }

  function updateCustomerSidebar(user) {
    const profile = document.querySelector(".account-sidebar .profile-mini");
    if (!profile) return;
    const email = user && typeof user === "object" ? user.email || "" : "";
    const name = user && typeof user === "object" ? user.user_metadata?.display_name || user.display_name || user.name || (email ? email.split("@")[0] : "Signed-in customer") : "Signed-in customer";
    const initials = String(name).trim().split(/\s+/).slice(0, 2).map((part) => part.charAt(0).toUpperCase()).join("") || "WC";
    const avatar = profile.querySelector(".avatar");
    const label = profile.querySelector("b");
    const emailLabel = profile.querySelector("small");
    if (avatar) avatar.textContent = initials;
    if (label) label.textContent = name;
    if (emailLabel) emailLabel.textContent = email;
  }

  function orderMarkup(order, compact) {
    const items = order.items || [];
    const status = String(order.status || "pending_payment").replaceAll("_", " ");
    const orderNumber = order.order_number || order.orderNumber || "Order";
    const paymentStatus = order.payment_status || order.paymentStatus || (String(order.status || "").toLowerCase() === "paid" ? "captured" : "pending");
    const first = items[0];
    return "<article class=\"order-card compact-order\"><header><div><p class=\"eyebrow\">" + esc(status) + "</p><h2><a href=\"/orders/" + encodeURIComponent(orderNumber) + "\">" + esc(orderNumber) + "</a></h2><small>Placed on " + esc(orderDate(order.created_at || order.createdAt)) + " | Payment " + esc(paymentStatus) + "</small></div><div><strong>" + money(order.total_cents || order.totalCents) + "</strong><span class=\"status\">" + esc(status) + "</span></div></header><div class=\"compact-order-item\">" + (first ? "<img src=\"" + esc(first.image || first.image_url || "/assets/images/product-unavailable.svg") + "\" alt=\"" + esc(first.name || first.product_name || "Handmade product") + "\" width=\"160\" height=\"173\"/><span>" + esc(first.name || first.product_name || "Handmade product") + "<small>" + items.length + " item" + (items.length === 1 ? "" : "s") + "</small></span>" : "<span>Order items</span>") + "</div></article>";
  }

  async function hydrateOrders() {
    const host = document.querySelector("[data-backend-orders]");
    if (!host) return;
    if (host.hasAttribute("data-server-orders")) { markReady(host); return; }
    const tabs = document.querySelector(".account-intro nav.tabs");
    if (tabs) tabs.style.visibility = "hidden";
    try {
      const user = await requireUser();
      if (user && typeof user === "object") updateCustomerSidebar(user);
      let orders = await ensureBackend().orders();
      const allOrders = orders;
      document.querySelectorAll(".account-intro nav.tabs a").forEach((anchor) => {
        const tabStatus = new URL(anchor.href).searchParams.get("status") || "";
        const count = !tabStatus ? allOrders.length : tabStatus === "active" ? allOrders.filter((order) => !["delivered", "cancelled", "refunded"].includes(order.status)).length : tabStatus === "delivered" ? allOrders.filter((order) => order.status === "delivered").length : allOrders.filter((order) => ["cancelled", "refunded"].includes(order.status)).length;
        anchor.textContent = anchor.textContent.replace(/\s*\(\d+\)$/, "") + " (" + count + ")";
      });
      const status = new URLSearchParams(global.location.search).get("status") || "";
      if (status === "active") orders = orders.filter((order) => !["delivered", "cancelled", "refunded"].includes(order.status));
      else if (status === "delivered") orders = orders.filter((order) => order.status === "delivered");
      else if (status === "cancelled") orders = orders.filter((order) => ["cancelled", "refunded"].includes(order.status));
      host.innerHTML = orders.length ? orders.map((order) => orderMarkup(order, true)).join("") : "<div class=\"empty-state\"><h2>No orders in this view</h2><p>Your orders will appear here after checkout.</p><a class=\"primary-button\" href=\"/products\">Browse products</a></div>";
    } catch (error) {
      document.querySelectorAll(".account-intro nav.tabs a").forEach((anchor) => { anchor.textContent = anchor.textContent.replace(/\s*\(\d+\)$/, ""); });
      host.innerHTML = "<div class=\"empty-state\" role=\"alert\"><h2>Orders unavailable</h2><p>" + esc(error.message) + "</p><a class=\"primary-button\" href=\"/login\">Sign in</a></div>";
    } finally {
      if (tabs) tabs.style.visibility = "";
      markReady(host);
    }
  }


  function normalizeCart(payload) {
    const data = unwrap(payload) || {};
    const items = (data.items || []).map((item) => {
      if (item.slug && item.name) return item;
      return global.WeevCraftsBackend.normalizeCartItem(item);
    }).filter((item) => item && item.slug);
    return { id: data.id, items: items };
  }

  function cartItemCount(items) {
    return items.reduce((sum, item) => sum + Number(item.quantity || 0), 0);
  }

  function updateBadges(cart, wishlist) {
    const cartCount = document.querySelector("#cart-count em");
    if (cart) {
      const count = cartItemCount(cart.items);
      if (cartCount) { cartCount.textContent = String(count); cartCount.closest('a').setAttribute('aria-label','Cart, '+count+' items'); }
      markReady(document.getElementById("cart-count"));
    }
    if (wishlist) updateWishlistCount(wishlist.length);
  }

  function updateWishlistCount(count) {
    const link = document.getElementById('wishlist-count');
    const counter = link?.querySelector('em');
    if (counter) counter.textContent = String(count);
    if (link) {
      link.setAttribute('aria-label', 'Wishlist, ' + count + ' items');
      link.classList.toggle('is-active', count > 0);
      const icon = link.querySelector('.material-symbols-outlined');
      if (icon) icon.textContent = count > 0 ? 'favorite' : 'favorite_border';
      markReady(link);
    }
    document.querySelectorAll('[data-backend-wishlist-count]').forEach(label => {
      label.textContent = (count === 1 ? '1 item' : count + ' items') + ' in your wishlist';
      markReady(label);
    });
  }

  function renderCartDrawer(cart, signedIn) {
    const drawer = document.querySelector("[data-backend-cart-drawer]");
    if (!drawer) return;
    const itemsHost = drawer.querySelector("[data-cart-drawer-items]");
    const summary = drawer.querySelector("[data-cart-drawer-summary]");
    const countLabel = drawer.querySelector("[data-cart-drawer-count]");
    const items = cart.items || [];
    const count = cartItemCount(items);
    if (countLabel) countLabel.textContent = count + " item" + (count === 1 ? "" : "s") + " from independent Indian makers";
    if (itemsHost) {
      if (!signedIn) {
        itemsHost.innerHTML = '<div class="empty-state"><span class="material-symbols-outlined" aria-hidden="true">person</span><h3>Sign in to see your cart</h3><p>Your saved items will follow you across devices.</p><a class="primary-button" href="/login">Sign in</a></div>';
      } else if (!items.length) {
        itemsHost.innerHTML = '<div class="empty-state"><span class="material-symbols-outlined" aria-hidden="true">shopping_bag</span><h3>Your cart is empty</h3><p>Browse handmade treasures and bring a little India home.</p><a class="primary-button" href="/products">Continue shopping</a></div>';
      } else {
        itemsHost.innerHTML = items.map(cartItemMarkup).join("");
        wireCartLineActions(itemsHost, items);
      }
    }
    if (summary) {
      summary.hidden = !signedIn || !items.length;
    }
  }

  function openCartDrawer(trigger) {
    const drawer = document.querySelector("[data-backend-cart-drawer]");
    if (!drawer) return;
    drawerRestoreFocus = trigger || document.activeElement;
    if (!drawer.open && typeof drawer.showModal === "function") drawer.showModal();
    else if (!drawer.open) drawer.setAttribute("open", "");
    requestAnimationFrame(function () {
      drawer.querySelector("[data-cart-drawer-close]")?.focus({ preventScroll: true });
    });
  }

  function closeCartDrawer() {
    const drawer = document.querySelector("[data-backend-cart-drawer]");
    if (!drawer) return;
    if (drawer.open && typeof drawer.close === "function") drawer.close();
    else drawer.removeAttribute("open");
    if (drawerRestoreFocus && typeof drawerRestoreFocus.focus === "function") drawerRestoreFocus.focus({ preventScroll: true });
    drawerRestoreFocus = null;
  }

  function cartItemMarkup(item) {
    const slug = esc(item.slug);
      return "<article class=\"cart-seller\"><div class=\"cart-seller-head\"><div><span class=\"material-symbols-outlined\" aria-hidden=\"true\">storefront</span><b>" + esc(item.seller) + "</b></div></div><div class=\"cart-line\"><img src=\"" + esc(item.image) + "\" alt=\"" + esc(item.name) + "\" width=\"160\" height=\"173\"/><div class=\"cart-line-copy\"><span class=\"badge\">" + (item.available > 0 ? "In stock" : "Sold out") + "</span><h2>" + esc(item.name) + "</h2><p class=\"cart-variant\">Option: <b>" + esc(item.optionLabel || "Standard") + "</b></p><div class=\"price\"><strong>" + money(item.priceCents) + "</strong>" + (item.compareAtCents > item.priceCents ? "<del>" + money(item.compareAtCents) + "</del>" : "") + "</div><div class=\"cart-line-actions\"><span class=\"quantity-control\"><button type=\"button\" data-backend-action=\"cart-change\" data-product-slug=\"" + slug + "\" data-delta=\"-1\" aria-label=\"Decrease quantity\">-</button><output aria-live=\"polite\">" + Number(item.quantity || 0) + "</output><button type=\"button\" data-backend-action=\"cart-change\" data-product-slug=\"" + slug + "\" data-delta=\"1\" aria-label=\"Increase quantity\">+</button></span><button class=\"text-button\" type=\"button\" data-backend-action=\"cart-change\" data-product-slug=\"" + slug + "\" data-delta=\"-" + Number(item.quantity || 0) + "\">Remove</button><button class=\"text-button\" type=\"button\" data-backend-action=\"wishlist-move\" data-product-slug=\"" + slug + "\">Move to wishlist</button></div></div></div></article>";
  }

  function wireCartLineActions(host, items) {
    if (!host) return;
    const lines = host.querySelectorAll(".cart-line");
    items.forEach(function (item, index) {
      const line = lines[index];
      if (!line) return;
      const variantID = String(item.variantId || "");
      // Some cart SSR responses omit media/maker metadata. Enhance only these
      // fields from the existing product response; never replace price/quantity.
      const photo=line.querySelector('img');
      if(photo && item.image) {photo.src=item.image;photo.alt=item.name;}
      const maker=line.closest('.cart-seller')?.querySelector('.cart-seller-head b');
      if(maker && item.seller)maker.textContent=item.seller;
      line.querySelectorAll(".quantity-control button").forEach(function (button) { button.dataset.variantId = variantID; });
      const actions = line.querySelectorAll(".cart-line-actions > .text-button");
      if (actions[0]) {
        actions[0].dataset.backendAction = "cart-remove";
        actions[0].dataset.variantId = variantID;
        delete actions[0].dataset.delta;
      }
      if (actions[1]) actions[1].dataset.variantId = variantID;
    });
  }

  async function hydrateCart(options) {
    options = options || {};
    const cartRoot = document.querySelector("[data-backend-cart]");
    const wishlistRoot = document.getElementById("wishlist-grid");
    const backendInstance = ensureBackend();
    const user = await backendInstance.currentUser();
    if (!user) {
      if (cartRoot) cartRoot.querySelector(".cart-items").innerHTML = "<div class=\"empty-state\"><h2>Sign in to see your cart</h2><a class=\"primary-button\" href=\"/login\">Sign in</a></div>";
      if (wishlistRoot) wishlistRoot.innerHTML = "<div class=\"empty-state\"><h2>Sign in to see your wishlist</h2><a class=\"primary-button\" href=\"/login\">Sign in</a></div>";
      updateBadges({ items: [] }, []);
      if (cartRoot) markReady(cartRoot);
      if (wishlistRoot) markReady(wishlistRoot);
      const emptyCartCount = document.querySelector("[data-backend-cart-count]");
      const emptyWishlistCount = document.querySelector("[data-backend-wishlist-count]");
      if (emptyCartCount) { emptyCartCount.textContent = "0 items from independent Indian makers"; markReady(emptyCartCount); }
      if (emptyWishlistCount) { emptyWishlistCount.textContent = "0 items in your wishlist"; markReady(emptyWishlistCount); }
      renderCartDrawer({ items: [] }, false);
      if (options.open) openCartDrawer(options.trigger);
      return;
    }
    const cart = normalizeCart(await backendInstance.cart());
    const wishlist = await backendInstance.wishlist();
    // Reuse the maintained cart page for authoritative amounts/rows. Do not
    // reconstruct its payable total or option/availability copy in JavaScript.
    const cartResponse = await fetch('/cart', {credentials:'same-origin',cache:'no-store'});
    if (!cartResponse.ok || cartResponse.redirected) throw new Error('Cart updated, but its latest summary could not be loaded. Open your cart to review it.');
    const cartDocument = new DOMParser().parseFromString(await cartResponse.text(),'text/html');
    const nextCart = cartDocument.querySelector('[data-backend-cart]');
    if (!nextCart) throw new Error('Could not load the latest cart. Open your cart to review it.');
    updateBadges(cart, wishlist);
    renderCartDrawer(cart, true);
    const drawerItems=document.querySelector('[data-cart-drawer-items]');
    if(drawerItems) {
      drawerItems.replaceChildren(...Array.from(nextCart.querySelector('.cart-items').childNodes).map(node=>node.cloneNode(true)));
      wireCartLineActions(drawerItems,cart.items);global.WVUI?.enhance(drawerItems);
    }
    const latestSummary = nextCart.querySelector('.cart-summary');
    if (latestSummary) {
      const amounts = latestSummary.querySelectorAll('p b');
      const drawer = document.querySelector('[data-backend-cart-drawer]');
      if (drawer) {
        drawer.querySelector('[data-cart-drawer-subtotal]').textContent = amounts[0]?.textContent || 'Review in cart';
        drawer.querySelector('[data-cart-drawer-shipping]').textContent = amounts[1]?.textContent || 'Review in cart';
        drawer.querySelector('[data-cart-drawer-total]').textContent = latestSummary.querySelector('h3 strong')?.textContent || 'Review in cart';
      }
    }
    if (options.open) openCartDrawer(options.trigger);
    const cartCountLabel = document.querySelector("[data-backend-cart-count]");
    if (cartCountLabel) { cartCountLabel.textContent = cartItemCount(cart.items) + " items from independent Indian makers"; markReady(cartCountLabel); }
    const wishlistCountLabel = document.querySelector("[data-backend-wishlist-count]");
    if (wishlistCountLabel) { wishlistCountLabel.textContent = (wishlist.length === 1 ? "1 item" : wishlist.length + " items") + " in your wishlist"; markReady(wishlistCountLabel); }
    if (cartRoot) {
      cartRoot.className = nextCart.className;
      cartRoot.replaceChildren(...Array.from(nextCart.childNodes));
      const host = cartRoot.querySelector('.cart-items');
      wireCartLineActions(host, cart.items);
      global.htmx?.process(cartRoot); global.WVUI?.enhance(cartRoot);
      markReady(cartRoot);
    }
    if (wishlistRoot) {
      wishlistRoot.innerHTML = wishlist.length ? wishlist.map(savedMarkup).join("") : "<div class=\"empty-state\"><h2>Your wishlist is empty</h2><p>Save pieces you love and they will appear here.</p><a class=\"primary-button\" href=\"/products\">Discover handmade finds</a></div>";
      markReady(wishlistRoot);
    }
  }

  let countsRequest = 0, wishlistRevision = 0;
  const wishlistState = new Map();
  const wishlistPending = new Set();
  let wishlistSnapshot = null;
  const transientRead = error => error.name !== 'AbortError' &&
    (error instanceof TypeError || error.status === 408 || error.status === 429 || error.status >= 500);

  async function refreshCounts() {
    if (!backend || !authenticated || document.hidden) return;
    const request = ++countsRequest, revision = wishlistRevision;
    // A count failure must never disable mutations or invalidate usable SSR data.
    const read = async (load, apply) => {
      for (let attempt = 0; attempt < 2; attempt++) {
        try {
          const value = await load();
          if (request === countsRequest) apply(value);
          return;
        } catch (error) {
          if (attempt || !transientRead(error) || request !== countsRequest) return;
          await new Promise(resolve => global.setTimeout(resolve, 1000));
          if (request !== countsRequest || document.hidden || backend.readController.signal.aborted) return;
        }
      }
    };
    await Promise.all([
      read(() => backend.cart({countsOnly:true}), cart => updateBadges(normalizeCart(cart), null)),
      document.body.dataset.wishlistEnabled === 'false' ? Promise.resolve() :
        read(() => backend.wishlistSummary(), items => {
          if (revision !== wishlistRevision || wishlistPending.size) return;
          wishlistSnapshot = new Set(items.map(item => item.slug));
          wishlistState.forEach((_, slug) => wishlistState.set(slug, wishlistSnapshot.has(slug)));
          updateWishlistCount(items.length);
          reconcileWishlistHearts();
        })
    ]);
  }

  async function requireUser() {
    const user = await ensureBackend().currentUser();
    if (!user) {
      authenticated = false;
      const error = new Error("Sign in to save a cart, wishlist, or order.");
      error.status = 401;
      throw error;
    }
    return user;
  }

  function syncWishlistHearts(slug, saved) {
    wishlistState.set(slug, saved);
    document.querySelectorAll(".heart, .gallery-heart").forEach(function (button) {
      const path = button.getAttribute("hx-post") || "";
      const buttonSlug = button.dataset.productSlug || (path.startsWith("/ui/wishlist/") ? decodeURIComponent(path.slice(13)) : "");
      if (buttonSlug !== slug) return;
      button.dataset.productSlug = slug;
      button.dataset.backendAction = saved ? "wishlist-remove" : "wishlist-add";
      button.classList.toggle("is-active", saved);
      button.classList.remove('is-saved');
      button.setAttribute("aria-pressed", String(saved));
      const name = button.dataset.productName || button.closest("article")?.querySelector("h3")?.textContent || document.querySelector('.detail-copy h1')?.textContent || "product";
      button.setAttribute("aria-label", (saved ? "Remove " : "Save ") + name + (saved ? " from wishlist" : " to wishlist"));
      let values = {};
      try { values = JSON.parse(button.getAttribute('hx-vals') || '{}'); } catch (_) { /* Keep the button usable. */ }
      if (saved) values.source = 'wishlist'; else delete values.source;
      button.setAttribute('hx-vals', JSON.stringify(values));
      const icon = button.querySelector(".material-symbols-outlined");
      if (icon) icon.textContent = saved ? "favorite" : "favorite_border";
    });
  }

  function wishlistSlug(button) {
    const path = button.getAttribute('hx-post') || '';
    return button.dataset.productSlug || (path.startsWith('/ui/wishlist/') ? decodeURIComponent(path.slice(13)) : '');
  }

  function wishlistBusy(slug, active) {
    document.querySelectorAll('.heart, .gallery-heart, .saved-card .text-button').forEach(button => {
      if (wishlistSlug(button) !== slug) return;
      if (active && !button.disabled) {
        button.dataset.wishlistPending = 'true';
        button.disabled = true;
        global.WVUI?.busy(button, true);
        button.setAttribute('aria-busy', 'true');
      } else if (!active && button.dataset.wishlistPending) {
        delete button.dataset.wishlistPending;
        button.disabled = false;
        global.WVUI?.busy(button, false);
        button.removeAttribute('aria-busy');
      }
    });
  }

  function reconcileWishlistHearts() {
    const slugs = new Set();
    document.querySelectorAll('.heart, .gallery-heart').forEach(button => {
      const slug = wishlistSlug(button);
      if (!slug) return;
      if (!wishlistState.has(slug)) wishlistState.set(slug, wishlistSnapshot ? wishlistSnapshot.has(slug) : button.getAttribute('aria-pressed') === 'true');
      slugs.add(slug);
    });
    slugs.forEach(slug => {
      syncWishlistHearts(slug, wishlistState.get(slug));
      if (wishlistPending.has(slug)) wishlistBusy(slug, true);
    });
  }

  function commitWishlist(slug, saved) {
    const previous = wishlistState.get(slug);
    ++wishlistRevision;
    syncWishlistHearts(slug, saved);
    if (wishlistSnapshot) { if (saved) wishlistSnapshot.add(slug); else wishlistSnapshot.delete(slug); }
    const counter = document.querySelector('#wishlist-count em');
    const count = counter ? Number(counter.textContent) : NaN;
    if (Number.isFinite(count) && typeof previous === 'boolean' && previous !== saved) updateWishlistCount(Math.max(0, count + (saved ? 1 : -1)));
  }

  function removeSavedCard(slug, trigger) {
    const root = document.getElementById('wishlist-grid');
    if (!root) return;
    const cards = Array.from(root.querySelectorAll('.saved-card'));
    const card = cards.find(item => item.dataset.product === slug);
    if (!card) return;
    const focused = card.contains(document.activeElement) || card.contains(trigger);
    const index = cards.indexOf(card);
    card.remove();
    const remaining = Array.from(root.querySelectorAll('.saved-card'));
    if (!remaining.length) root.innerHTML = '<div class="empty-state"><h2>Your wishlist is empty</h2><p>Save pieces you love and they will appear here.</p><a class="primary-button" href="/products">Discover handmade finds</a></div>';
    if (focused) (remaining[Math.min(index, remaining.length - 1)]?.querySelector('button') || root.querySelector('.empty-state a'))?.focus();
  }

  const cartPending = new Set();
  async function runAction(action, slug, delta, selectedVariantId, triggerElement) {
    const api = ensureBackend();
    if (action === "cart-add") {
      await requireUser();
      const quantity = Number(delta || document.getElementById("detail-quantity")?.value || 1);
      await api.addCart(slug, quantity, selectedVariantId);
      global.sessionStorage.removeItem(SHOPPING_CHECKOUT_KEY);
      global.WVQuickView?.close();
      global.location.assign('/cart');
      return;
    }
    if (action === "cart-buy") {
      await requireUser();
      // Keep this request key on the trigger for uncertain-response retries.
      const purchaseSelection=JSON.stringify([slug,selectedVariantId,Number(delta || document.getElementById('detail-quantity')?.value || 1)]);
      if(triggerElement.dataset.purchaseSelection!==purchaseSelection){triggerElement.dataset.purchaseKey=global.crypto.randomUUID();triggerElement.dataset.purchaseSelection=purchaseSelection;}
      triggerElement.dataset.purchaseKey ||= global.crypto.randomUUID();
      const intent=await api.buyNow(slug,Number(delta || document.getElementById('detail-quantity')?.value || 1),selectedVariantId,triggerElement.dataset.purchaseKey);
      global.WVQuickView?.close();
      delete triggerElement.dataset.purchaseKey;delete triggerElement.dataset.purchaseSelection;
      global.location.assign(intent.checkout_url);
      return;
    }
    if (action === "cart-change") {
      await requireUser();
      await api.changeCart(slug, Number(delta), selectedVariantId);
      global.sessionStorage.removeItem(SHOPPING_CHECKOUT_KEY);
      await hydrateCart();
      return;
    }
    if (action === "cart-remove") {
      await requireUser();
      await api.removeCart(slug, selectedVariantId);
      global.sessionStorage.removeItem(SHOPPING_CHECKOUT_KEY);
      await hydrateCart();
      return;
    }
    if (action === "wishlist-add") {
      await requireUser();
      await api.addWishlist(slug);
      commitWishlist(slug, true);
      notice("Saved to your wishlist.", false);
      return;
    }
    if (action === "wishlist-remove") {
      await requireUser();
      await api.removeWishlist(slug);
      commitWishlist(slug, false);
      removeSavedCard(slug, triggerElement);
      notice("Removed from your wishlist.", false);
      return;
    }
    if (action === "wishlist-move") {
      await requireUser();
      await api.moveCartToWishlist(slug, selectedVariantId);
      commitWishlist(slug, true);
      global.sessionStorage.removeItem(SHOPPING_CHECKOUT_KEY);
      await hydrateCart();
      notice("Moved to your wishlist.", false);
      return;
    }
    if (action === "wishlist-to-cart") {
      await requireUser();
      await api.addCart(slug, 1);
      await api.removeWishlist(slug);
      commitWishlist(slug, false);
      global.sessionStorage.removeItem(SHOPPING_CHECKOUT_KEY);
      global.location.assign('/cart');
    }
  }

  function formData(form) {
    return Object.fromEntries(new FormData(form).entries());
  }

  function showFormMessage(form, text, error) {
    let target = form.querySelector("[data-backend-form-message]");
    if (!target) {
      target = document.createElement("p");
      target.dataset.backendFormMessage = "true";
      target.setAttribute("role", "status"); target.tabIndex=-1;
      form.appendChild(target);
    }
    target.textContent = text;
    target.hidden = false;
    target.setAttribute('role',error?'alert':'status');
    target.classList.toggle("is-error", Boolean(error));
    if(error){target.focus({preventScroll:true});target.scrollIntoView({block:'nearest'});}
  }

  async function submitAuth(form) {
    if (form.dataset.submitting === "true") return;
    if (global.WVUI && !global.WVUI.validate(form)) return;
    form.dataset.submitting = "true";
    const authButton = form.querySelector('button[type="submit"]');
    if (authButton) {authButton.disabled = true; global.WVUI?.busy(authButton,true);}
    const action = form.dataset.backendAuth;
    const values = formData(form);
    const message=form.querySelector('[data-backend-form-message]');
    if(message){message.hidden=true;message.textContent='';}
    try {
      const api = backend || await create();
      if(!backend){backend=api;global.weevcraftsBackend=api;backendError=null;}
      if (action === "login") {
        const result = await api.signIn({ email: values.email, password: values.password, remember: values.remember === "on", next: values.next || "", authRoot: form.closest('[data-auth-surface]') });
        global.location.assign(result.redirect_to || "/products");
      } else if (action === "register") {
        const result = await api.signUp({ email: values.email, password: values.password, displayName: values.display_name, phone: values.phone, acceptedTerms: values.consent === "on", next: values.next || "" });
        const data = unwrap(result) || result;
        if (data && (data.access_token || result.csrf_token)) global.location.assign(result.redirect_to || "/products");
        else showFormMessage(form, "Check your email to verify your new account.", false);
      } else if (action === "recover") {
        await api.recover(values.email);
        showFormMessage(form, "If the address is registered, recovery instructions have been sent.", false);
      } else if (action === "reset") {
        if (values.password !== values.confirm_password) throw new Error("The passwords do not match.");
        const hash = new URLSearchParams(global.location.hash.replace(/^#/, ""));
        const query = new URLSearchParams(global.location.search);
        await api.updatePassword(values.password, values.token || query.get("token") || hash.get("access_token") || hash.get("token") || "");
        global.history.replaceState(null, "", global.location.pathname);
        showFormMessage(form, "Password updated. You can now sign in.", false);
      } else if (action === "verify") {
        await api.verifyEmail(values.token || new URLSearchParams(global.location.search).get("token") || "");
        showFormMessage(form, "Your email has been verified. You can now sign in.", false);
      }
    } catch (error) {
      showFormMessage(form, error.message, true);
    } finally {
      form.dataset.submitting = "false";
      if (authButton) {authButton.disabled = false; global.WVUI?.busy(authButton,false);}
    }
  }

  function paymentReceipt(orderNumber, message, localSimulator) {
    // Confirmation always reads the committed payment/order state from the server.
    global.location.assign("/orders/" + encodeURIComponent(orderNumber) + "/confirmation");
  }

  function loadRazorpay() {
    if (global.Razorpay) return Promise.resolve();
    if (razorpayLoader) return razorpayLoader;
    razorpayLoader = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = "https://checkout.razorpay.com/v1/checkout.js";
      script.async = true;
      script.onload = () => global.Razorpay ? resolve() : reject(new Error("The payment window could not be loaded."));
      script.onerror = () => { razorpayLoader = null; reject(new Error("The payment provider could not be reached. Your order is saved.")); };
      document.head.appendChild(script);
    });
    return razorpayLoader;
  }

  async function beginPayment(orderNumber) {
    const config = document.getElementById("checkout-payment-config");
    const mode = config && config.dataset.paymentMode;
    const intent = unwrap(await ensureBackend().createPayment(orderNumber)) || {};
    if (mode === "local") {
      await ensureBackend().localPayment(orderNumber, "capture");
      global.location.assign("/orders/" + encodeURIComponent(orderNumber) + "/confirmation");
      return;
    }
    const key = config && config.dataset.paymentPublicKey;
    if (!key || !intent.provider_order_id || !intent.amount_cents || !intent.currency) {
      throw new Error("The payment provider returned incomplete checkout details.");
    }
    await loadRazorpay();
    const checkout = new global.Razorpay({
      key: key,
      amount: Number(intent.amount_cents),
      currency: intent.currency,
      name: "WeevCrafts",
      description: "Handmade goods from independent makers",
      order_id: intent.provider_order_id,
      theme: { color: "#1f4b3a" },
      modal: { ondismiss: () => paymentReceipt(orderNumber, "Your order is saved. Payment was not completed; check the order status before trying again.", false) },
      handler: async function (response) {
        try {
          await ensureBackend().verifyPayment(orderNumber, response);
          paymentReceipt(orderNumber, "Payment details were verified securely. The provider is confirming capture; your order status will update shortly.", false);
        } catch (error) {
          paymentReceipt(orderNumber, error.message + " Your order is saved; check its current status before trying again.", false);
        }
      }
    });
    checkout.on("payment.failed", function () {
      paymentReceipt(orderNumber, "The payment was not completed. Your order is saved; check its current status for the latest update.", false);
    });
    checkout.open();
  }

  async function previewCheckoutQuote() {
    const code = document.getElementById("checkout-coupon-code")?.value.trim() || "";
    const response = await fetch("/api/v1/checkout/quote?coupon_code=" + encodeURIComponent(code) + (checkoutIntent?"&intent="+encodeURIComponent(checkoutIntent):""), { credentials: "same-origin", cache: "no-store" });
    const body = await response.json();
    if (!response.ok) throw new Error(body.error?.message || "Could not preview checkout. Please try again.");
    const quote = unwrap(body);
    global.weevcraftsPolicyRevision=quote.policy_revision;
    for (const field of ["subtotal", "shipping", "discount", "total"]) {
      const elements = document.querySelectorAll("[data-quote-" + field + "]");
      elements.forEach((element) => { element.textContent = money(quote[field + "_cents"]); element.dataset.cents = String(quote[field + "_cents"]); });
    }
    document.querySelectorAll("[data-quote-shipping-label]").forEach(label => { label.textContent="Shipping"; });
    return quote;
  }

  async function checkout() {
    const button = document.querySelector('[data-backend-action="checkout"], [data-backend-action="checkout-no-payment"]');
    const noPaymentDemo = button && button.dataset.backendAction === "checkout-no-payment";
    const form = document.getElementById("checkout-address-form");
    if (!button || !form || !global.WVUI.validate(form)) return;
    await requireUser();
    const couponInput = document.getElementById("checkout-coupon-code");
    const acceptedCoupon = couponInput ? couponInput.value.trim() : "";
    const totalElement = document.querySelector("[data-quote-total]");
    const previousTotal = totalElement?.dataset.cents || (totalElement ? String(Math.round(Number(totalElement.textContent.replace(/[^0-9.]/g, "")) * 100)) : undefined);
    const values = formData(form);
    const resolvedState = (values.state || values.region || "").trim();
    const resolvedCity = (values.city || "").trim();
    const resolvedPhone = (values.phone || "").trim();
    const resolvedPostal = (values.postal_code || "").replace(/[\s-]/g, "").trim();
    const address = {
      recipient_name: (values.recipient_name || "").trim(),
      phone: resolvedPhone,
      line1: (values.line1 || "").trim(),
      line2: (values.line2 || "").trim(),
      city: resolvedCity,
      state: resolvedState,
      postal_code: resolvedPostal,
      country_code: (values.country_code || "IN").toUpperCase()
    };
    const summary = document.querySelector("[data-checkout-address-summary]");
    if (summary) summary.textContent = [address.recipient_name, address.line1, address.city, address.state, address.postal_code].filter(Boolean).join(" · ");
    const fingerprint = JSON.stringify({ address, couponCode: acceptedCoupon });
    let pending = null;
    try { pending = JSON.parse(global.sessionStorage.getItem(CHECKOUT_KEY)); } catch (_) { /* Invalid stored draft. */ }
    const retry = pending && pending.fingerprint === fingerprint && pending.key && pending.quote;
    const currentQuote = retry ? pending.quote : await previewCheckoutQuote();
    if (!retry && previousTotal !== undefined && previousTotal !== String(currentQuote.total_cents)) {
      notice("Your total has changed. Review the updated amount, then place your order again.", true);
      return;
    }
    if (!retry) {
      pending = { fingerprint, key: global.crypto.randomUUID(), cartID: currentQuote.cart_id, quote: currentQuote };
      global.sessionStorage.setItem(CHECKOUT_KEY, JSON.stringify(pending));
    }
    button.disabled = true; global.WVUI?.busy(button,true);
    let orderNumber = "";
    try {
      // The order transaction saves this address once, including idempotent retries.
      const couponCode = acceptedCoupon;
      const key = pending.key;
      const result = unwrap(await (noPaymentDemo ? ensureBackend().checkoutNoPayment(address, key, couponCode.trim(), currentQuote.total_cents, pending.cartID, currentQuote.policy_revision) : ensureBackend().checkout(address, key, couponCode.trim(), currentQuote.total_cents, pending.cartID, currentQuote.policy_revision))) || {};
      orderNumber = result.order_number || result.orderNumber || "";
      if (!orderNumber) throw new Error("The order was created, but the server did not return its number. Open your orders to check its status.");
      button.dataset.orderCreated='true';
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      const cartBadge = document.querySelector("#cart-count em");
      if (cartBadge && !checkoutIntent) { cartBadge.textContent = "0"; cartBadge.closest('a').setAttribute('aria-label','Cart, 0 items'); markReady(document.getElementById("cart-count")); }
      if (noPaymentDemo) {
        paymentReceipt(orderNumber, "Your order is saved with payment pending. No payment was collected.", false);
      } else {
        try {
          await beginPayment(orderNumber);
        } catch (paymentError) {
          paymentReceipt(orderNumber, paymentError.message + " Your order is saved; check its current status before trying again.", false);
        }
      }
    } catch (error) {
      if (error.code === "QUOTE_CHANGED" || error.code === "COUPON_INVALID" || error.code === "INSUFFICIENT_STOCK") global.sessionStorage.removeItem(CHECKOUT_KEY);
      if (orderNumber) {
        paymentReceipt(orderNumber, error.message + " Your order is saved; check its current status before trying again.", false);
      } else {
        notice(error instanceof TypeError ? 'We could not confirm whether your order was saved. Keep this page open and try again; the same request will be checked before creating another order.' : error.message, true);
      }
    } finally {
      if (!orderNumber) {
        button.disabled = false; global.WVUI?.busy(button,false);
      }
    }
  }

  function bindActions() {
    document.addEventListener('click',async event=>{
      const button=event.target.closest('[data-retry-payment]');
      if (!button || button.disabled) return;
      button.disabled=true; global.WVUI?.busy(button,true);
      try { await beginPayment(button.dataset.retryPayment); }
      catch(error) { notice(error.message,true); }
      finally { button.disabled=false; global.WVUI?.busy(button,false); }
    });
    const quoteButton = document.querySelector("[data-preview-quote]");
    if (quoteButton) quoteButton.addEventListener("click", async function () {
      quoteButton.disabled = true; global.WVUI?.busy(quoteButton,true);
      try { await previewCheckoutQuote(); notice("Checkout total updated.", false); }
      catch (error) { notice(error.message, true); }
      finally { quoteButton.disabled = false; global.WVUI?.busy(quoteButton,false); }
    });
    const checkoutAddressForm = document.getElementById("checkout-address-form");
    if (checkoutAddressForm) {
      const updateSummary = function () {
        const values = formData(checkoutAddressForm);
        const summary = document.querySelector("[data-checkout-address-summary]");
        if (summary) summary.textContent = [values.recipient_name, values.line1, values.city, values.state || values.region, values.postal_code].filter(Boolean).join(" · ") || "Enter your delivery address";
        const deliveryLocation = document.querySelector('.header-main .location small');
        if (deliveryLocation) deliveryLocation.textContent = [values.city, values.state || values.region].filter(Boolean).join(', ') || 'Choose delivery location';
      };
      checkoutAddressForm.addEventListener("input", updateSummary);
      checkoutAddressForm.addEventListener("change", function () { queueMicrotask(updateSummary); });
      updateSummary();
    }
    const savedAddressSelect = document.querySelector("[data-checkout-address-select]");
    if (savedAddressSelect) {
      const addressForm = document.getElementById("checkout-address-form");
      if (addressForm) {
        addressForm.addEventListener("input", function (event) {
          if (event.target !== savedAddressSelect) savedAddressSelect.value = "";
          if (event.target.name === "state") {
            const reg = addressForm.elements.namedItem("region");
            if (reg) reg.value = event.target.value;
          } else if (event.target.name === "region") {
            const st = addressForm.elements.namedItem("state");
            if (st) st.value = event.target.value;
          }
        });
      }
      savedAddressSelect.addEventListener("change", function () {
        const option = savedAddressSelect.selectedOptions[0];
        if (!option) return;
        if (!option.value) {
          const form = document.getElementById("checkout-address-form");
          ["line1","line2","city","state","region","postal_code"].forEach(name => { const field=form?.elements.namedItem(name); if(field) field.value=""; });
          form?.dispatchEvent(new Event("change", {bubbles:true}));
          form?.elements.namedItem("line1")?.focus();
          return;
        }
        const form = document.getElementById("checkout-address-form");
        if (!form) return;
        const fields = {
          recipient_name: option.dataset.name,
          phone: option.dataset.phone,
          line1: option.dataset.line1,
          line2: option.dataset.line2,
          city: option.dataset.city,
          state: option.dataset.state,
          postal_code: option.dataset.postalCode,
          country_code: "IN"
        };
        for (const [name, value] of Object.entries(fields)) {
          const field = form.elements.namedItem(name);
          if (field) field.value = value || "";
        }
        const regionField = form.elements.namedItem("region");
        if (regionField) regionField.value = option.dataset.state || "";
        const stateField = form.elements.namedItem("state");
        if (stateField) stateField.value = option.dataset.state || "";
        form.dispatchEvent(new Event("change", {bubbles:true}));
      });
    }
    document.addEventListener('cancel',function(event){
      if(event.target.matches('[data-backend-cart-drawer]')) {
        event.preventDefault();
        closeCartDrawer();
      }
    },true);
    document.addEventListener("click", async function (event) {
      if(event.target.matches('[data-backend-cart-drawer]')) closeCartDrawer();
      const link = event.target.closest("a[href]");
      if (!authenticated && link && !link.closest("#guest-login-dialog") && event.button===0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey && link.target!=='_blank' && !link.hasAttribute('download')) {
        const target = new URL(link.href, global.location.href);
        if (target.origin === global.location.origin && /^\/(cart|checkout|orders|wishlist|account|tracking|returns|payments|notifications)(\/|$)/.test(target.pathname)) {
          event.preventDefault();
          event.stopImmediatePropagation();
          promptLogin(target.pathname + target.search,link);
          return;
        }
      }
      const cartDrawerClose = event.target.closest("[data-cart-drawer-close]");
      if (cartDrawerClose) {
        event.preventDefault();
        closeCartDrawer();
        return;
      }
      const accountPrefill = event.target.closest('[data-checkout-prefill="account"]');
      if (accountPrefill) {
        event.preventDefault();
        const form = document.getElementById("checkout-address-form");
        if (!form) return;
        const name = form.elements.namedItem("recipient_name");
        const phone = form.elements.namedItem("phone");
        if (name && accountPrefill.dataset.name) name.value = accountPrefill.dataset.name;
        if (phone && accountPrefill.dataset.phone) phone.value = accountPrefill.dataset.phone;
        form.dispatchEvent(new Event("input", { bubbles: true }));
        form.elements.namedItem("line1")?.focus();
        notice("Account details added. Add your delivery address to continue.", false);
        return;
      }
      const passwordToggle = event.target.closest("[data-password-toggle]");
      if (passwordToggle) {
        event.preventDefault();
        const input = passwordToggle.parentElement && passwordToggle.parentElement.querySelector("input");
        if (!input) return;
        const visible = input.type === "password";
        input.type = visible ? "text" : "password";
        passwordToggle.setAttribute("aria-pressed", String(visible));
        passwordToggle.setAttribute("aria-label", (visible ? "Hide " : "Show ") + (passwordToggle.dataset.passwordLabel || "password"));
        const icon = passwordToggle.querySelector(".material-symbols-outlined");
        if (icon) icon.textContent = visible ? "visibility_off" : "visibility";
        input.focus({ preventScroll: true });
        return;
      }
      const localPaymentButton = event.target.closest("[data-local-payment]");
      if (localPaymentButton) {
        event.preventDefault();
        localPaymentButton.disabled = true; global.WVUI?.busy(localPaymentButton,true);
        try {
          await ensureBackend().localPayment(localPaymentButton.dataset.orderNumber, localPaymentButton.dataset.localPayment);
          global.location.assign("/orders/" + encodeURIComponent(localPaymentButton.dataset.orderNumber));
        } catch (error) {
          localPaymentButton.disabled = false; global.WVUI?.busy(localPaymentButton,false);
          notice(error.message, true);
        }
        return;
      }
      const checkoutButton = event.target.closest('[data-backend-action="checkout"], [data-backend-action="checkout-no-payment"]');
      if (checkoutButton) {
        event.preventDefault();
        if (!authenticated) { event.stopImmediatePropagation(); promptLogin("/checkout",checkoutButton); return; }
        if(checkoutInFlight || checkoutButton.disabled)return;
        checkoutInFlight=true;checkoutButton.disabled=true;global.WVUI?.busy(checkoutButton,true);
        try { await checkout(); } catch (error) { notice(error.message, true); }
        finally {
          checkoutInFlight=false;
          if(!checkoutButton.dataset.orderCreated){checkoutButton.disabled=false;global.WVUI?.busy(checkoutButton,false);}
        }
        return;
      }
      const element = event.target.closest("[data-backend-action], [hx-post]");
      if (!element) return;
      let action = element.dataset.backendAction;
      let slug = element.dataset.productSlug || "";
      let delta = element.dataset.delta || "";
      let selectedVariantId = element.dataset.variantId || "";
      if (!action || !slug) {
        const path = element.getAttribute("hx-post") || "";
        const match = path.match(/^\/ui\/(cart|wishlist)\/([^/?#]+)/);
        if (!match) return;
        slug = decodeURIComponent(match[2]);
        const values = element.getAttribute("hx-vals");
        let extras = {};
        try { extras = values ? JSON.parse(values) : {}; } catch (_) { extras = {}; }
        selectedVariantId = extras.variant_id || selectedVariantId;
        if (match[1] === "cart") {
          action = extras.remove ? "cart-remove" : extras.delta ? "cart-change" : element.closest(".saved-card") ? "wishlist-to-cart" : extras.intent === "buy" ? "cart-buy" : "cart-add";
          delta = extras.delta || document.getElementById("detail-quantity")?.value || "1";
        } else {
          action = action || (extras.source === "cart" ? "wishlist-move" : extras.source === "wishlist" ? "wishlist-remove" : "wishlist-add");
        }
      }
      if (!authenticated) {
        event.preventDefault();
        event.stopImmediatePropagation();
        const detailOption=selectedVariantId||document.querySelector('#detail-variant')?.value||'';
        const preserved=new URLSearchParams();if(detailOption)preserved.set('variant_id',detailOption);if(delta)preserved.set('quantity',delta);
        const loginTrigger = element.closest('#product-quick-view') ? global.WVQuickView?.close() : event.target.closest('button,a');
        promptLogin(slug ? "/products/" + encodeURIComponent(slug)+(preserved.size?'?'+preserved:'') : "/cart", loginTrigger);
        return;
      }
      if (backendError) {
        event.preventDefault();
        event.stopImmediatePropagation();
        notice(backendError.message, true);
        return;
      }
      if (!backend) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      const detailActions = element.closest(".detail-actions, .mobile-buy-bar");
      selectedVariantId = selectedVariantId || (detailActions ? document.querySelector("#detail-variant")?.value : undefined);
      if (element.dataset.pending === "true") return;
      const wishlistToggle = action === "wishlist-add" || action === "wishlist-remove";
      if (wishlistToggle && wishlistPending.has(slug)) return;
      const cartMutation = ['cart-add','cart-buy','cart-change','cart-remove','wishlist-move','wishlist-to-cart'].includes(action);
      const cartKey = slug + ':' + (selectedVariantId || 'default');
      if (cartMutation && cartPending.has(cartKey)) return;
      if (cartMutation) cartPending.add(cartKey);
      if (wishlistToggle) {
        ++wishlistRevision;
        wishlistPending.add(slug);
        wishlistBusy(slug, true);
      }
      const row=element.closest('.cart-line');
      const rowIndex=row?Array.from(document.querySelectorAll('.cart-items .cart-line')).indexOf(row):-1;
      const affected=row||element.closest('.detail-copy')||element.closest('.mobile-buy-bar');
      const peers=(detailActions?Array.from(document.querySelectorAll('.detail-actions button,.mobile-buy-bar button')):affected?Array.from(affected.querySelectorAll('button')):[]).filter(button=>!button.disabled);
      peers.forEach(button=>button.disabled=true);if(row)row.setAttribute('aria-busy','true');
      element.dataset.pending = "true";
      element.setAttribute("aria-busy", "true"); global.WVUI?.busy(element,true);
      if ("disabled" in element) element.disabled = true;
      try {
        await runAction(action, slug, delta, selectedVariantId, element);
        if (row && !element.isConnected) {
          const rows=Array.from(document.querySelectorAll('.cart-items .cart-line'));
          const same=Array.from(document.querySelectorAll('.cart-items [data-product-slug]')).find(button=>button.dataset.productSlug===slug && (!selectedVariantId || button.dataset.variantId===String(selectedVariantId)) && button.dataset.backendAction===action && (!delta || button.dataset.delta===String(delta)));
          const destination=same||rows[Math.min(rowIndex,rows.length-1)]?.querySelector('button')||document.querySelector('.cart-items .empty-state a');
          destination?.focus();
        }
        if(action==='cart-change')notice('Cart quantity updated.',false);
        if(action==='cart-remove')notice('Item removed from your cart.',false);
      }
      catch (error) {
        if (error.status === 401) {
          authenticated = false;
          promptLogin(global.location.pathname + global.location.search, element);
        } else notice(error.message, true);
        const quickFeedback = element.closest('[data-quick-product]')?.querySelector('[data-quick-feedback]');
        if (quickFeedback) quickFeedback.textContent = error.message;
      }
      finally {
        if (wishlistToggle) {
          wishlistPending.delete(slug);
          wishlistBusy(slug, false);
          ++wishlistRevision;
          void refreshCounts();
        }
        if (cartMutation) cartPending.delete(cartKey);
        peers.forEach(button=>{if(button.isConnected)button.disabled=false;});row?.removeAttribute('aria-busy');
        delete element.dataset.pending;
        element.removeAttribute("aria-busy"); global.WVUI?.busy(element,false);
        if ("disabled" in element) element.disabled = false;
        const variant=document.querySelector('#detail-variant');
        if (variant) variant.dispatchEvent(new Event('change',{bubbles:true}));
      }
    }, true);

    document.addEventListener("submit", function (event) {
      const form = event.target.closest("form[data-backend-auth]");
      if (form) { event.preventDefault(); event.stopImmediatePropagation(); submitAuth(form); return; }
      // Search/filter GETs are owned by their existing HTMX forms. Ordinary
      // links stay native, including modified clicks, history and new tabs.
    }, true);
    document.addEventListener("htmx:beforeRequest", function (event) {
      if (!backend || !authenticated) return;
      const target = event.detail && event.detail.target;
      const path = event.detail && event.detail.pathInfo && event.detail.pathInfo.requestPath;
      if (path && (path.startsWith("/ui/cart/") || path.startsWith("/ui/wishlist/"))) {
        event.preventDefault();
        notice("This action could not be completed. Please try again.", true);
      }
    });
    // Suggestions likewise use the maintained /ui/search-suggest response,
    // debounce and latest-request cancellation, not a second JSON renderer.
  }

  function startOrderStatusRefresh() {
    let region = document.querySelector("[data-order-status-region]");
    if (!region || !region.dataset.orderNumber) return;
    let timer = null;
    let busy = false;
    let paused = false;
    const refresh = async () => {
      if (busy || paused || document.visibilityState !== "visible") return;
      busy = true;
      try {
        const response = await fetch("/ui/orders/status?order=" + encodeURIComponent(region.dataset.orderNumber), { headers: { Accept: "text/html" }, credentials: "same-origin" });
        if (!response.ok) throw new Error("refresh failed");
        const html = await response.text();
        const parsed = document.createRange().createContextualFragment(html);
        const next = parsed.querySelector("[data-order-status-region]");
        if (next) {
          const active = document.activeElement;
          if (!region.contains(active)) { region.replaceWith(next); region = next; }
        } else throw new Error("refresh failed");
        let state = document.querySelector("[data-order-refresh-state]");
        if (state) { state.querySelector("[data-order-refresh-message]")?.replaceChildren(document.createTextNode("Last updated " + new Date().toLocaleTimeString())); state.classList.remove("is-error"); const retry = state.querySelector("[data-order-refresh-retry]"); if (retry) retry.hidden = true; }
      } catch (_) {
        const state = document.querySelector("[data-order-refresh-state]");
        if (state) { state.querySelector("[data-order-refresh-message]")?.replaceChildren(document.createTextNode("Live update unavailable")); state.classList.add("is-error"); const retry = state.querySelector("[data-order-refresh-retry]"); if (retry) retry.hidden = false; }
      } finally {
        busy = false;
      }
    };
    const schedule = () => { if (timer) global.clearInterval(timer); timer = global.setInterval(refresh, 15000); };
    const state = document.createElement("p");
    state.className = "order-refresh-state";
    state.dataset.orderRefreshState = "true";
    state.setAttribute("role", "status");
    const message = document.createElement("span");
    message.dataset.orderRefreshMessage = "true";
    message.textContent = "Live updates every 15 seconds";
    state.appendChild(message);
    const retry = document.createElement("button");
    retry.type = "button";
    retry.className = "text-button order-refresh-retry";
    retry.dataset.orderRefreshRetry = "true";
    retry.textContent = "Retry";
    retry.hidden = true;
    retry.addEventListener("click", refresh);
    state.appendChild(retry);
    const pause = document.createElement("button");
    pause.type = "button"; pause.className = "text-button"; pause.textContent = "Pause updates";
    pause.addEventListener("click", () => {
      paused = !paused; pause.textContent = paused ? "Resume updates" : "Pause updates";
      message.textContent = paused ? "Live updates paused" : "Live updates every 15 seconds";
      if (!paused) refresh();
    });
    state.appendChild(pause);
    region.parentElement?.insertBefore(state, region);
    document.addEventListener("visibilitychange", () => { if (document.visibilityState === "visible") refresh(); });
    refresh();
    schedule();
  }

  async function start() {
    if(document.body.dataset.storefrontPreviewId)return;
    if (/^\/orders\/[^/]+\/confirmation$/.test(global.location.pathname)) {
      const url = new URL(global.location.href);
      if (url.searchParams.has("notice")) {
        url.searchParams.delete("notice");
        global.history.replaceState(null, "", url.pathname + url.search + url.hash);
      }
    }
    const siteHeader = document.querySelector('.site-header');
    if (siteHeader) {
      const updateHeader = () => {
        const compact = true;
        siteHeader.classList.toggle('is-compact', compact);
        siteHeader.querySelector('.header-main')?.classList.toggle('is-compact', compact);
      };
      // One stable toolbar at every scroll position.
      updateHeader();
    }
    const photoInput = document.querySelector("[data-photo-input]");
    if (photoInput) {
      const preview = document.querySelector("[data-photo-preview]");
      const initialPreview = preview.innerHTML;
      let previewURL;
      photoInput.addEventListener("change", function () {
        if (previewURL) global.URL.revokeObjectURL(previewURL);
        const file = photoInput.files[0];
        preview.innerHTML = initialPreview;
        if (!file) return;
        // Some browsers report WebP as application/octet-stream (or omit the
        // type). The server verifies the actual decoded image before saving.
        const imageType = ["image/jpeg", "image/png", "image/webp"].includes(file.type);
        const unknownType = !file.type || file.type === "application/octet-stream";
        const imageName = /\.(jpe?g|png|webp)$/i.test(file.name);
        if (file.size > 5 * 1024 * 1024 || (!imageType && !(unknownType && imageName))) {
          photoInput.value = "";
          notice("Choose a JPEG, PNG or WebP photo up to 5 MiB.", true);
          return;
        }
        const removal = document.querySelector('[name="remove_photo"]');
        if (removal) removal.checked = false;
        previewURL = global.URL.createObjectURL(file);
        const img = document.createElement("img");
        img.className = "customer-avatar"; img.alt = "Selected photo preview";
        img.src = previewURL; img.width = 96; img.height = 96;
        preview.replaceChildren(img);
      });
    }
    bindActions();
    reconcileWishlistHearts();
    document.addEventListener('htmx:afterSettle', reconcileWishlistHearts);
    document.addEventListener('htmx:historyRestore', reconcileWishlistHearts);
    try {
      backend = await create();
      global.weevcraftsBackend = backend;
    } catch (error) {
      backendError = error;
      global.weevcraftsBackendError = error;
      return;
    }
    // The server-rendered session is authoritative for this document. Optional
    // startup reads are isolated from adapter creation and from one another.
    void refreshCounts();
    try {
      if (global.location.pathname === "/deals") {
        const offerBadge = document.querySelector(".deals-banner .badge");
        if (offerBadge) offerBadge.textContent = "Current catalog offers";
      }
      const verification = document.querySelector("[data-backend-verification]");
      if (verification) {
        const user = await backend.currentUser();
        const message = verification.querySelector("p");
        if (message) message.textContent = user ? "Your sign-in is ready. Continue to your account." : "Use the verification link sent to your email, then sign in to continue.";
        const continueLink = document.querySelector("[data-backend-verification-continue]");
        if (user && continueLink) { continueLink.href = "/orders"; continueLink.textContent = "View your orders"; }
      }
      // Catalog and product markup is rendered from the Go services before
      // this script runs. Keep that response authoritative; only mutations
      // and account state go through the JSON API.
      // Fresh cart/wishlist pages are already server-rendered. Hydrate only
      // after a mutation or when opening the drawer; avoid replacing usable
      // rows (and their focus/geometry) during initial navigation.
      const initialCart=document.querySelector('[data-backend-cart] .cart-items');
      if(initialCart) void backend.cart().then(mediaCart => wireCartLineActions(initialCart,mediaCart.items)).catch(() => {});
      if (global.location.pathname === "/orders") void hydrateOrders().catch(() => {});
      startOrderStatusRefresh();
      try {
        const authNotice = global.sessionStorage.getItem("weevcrafts_auth_notice");
        if (authNotice) {
          global.sessionStorage.removeItem("weevcrafts_auth_notice");
          notice(authNotice, true);
        }
      } catch (_) { /* Storage can be disabled by the browser. */ }
    } catch (_) { /* Preserve rendered content when an optional startup read fails. */ }
  }

  // The adapter renews its read controller in its own pageshow listener. Run
  // after every listener so a restored document does not reuse an aborted read.
  global.addEventListener('pageshow', event => {
    if (event.persisted && backend) global.setTimeout(() => { void refreshCounts(); }, 0);
  });
  document.addEventListener('visibilitychange', () => { if (!document.hidden) void refreshCounts(); });

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start, { once: true });
  else start();
})(window);
