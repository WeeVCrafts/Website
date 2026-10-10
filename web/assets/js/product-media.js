document.addEventListener("DOMContentLoaded", function () {
  document.querySelectorAll("[data-print-kind]").forEach(function (button) {
    button.addEventListener("click", function () {
      document.body.dataset.printKind = button.dataset.printKind;
      window.print();
      window.setTimeout(function () { delete document.body.dataset.printKind; }, 500);
    });
  });
  var builder = document.querySelector("[data-product-builder]");
  var selector = builder && builder.querySelector('[name="category_slug"]');
  var categoryName = builder && builder.querySelector("[data-category-name]");
  function updateCategoryName() {
    if (!selector || !categoryName) return;
    categoryName.value = selector.options[selector.selectedIndex]?.text || "";
  }
  if (selector) { selector.addEventListener("change", updateCategoryName); updateCategoryName(); }

  document.querySelectorAll("[data-product-media]").forEach(function (panel) {
    var input = panel.querySelector("[data-media-file]");
    var uploadButton = panel.querySelector("[data-media-upload]");
    var status = panel.querySelector("[data-media-status]");
    var selection = panel.querySelector("[data-media-selection]");
    if (!input || !status) return;
    function previewFiles() {
      if (!selection) return;
      selection.replaceChildren();
      Array.from(input.files || []).forEach(function (file) {
        if (!file.type.startsWith("image/") && !file.type.startsWith("video/")) return;
        var preview = document.createElement(file.type.startsWith("video/") ? "video" : "img");
        preview.className = "product-media-square";
        if (preview.tagName === "VIDEO") preview.controls = true;
        else preview.alt = "Selected product image preview";
        preview.src = URL.createObjectURL(file);
        preview.onload = preview.onloadeddata = function () { URL.revokeObjectURL(preview.src); };
        selection.appendChild(preview);
      });
    }
    input.addEventListener("change", previewFiles);
    var dropzone = panel.querySelector("[data-media-drop]");
    if (dropzone) {
      ["dragenter", "dragover"].forEach(function (name) { dropzone.addEventListener(name, function (event) { event.preventDefault(); dropzone.classList.add("is-dragging"); }); });
      ["dragleave", "drop"].forEach(function (name) { dropzone.addEventListener(name, function (event) { event.preventDefault(); dropzone.classList.remove("is-dragging"); }); });
      dropzone.addEventListener("drop", function (event) { if (event.dataTransfer?.files) { input.files = event.dataTransfer.files; input.dispatchEvent(new Event("change", { bubbles: true })); } });
    }

    async function uploadFile(file, productID) {
      async function responseError(response, fallback) {
        try {
          var payload = await response.json();
          return payload.error?.message || payload.message || fallback;
        } catch (error) {
          return fallback;
        }
      }
      var headers = { "Content-Type": "application/json", "X-CSRF-Token": panel.dataset.csrfToken };
      status.textContent = "Preparing secure media upload...";
      var prepare = await fetch("/api/v1/seller/products/media-url", { method: "POST", credentials: "same-origin", headers: headers, body: JSON.stringify({ product_id: Number(productID), content_type: file.type, byte_size: file.size }) });
      var prepared = await prepare.json();
      if (!prepare.ok || !prepared.data) throw new Error("Media upload could not be prepared.");
      var mediaID = Number(prepared.data.id);
      try {
        status.textContent = "Uploading " + file.name + "...";
        var stored = await fetch(prepared.data.upload_url, { method: "PUT", credentials: "same-origin", headers: { "Content-Type": file.type, "X-CSRF-Token": panel.dataset.csrfToken }, body: file });
        if (!stored.ok) throw new Error(await responseError(stored, "Media could not be stored. Check the storage connection and try again."));
        status.textContent = "Verifying " + file.name + "...";
        var done = await fetch("/api/v1/seller/products/media-finalize", { method: "POST", credentials: "same-origin", headers: headers, body: JSON.stringify({ media_id: mediaID }) });
        var finalized = await done.json();
        if (!done.ok || !finalized.data) throw new Error(await responseError(done, "Uploaded media did not pass verification."));
      } catch (error) {
        await fetch("/api/v1/seller/products/media-delete", { method: "POST", credentials: "same-origin", headers: headers, body: JSON.stringify({ media_id: mediaID }) }).catch(function () {});
        throw error;
      }
    }

    if (builder && panel.dataset.productId === "") {
      builder.addEventListener("submit", async function (event) {
        if (!input.files?.length) return;
        event.preventDefault();
        updateCategoryName();
        var files = Array.from(input.files);
        var submit = builder.querySelector('[type="submit"]');
        submit.disabled = true;
        try {
          status.textContent = "Saving product draft...";
          var response = await fetch(builder.action, { method: "POST", body: new FormData(builder), credentials: "same-origin" });
          var match = response.url.match(/\/seller-admin\/products\/(\d+)\/edit/);
          if (!response.ok || !match) throw new Error("Product draft could not be saved. Review the listing details and try again.");
          for (var index = 0; index < files.length; index++) await uploadFile(files[index], match[1]);
          window.location.assign(response.url);
        } catch (error) {
        status.textContent = error.message || "Media upload failed. Please try again.";
          submit.disabled = false;
        }
      });
    }
    if (uploadButton) uploadButton.addEventListener("click", async function () {
      var files = Array.from(input.files || []);
      var productID = panel.dataset.productId;
      if (!files.length || !productID) { status.textContent = "Save this product draft before uploading images."; input.focus(); return; }
      if (files.some(function (file) { return !["image/jpeg", "image/png", "image/webp", "video/mp4", "video/webm"].includes(file.type); })) { status.textContent = "Choose a JPEG, PNG, WebP, MP4 or WebM file."; return; }
      if (files.some(function (file) { return file.size <= 0 || file.size > 25 * 1024 * 1024; })) { status.textContent = "Product media must be 25 MB or smaller."; return; }
      uploadButton.disabled = true;
      try { for (var index = 0; index < files.length; index++) {
        await uploadFile(files[index], productID);
        // Only retry files that have not already been finalized.
        var remaining = new DataTransfer();
        files.slice(index + 1).forEach(function(file) { remaining.items.add(file); });
        input.files=remaining.files; previewFiles();
      } status.textContent = files.length === 1 && files[0].type.startsWith("video/") ? "Product video saved." : files.length === 1 ? "Product image saved." : "Product media saved."; window.setTimeout(function () { window.location.reload(); }, 1000); }
      catch (error) { status.textContent = error.message || "Media upload failed. Please try again."; }
      finally { uploadButton.disabled = false; }
    });
    panel.querySelectorAll("[data-media-delete]").forEach(function (button) {
      button.addEventListener("click", async function () {
        if (!await window.WVUI.confirmAction("Delete this uploaded product media? It will be removed from the product gallery.", button, "Delete media")) return;
        button.disabled = true; status.textContent = "Removing product image...";
        try {
          var result = await fetch("/api/v1/seller/products/media-delete", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": panel.dataset.csrfToken }, body: JSON.stringify({ media_id: Number(button.dataset.mediaId) }) }).then(function (response) { return response.json(); });
          if (!result.data) throw new Error("Image could not be removed.");
          window.location.reload();
        } catch (error) { status.textContent = error.message || "Media removal failed."; button.disabled = false; }
      });
    });
  });
});
