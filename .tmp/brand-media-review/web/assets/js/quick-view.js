(function () {
  "use strict";
  let origin = null;
  let request = null;
  let boundDialog = null;

  function dialog() { return document.getElementById("product-quick-view"); }
  function close() {
    const trigger = origin;
    request?.abort();
    request = null;
    if (dialog()?.open) dialog().close();
    return trigger;
  }
  window.WVQuickView = { close };

  function bind() {
    const modal = dialog();
    if (!modal || boundDialog === modal) return;
    boundDialog = modal;
    modal.querySelector("[data-quick-close]").addEventListener("click", close);
    modal.addEventListener("click", event => {
      if (event.target !== modal) return;
      const rect = modal.getBoundingClientRect();
      if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) close();
    });
    modal.addEventListener("close", () => {
      request?.abort(); request = null;
      document.body.classList.remove("quick-view-open");
      modal.querySelector("#product-quick-view-content").replaceChildren();
      const trigger = origin;
      origin = null;
      if (trigger?.isConnected) trigger.focus({ preventScroll: true });
    });
    modal.addEventListener("change", event => {
      if (!event.target.matches("[data-quick-variant]")) return;
      const option = event.target.selectedOptions?.[0];
      const add = modal.querySelector("[data-quick-add]");
      if (!option || !add) return;
      add.dataset.variantId = option.value;
      add.disabled = !option.value || option.dataset.inStock !== "true";
      modal.querySelector("[data-quick-price]").textContent = option.dataset.price || "";
      modal.querySelector("[data-quick-stock]").textContent = option.dataset.stockMessage || "Currently unavailable";
      modal.querySelector("[data-quick-feedback]").textContent = "";
    });
  }

  async function open(link) {
    bind();
    const modal = dialog();
    if (!modal) { location.assign(link.href); return; }
    request?.abort();
    const controller = new AbortController();
    request = controller;
    origin = link;
    const content = modal.querySelector("#product-quick-view-content");
    content.replaceChildren();
    const loading = document.createElement("p");
    loading.className = "quick-view-loading";
    loading.textContent = "Loading product…";
    content.append(loading);
    content.setAttribute("aria-busy", "true");
    if (!modal.open) modal.showModal();
    document.body.classList.add("quick-view-open");
    modal.querySelector("[data-quick-close]").focus();
    try {
      const response = await fetch(link.href, {
        credentials: "same-origin", signal: controller.signal,
        headers: { "HX-Request": "true", "HX-Target": "product-quick-view-content", "Accept": "text/html" }
      });
      if (!response.ok) throw new Error("Product could not be loaded.");
      const html = await response.text();
      if (request !== controller || !modal.open) return;
      const documentFragment = new DOMParser().parseFromString(html, "text/html");
      const product = documentFragment.querySelector("[data-quick-product]");
      if (!product) throw new Error("Product could not be loaded.");
      content.replaceChildren(product);
      window.htmx?.process(content);
    } catch (error) {
      if (error.name === "AbortError" || request !== controller || !modal.open) return;
      content.replaceChildren();
      const message = document.createElement("p");
      message.textContent = "We couldn’t load this piece. Try again or open the full product page.";
      const retry = document.createElement("button");
      retry.type = "button"; retry.className = "outline-button"; retry.textContent = "Try again";
      retry.addEventListener("click", () => open(link));
      const detail = document.createElement("a");
      detail.href = link.href; detail.className = "text-link quick-view-detail"; detail.textContent = "View full product details";
      content.append(message, retry, detail);
    } finally {
      if (request === controller) content.removeAttribute("aria-busy");
    }
  }

  document.addEventListener("click", event => {
    const link = event.target.closest("a[data-quick-view]");
    if (!link || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    open(link);
  });
  document.addEventListener("DOMContentLoaded", bind);
  document.addEventListener("htmx:afterSettle", bind);
  window.addEventListener("pagehide", close);
})();
