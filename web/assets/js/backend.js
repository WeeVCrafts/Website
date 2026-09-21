/* First-party frontend adapter for the WeeVCrafts Go API. */
(function (global) {
  "use strict";
  const ROOT = "/api/v1";

  function messageFrom(response, payload) {
    if (typeof payload === "string" && payload) return payload;
    const detail = payload && payload.error;
    return (payload && (payload.message || payload.msg || payload.error_description || (typeof detail === "string" ? detail : detail && detail.message))) ||
      (response && response.statusText) || "The request could not be completed.";
  }

  async function readJSON(response) {
    const text = await response.text();
    let body = {};
    if (text) {
      try { body = JSON.parse(text); } catch (_) { body = { message: text }; }
    }
    if (!response.ok) throw new Error(messageFrom(response, body.error || body));
    return body;
  }

  function unwrap(payload) {
    return payload && Object.prototype.hasOwnProperty.call(payload, "data") ? payload.data : payload;
  }

  function normalizeProduct(row) {
    if (!row) return null;
    const seller = Array.isArray(row.seller) ? row.seller[0] : row.seller;
    const variants = row.variants || row.product_variants || [];
    const variant = Array.isArray(variants) ? variants[0] : variants;
    const price = Number(variant && variant.price_cents != null ? variant.price_cents : row.price_cents || 0);
    return {
      id: row.id,
      variantId: (variant && (variant.id || variant.variant_id)) || row.variant_id,
      optionLabel: (variant && variant.option_label) || row.option_label || "Standard",
      slug: row.slug || row.product_slug || "",
      name: row.name || row.product_name || "Handmade product",
      description: row.description || "",
      seller: (seller && (seller.display_name || seller.name)) || row.brand || "Independent maker",
      sellerSlug: (seller && seller.slug) || row.seller_slug || "",
      location: (seller && seller.location) || "",
      image: row.image_url || row.image || "/assets/images/customer/maker-mithila.webp",
      priceCents: price,
      compareAtCents: Number(row.compare_at_cents || 0),
      available: Number(variant && variant.inventory_quantity != null ? variant.inventory_quantity : row.available || 0),
      category: (row.category && row.category.name) || row.category_name || "",
      categorySlug: (row.category && row.category.slug) || row.category_slug || "",
      brand: (row.brand && row.brand.name) || row.brand_name || "",
      brandSlug: (row.brand && row.brand.slug) || row.brand_slug || "",
      rating: Number(row.rating || 0),
      reviews: Number(row.review_count || 0)
    };
  }

  function normalizeCartItem(row) {
    if (row.product_slug || row.product_name) {
      return {
        slug: row.product_slug,
        name: row.product_name,
        seller: row.brand || "Independent maker",
        image: row.image_url || "/assets/images/customer/maker-mithila.webp",
        quantity: Number(row.quantity || 0),
        priceCents: Number(row.unit_price_cents || 0),
        variantId: row.variant_id,
        optionLabel: row.variant_name || row.option_label || "Standard",
        available: Number(row.available || 0)
      };
    }
    const variant = Array.isArray(row.variant) ? row.variant[0] : row.variant;
    const product = variant && (Array.isArray(variant.product) ? variant.product[0] : variant.product);
    const normalized = normalizeProduct(Object.assign({}, product || {}, {
      variants: variant ? [variant] : []
    }));
    return Object.assign(normalized || {}, {
      quantity: Number(row.quantity || 0),
      variantId: variant && variant.id
    });
  }

  class CustomBackend {
    constructor() {
      let pageCSRF = "";
      try { pageCSRF = JSON.parse(document.body.getAttribute("hx-headers") || "{}")["X-CSRF-Token"] || ""; }
      catch (_) { pageCSRF = ""; }
      this.base = "";
      this.csrf = pageCSRF;
    }
    async call(path, options) {
      const opts = options || {};
      const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
      if (opts.authenticated) headers["X-CSRF-Token"] = this.csrf;
      const response = await fetch(this.base + path, {
        method: opts.method || "GET", headers, credentials: "include",
        body: opts.body === undefined ? undefined : JSON.stringify(opts.body)
      });
      return readJSON(response);
    }

    async signIn(input) {
      const result = await this.call(ROOT + "/auth/login", { method: "POST", body: input });
      this.csrf = result.csrf_token || "";
      return result;
    }

    async signUp(input) {
      const result = await this.call(ROOT + "/auth/register", {
        method: "POST", body: { email: input.email, password: input.password, display_name: input.displayName || "" }
      });
      this.csrf = result.csrf_token || "";
      return result;
    }

    async recover(email) { return this.call(ROOT + "/auth/password-reset/request", { method: "POST", body: { email: email } }); }
    async updatePassword(password, token) { return this.call(ROOT + "/auth/password-reset/confirm", { method: "POST", body: { token: token, password: password } }); }
    async verifyEmail(token) { return this.call(ROOT + "/auth/email-verification/confirm", { method: "POST", body: { token: token } }); }

    async signOut() {
      await this.call(ROOT + "/auth/logout", { method: "POST", authenticated: true, body: {} });
      this.csrf = "";
    }

    async currentUser() {
      try { return Boolean(unwrap(await this.call(ROOT + "/auth/session"))); }
      catch (_) { return null; }
    }

    async products(filters) {
      const input = filters || {};
      const endpoint = input.query ? "/search" : "/products";
      const query = new URLSearchParams();
      if (input.query) query.set("q", input.query);
      const categories = input.categories || [];
      if (input.category && input.category !== "offers" && categories.length <= 1) query.set("category", input.category);
      if (input.sort) query.set("sort", input.sort);
      if (input.limit) query.set("limit", String(input.limit));
      if (input.offset) query.set("offset", String(input.offset));
      const result = await this.call(ROOT + endpoint + (query.size ? "?" + query.toString() : ""));
      let products = (unwrap(result) || []).map(normalizeProduct);
      if (input.category === "offers") products = products.filter((product) => product.compareAtCents > product.priceCents);
      return products;
    }

    async product(slug) { return normalizeProduct(unwrap(await this.call(ROOT + "/products/" + encodeURIComponent(slug)))); }

    async cart() {
      const payload = unwrap(await this.call(ROOT + "/cart"));
      const items = [];
      for (const item of (payload && payload.items) || []) {
        const product = await this.product(item.product_slug);
        items.push(Object.assign(product, {
          quantity: Number(item.quantity),
          variantId: item.variant_id,
          optionLabel: item.variant_name || product.optionLabel,
          priceCents: Number(item.unit_price_cents),
          available: Number(item.available || 0)
        }));
      }
      return { id: payload && payload.id, items: items };
    }

    async changeCart(slug, delta) {
      await fetch(this.base + "/ui/cart/" + encodeURIComponent(slug), {
        method: "POST", credentials: "include",
        headers: { "Content-Type": "application/x-www-form-urlencoded", "X-CSRF-Token": this.csrf, "HX-Request": "true" },
        body: new URLSearchParams({ delta: String(delta) }).toString()
      }).then(async function (response) { if (!response.ok) throw new Error(messageFrom(response, await response.json().catch(function () { return {}; }))); });
      return this.cart();
    }

    async addCart(slug, quantity, selectedVariantId) {
      const product = await this.product(slug);
      if (!product || !product.variantId) throw new Error("This product is unavailable for your cart.");
      const variantId = Number(selectedVariantId) || Number(product.variantId);
      await this.call(ROOT + "/cart/add", {
        method: "POST", authenticated: true,
        body: { variant_id: variantId, quantity: Number(quantity || 1) }
      });
      return this.cart();
    }
    async moveCartToWishlist(slug) {
      const response = await fetch(this.base + "/ui/wishlist/" + encodeURIComponent(slug), {
        method: "POST", credentials: "include",
        headers: { "Content-Type": "application/x-www-form-urlencoded", "X-CSRF-Token": this.csrf, "HX-Request": "true" },
        body: "source=cart"
      });
      if (!response.ok) throw new Error("The item could not be moved to the wishlist.");
      return this.cart();
    }

    async wishlist() {
      const payload = unwrap(await this.call(ROOT + "/wishlist"));
      const result = [];
      for (const item of payload || []) result.push(await this.product(item.slug));
      return result;
    }

    async addWishlist(slug) {
      const product = await this.product(slug);
      await this.call(ROOT + "/wishlist/add", { method: "POST", authenticated: true, body: { product_id: product.id } });
      return this.wishlist();
    }

    async removeWishlist(slug) {
      const product = await this.product(slug);
      await this.call(ROOT + "/wishlist/" + product.id, { method: "DELETE", authenticated: true });
      return this.wishlist();
    }

    async checkout(address, idempotencyKey, couponCode) {
      const cart = await this.call(ROOT + "/cart");
      return this.call(ROOT + "/checkout", {
        method: "POST", authenticated: true,
        headers: { "Idempotency-Key": idempotencyKey },
        body: { cart_id: unwrap(cart).id, idempotency_key: idempotencyKey, address: {
          recipient_name: address.recipient_name, phone: address.phone, line1: address.line1,
          line2: address.line2 || "", city: address.city, state: address.region,
          postal_code: address.postal_code, country_code: address.country_code || "IN"
        }, coupon_code: couponCode || "" }
      });
    }

    async createPayment(orderNumber) {
      return this.call(ROOT + "/orders/" + encodeURIComponent(orderNumber) + "/payment", {
        method: "POST", authenticated: true, body: {}
      });
    }

    async verifyPayment(orderNumber, payment) {
      return this.call(ROOT + "/orders/" + encodeURIComponent(orderNumber) + "/payment/verify", {
        method: "POST", authenticated: true, body: {
          provider_order_id: payment.razorpay_order_id,
          provider_payment_id: payment.razorpay_payment_id,
          signature: payment.razorpay_signature
        }
      });
    }

    async localPayment(orderNumber, action) {
      if (!["capture", "fail", "pending"].includes(action)) throw new Error("Unknown local payment action.");
      const response = await fetch("/dev/payments/" + encodeURIComponent(orderNumber) + "/" + action, {
        method: "POST", credentials: "include", headers: { "X-CSRF-Token": this.csrf }
      });
      return readJSON(response);
    }

    async orders() { return unwrap(await this.call(ROOT + "/orders")) || []; }
  }

  async function create() {
    return new CustomBackend();
  }

  global.WeeVCraftsBackend = { create: create, normalizeProduct: normalizeProduct, normalizeCartItem: normalizeCartItem };
})(window);
