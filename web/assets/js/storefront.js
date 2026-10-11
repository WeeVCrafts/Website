(function (global) {
  "use strict";

  const { create, normalizeProduct } = global.WeevcraftsBackend;
  const CHECKOUT_KEY = "weevcrafts.checkout.idempotency.v1";
  let backend = null;
  let backendError = null;
  let authenticated = false;
  let razorpayLoader = null;
  let drawerRestoreFocus = null;

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

  function showUnknownCounts() {
    document.querySelectorAll("[data-backend-counter] em").forEach((counter) => { counter.textContent = "—"; });
    document.querySelectorAll("[data-backend-counter]").forEach(markReady);
  }

  function priceMarkup(product) {
    return "<div class=\"price\"><strong>" + money(product.priceCents) + "</strong>" +
      (product.compareAtCents > product.priceCents ? "<del>" + money(product.compareAtCents) + "</del>" : "") + "</div>";
  }

  function cardMarkup(product) {
    const slug = esc(product.slug);
    return "<article class=\"product-card\" data-product=\"" + slug + "\"><div class=\"product-media\"><a href=\"/products/" + slug + "\" tabindex=\"-1\" aria-hidden=\"true\"><img src=\"" + esc(product.image) + "\" alt=\"" + esc(product.name) + "\" loading=\"lazy\" width=\"640\" height=\"691\"/></a><button class=\"heart\" type=\"button\" data-backend-action=\"wishlist-add\" data-product-slug=\"" + slug + "\" aria-label=\"Save " + esc(product.name) + "\"><span class=\"material-symbols-outlined\" aria-hidden=\"true\">favorite</span></button><span class=\"badge\">" + (product.available > 0 ? "In stock" : "Sold out") + "</span></div><div class=\"product-copy\"><h3><a href=\"/products/" + slug + "\">" + esc(product.name) + "</a></h3><p>By " + esc(product.seller) + "</p><div class=\"rating\"><span>" + (product.rating ? product.rating.toFixed(1) : "New") + " (" + Number(product.reviews || 0) + ")</span></div>" + priceMarkup(product) + "<button class=\"outline-button add-button\" type=\"button\" data-backend-action=\"cart-add\" data-product-slug=\"" + slug + "\" " + (product.available > 0 ? "" : "disabled") + "><span class=\"material-symbols-outlined\" aria-hidden=\"true\">shopping_bag</span> Add to Cart</button></div></article>";
  }

  function savedMarkup(product) {
    const slug = esc(product.slug);
    return "<article class=\"saved-card\" data-product=\"" + slug + "\"><div class=\"saved-media\"><a href=\"/products/" + slug + "\" tabindex=\"-1\" aria-hidden=\"true\"><img src=\"" + esc(product.image) + "\" alt=\"" + esc(product.name) + "\" width=\"320\" height=\"346\"/></a><button class=\"heart is-active\" aria-pressed=\"true\" type=\"button\" data-backend-action=\"wishlist-remove\" data-product-slug=\"" + slug + "\" aria-label=\"Remove from wishlist\"><span class=\"material-symbols-outlined\" aria-hidden=\"true\">favorite</span></button></div><div class=\"saved-copy\"><span class=\"badge\">" + (product.available > 0 ? "In stock" : "Sold out") + "</span><h3><a href=\"/products/" + slug + "\">" + esc(product.name) + "</a></h3><p>By " + esc(product.seller) + "</p>" + priceMarkup(product) + "<small class=\"availability\">" + (product.available > 0 ? "In stock" : "Currently unavailable") + "</small><div><button class=\"primary-button\" type=\"button\" data-backend-action=\"wishlist-to-cart\" data-product-slug=\"" + slug + "\">Move to cart</button><button class=\"text-button\" type=\"button\" data-backend-action=\"wishlist-remove\" data-product-slug=\"" + slug + "\" aria-label=\"Remove from wishlist\">Remove</button></div></div></article>";
  }

  async function hydrateCatalog() {
    const grids = document.querySelectorAll("[data-backend-products]");
    if (!grids.length) return;
    const params = new URLSearchParams(global.location.search);
    for (const grid of grids) {
      const mode = grid.dataset.backendProducts;
      const category = grid.dataset.backendCategory || params.get("category") || "";
      const brand = grid.dataset.backendBrand || params.get("brand") || "";
      const seller = grid.dataset.backendSeller || "";
      const rawCategories = params.getAll("category");
      const rawBrands = params.getAll("brand");
      const filters = {
        category: category === "offers" ? "offers" : rawCategories.length === 1 ? rawCategories[0] : "",
        categories: rawCategories,
        brand: brand,
        brands: rawBrands,
        seller: seller,
        query: params.get("q") || "",
        sort: params.get("sort") || "",
        limit: mode === "featured" ? 6 : 48
      };
      if (category === "offers") filters.category = "offers";
      if (brand) filters.brands = [brand];
      if (seller) filters.seller = seller;
      grid.setAttribute("aria-busy", "true");
      try {
        let products = await ensureBackend().products(filters);
        const brands = filters.brands.map((value) => value.toLowerCase());
        const categories = filters.categories.map((value) => value.toLowerCase());
        if (brands.length) products = products.filter((product) => brands.some((value) => product.brand.toLowerCase() === value || product.brandSlug.toLowerCase() === value));
        if (categories.length) products = products.filter((product) => categories.includes(product.categorySlug.toLowerCase()));
        if (seller) products = products.filter((product) => product.sellerSlug.toLowerCase() === seller.toLowerCase());
        const price = params.get("price") || "";
        if (price === "under-1000") products = products.filter((product) => product.priceCents < 100000);
        if (price === "1000-2500") products = products.filter((product) => product.priceCents >= 100000 && product.priceCents <= 250000);
        if (price === "2500-5000") products = products.filter((product) => product.priceCents >= 250000 && product.priceCents <= 500000);
        if (price === "over-5000") products = products.filter((product) => product.priceCents > 500000);
        if (params.get("availability") === "in-stock") products = products.filter((product) => product.available > 0);
        grid.innerHTML = products.length ? products.map(cardMarkup).join("") : "<div class=\"empty-state\" role=\"status\"><h2>No products found</h2><p>Try another category or search.</p></div>";
        const count = document.querySelector("[data-backend-result-count]");
        if (count && grid.closest("#listing-results")) { count.textContent = products.length + " results"; markReady(count); }
      } catch (error) {
        grid.innerHTML = "<div class=\"empty-state\" role=\"alert\"><h2>Catalog is unavailable</h2><p>" + esc(error.message) + "</p></div>";
      } finally {
        markReady(grid);
        const count = document.querySelector("[data-backend-result-count]");
        if (count && !count.dataset.backendReady) { count.textContent = ""; markReady(count); }
        grid.removeAttribute("aria-busy");
      }
    }
  }

  async function hydrateProduct() {
    const root = document.querySelector("[data-backend-product]");
    if (!root) return;
    try {
      const product = await ensureBackend().product(root.dataset.backendProduct);
      const name = root.querySelector("h1");
      const description = root.querySelector(".description");
      const lead = root.querySelector(".lead");
      const price = root.querySelector(".detail-price");
      if (name) name.textContent = product.name;
      if (description && product.description) description.textContent = product.description;
      if (lead && product.description) lead.textContent = product.description;
      if (price) price.innerHTML = priceMarkup(product);
      root.querySelectorAll(".gallery img").forEach((image) => { image.src = product.image; image.alt = product.name; });
      const optionDetails = root.querySelector(".detail-copy fieldset");
      if (optionDetails) optionDetails.innerHTML = "<legend>Option: <b>" + esc(product.optionLabel) + "</b></legend>";
      const specifications = root.querySelector("#product-panel-specs");
      if (specifications) specifications.innerHTML = "<div class=\"spec-grid\"><span>Category <b>" + esc(product.category || "—") + "</b></span><span>Maker <b>" + esc(product.seller) + "</b></span><span>Brand <b>" + esc(product.brand || "—") + "</b></span><span>Available units <b>" + Number(product.available) + "</b></span></div>";
      const makerStory = root.querySelector("#product-panel-story");
      if (makerStory) makerStory.innerHTML = "<p>The maker story is not available for this product yet.</p>";
      const sellerLine = root.querySelector(".seller-line");
      if (sellerLine) {
        sellerLine.href = product.brandSlug ? "/products?brand=" + encodeURIComponent(product.brandSlug) : "/products";
        const label = sellerLine.querySelector("span");
        if (label && label.firstChild) label.firstChild.textContent = "By " + product.seller;
        const location = sellerLine.querySelector("small");
        if (location && location.lastChild) location.lastChild.textContent = " " + (product.location || "Independent maker");
      }
      const caption = root.querySelector(".gallery-caption");
      if (caption) caption.textContent = product.location ? "Made in " + product.location : "Made by " + product.seller;
      const makerLink = document.querySelector(".seller-callout a.text-link");
      if (makerLink) makerLink.href = product.brandSlug ? "/products?brand=" + encodeURIComponent(product.brandSlug) : "/products";
      const makerSection = document.querySelector(".seller-callout");
      if (makerSection) {
        const makerImage = makerSection.querySelector("img");
        const makerHeading = makerSection.querySelector("h2");
        const makerCopy = makerSection.querySelector("div > p:not(.eyebrow)");
        if (makerImage) { makerImage.src = product.image; makerImage.alt = product.name; }
        if (makerHeading) makerHeading.textContent = "More from " + product.seller;
        if (makerCopy) makerCopy.textContent = "Explore more live catalog items from this brand.";
      }
      const rating = root.querySelector(".stars");
      if (rating) rating.textContent = "No verified reviews yet.";
      const reviewPanel = root.querySelector("#product-panel-reviews");
      if (reviewPanel) reviewPanel.innerHTML = "<p>Verified reviews from delivered purchases will appear here after moderation.</p>";
      const detailActions = root.querySelectorAll(".detail-actions button");
      detailActions.forEach((button, index) => {
        button.dataset.backendAction = index === 1 ? "cart-buy" : "cart-add";
        button.dataset.productSlug = product.slug;
      });
      root.querySelectorAll(".gallery-heart").forEach((button) => {
        button.dataset.backendAction = "wishlist-add";
        button.dataset.productSlug = product.slug;
      });
      root.querySelectorAll(".detail-actions, .quantity-row").forEach((element) => { element.dataset.productSlug = product.slug; });
      root.querySelectorAll("[hx-post*='/ui/cart/']").forEach((button) => {
        button.dataset.productSlug = product.slug;
        button.dataset.backendAction = button.getAttribute("hx-vals")?.includes("buy") ? "cart-buy" : "cart-add";
      });
      document.querySelectorAll(".mobile-buy-bar [hx-post*='/ui/cart/']").forEach((button) => {
        button.dataset.productSlug = product.slug;
        button.dataset.backendAction = button.getAttribute("hx-vals")?.includes("buy") ? "cart-buy" : "cart-add";
      });
      const stock = root.querySelector(".stock");
      if (stock) stock.textContent = product.available > 0 ? product.available + " available" : "Currently unavailable";
      const benefitCopy = [["Delivery", "Shown before order placement"], ["Returns", "See the current returns policy"], ["Payment", "Payment is handled at checkout"], ["Shipping", "Availability is confirmed at checkout"]];
      root.querySelectorAll(".benefits > span").forEach((benefit, index) => {
        const title = benefit.querySelector("b");
        const detail = benefit.querySelector("small");
        if (title && benefitCopy[index]) title.textContent = benefitCopy[index][0];
        if (detail && benefitCopy[index]) detail.textContent = benefitCopy[index][1];
      });
      if (product.available <= 0) root.querySelectorAll('[data-backend-action="cart-add"], [data-backend-action="cart-buy"]').forEach((button) => { button.disabled = true; });
    } catch (error) {
      root.innerHTML = "<div class=\"empty-state\" role=\"alert\"><h1>Product unavailable</h1><p>" + esc(error.message) + "</p><a class=\"primary-button\" href=\"/products\">Browse products</a></div>";
    } finally {
      markReady(root);
    }
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

  function searchToURL(input) {
    const query = input.value.trim();
    global.location.assign("/search" + (query ? "?q=" + encodeURIComponent(query) : ""));
  }

  function normalizeCart(payload) {
    const data = unwrap(payload) || {};
    const items = (data.items || []).map((item) => {
      if (item.slug && item.name) return item;
      return global.WeevcraftsBackend.normalizeCartItem(item);
    }).filter((item) => item && item.slug);
    return { id: data.id, items: items };
  }

  function cartTotals(items) {
    const subtotal = items.reduce((sum, item) => sum + Number(item.priceCents || 0) * Number(item.quantity || 0), 0);
    const savings = items.reduce((sum, item) => sum + Math.max(0, Number(item.compareAtCents || 0) - Number(item.priceCents || 0)) * Number(item.quantity || 0), 0);
    const shipping = subtotal > 0 && subtotal < 99900 ? 9900 : 0;
    return { subtotal: subtotal, savings: savings, shipping: shipping, total: subtotal + shipping, count: items.reduce((sum, item) => sum + Number(item.quantity || 0), 0) };
  }

  function updateBadges(cart, wishlist) {
    const cartCount = document.querySelector("#cart-count em");
    const wishlistCount = document.querySelector("#wishlist-count em");
    const totals = cartTotals(cart.items);
    if (cartCount) { cartCount.textContent = String(totals.count); cartCount.closest('a').setAttribute('aria-label','Cart, '+totals.count+' items'); }
    if (wishlistCount) { wishlistCount.textContent = String(wishlist.length); wishlistCount.closest('a').setAttribute('aria-label','Wishlist, '+wishlist.length+' items'); }
    markReady(document.getElementById("cart-count"));
    markReady(document.getElementById("wishlist-count"));
  }

  function renderCartDrawer(cart, signedIn) {
    const drawer = document.querySelector("[data-backend-cart-drawer]");
    if (!drawer) return;
    const itemsHost = drawer.querySelector("[data-cart-drawer-items]");
    const summary = drawer.querySelector("[data-cart-drawer-summary]");
    const countLabel = drawer.querySelector("[data-cart-drawer-count]");
    const items = cart.items || [];
    const totals = cartTotals(items);
    if (countLabel) countLabel.textContent = totals.count + " item" + (totals.count === 1 ? "" : "s") + " from independent Indian makers";
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
      const subtotal = summary.querySelector("[data-cart-drawer-subtotal]");
      const shipping = summary.querySelector("[data-cart-drawer-shipping]");
      const total = summary.querySelector("[data-cart-drawer-total]");
      if (subtotal) subtotal.textContent = money(totals.subtotal);
      if (shipping) shipping.textContent = money(totals.shipping);
      if (total) total.textContent = money(totals.total);
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
    updateBadges(cart, wishlist);
    renderCartDrawer(cart, true);
    if (options.open) openCartDrawer(options.trigger);
    const cartCountLabel = document.querySelector("[data-backend-cart-count]");
    if (cartCountLabel) { cartCountLabel.textContent = cartTotals(cart.items).count + " items from independent Indian makers"; markReady(cartCountLabel); }
    const wishlistCountLabel = document.querySelector("[data-backend-wishlist-count]");
    if (wishlistCountLabel) { wishlistCountLabel.textContent = wishlist.length + " items in your wishlist"; markReady(wishlistCountLabel); }
    if (cartRoot) {
      const host = cartRoot.querySelector(".cart-items");
      host.innerHTML = cart.items.length ? cart.items.map(cartItemMarkup).join("") : "<div class=\"empty-state\"><h2>Your cart is empty</h2><p>Browse handmade treasures and bring a little India home.</p><a class=\"primary-button\" href=\"/products\">Continue shopping</a></div>";
      wireCartLineActions(host, cart.items);
      const totals = cartTotals(cart.items);
      const summary = cartRoot.querySelector(".cart-summary");
      const layout=document.getElementById("cart-layout");
      if(layout) { layout.classList.toggle("cart-layout",cart.items.length>0); layout.classList.toggle("cart-layout--empty",cart.items.length===0); }
      if (summary) {
        summary.hidden=cart.items.length===0;
        const values = summary.querySelectorAll("p b");
        if (values[0]) values[0].textContent = money(totals.subtotal);
        if (values[1]) values[1].textContent = money(totals.shipping);
        if (values[2]) values[2].textContent = money(totals.savings);
        const total = summary.querySelector("h3 strong");
        if (total) total.textContent = money(totals.total);
        const heading = summary.querySelector("h2");
        if (heading) heading.textContent = "Order summary (" + totals.count + " items)";
      }
      markReady(cartRoot);
    }
    if (wishlistRoot) {
      wishlistRoot.innerHTML = wishlist.length ? wishlist.map(savedMarkup).join("") : "<div class=\"empty-state\"><h2>Your wishlist is empty</h2><p>Save pieces you love and they will appear here.</p><a class=\"primary-button\" href=\"/products\">Discover handmade finds</a></div>";
      markReady(wishlistRoot);
    }
  }

  async function refreshCounts() {
    const user = await ensureBackend().currentUser();
    authenticated = Boolean(user);
    if (!user) return;
    const cart = normalizeCart(await backend.cart());
    const wishlist = await backend.wishlist();
    updateBadges(cart, wishlist);
  }

  async function requireUser() {
    const user = await ensureBackend().currentUser();
    if (!user) throw new Error("Sign in to save a cart, wishlist, or order.");
    return user;
  }

  function syncWishlistHearts(slug, saved) {
    document.querySelectorAll(".heart, .gallery-heart").forEach(function (button) {
      const path = button.getAttribute("hx-post") || "";
      const buttonSlug = button.dataset.productSlug || (path.startsWith("/ui/wishlist/") ? decodeURIComponent(path.slice(13)) : "");
      if (buttonSlug !== slug) return;
      button.dataset.productSlug = slug;
      button.dataset.backendAction = saved ? "wishlist-remove" : "wishlist-add";
      button.classList.toggle("is-active", saved);
      button.setAttribute("aria-pressed", String(saved));
      const name = button.closest("article")?.querySelector("h3")?.textContent || "product";
      button.setAttribute("aria-label", (saved ? "Remove " : "Save ") + name + (saved ? " from wishlist" : " to wishlist"));
      const icon = button.querySelector(".material-symbols-outlined");
      if (icon) icon.textContent = saved ? "favorite" : "favorite_border";
    });
  }

  const wishlistPending = new Set();
  async function runAction(action, slug, delta, selectedVariantId, triggerElement) {
    const api = ensureBackend();
    if (action === "cart-add") {
      await requireUser();
      const quantity = Number(delta || document.getElementById("detail-quantity")?.value || 1);
      await api.addCart(slug, quantity, selectedVariantId);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await refreshCounts();
      await hydrateCart({ open: true, trigger: triggerElement });
      notice("Added to your cart.", false);
      return;
    }
    if (action === "cart-buy") {
      await requireUser();
      await api.addCart(slug, Number(delta || document.getElementById("detail-quantity")?.value || 1), selectedVariantId);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await refreshCounts();
      global.location.assign("/checkout");
      return;
    }
    if (action === "cart-change") {
      await requireUser();
      await api.changeCart(slug, Number(delta), selectedVariantId);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await hydrateCart();
      return;
    }
    if (action === "cart-remove") {
      await requireUser();
      await api.removeCart(slug, selectedVariantId);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await hydrateCart();
      return;
    }
    if (action === "wishlist-add") {
      await requireUser();
      const heartBtn = triggerElement ? (triggerElement.closest(".heart, .gallery-heart") || triggerElement) : null;
      const optimisticallyGlowed = Boolean(heartBtn) && !heartBtn.classList.contains("is-active");
      if (optimisticallyGlowed) heartBtn.classList.add("is-active");
      try {
        await api.addWishlist(slug);
      } catch (error) {
        if (optimisticallyGlowed) heartBtn.classList.remove("is-active");
        throw error;
      }
      if (heartBtn) {
        heartBtn.classList.add("is-active");
        heartBtn.setAttribute("aria-pressed", "true");
        const icon = heartBtn.querySelector(".material-symbols-outlined");
        if (icon) icon.textContent = "favorite";
        heartBtn.dataset.backendAction = "wishlist-remove";
        heartBtn.dataset.productSlug = slug;
        heartBtn.setAttribute("aria-label", "Remove product from wishlist");
      }
      syncWishlistHearts(slug, true);
      await refreshCounts().catch(function () { notice("Saved. Refresh the page to update the wishlist count.", false); });
      notice("Saved to your wishlist.", false);
      return;
    }
    if (action === "wishlist-remove") {
      await requireUser();
      await api.removeWishlist(slug);
      syncWishlistHearts(slug, false);
      if (triggerElement) {
        const heartBtn = triggerElement.closest(".heart, .gallery-heart") || triggerElement;
        const isOnProductCard = heartBtn.closest(".product-card, .gallery-main");
        if (isOnProductCard) {
          heartBtn.classList.remove("is-active");
          heartBtn.setAttribute("aria-pressed", "false");
          const icon = heartBtn.querySelector(".material-symbols-outlined");
          if (icon) icon.textContent = "favorite_border";
          heartBtn.dataset.backendAction = "wishlist-add";
          heartBtn.dataset.productSlug = slug;
          heartBtn.setAttribute("aria-label", "Save product to wishlist");
          await refreshCounts();
          return;
        }
      }
      await hydrateCart();
      return;
    }
    if (action === "wishlist-move") {
      await requireUser();
      await api.moveCartToWishlist(slug, selectedVariantId);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await hydrateCart();
      notice("Moved to your wishlist.", false);
      return;
    }
    if (action === "wishlist-to-cart") {
      await requireUser();
      await api.addCart(slug, 1);
      await api.removeWishlist(slug);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await hydrateCart();
      notice("Moved to your cart.", false);
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
      target.setAttribute("role", "status");
      form.appendChild(target);
    }
    target.textContent = text;
    target.classList.toggle("is-error", Boolean(error));
  }

  async function submitAuth(form) {
    if (form.dataset.submitting === "true") return;
    form.dataset.submitting = "true";
    const authButton = form.querySelector('button[type="submit"]');
    if (authButton) {authButton.disabled = true; global.WVUI?.busy(authButton,true);}
    const api = ensureBackend();
    const action = form.dataset.backendAuth;
    const values = formData(form);
    try {
      if (action === "login") {
        const result = await api.signIn({ email: values.email, password: values.password, remember: values.remember === "on", next: values.next || "" });
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
    const main = document.getElementById("page");
    if (!main) return;
    const encoded = encodeURIComponent(orderNumber);
    const simulator = localSimulator
      ? "<div class=\"payment-simulator\" aria-label=\"Local payment test actions\"><button class=\"primary-button\" type=\"button\" data-local-payment=\"capture\" data-order-number=\"" + esc(orderNumber) + "\">Simulate successful payment</button><button class=\"outline-button\" type=\"button\" data-local-payment=\"fail\" data-order-number=\"" + esc(orderNumber) + "\">Simulate failed payment</button><button class=\"text-button\" type=\"button\" data-local-payment=\"pending\" data-order-number=\"" + esc(orderNumber) + "\">Leave payment pending</button></div>"
      : "";
    main.innerHTML = "<section class=\"container checkout-complete\" role=\"status\"><span class=\"material-symbols-outlined\" aria-hidden=\"true\">receipt_long</span><h1>Order " + esc(orderNumber) + "</h1><p>" + esc(message) + "</p>" + simulator + "<a class=\"primary-button\" href=\"/orders/" + encoded + "\">View order details</a><a class=\"outline-button\" href=\"/orders/" + encoded + "/tracking\">Track delivery</a></section>";
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
      paymentReceipt(orderNumber, "Your order is saved. No money will be charged. Choose a mock payment result below.", true);
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
      name: "Weevcrafts",
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
    const response = await fetch("/api/v1/checkout/quote?coupon_code=" + encodeURIComponent(code), { credentials: "same-origin", cache: "no-store" });
    const body = await response.json();
    if (!response.ok) throw new Error(body.error?.message || "Could not preview checkout. Please try again.");
    const quote = unwrap(body);
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
      const addressFormData = new URLSearchParams(address);
      const savedAddressSelect = document.querySelector("[data-checkout-address-select]");
      if (savedAddressSelect && savedAddressSelect.value) addressFormData.set("address_id", savedAddressSelect.value);
      try {
        const addressResponse = await fetch("/account/addresses", {
          method: "POST",
          credentials: "same-origin",
          headers: { "X-CSRF-Token": ensureBackend().csrf || form.querySelector('[name="csrf_token"]')?.value || "" },
          body: addressFormData
        });
        if (!addressResponse.ok) {
          console.warn("Could not save address to address book:", await addressResponse.text().catch(() => ""));
        }
      } catch (addressErr) {
        console.warn("Could not save address to address book:", addressErr);
      }
      const couponCode = acceptedCoupon;
      const key = pending.key;
      const result = unwrap(await (noPaymentDemo ? ensureBackend().checkoutNoPayment(address, key, couponCode.trim()) : ensureBackend().checkout(address, key, couponCode.trim(), currentQuote.total_cents, pending.cartID))) || {};
      orderNumber = result.order_number || result.orderNumber || "";
      if (!orderNumber) throw new Error("The order was created, but the server did not return its number. Open your orders to check its status.");
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      const cartBadge = document.querySelector("#cart-count em");
      if (cartBadge) { cartBadge.textContent = "0"; cartBadge.closest('a').setAttribute('aria-label','Cart, 0 items'); markReady(document.getElementById("cart-count")); }
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
        notice(error.message, true);
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
    const cartDrawer = document.querySelector("[data-backend-cart-drawer]");
    if (cartDrawer) {
      cartDrawer.addEventListener("cancel", function (event) {
        event.preventDefault();
        closeCartDrawer();
      });
      cartDrawer.addEventListener("click", function (event) {
        if (event.target === cartDrawer) closeCartDrawer();
      });
    }
    document.addEventListener("click", async function (event) {
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
        passwordToggle.setAttribute("aria-label", visible ? "Hide password" : "Show password");
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
        try { await checkout(); } catch (error) { notice(error.message, true); }
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
      // The server-rendered storefront owns guest cart mutations. Let HTMX
      // submit them with the anonymous CSRF token instead of requiring the
      // account-only JSON API.
      if (!authenticated && (action === "cart-add" || action === "cart-buy" || action === "cart-change" || action === "cart-remove")) return;
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
      if (wishlistToggle) wishlistPending.add(slug);
      element.dataset.pending = "true";
      element.setAttribute("aria-busy", "true"); global.WVUI?.busy(element,true);
      if ("disabled" in element) element.disabled = true;
      try { await runAction(action, slug, delta, selectedVariantId, element); }
      catch (error) { notice(error.message, true); }
      finally {
        if (wishlistToggle) wishlistPending.delete(slug);
        delete element.dataset.pending;
        element.removeAttribute("aria-busy"); global.WVUI?.busy(element,false);
        if ("disabled" in element) element.disabled = false;
      }
    }, true);

    document.addEventListener("submit", function (event) {
      const form = event.target.closest("form[data-backend-auth]");
      if (form) { event.preventDefault(); event.stopImmediatePropagation(); submitAuth(form); return; }
      const search = event.target.closest("form.global-search");
      if (search && backend) { event.preventDefault(); searchToURL(search.querySelector("#global-search")); }
    }, true);
    document.addEventListener("change", function (event) {
      const form = event.target.closest("#listing-filters");
      if (!form || !backend) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      const query = new URLSearchParams(new FormData(form));
      const target = form.action + (query.size ? "?" + query.toString() : "");
      global.location.assign(target);
    }, true);
    document.addEventListener("click", function (event) {
      if (!backend) return;
      const anchor = event.target.closest("a[href]");
      if (!anchor || anchor.target || anchor.hasAttribute("download") || anchor.getAttribute("href").startsWith("#")) return;
      let target;
      try { target = new URL(anchor.href, global.location.href); } catch (_) { return; }
      if (target.origin !== global.location.origin) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      global.location.assign(target.href);
    }, true);
    document.addEventListener("htmx:beforeRequest", function (event) {
      if (!backend || !authenticated) return;
      const target = event.detail && event.detail.target;
      const path = event.detail && event.detail.pathInfo && event.detail.pathInfo.requestPath;
      if (target && target.id === "search-suggestions") { event.preventDefault(); return; }
      if (path && (path.startsWith("/ui/cart/") || path.startsWith("/ui/wishlist/"))) {
        event.preventDefault();
        notice("This action could not be completed. Please try again.", true);
      }
    });
    const search = document.getElementById("global-search");
    if (search) {
      let suggestionTimer;
      search.addEventListener("input", function () {
        global.clearTimeout(suggestionTimer);
        suggestionTimer = global.setTimeout(async function () {
          const list = document.getElementById("search-suggestions");
          const term = search.value.trim();
          if (!backend || !list || term.length < 2) { if (list) list.innerHTML = ""; return; }
          try {
            const products = await backend.products({ query: term, limit: 5 });
            list.innerHTML = products.map((product, index) => "<a id=\"search-option-" + index + "\" role=\"option\" aria-selected=\"false\" href=\"/products/" + encodeURIComponent(product.slug) + "\">" + esc(product.name) + "</a>").join("");
          } catch (_) { list.innerHTML = ""; }
        }, 300);
      });
    }
  }

  async function start() {
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
    try {
      backend = await create();
      global.weevcraftsBackend = backend;
      authenticated = Boolean(await backend.currentUser());
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
      if (authenticated && ["/cart", "/wishlist"].includes(global.location.pathname)) await hydrateCart();
      if (global.location.pathname === "/orders") await hydrateOrders();
      await refreshCounts();
      try {
        const authNotice = global.sessionStorage.getItem("weevcrafts_auth_notice");
        if (authNotice) {
          global.sessionStorage.removeItem("weevcrafts_auth_notice");
          notice(authNotice, true);
        }
      } catch (_) { /* Storage can be disabled by the browser. */ }
    } catch (error) {
      backend = null;
      backendError = error;
      global.weevcraftsBackendError = error;
      showUnknownCounts();
      notice("Some live account counts could not be refreshed. Please try again.", true);
    }
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start, { once: true });
  else start();
})(window);
