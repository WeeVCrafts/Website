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
    var previewURLs = [];
    var savedDraftURL = '';
    function validateFiles(files) {
      if (files.some(function (file) { return !['image/jpeg', 'image/png', 'image/webp', 'video/mp4', 'video/webm'].includes(file.type); })) {
        status.textContent = 'Choose a JPEG, PNG, WebP, MP4 or WebM file.'; input.focus(); return false;
      }
      if (files.some(function (file) { return file.size <= 0 || file.size > 25 * 1024 * 1024; })) {
        status.textContent = 'Product media must be 25 MB or smaller.'; input.focus(); return false;
      }
      return true;
    }
    function previewFiles() {
      if (!selection) return;
      previewURLs.forEach(function (url) { URL.revokeObjectURL(url); }); previewURLs = [];
      selection.replaceChildren();
      Array.from(input.files || []).forEach(function (file,index) {
        var figure = document.createElement('figure');
        var caption = document.createElement('figcaption');
        var supported=['image/jpeg','image/png','image/webp','video/mp4','video/webm'].includes(file.type);
        caption.textContent = file.name + ' · ' + (file.size / 1024 / 1024).toFixed(2) + ' MiB · ' + (supported?'Selected':'Unsupported type');
        if(!supported || index>=6 || file.size>10*1024*1024){figure.append(caption);selection.append(figure);return;}
        var preview = document.createElement(file.type.startsWith("video/") ? "video" : "img");
        preview.className = "product-media-square";
        if (preview.tagName === "VIDEO") {preview.controls = true;preview.preload='metadata';}
        else preview.alt = "Selected product image preview";
        preview.src = URL.createObjectURL(file);
        previewURLs.push(preview.src);
        figure.append(preview, caption); selection.appendChild(figure);
      });
    }
    input.addEventListener("change", previewFiles);
    var dropzone = panel.querySelector("[data-media-drop]");
    if (dropzone) {
      ["dragenter", "dragover"].forEach(function (name) { dropzone.addEventListener(name, function (event) { event.preventDefault(); dropzone.classList.add("is-dragging"); }); });
      ["dragleave", "drop"].forEach(function (name) { dropzone.addEventListener(name, function (event) { event.preventDefault(); dropzone.classList.remove("is-dragging"); }); });
      dropzone.addEventListener("drop", function (event) { if (!input.disabled && event.dataTransfer?.files) { input.files = event.dataTransfer.files; input.dispatchEvent(new Event("change", { bubbles: true })); } });
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
        if (savedDraftURL) { window.location.assign(savedDraftURL); return; }
        updateCategoryName();
        var files = Array.from(input.files);
        if (!validateFiles(files)) return;
        var submit = builder.querySelector('[type="submit"]');
        submit.disabled = true;
        builder.dataset.saving = 'true'; window.WVUI.busy(submit, true); panel.setAttribute('aria-busy', 'true');
        try {
          status.textContent = "Saving product draft...";
          var response = await fetch(builder.action, { method: "POST", body: new FormData(builder), credentials: "same-origin" });
          var match = response.url.match(/\/seller-admin\/products\/(\d+)\/edit/);
          if (!response.ok || !match) throw new Error("Product draft could not be saved. Review the listing details and try again.");
          savedDraftURL = response.url;
          for (var index = 0; index < files.length; index++) await uploadFile(files[index], match[1]);
          window.location.assign(response.url);
        } catch (error) {
          status.textContent = error.message || "Media upload failed. Please try again.";
          if (savedDraftURL) {
            status.append(' Your product draft is saved. Open it to review uploaded files and add the remaining media. ');
            var link = document.createElement('a'); link.href = savedDraftURL; link.textContent = 'Open saved draft'; status.append(link);
            // The product already exists. Never offer a second create submission.
            submit.textContent = 'Open saved draft'; builder.dataset.discarded = 'true';
          }
          submit.disabled = false;
          delete builder.dataset.saving;
        } finally {
          window.WVUI.busy(submit, false); panel.removeAttribute('aria-busy');
        }
      });
    }
    if (uploadButton) uploadButton.addEventListener("click", async function () {
      var files = Array.from(input.files || []);
      var productID = panel.dataset.productId;
      if (!files.length || !productID) { status.textContent = "Save this product draft before uploading images."; input.focus(); return; }
      if (!validateFiles(files)) return;
      uploadButton.disabled = true;
      input.disabled=true;
      window.WVUI.busy(uploadButton, true); panel.setAttribute('aria-busy', 'true');
      try { for (var index = 0; index < files.length; index++) {
        await uploadFile(files[index], productID);
        // Only retry files that have not already been finalized.
        var remaining = new DataTransfer();
        files.slice(index + 1).forEach(function(file) { remaining.items.add(file); });
        input.files=remaining.files; previewFiles();
      } status.textContent = files.length === 1 && files[0].type.startsWith("video/") ? "Product video saved." : files.length === 1 ? "Product image saved." : "Product media saved."; window.setTimeout(function () { window.location.reload(); }, 1000); }
      catch (error) { status.textContent = error.message || "Media upload failed. Please try again."; }
      finally { uploadButton.disabled = false; input.disabled=false; window.WVUI.busy(uploadButton, false); panel.removeAttribute('aria-busy'); }
    });
    panel.querySelectorAll("[data-media-delete]").forEach(function (button) {
      button.addEventListener("click", async function () {
        if (!await window.WVUI.confirmAction("Delete this uploaded product media? It will be removed from the product gallery.", button, "Delete media", {intent:'danger'})) return;
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
