(function (global) {
  "use strict";

  const { create, normalizeProduct } = global.WeeVCraftsBackend;
  const CHECKOUT_KEY = "weevcrafts.checkout.idempotency.v1";
  let backend = null;
  let backendError = null;
  let razorpayLoader = null;

  const esc = (value) => String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;"
  })[char]);
  const money = (cents) => "INR " + (Number(cents || 0) / 100).toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  const unwrap = (value) => value && Object.prototype.hasOwnProperty.call(value, "data") ? value.data : value;

  function notice(message, error) {
    let target = document.querySelector("[data-backend-message]") || document.getElementById("ui-toast");
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
    return "<article class=\"product-card\" data-product=\"" + slug + "\"><div class=\"product-media\"><img src=\"" + esc(product.image) + "\" alt=\"" + esc(product.name) + "\" loading=\"lazy\" width=\"640\" height=\"691\"/><button class=\"heart\" type=\"button\" data-backend-action=\"wishlist-add\" data-product-slug=\"" + slug + "\" aria-label=\"Save " + esc(product.name) + "\"><span class=\"material-symbols-outlined\" aria-hidden=\"true\">favorite</span></button><span class=\"badge\">" + (product.available > 0 ? "In stock" : "Sold out") + "</span></div><div class=\"product-copy\"><h3><a href=\"/products/" + slug + "\">" + esc(product.name) + "</a></h3><p>By " + esc(product.seller) + "</p><div class=\"rating\"><span>" + (product.rating ? product.rating.toFixed(1) : "New") + " (" + Number(product.reviews || 0) + ")</span></div>" + priceMarkup(product) + "<button class=\"outline-button add-button\" type=\"button\" data-backend-action=\"cart-add\" data-product-slug=\"" + slug + "\" " + (product.available > 0 ? "" : "disabled") + "><span class=\"material-symbols-outlined\" aria-hidden=\"true\">shopping_bag</span> Add to Cart</button></div></article>";
  }

  function savedMarkup(product) {
    const slug = esc(product.slug);
    return "<article class=\"saved-card\" data-product=\"" + slug + "\"><div class=\"saved-media\"><img src=\"" + esc(product.image) + "\" alt=\"" + esc(product.name) + "\" width=\"320\" height=\"346\"/><button class=\"heart\" type=\"button\" data-backend-action=\"wishlist-remove\" data-product-slug=\"" + slug + "\" aria-label=\"Remove from wishlist\"><span class=\"material-symbols-outlined\" aria-hidden=\"true\">favorite</span></button></div><div class=\"saved-copy\"><span class=\"badge\">" + (product.available > 0 ? "In stock" : "Sold out") + "</span><h3><a href=\"/products/" + slug + "\">" + esc(product.name) + "</a></h3><p>By " + esc(product.seller) + "</p>" + priceMarkup(product) + "<small class=\"availability\">" + (product.available > 0 ? "In stock" : "Currently unavailable") + "</small><div><button class=\"primary-button\" type=\"button\" data-backend-action=\"wishlist-to-cart\" data-product-slug=\"" + slug + "\">Move to cart</button><button class=\"text-button\" type=\"button\" data-backend-action=\"wishlist-remove\" data-product-slug=\"" + slug + "\">Remove</button></div></div></article>";
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
      if (makerStory) makerStory.innerHTML = "<p>Maker profile stories are not connected to the live catalog yet.</p>";
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
      if (rating) rating.textContent = "Verified review data is not enabled in this MVP.";
      const reviewPanel = root.querySelector("#product-panel-reviews");
      if (reviewPanel) reviewPanel.innerHTML = "<p>Verified customer reviews are not connected in this MVP. No review count or rating is being shown.</p>";
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
      const benefitCopy = [["Delivery", "Shown before order placement"], ["Returns", "Return requests are not enabled yet"], ["Payment", "No payment is taken in this MVP"], ["Shipping", "Availability is confirmed later"]];
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
    const first = items[0];
    return "<article class=\"order-card compact-order\"><header><div><p class=\"eyebrow\">" + esc(status) + "</p><h2>" + esc(order.order_number || order.orderNumber || "Order") + "</h2><small>Placed on " + esc(orderDate(order.created_at || order.createdAt)) + " | Payment " + esc(order.payment_status || order.paymentStatus || "pending") + "</small></div><div><strong>" + money(order.total_cents || order.totalCents) + "</strong><span class=\"status\">" + esc(status) + "</span></div></header><div class=\"compact-order-item\">" + (first ? "<img src=\"" + esc(first.image || first.image_url || "/assets/images/customer/maker-mithila.webp") + "\" alt=\"" + esc(first.name || first.product_name || "Handmade product") + "\" width=\"160\" height=\"173\"/><span>" + esc(first.name || first.product_name || "Handmade product") + "<small>" + items.length + " item" + (items.length === 1 ? "" : "s") + "</small></span>" : "<span>Order items</span>") + "<span class=\"mvp-unavailable\">Order detail view is not connected yet</span></div></article>";
  }

  async function hydrateOrders() {
    const host = document.querySelector("[data-backend-orders]");
    if (!host) return;
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
      return global.WeeVCraftsBackend.normalizeCartItem(item);
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
    if (cartCount) cartCount.textContent = String(totals.count);
    if (wishlistCount) wishlistCount.textContent = String(wishlist.length);
    markReady(document.getElementById("cart-count"));
    markReady(document.getElementById("wishlist-count"));
  }

  function cartItemMarkup(item) {
    const slug = esc(item.slug);
      return "<article class=\"cart-seller\"><div class=\"cart-seller-head\"><div><span class=\"material-symbols-outlined\" aria-hidden=\"true\">storefront</span><b>" + esc(item.seller) + "</b></div></div><div class=\"cart-line\"><img src=\"" + esc(item.image) + "\" alt=\"" + esc(item.name) + "\" width=\"160\" height=\"173\"/><div class=\"cart-line-copy\"><span class=\"badge\">" + (item.available > 0 ? "In stock" : "Sold out") + "</span><h2>" + esc(item.name) + "</h2><p class=\"cart-variant\">Option: <b>" + esc(item.optionLabel || "Standard") + "</b></p><div class=\"price\"><strong>" + money(item.priceCents) + "</strong>" + (item.compareAtCents > item.priceCents ? "<del>" + money(item.compareAtCents) + "</del>" : "") + "</div><div class=\"cart-line-actions\"><span class=\"quantity-control\"><button type=\"button\" data-backend-action=\"cart-change\" data-product-slug=\"" + slug + "\" data-delta=\"-1\" aria-label=\"Decrease quantity\">-</button><output aria-live=\"polite\">" + Number(item.quantity || 0) + "</output><button type=\"button\" data-backend-action=\"cart-change\" data-product-slug=\"" + slug + "\" data-delta=\"1\" aria-label=\"Increase quantity\">+</button></span><button class=\"text-button\" type=\"button\" data-backend-action=\"cart-change\" data-product-slug=\"" + slug + "\" data-delta=\"-" + Number(item.quantity || 0) + "\">Remove</button><button class=\"text-button\" type=\"button\" data-backend-action=\"wishlist-move\" data-product-slug=\"" + slug + "\">Move to wishlist</button></div></div></div></article>";
  }

  async function hydrateCart() {
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
      return;
    }
    const cart = normalizeCart(await backendInstance.cart());
    const wishlist = await backendInstance.wishlist();
    updateBadges(cart, wishlist);
    const cartCountLabel = document.querySelector("[data-backend-cart-count]");
    if (cartCountLabel) { cartCountLabel.textContent = cartTotals(cart.items).count + " items from independent Indian makers"; markReady(cartCountLabel); }
    const wishlistCountLabel = document.querySelector("[data-backend-wishlist-count]");
    if (wishlistCountLabel) { wishlistCountLabel.textContent = wishlist.length + " items in your wishlist"; markReady(wishlistCountLabel); }
    if (cartRoot) {
      const host = cartRoot.querySelector(".cart-items");
      host.innerHTML = cart.items.length ? cart.items.map(cartItemMarkup).join("") : "<div class=\"empty-state\"><h2>Your cart is empty</h2><p>Browse handmade treasures and bring a little India home.</p><a class=\"primary-button\" href=\"/products\">Continue shopping</a></div>";
      const totals = cartTotals(cart.items);
      const summary = cartRoot.querySelector(".cart-summary");
      if (summary) {
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
    if (!user) { updateBadges({ items: [] }, []); return; }
    const cart = normalizeCart(await backend.cart());
    const wishlist = await backend.wishlist();
    updateBadges(cart, wishlist);
  }

  async function requireUser() {
    const user = await ensureBackend().currentUser();
    if (!user) throw new Error("Sign in to save a cart, wishlist, or order.");
    return user;
  }

  async function runAction(action, slug, delta, selectedVariantId) {
    const api = ensureBackend();
    if (action === "cart-add") {
      await requireUser();
      const quantity = Number(delta || document.getElementById("detail-quantity")?.value || 1);
      await api.addCart(slug, quantity, selectedVariantId);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await refreshCounts();
      if (document.querySelector("[data-backend-cart]")) await hydrateCart();
      else notice("Added to your cart.", false);
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
      await api.changeCart(slug, Number(delta));
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await hydrateCart();
      return;
    }
    if (action === "wishlist-add") {
      await requireUser();
      await api.addWishlist(slug);
      await refreshCounts();
      notice("Saved to your wishlist.", false);
      return;
    }
    if (action === "wishlist-remove") {
      await requireUser();
      await api.removeWishlist(slug);
      await hydrateCart();
      return;
    }
    if (action === "wishlist-move") {
      await requireUser();
      await api.moveCartToWishlist(slug);
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      await hydrateCart();
      notice("Moved to your wishlist.", false);
      return;
    }
    if (action === "wishlist-to-cart") {
      await requireUser();
      await api.changeCart(slug, 1);
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

  function authDestination(candidate, user) {
    const roleDestinations = {
      customer: "/account",
      seller_owner: "/seller-admin",
      seller_staff: "/seller-admin",
      marketplace_admin: "/marketplace-admin",
      super_admin: "/super-admin",
      finance_operator: "/finance-portal",
      operations_returns: "/operations-portal",
      support_agent: "/support-portal"
    };
    const roles = new Set(Array.isArray(user && user.roles) ? user.roles : []);
    if (roles.has("seller_owner")) roles.delete("seller_staff");
    const destinations = [...new Set([...roles].map(role => roleDestinations[role]).filter(Boolean))];
    if (destinations.length !== 1) return "/workspaces";

    const destination = destinations[0];
    const safeCandidate = typeof candidate === "string" && candidate.startsWith("/") && !candidate.startsWith("//") && !candidate.includes("\\");
    if (destination === "/account" && safeCandidate) return candidate;
    return destination;
  }

  async function submitAuth(form) {
    const api = ensureBackend();
    const action = form.dataset.backendAuth;
    const values = formData(form);
    try {
      if (action === "login") {
        const result = await api.signIn({ email: values.email, password: values.password });
        global.location.assign(authDestination(values.next, unwrap(result) || result));
      } else if (action === "register") {
        const result = await api.signUp({ email: values.email, password: values.password, displayName: values.name });
        const data = unwrap(result) || result;
        if (data && (data.access_token || result.csrf_token)) global.location.assign(authDestination(values.next));
        else showFormMessage(form, "Check your email to verify your new account.", false);
      } else if (action === "recover") {
        await api.recover(values.email);
        showFormMessage(form, "If the address is registered, recovery instructions have been sent.", false);
      } else if (action === "reset") {
        if (values.password !== values.confirm_password) throw new Error("The passwords do not match.");
        const hash = new URLSearchParams(global.location.hash.replace(/^#/, ""));
        await api.updatePassword(values.password, hash.get("access_token") || hash.get("token") || "");
        global.history.replaceState(null, "", global.location.pathname);
        showFormMessage(form, "Password updated. You can now sign in.", false);
      }
    } catch (error) {
      showFormMessage(form, error.message, true);
    }
  }

  function paymentReceipt(orderNumber, message, localSimulator) {
    const main = document.getElementById("page");
    if (!main) return;
    const encoded = encodeURIComponent(orderNumber);
    const simulator = localSimulator
      ? "<div class=\"payment-simulator\" aria-label=\"Local payment test actions\"><button class=\"primary-button\" type=\"button\" data-local-payment=\"capture\" data-order-number=\"" + esc(orderNumber) + "\">Simulate successful payment</button><button class=\"outline-button\" type=\"button\" data-local-payment=\"fail\" data-order-number=\"" + esc(orderNumber) + "\">Simulate failed payment</button><button class=\"text-button\" type=\"button\" data-local-payment=\"pending\" data-order-number=\"" + esc(orderNumber) + "\">Leave payment pending</button></div>"
      : "";
    main.innerHTML = "<section class=\"container checkout-complete\" role=\"status\"><span class=\"material-symbols-outlined\" aria-hidden=\"true\">receipt_long</span><h1>Order " + esc(orderNumber) + "</h1><p>" + esc(message) + "</p>" + simulator + "<a class=\"primary-button\" href=\"/orders/" + encoded + "\">View order details</a></section>";
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
      paymentReceipt(orderNumber, "Your order is saved. Choose a local test result to exercise payment confirmation.", true);
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
      name: "WeeVCrafts",
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

  async function checkout() {
    const button = document.querySelector('[data-backend-action="checkout"]');
    const form = document.getElementById("checkout-address-form");
    if (!button || !form || !form.reportValidity()) return;
    await requireUser();
    const values = formData(form);
    const address = {
      recipient_name: values.recipient_name, phone: values.phone,
      line1: values.line1, line2: values.line2 || "", city: values.city,
      region: values.region, postal_code: values.postal_code,
      country_code: (values.country_code || "IN").toUpperCase()
    };
    const summary = document.querySelector("[data-checkout-address-summary]");
    if (summary) summary.textContent = [address.recipient_name, address.line1, address.city, address.region, address.postal_code].join(" · ");
    button.disabled = true;
    let orderNumber = "";
    try {
      let key = global.sessionStorage.getItem(CHECKOUT_KEY);
      if (!key) {
        key = global.crypto && global.crypto.randomUUID ? global.crypto.randomUUID() : String(Date.now()) + "-" + Math.random().toString(16).slice(2);
        global.sessionStorage.setItem(CHECKOUT_KEY, key);
      }
      const result = unwrap(await ensureBackend().checkout(address, key)) || {};
      orderNumber = result.order_number || result.orderNumber || "";
      if (!orderNumber) throw new Error("The order was created, but the server did not return its number. Open your orders to check its status.");
      global.sessionStorage.removeItem(CHECKOUT_KEY);
      const cartBadge = document.querySelector("#cart-count em");
      if (cartBadge) { cartBadge.textContent = "0"; markReady(document.getElementById("cart-count")); }
      try {
        await beginPayment(orderNumber);
      } catch (paymentError) {
        paymentReceipt(orderNumber, paymentError.message + " Your order is saved; check its current status before trying again.", false);
      }
    } catch (error) {
      if (orderNumber) {
        paymentReceipt(orderNumber, error.message + " Your order is saved; check its current status before trying again.", false);
      } else {
        notice(error.message, true);
        button.disabled = false;
      }
    }
  }

  function bindActions() {
    document.addEventListener("click", async function (event) {
      const localPaymentButton = event.target.closest("[data-local-payment]");
      if (localPaymentButton) {
        event.preventDefault();
        localPaymentButton.disabled = true;
        try {
          await ensureBackend().localPayment(localPaymentButton.dataset.orderNumber, localPaymentButton.dataset.localPayment);
          global.location.assign("/orders/" + encodeURIComponent(localPaymentButton.dataset.orderNumber));
        } catch (error) {
          localPaymentButton.disabled = false;
          notice(error.message, true);
        }
        return;
      }
      const checkoutButton = event.target.closest('[data-backend-action="checkout"]');
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
      if (!action) {
        const path = element.getAttribute("hx-post") || "";
        const match = path.match(/^\/ui\/(cart|wishlist)\/([^/?#]+)/);
        if (!match) return;
        slug = decodeURIComponent(match[2]);
        const values = element.getAttribute("hx-vals");
        let extras = {};
        try { extras = values ? JSON.parse(values) : {}; } catch (_) { extras = {}; }
        if (match[1] === "cart") {
          action = extras.delta ? "cart-change" : element.closest(".saved-card") ? "wishlist-to-cart" : extras.intent === "buy" ? "cart-buy" : "cart-add";
          delta = extras.delta || document.getElementById("detail-quantity")?.value || "1";
        } else {
          action = extras.source === "cart" ? "wishlist-move" : extras.source === "wishlist" ? "wishlist-remove" : "wishlist-add";
        }
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
      const selectedVariantId = detailActions ? document.querySelector("#detail-variant")?.value : undefined;
      try { await runAction(action, slug, delta, selectedVariantId); }
      catch (error) { notice(error.message, true); }
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
      if (!backend) return;
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
    bindActions();
    try {
      backend = await create();
      global.weevcraftsBackend = backend;
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
      if (["/cart", "/wishlist"].includes(global.location.pathname)) await hydrateCart();
      if (global.location.pathname === "/orders") await hydrateOrders();
      await refreshCounts();
    } catch (error) {
      backend = null;
      backendError = error;
      global.weevcraftsBackendError = error;
      showUnknownCounts();
      notice("Some live account counts could not be refreshed. Please try again.", true);
    }
  }

  document.addEventListener("DOMContentLoaded", start);
})(window);
