document.addEventListener("DOMContentLoaded", function () {
  document.querySelectorAll("[data-product-media]").forEach(function (panel) {
    var input = panel.querySelector("[data-media-file]");
    var uploadButton = panel.querySelector("[data-media-upload]");
    var status = panel.querySelector("[data-media-status]");
    if (!input || !uploadButton || !status) return;

    uploadButton.addEventListener("click", async function () {
      var file = input.files && input.files[0];
      if (!file) {
        status.textContent = "Choose an image or video first.";
        input.focus();
        return;
      }
      if (file.size <= 0 || file.size > 25 * 1024 * 1024) {
        status.textContent = "Image or video must be 25 MB or smaller.";
        return;
      }
      var allowedTypes = ["image/jpeg", "image/png", "image/webp", "video/mp4", "video/webm"];
      if (!allowedTypes.includes(file.type)) {
        status.textContent = "Choose a JPEG, PNG, WebP, MP4 or WebM file.";
        return;
      }

      uploadButton.disabled = true;
      var mediaKind = file.type.startsWith("video/") ? "video" : "image";
      var preparedMediaID = 0;
      try {
        var headers = { "Content-Type": "application/json", "X-CSRF-Token": panel.dataset.csrfToken };
        status.textContent = "Preparing secure upload...";
        var prepare = await fetch("/api/v1/seller/products/media-url", {
          method: "POST", credentials: "same-origin", headers: headers,
          body: JSON.stringify({ product_id: Number(panel.dataset.productId), content_type: file.type, byte_size: file.size })
        });
        var prepareBody = await prepare.json();
        if (!prepare.ok || !prepareBody.data) throw new Error("Media upload could not be prepared.");
        preparedMediaID = Number(prepareBody.data.id) || 0;

        status.textContent = "Uploading " + mediaKind + "...";
        var uploaded = await fetch(prepareBody.data.upload_url, {
          method: "PUT", credentials: "same-origin",
          headers: { "Content-Type": file.type, "X-CSRF-Token": panel.dataset.csrfToken }, body: file
        });
        if (!uploaded.ok) {
          var uploadFailure = await uploaded.json().catch(function () { return {}; });
          throw new Error((uploadFailure.error && uploadFailure.error.message) || "Media could not be stored. Check the storage connection and try again.");
        }

        status.textContent = "Checking and saving " + mediaKind + "...";
        var finalize = await fetch("/api/v1/seller/products/media-finalize", {
          method: "POST", credentials: "same-origin", headers: headers,
          body: JSON.stringify({ media_id: prepareBody.data.id })
        });
        var finalized = await finalize.json();
        if (!finalize.ok || !finalized.data) throw new Error("Stored media did not pass verification.");
        preparedMediaID = 0;
        status.textContent = mediaKind === "video" ? "Product video saved." : "Product image saved.";
        window.setTimeout(function () { window.location.reload(); }, 1500);
      } catch (error) {
        if (preparedMediaID) {
          await fetch("/api/v1/seller/products/media-delete", {
            method: "POST", credentials: "same-origin",
            headers: { "Content-Type": "application/json", "X-CSRF-Token": panel.dataset.csrfToken },
            body: JSON.stringify({ media_id: preparedMediaID })
          }).catch(function () {});
        }
        status.textContent = error && error.message ? error.message : "Media upload failed. Please try again.";
      } finally {
        uploadButton.disabled = false;
      }
    });

    panel.querySelectorAll("[data-media-delete]").forEach(function (button) {
      button.addEventListener("click", async function () {
        var mediaID = Number(button.dataset.mediaId);
        if (!mediaID || !window.confirm("Delete this product media?")) return;
        button.disabled = true;
        status.textContent = "Removing product media...";
        try {
          var response = await fetch("/api/v1/seller/products/media-delete", {
            method: "POST", credentials: "same-origin",
            headers: { "Content-Type": "application/json", "X-CSRF-Token": panel.dataset.csrfToken },
            body: JSON.stringify({ media_id: mediaID })
          });
          var result = await response.json();
          if (!response.ok || !result.data) throw new Error("Media could not be removed.");
          status.textContent = "Product media removed.";
          window.setTimeout(function () { window.location.reload(); }, 1500);
        } catch (error) {
          status.textContent = error && error.message ? error.message : "Media removal failed. Please try again.";
          button.disabled = false;
        }
      });
    });
  });
});
