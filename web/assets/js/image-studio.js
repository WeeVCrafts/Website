(function () {
  "use strict";

  document.addEventListener("DOMContentLoaded", function () {
    const root = document.querySelector("[data-image-studio]");
    if (!root) return;

    const csrfToken = root.dataset.csrfToken || "";
    const productSelect = root.querySelector("[data-studio-product]");
    const fileInput = root.querySelector("[data-studio-file]");
    const dropzone = root.querySelector("[data-studio-dropzone]");
    const uploadForm = root.querySelector("[data-studio-upload-form]");
    const processBtn = root.querySelector("[data-studio-process-btn]");
    const statusEl = root.querySelector("[data-studio-status]");
    const jobsListEl = root.querySelector("[data-studio-jobs-list]");

    const compareGrid = root.querySelector("[data-studio-compare]");
    const analysisEmpty = root.querySelector("[data-studio-analysis-empty]");
    const origImg = root.querySelector("[data-studio-original-img]");
    const cutoutImg = root.querySelector("[data-studio-cutout-img]");
    const insightsWrap = root.querySelector("[data-studio-insights]");
    const engineBadge = root.querySelector("[data-studio-engine-badge]");
    const dimsEl = root.querySelector("[data-studio-dimensions]");
    const swatchesEl = root.querySelector("[data-studio-swatches]");
    const recBox = root.querySelector("[data-studio-recommendation]");

    const editorSection = root.querySelector("[data-studio-editor]");
    const canvas = root.querySelector("[data-studio-canvas]");
    const ctx = canvas ? canvas.getContext("2d") : null;
    const canvasHint = root.querySelector("[data-studio-canvas-hint]");
    const toolBtns = Array.from(root.querySelectorAll("[data-studio-tool]"));
    const brushControls = root.querySelector("[data-studio-brush-controls]");
    const brushSizeInput = root.querySelector("[data-studio-brush-size]");
    const brushVal = root.querySelector("[data-studio-brush-val]");
    const maskUndoBtn = root.querySelector("[data-studio-mask-undo]");
    const maskResetBtn = root.querySelector("[data-studio-mask-reset]");
    const maskSaveBtn = root.querySelector("[data-studio-mask-save]");
    const presetGrid = root.querySelector("[data-studio-presets]");
    const fitSelect = root.querySelector("[data-studio-fit]");
    const scaleInput = root.querySelector("[data-studio-scale]");
    const xInput = root.querySelector("[data-studio-x]");
    const yInput = root.querySelector("[data-studio-y]");
    const shadowInput = root.querySelector("[data-studio-shadow]");
    const scaleVal = root.querySelector("[data-studio-scale-val]");
    const xVal = root.querySelector("[data-studio-x-val]");
    const yVal = root.querySelector("[data-studio-y-val]");
    const shadowVal = root.querySelector("[data-studio-shadow-val]");
    const resetBtn = root.querySelector("[data-studio-reset]");
    const applyBtn = root.querySelector("[data-studio-apply]");

    const variationsSection = root.querySelector("[data-studio-variations-section]");
    const variationsGrid = root.querySelector("[data-studio-variations-grid]");
    const compareToggle = root.querySelector("[data-studio-compare-toggle]");
    const regenerateBtn = root.querySelector("[data-studio-regenerate]");
    const rejectJobBtn = root.querySelector("[data-studio-reject-job]");

    const state = {
      presets: [],
      jobs: [],
      activeJob: null,
      cutoutBitmap: null,
      sourceBitmap: null,
      editCutoutCanvas: null,
      maskUndoStack: [],
      maskDirty: false,
      toolMode: "move",
      brushSize: 24,
      hoverPoint: null,
      presetSlug: "pure-white",
      offsetX: 0,
      offsetY: 0,
      scale: 0.80,
      fitMode: "contain",
      shadowStrength: 0.35,
      compareWithOriginal: false,
      pollTimer: null
    };

    function setStatus(msg, isError) {
      if (!statusEl) return;
      statusEl.textContent = msg || "";
      statusEl.classList.toggle("is-error", Boolean(isError));
    }

    async function apiFetch(url, options) {
      const opts = Object.assign({ credentials: "same-origin" }, options || {});
      opts.headers = Object.assign({ "X-CSRF-Token": csrfToken }, opts.headers || {});
      const resp = await fetch(url, opts);
      let payload = null;
      try {
        payload = await resp.json();
      } catch (_) {}
      if (!resp.ok) {
        const msg = (payload && payload.error && payload.error.message) || "Image studio request failed.";
        throw new Error(msg);
      }
      return payload && payload.data ? payload.data : payload;
    }

    async function loadWorkspace(productId) {
      const qs = productId ? "?product_id=" + encodeURIComponent(productId) : "";
      try {
        const data = await apiFetch("/api/v1/seller/image-studio/workspace" + qs, { method: "GET" });
        state.presets = data.presets || [];
        state.jobs = data.jobs || [];
        renderJobsList();
        if (state.jobs.length > 0) {
          selectJob(state.jobs[0]);
        } else {
          clearActiveJob();
        }
      } catch (err) {
        setStatus(err.message, true);
      }
    }

    function clearActiveJob() {
      if (state.pollTimer) {
        clearTimeout(state.pollTimer);
        state.pollTimer = null;
      }
      state.activeJob = null;
      state.cutoutBitmap = null;
      state.sourceBitmap = null;
      state.editCutoutCanvas = null;
      state.maskUndoStack = [];
      state.maskDirty = false;
      updateMaskButtons();
      if (compareGrid) compareGrid.hidden = true;
      if (insightsWrap) insightsWrap.hidden = true;
      if (analysisEmpty) analysisEmpty.hidden = false;
      if (editorSection) editorSection.hidden = true;
      if (variationsSection) variationsSection.hidden = true;
    }

    function renderJobsList() {
      if (!jobsListEl) return;
      jobsListEl.replaceChildren();
      if (!state.jobs.length) {
        const p = document.createElement("p");
        p.className = "studio-empty-note";
        p.textContent = "No AI studio sessions for this product yet. Upload a photograph above.";
        jobsListEl.append(p);
        return;
      }
      state.jobs.forEach(function (job) {
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "studio-job-pill" + (state.activeJob && state.activeJob.id === job.id ? " is-active" : "");
        const thumb = document.createElement("img");
        thumb.src = job.source_preview_url;
        thumb.alt = "";
        thumb.width = 40;
        thumb.height = 40;
        const meta = document.createElement("span");
        const title = document.createElement("strong");
        title.textContent = "Session #" + job.id;
        const sub = document.createElement("small");
        sub.textContent = job.status.replace(/_/g, " ") + " · " + (job.variations ? job.variations.length : 0) + " variations";
        meta.append(title, sub);
        btn.append(thumb, meta);
        btn.addEventListener("click", function () {
          selectJob(job);
        });
        jobsListEl.append(btn);
      });
    }

    function selectJob(job) {
      state.activeJob = job;
      renderJobsList();
      if (state.pollTimer) {
        window.clearTimeout(state.pollTimer);
        state.pollTimer = null;
      }

      if (job.status === "uploaded" || job.status === "processing") {
        setStatus("Processing photograph locally (removing background & creating studio previews)...", false);
        state.pollTimer = window.setTimeout(async function () {
          try {
            const latest = await apiFetch("/api/v1/seller/image-studio/jobs/" + job.id, { method: "GET" });
            upsertJob(latest);
            selectJob(latest);
          } catch (err) {
            setStatus(err.message, true);
          }
        }, 900);
      } else if (job.status === "failed") {
        setStatus(job.error_message || "Background removal failed for this photograph.", true);
      } else {
        setStatus("Ready for editing and gallery approval.", false);
      }

      if (analysisEmpty) analysisEmpty.hidden = true;
      if (compareGrid) compareGrid.hidden = false;
      if (origImg) origImg.src = job.source_preview_url;
      const cutoutSrc = job.cutout_preview_url
        ? job.cutout_preview_url + (job.cutout_object_key ? "?k=" + encodeURIComponent(job.cutout_object_key) : "")
        : "";
      if (cutoutImg) {
        cutoutImg.src = cutoutSrc;
        cutoutImg.hidden = !cutoutSrc;
      }

      if (insightsWrap) {
        insightsWrap.hidden = false;
        if (engineBadge) {
          const eng = job.engine_used || "processing";
          if (eng === "onnx-isnet") engineBadge.textContent = "AI Model: IS-Net 1024px (ONNX CPU)";
          else if (eng === "onnx-u2netp") engineBadge.textContent = "AI Model: U²-Net-P (ONNX CPU)";
          else if (eng === "manual-edit") engineBadge.textContent = "Manual Cutout Touch-Up";
          else engineBadge.textContent = "Local CV Engine: " + eng;
        }
        if (dimsEl) {
          dimsEl.textContent = (job.source_width || 0) + "×" + (job.source_height || 0) + " px · " + (job.aspect_ratio || "square");
        }
        if (swatchesEl) {
          swatchesEl.replaceChildren();
          (job.dominant_colors || []).forEach(function (c) {
            const chip = document.createElement("span");
            chip.className = "studio-swatch";
            chip.style.backgroundColor = c.hex;
            chip.title = c.hex;
            swatchesEl.append(chip);
          });
        }
        if (recBox && job.recommendation) {
          recBox.replaceChildren();
          const strong = document.createElement("strong");
          strong.textContent = "Recommended for " + (job.recommendation.category_name || job.category_name || "this category") + ": ";
          const span = document.createElement("span");
          span.textContent = job.recommendation.reasoning || "";
          recBox.append(strong, span);
        }
      }

      if (job.cutout_preview_url) {
        if (editorSection) editorSection.hidden = false;
        if (variationsSection) variationsSection.hidden = false;
        const firstRec = (job.recommendation && job.recommendation.recommended_presets && job.recommendation.recommended_presets[0]) || "pure-white";
        if (!job.variations || !job.variations.some(function (v) { return v.preset_slug === state.presetSlug; })) {
          state.presetSlug = firstRec;
        }
        renderPresets(job);
        renderVariations(job);
        loadCutoutToCanvas(cutoutSrc, job.source_preview_url);
      } else {
        if (editorSection) editorSection.hidden = true;
        if (variationsSection) variationsSection.hidden = true;
      }
    }

    function upsertJob(job) {
      const idx = state.jobs.findIndex(function (j) { return j.id === job.id; });
      if (idx >= 0) state.jobs[idx] = job;
      else state.jobs.unshift(job);
    }

    function loadCutoutToCanvas(cutoutUrl, sourceUrl) {
      state.editCutoutCanvas = null;
      state.maskUndoStack = [];
      state.maskDirty = false;
      updateMaskButtons();

      const img = new Image();
      img.onload = function () {
        state.cutoutBitmap = img;
        drawCanvas();
      };
      img.src = cutoutUrl;

      if (sourceUrl) {
        const srcImg = new Image();
        srcImg.onload = function () {
          state.sourceBitmap = srcImg;
        };
        srcImg.src = sourceUrl;
      } else {
        state.sourceBitmap = null;
      }
    }

    function ensureEditableCutoutCanvas() {
      if (state.editCutoutCanvas || !state.cutoutBitmap) return state.editCutoutCanvas;
      const iw = state.cutoutBitmap.naturalWidth || state.cutoutBitmap.width;
      const ih = state.cutoutBitmap.naturalHeight || state.cutoutBitmap.height;
      const c = document.createElement("canvas");
      c.width = iw;
      c.height = ih;
      const cctx = c.getContext("2d");
      cctx.drawImage(state.cutoutBitmap, 0, 0, iw, ih);
      state.editCutoutCanvas = c;
      return c;
    }

    function updateMaskButtons() {
      if (maskUndoBtn) maskUndoBtn.disabled = state.maskUndoStack.length === 0;
      if (maskResetBtn) maskResetBtn.disabled = !state.maskDirty;
      if (maskSaveBtn) maskSaveBtn.disabled = !state.maskDirty;
    }

    function setToolMode(mode) {
      state.toolMode = mode;
      toolBtns.forEach(function (btn) {
        const active = btn.dataset.studioTool === mode;
        btn.classList.toggle("is-selected", active);
        btn.setAttribute("aria-pressed", String(active));
      });
      if (brushControls) brushControls.hidden = mode === "move";
      if (canvas) canvas.classList.toggle("is-brush", mode !== "move");
      if (canvasHint) {
        if (mode === "erase") canvasHint.textContent = "Erase mode: Paint over stray background props or spots to remove them, then click Save cutout edits.";
        else if (mode === "restore") canvasHint.textContent = "Restore mode: Paint over missing edges to bring back details from your original photograph.";
        else canvasHint.textContent = "Tip: Drag product to reposition · Arrow keys nudge · + / - keys scale";
      }
      if (mode !== "move") ensureEditableCutoutCanvas();
      state.hoverPoint = null;
      drawCanvas();
    }

    function renderPresets(job) {
      if (!presetGrid) return;
      const recSet = new Set((job.recommendation && job.recommendation.recommended_presets) || []);
      presetGrid.replaceChildren();
      state.presets.forEach(function (preset) {
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "studio-preset-btn" + (state.presetSlug === preset.slug ? " is-selected" : "");
        btn.dataset.presetSlug = preset.slug;
        const swatch = document.createElement("span");
        swatch.className = "studio-preset-swatch";
        swatch.style.background = preset.swatch_hex;
        const label = document.createElement("span");
        label.textContent = preset.label;
        btn.append(swatch, label);
        if (recSet.has(preset.slug)) {
          const badge = document.createElement("em");
          badge.className = "studio-rec-badge";
          badge.textContent = "Recommended";
          btn.append(badge);
        }
        btn.addEventListener("click", function () {
          state.presetSlug = preset.slug;
          renderPresets(job);
          drawCanvas();
        });
        presetGrid.append(btn);
      });
    }

    function paintCanvasBackground(slug, w, h) {
      if (!ctx) return;
      if (slug === "pure-white") {
        ctx.fillStyle = "#ffffff";
        ctx.fillRect(0, 0, w, h);
        return;
      }
      if (slug === "light-grey") {
        const g = ctx.createRadialGradient(w * 0.5, h * 0.44, 10, w * 0.5, h * 0.44, w * 0.78);
        g.addColorStop(0, "#f7f8fa");
        g.addColorStop(1, "#dee2e8");
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, w, h);
        return;
      }
      if (slug === "dark-studio") {
        const g = ctx.createRadialGradient(w * 0.5, h * 0.40, 10, w * 0.5, h * 0.40, w * 0.78);
        g.addColorStop(0, "#383d48");
        g.addColorStop(1, "#14161b");
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, w, h);
        return;
      }
      if (slug === "soft-gradient") {
        const g = ctx.createLinearGradient(0, 0, 0, h);
        g.addColorStop(0, "#fbf4ee");
        g.addColorStop(1, "#f1dfd6");
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, w, h);
        return;
      }
      if (slug === "luxury-studio") {
        const g = ctx.createRadialGradient(w * 0.5, h * 0.42, 10, w * 0.5, h * 0.42, w * 0.78);
        g.addColorStop(0, "#34524a");
        g.addColorStop(1, "#15231f");
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, w, h);
        ctx.fillStyle = "rgba(18, 28, 25, 0.35)";
        ctx.fillRect(0, h * 0.72, w, h * 0.28);
        return;
      }
      if (slug === "natural") {
        const g = ctx.createLinearGradient(0, 0, 0, h);
        g.addColorStop(0, "#eaf1e8");
        g.addColorStop(1, "#d2ded0");
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, w, h);
        ctx.fillStyle = "rgba(190, 200, 184, 0.45)";
        ctx.fillRect(0, h * 0.75, w, h * 0.25);
        return;
      }
      if (slug === "lifestyle") {
        const g = ctx.createLinearGradient(0, 0, 0, h);
        g.addColorStop(0, "#f7f1e9");
        g.addColorStop(1, "#ebe0d0");
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, w, h);
        ctx.fillStyle = "rgba(214, 194, 168, 0.65)";
        ctx.fillRect(0, h * 0.72, w, h * 0.28);
        return;
      }
      const g = ctx.createRadialGradient(w * 0.5, h * 0.45, 10, w * 0.5, h * 0.45, w * 0.78);
      g.addColorStop(0, "#fcf9f3");
      g.addColorStop(1, "#ede4d7");
      ctx.fillStyle = g;
      ctx.fillRect(0, 0, w, h);
    }

    function getPlacementRect(imgSource) {
      const size = canvas.width;
      const iw = imgSource.naturalWidth || imgSource.width;
      const ih = imgSource.naturalHeight || imgSource.height;
      let targetW, targetH;
      if (state.fitMode === "cover") {
        const sc = Math.max(size / iw, size / ih) * state.scale;
        targetW = iw * sc;
        targetH = ih * sc;
      } else if (state.fitMode === "fill") {
        targetW = size * state.scale;
        targetH = size * state.scale;
      } else {
        const sc = Math.min(size / iw, size / ih) * state.scale;
        targetW = iw * sc;
        targetH = ih * sc;
      }
      const cx = size / 2 + state.offsetX * size;
      const cy = size / 2 + state.offsetY * size;
      return { iw, ih, cx, cy, dx: cx - targetW / 2, dy: cy - targetH / 2, targetW, targetH };
    }

    function drawCanvas() {
      if (!canvas || !ctx) return;
      const size = canvas.width;
      paintCanvasBackground(state.presetSlug, size, size);
      const activeSource = state.editCutoutCanvas || state.cutoutBitmap;
      if (!activeSource) return;

      const box = getPlacementRect(activeSource);

      if (state.shadowStrength > 0.01) {
        const sAlpha = (state.presetSlug === "pure-white" ? 0.16 : 0.28) * state.shadowStrength;
        const rx = Math.max(16, box.targetW * 0.42);
        const ry = Math.max(8, box.targetH * 0.065);
        const sy = box.dy + box.targetH - box.targetH * 0.03;
        ctx.save();
        const sg = ctx.createRadialGradient(box.cx, sy, 2, box.cx, sy, rx);
        sg.addColorStop(0, "rgba(18, 18, 22, " + sAlpha.toFixed(3) + ")");
        sg.addColorStop(1, "rgba(18, 18, 22, 0)");
        ctx.translate(box.cx, sy);
        ctx.scale(1, ry / rx);
        ctx.translate(-box.cx, -sy);
        ctx.fillStyle = sg;
        ctx.beginPath();
        ctx.arc(box.cx, sy, rx, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();
      }

      ctx.drawImage(activeSource, box.dx, box.dy, box.targetW, box.targetH);
      if (state.toolMode !== "move" && state.hoverPoint) {
        ctx.save();
        ctx.beginPath();
        ctx.arc(state.hoverPoint.x, state.hoverPoint.y, state.brushSize / 2, 0, Math.PI * 2);
        ctx.strokeStyle = state.toolMode === "erase" ? "#b91c1c" : "#14532d";
        ctx.lineWidth = 2;
        ctx.setLineDash([4, 3]);
        ctx.stroke();
        ctx.restore();
      }
      syncControlLabels();
    }

    function syncControlLabels() {
      if (fitSelect) fitSelect.value = state.fitMode;
      if (scaleInput) scaleInput.value = String(state.scale);
      if (xInput) xInput.value = String(state.offsetX);
      if (yInput) yInput.value = String(state.offsetY);
      if (shadowInput) shadowInput.value = String(state.shadowStrength);
      if (scaleVal) scaleVal.textContent = state.scale.toFixed(2);
      if (xVal) xVal.textContent = Math.round(state.offsetX * 100) + "%";
      if (yVal) yVal.textContent = Math.round(state.offsetY * 100) + "%";
      if (shadowVal) shadowVal.textContent = Math.round(state.shadowStrength * 100) + "%";
    }

    function renderVariations(job) {
      if (!variationsGrid) return;
      variationsGrid.replaceChildren();
      (job.variations || []).forEach(function (v) {
        const card = document.createElement("article");
        card.className = "studio-variation-card is-" + v.status;

        const header = document.createElement("header");
        const title = document.createElement("strong");
        title.textContent = v.preset_label;
        const badge = document.createElement("span");
        badge.className = "ui-status " + (v.status === "approved" ? "is-success" : v.status === "rejected" ? "is-danger" : "is-warning");
        badge.textContent = v.status.replace(/_/g, " ");
        header.append(title, badge);

        const mediaWrap = document.createElement("div");
        mediaWrap.className = "studio-variation-media" + (state.compareWithOriginal ? " is-comparing" : "");
        const img = document.createElement("img");
        img.src = v.preview_url + (v.object_key ? (v.preview_url.indexOf("?") >= 0 ? "&k=" : "?k=") + encodeURIComponent(v.object_key) : "");
        img.alt = v.preset_label + " variation";
        img.loading = "lazy";
        mediaWrap.append(img);
        if (state.compareWithOriginal) {
          const orig = document.createElement("img");
          orig.src = job.source_preview_url;
          orig.alt = "Original reference";
          orig.className = "studio-variation-orig";
          mediaWrap.append(orig);
        }

        const meta = document.createElement("p");
        meta.className = "studio-variation-meta";
        meta.textContent = (v.category_recommended ? "Recommended · " : "") + v.width + "×" + v.height + " px";

        const actions = document.createElement("div");
        actions.className = "studio-variation-actions";

        const editBtn = document.createElement("button");
        editBtn.type = "button";
        editBtn.className = "seller-button outline";
        editBtn.textContent = "Edit in canvas";
        editBtn.addEventListener("click", function () {
          state.presetSlug = v.preset_slug;
          state.offsetX = v.composition.offset_x || 0;
          state.offsetY = v.composition.offset_y || 0;
          state.scale = v.composition.scale || 0.80;
          state.fitMode = v.composition.fit_mode || "contain";
          state.shadowStrength = typeof v.composition.shadow_strength === "number" ? v.composition.shadow_strength : 0.35;
          renderPresets(job);
          drawCanvas();
          editorSection?.scrollIntoView({ behavior: "smooth", block: "start" });
        });

        const approveBtn = document.createElement("button");
        approveBtn.type = "button";
        approveBtn.className = "seller-button primary";
        approveBtn.textContent = v.status === "approved" ? "Saved to gallery" : "Approve & save to gallery";
        approveBtn.disabled = v.status === "approved";
        approveBtn.addEventListener("click", async function () {
          approveBtn.disabled = true;
          setStatus("Saving approved image to product gallery...", false);
          try {
            const res = await apiFetch("/api/v1/seller/image-studio/variations/" + v.id + "/approve", { method: "POST" });
            const latest = await apiFetch("/api/v1/seller/image-studio/jobs/" + job.id, { method: "GET" });
            upsertJob(latest);
            selectJob(latest);
            setStatus("Approved image saved to product gallery (" + (res.media && res.media.public_path ? res.media.public_path : "ready") + "). Original photo preserved.", false);
          } catch (err) {
            setStatus(err.message, true);
            approveBtn.disabled = false;
          }
        });

        const rejectBtn = document.createElement("button");
        rejectBtn.type = "button";
        rejectBtn.className = "seller-button danger";
        rejectBtn.textContent = "Reject";
        rejectBtn.disabled = v.status === "rejected" || v.status === "approved";
        rejectBtn.addEventListener("click", async function () {
          try {
            await apiFetch("/api/v1/seller/image-studio/variations/" + v.id + "/reject", { method: "POST" });
            const latest = await apiFetch("/api/v1/seller/image-studio/jobs/" + job.id, { method: "GET" });
            upsertJob(latest);
            selectJob(latest);
          } catch (err) {
            setStatus(err.message, true);
          }
        });

        actions.append(editBtn, approveBtn, rejectBtn);
        card.append(header, mediaWrap, meta, actions);
        variationsGrid.append(card);
      });
    }

    if (productSelect) {
      productSelect.addEventListener("change", function () {
        loadWorkspace(productSelect.value);
      });
    }

    if (dropzone && fileInput) {
      ["dragenter", "dragover"].forEach(function (ev) {
        dropzone.addEventListener(ev, function (e) { e.preventDefault(); dropzone.classList.add("is-dragging"); });
      });
      ["dragleave", "drop"].forEach(function (ev) {
        dropzone.addEventListener(ev, function (e) { e.preventDefault(); dropzone.classList.remove("is-dragging"); });
      });
      dropzone.addEventListener("drop", function (e) {
        if (!fileInput.disabled && e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length) {
          fileInput.files = e.dataTransfer.files;
        }
      });
    }

    if (uploadForm) {
      uploadForm.addEventListener("submit", async function (e) {
        e.preventDefault();
        const files = Array.from((fileInput && fileInput.files) || []);
        const productId = productSelect ? productSelect.value : "";
        if (!productId) {
          setStatus("Select a product first.", true);
          return;
        }
        if (!files.length) {
          setStatus("Choose one or more JPEG, PNG, or WebP photographs to process.", true);
          fileInput?.focus();
          return;
        }
        const invalid = files.find(function (f) { return f.size <= 0 || f.size > 25 * 1024 * 1024; });
        if (invalid) {
          setStatus(invalid.name + " must be 25 MB or smaller.", true);
          return;
        }

        if (processBtn) processBtn.disabled = true;
        try {
          for (let i = 0; i < files.length; i++) {
            const file = files[i];
            const fd = new FormData();
            fd.append("product_id", productId);
            fd.append("image", file);
            setStatus("Processing " + (i + 1) + "/" + files.length + ": " + file.name + " (removing background locally)...", false);
            const job = await apiFetch("/api/v1/seller/image-studio/upload", { method: "POST", body: fd });
            upsertJob(job);
            selectJob(job);
          }
          if (files.length > 1) {
            setStatus("Processed " + files.length + " product photographs.", false);
          }
        } catch (err) {
          setStatus(err.message, true);
        } finally {
          if (processBtn) processBtn.disabled = false;
        }
      });
    }

    [fitSelect, scaleInput, xInput, yInput, shadowInput].forEach(function (el) {
      if (!el) return;
      el.addEventListener("input", function () {
        if (fitSelect) state.fitMode = fitSelect.value;
        if (scaleInput) state.scale = parseFloat(scaleInput.value) || 0.80;
        if (xInput) state.offsetX = parseFloat(xInput.value) || 0;
        if (yInput) state.offsetY = parseFloat(yInput.value) || 0;
        if (shadowInput) state.shadowStrength = parseFloat(shadowInput.value) || 0;
        drawCanvas();
      });
    });

    function applyBrushStroke(canvasX, canvasY) {
      const target = ensureEditableCutoutCanvas();
      if (!target) return;
      const box = getPlacementRect(target);
      const scaleX = box.iw / Math.max(1, box.targetW);
      const scaleY = box.ih / Math.max(1, box.targetH);
      const px = (canvasX - box.dx) * scaleX;
      const py = (canvasY - box.dy) * scaleY;
      const radius = Math.max(2, (state.brushSize / 2) * ((scaleX + scaleY) / 2));

      const tctx = target.getContext("2d");
      if (state.toolMode === "erase") {
        tctx.save();
        tctx.globalCompositeOperation = "destination-out";
        const grad = tctx.createRadialGradient(px, py, radius * 0.55, px, py, radius);
        grad.addColorStop(0, "rgba(0,0,0,1)");
        grad.addColorStop(1, "rgba(0,0,0,0)");
        tctx.fillStyle = grad;
        tctx.beginPath();
        tctx.arc(px, py, radius, 0, Math.PI * 2);
        tctx.fill();
        tctx.restore();
      } else if (state.toolMode === "restore" && state.sourceBitmap) {
        tctx.save();
        tctx.beginPath();
        tctx.arc(px, py, radius, 0, Math.PI * 2);
        tctx.clip();
        tctx.drawImage(state.sourceBitmap, 0, 0, box.iw, box.ih);
        tctx.restore();
      }
      state.maskDirty = true;
      updateMaskButtons();
    }

    toolBtns.forEach(function (btn) {
      btn.addEventListener("click", function () {
        setToolMode(btn.dataset.studioTool || "move");
      });
    });

    if (brushSizeInput) {
      brushSizeInput.addEventListener("input", function () {
        state.brushSize = parseInt(brushSizeInput.value, 10) || 24;
        if (brushVal) brushVal.textContent = String(state.brushSize);
      });
    }

    if (maskUndoBtn) {
      maskUndoBtn.addEventListener("click", function () {
        if (!state.editCutoutCanvas || !state.maskUndoStack.length) return;
        const prev = state.maskUndoStack.pop();
        state.editCutoutCanvas.getContext("2d").putImageData(prev, 0, 0);
        state.maskDirty = state.maskUndoStack.length > 0;
        updateMaskButtons();
        drawCanvas();
      });
    }

    if (maskResetBtn) {
      maskResetBtn.addEventListener("click", function () {
        if (!state.cutoutBitmap) return;
        state.editCutoutCanvas = null;
        state.maskUndoStack = [];
        state.maskDirty = false;
        ensureEditableCutoutCanvas();
        updateMaskButtons();
        drawCanvas();
      });
    }

    if (maskSaveBtn) {
      maskSaveBtn.addEventListener("click", function () {
        if (!state.activeJob || !state.editCutoutCanvas) return;
        maskSaveBtn.disabled = true;
        setStatus("Saving manual cutout edits & refreshing studio variations...", false);
        state.editCutoutCanvas.toBlob(async function (blob) {
          if (!blob) {
            setStatus("Could not export edited cutout PNG.", true);
            updateMaskButtons();
            return;
          }
          try {
            const fd = new FormData();
            fd.append("cutout", blob, "cutout.png");
            const latest = await apiFetch("/api/v1/seller/image-studio/jobs/" + state.activeJob.id + "/cutout", {
              method: "POST",
              body: fd
            });
            upsertJob(latest);
            setToolMode("move");
            selectJob(latest);
            setStatus("Manual cutout saved and studio variations updated.", false);
          } catch (err) {
            setStatus(err.message, true);
            updateMaskButtons();
          }
        }, "image/png");
      });
    }

    if (canvas) {
      let dragging = false, painting = false, startX = 0, startY = 0, origOffX = 0, origOffY = 0;
      function toCanvasCoords(e) {
        const rect = canvas.getBoundingClientRect();
        return {
          x: ((e.clientX - rect.left) / Math.max(1, rect.width)) * canvas.width,
          y: ((e.clientY - rect.top) / Math.max(1, rect.height)) * canvas.height
        };
      }
      canvas.addEventListener("pointerdown", function (e) {
        if (!state.cutoutBitmap) return;
        if (state.toolMode !== "move") {
          const target = ensureEditableCutoutCanvas();
          if (target) {
            if (state.maskUndoStack.length >= 10) state.maskUndoStack.shift();
            state.maskUndoStack.push(target.getContext("2d").getImageData(0, 0, target.width, target.height));
          }
          painting = true;
          const pt = toCanvasCoords(e);
          state.hoverPoint = pt;
          applyBrushStroke(pt.x, pt.y);
          drawCanvas();
          canvas.setPointerCapture(e.pointerId);
          return;
        }
        dragging = true;
        startX = e.clientX;
        startY = e.clientY;
        origOffX = state.offsetX;
        origOffY = state.offsetY;
        canvas.setPointerCapture(e.pointerId);
      });
      canvas.addEventListener("pointermove", function (e) {
        if (state.toolMode !== "move") {
          const pt = toCanvasCoords(e);
          state.hoverPoint = pt;
          if (painting) applyBrushStroke(pt.x, pt.y);
          drawCanvas();
          return;
        }
        if (!dragging) return;
        const rect = canvas.getBoundingClientRect();
        const dx = (e.clientX - startX) / Math.max(1, rect.width);
        const dy = (e.clientY - startY) / Math.max(1, rect.height);
        state.offsetX = Math.max(-0.45, Math.min(0.45, origOffX + dx));
        state.offsetY = Math.max(-0.45, Math.min(0.45, origOffY + dy));
        drawCanvas();
      });
      canvas.addEventListener("pointerup", function () { dragging = false; painting = false; });
      canvas.addEventListener("pointercancel", function () { dragging = false; painting = false; });
      canvas.addEventListener("pointerleave", function () {
        if (!painting && state.hoverPoint) {
          state.hoverPoint = null;
          drawCanvas();
        }
      });
      canvas.addEventListener("keydown", function (e) {
        if (!state.cutoutBitmap) return;
        let handled = true;
        if (e.key === "ArrowLeft") state.offsetX = Math.max(-0.45, state.offsetX - 0.02);
        else if (e.key === "ArrowRight") state.offsetX = Math.min(0.45, state.offsetX + 0.02);
        else if (e.key === "ArrowUp") state.offsetY = Math.max(-0.45, state.offsetY - 0.02);
        else if (e.key === "ArrowDown") state.offsetY = Math.min(0.45, state.offsetY + 0.02);
        else if (e.key === "+" || e.key === "=") state.scale = Math.min(1.40, state.scale + 0.04);
        else if (e.key === "-" || e.key === "_") state.scale = Math.max(0.30, state.scale - 0.04);
        else handled = false;
        if (handled) {
          e.preventDefault();
          drawCanvas();
        }
      });
    }

    if (resetBtn) {
      resetBtn.addEventListener("click", function () {
        state.offsetX = 0;
        state.offsetY = 0;
        state.scale = 0.80;
        state.fitMode = "contain";
        state.shadowStrength = 0.35;
        drawCanvas();
      });
    }

    if (applyBtn) {
      applyBtn.addEventListener("click", async function () {
        if (!state.activeJob) return;
        applyBtn.disabled = true;
        setStatus("Rendering full-resolution studio variation...", false);
        try {
          await apiFetch("/api/v1/seller/image-studio/jobs/" + state.activeJob.id + "/compose", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              preset_slug: state.presetSlug,
              offset_x: state.offsetX,
              offset_y: state.offsetY,
              scale: state.scale,
              fit_mode: state.fitMode,
              shadow_strength: state.shadowStrength
            })
          });
          const latest = await apiFetch("/api/v1/seller/image-studio/jobs/" + state.activeJob.id, { method: "GET" });
          upsertJob(latest);
          selectJob(latest);
          setStatus("Custom studio variation updated.", false);
        } catch (err) {
          setStatus(err.message, true);
        } finally {
          applyBtn.disabled = false;
        }
      });
    }

    if (compareToggle) {
      compareToggle.addEventListener("click", function () {
        state.compareWithOriginal = !state.compareWithOriginal;
        compareToggle.setAttribute("aria-pressed", String(state.compareWithOriginal));
        if (state.activeJob) renderVariations(state.activeJob);
      });
    }

    if (regenerateBtn) {
      regenerateBtn.addEventListener("click", async function () {
        if (!state.activeJob) return;
        regenerateBtn.disabled = true;
        setStatus("Regenerating studio variations...", false);
        try {
          const latest = await apiFetch("/api/v1/seller/image-studio/jobs/" + state.activeJob.id + "/regenerate", { method: "POST" });
          upsertJob(latest);
          selectJob(latest);
          setStatus("Variations regenerated.", false);
        } catch (err) {
          setStatus(err.message, true);
        } finally {
          regenerateBtn.disabled = false;
        }
      });
    }

    if (rejectJobBtn) {
      rejectJobBtn.addEventListener("click", async function () {
        if (!state.activeJob) return;
        const ok = window.WVUI && window.WVUI.confirmAction
          ? await window.WVUI.confirmAction("Reject all variations in this session? Your original photo remains preserved.", rejectJobBtn, "Reject batch", { intent: "danger" })
          : window.confirm("Reject all variations in this session?");
        if (!ok) return;
        try {
          await apiFetch("/api/v1/seller/image-studio/jobs/" + state.activeJob.id + "/reject", { method: "POST" });
          const latest = await apiFetch("/api/v1/seller/image-studio/jobs/" + state.activeJob.id, { method: "GET" });
          upsertJob(latest);
          selectJob(latest);
        } catch (err) {
          setStatus(err.message, true);
        }
      });
    }

    if (root.dataset.sellerActive === "true") {
      loadWorkspace(root.dataset.selectedProductId || (productSelect && productSelect.value) || "");
    }
  });
})();
