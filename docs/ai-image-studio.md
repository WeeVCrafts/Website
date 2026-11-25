# WeevCrafts Local AI Product Image Studio

## Overview

AI Image Studio is integrated directly into the WeevCrafts Go backend and Seller Portal (`/seller-admin/image-studio`). All image validation, background removal, studio background composition, category-based recommendations, and product gallery publishing run locally on CPU with zero external AI APIs.

## Local Model & Runtime Setup (Windows 11)

The engine runs with:
1. **Primary AI Segmentation**: `u2netp.onnx` (~4.7 MB, Apache-2.0) via ONNX Runtime CPU (`onnxruntime.dll`, ~18 MB, MIT) using a CGO-free pure-Go C-API bridge (`CGO_ENABLED=0`).
2. **Automatic Lightweight Fallback**: If `models/u2netp.onnx` or `models/onnxruntime.dll` is not installed yet, the built-in pure-Go computer-vision segmenter (`CVSegmenter`) runs automatically.

Never downloaded at runtime. To install the local ONNX weights once on Windows 11:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\setup-image-model.ps1
```

Environment variables (optional; defaults shown):

```env
WECRATFS_IMAGE_MODEL_PATH=models/u2netp.onnx
WECRATFS_ONNXRUNTIME_PATH=
WECRATFS_IMAGE_WORKERS=1
WECRATFS_IMAGE_ORT_THREADS=2
```

## Storage & 8 GB RAM Budget

- **Disk**: ~4.7 MB (`u2netp.onnx`) + ~18 MB (`onnxruntime.dll`), no GPU or Docker required for inference.
- **RAM**: Bounded to 1 active segmentation worker by default (`WECRATFS_IMAGE_WORKERS=1`), `IntraOpNumThreads=2`, and working resolution capped at 1400 px longest edge (~180–260 MB peak transient RAM during ONNX inference; ~25–50 MB in pure-Go CV mode).
- **Original Photo Preservation**: Source uploads are stored under private S3 keys (`studio/<owner_id>/<hex>.<ext>`) and are never overwritten. Approved variations are promoted via `MediaService` into `product_media` (`products/<owner_id>/<hex>`), respecting `listing_change_reviews` when the listing is already `approved`.

## Seller Workflow

1. Navigate to **Seller Portal → AI Image Studio** (`/seller-admin/image-studio`) or click **Open AI Image Studio** inside any product editor (`#product-gallery`).
2. Select a product and upload a JPEG, PNG, or WebP photograph (up to 25 MB, 64×64 to 8192×8192 px).
3. Inspect the preserved original photo alongside the transparent PNG cutout, dominant colors, and category-tailored background recommendations.
4. Use the interactive `<canvas>` editor (pointer drag, keyboard arrow keys, `+`/`-` scale, fit mode, shadow slider) and click **Apply & save variation** to render a full 1200×1200 studio image on the server.
5. Compare variations against the original, reject unsuitable ones, or click **Approve & save to gallery** to publish directly into the product's gallery.
