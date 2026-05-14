/**
 * WP Image Resizer — Focal Point & Crop Editor
 *
 * - Shows live crop previews for every registered image size that uses cropping.
 * - Provides a per-size manual crop tool: a fixed-ratio draggable rectangle with
 *   rule-of-thirds grid and corner/midpoint handles (matching the reference UI).
 * - Per-size focal overrides are stored in post meta and take priority over the
 *   shared global focal point when thumbnails are regenerated.
 *
 * The crop math mirrors internal/image/processor.go focalPointCrop exactly so
 * that the live previews match the generated output.
 */
(function () {
    'use strict';

    var WPIR = window.WPIR || {};

    // -------------------------------------------------------------------------
    // Crop math — mirrors processor.go focalPointCrop exactly.
    // -------------------------------------------------------------------------

    function clamp(v, lo, hi) {
        return Math.max(lo, Math.min(hi, v));
    }

    /**
     * Calculate the crop rectangle in scaled-image pixel space.
     *
     * @param {number} origW   Original image width
     * @param {number} origH   Original image height
     * @param {number} targetW Target crop width
     * @param {number} targetH Target crop height
     * @param {number} focalX  Focal point X 0–1
     * @param {number} focalY  Focal point Y 0–1
     * @returns {{ left, top, cropW, cropH, scale }}
     */
    function calcCropRect(origW, origH, targetW, targetH, focalX, focalY) {
        var scale = Math.max(targetW / origW, targetH / origH);
        var scW   = origW * scale;
        var scH   = origH * scale;

        var left = Math.round(focalX * scW - targetW / 2);
        var top  = Math.round(focalY * scH - targetH / 2);

        left = clamp(left, 0, Math.max(0, scW - targetW));
        top  = clamp(top,  0, Math.max(0, scH - targetH));

        return { left: left, top: top, cropW: targetW, cropH: targetH, scale: scale };
    }

    /**
     * Apply a crop rectangle to an absolutely-positioned img inside an
     * overflow:hidden container of size displayW × displayH.
     */
    function applyPreviewCrop(imgEl, origW, origH, cropRect, displayW, displayH) {
        var dispScale = displayW / cropRect.cropW;
        var imgDispW  = origW * cropRect.scale * dispScale;
        var imgDispH  = origH * cropRect.scale * dispScale;
        var offsetX   = -(cropRect.left * cropRect.scale * dispScale);
        var offsetY   = -(cropRect.top  * cropRect.scale * dispScale);

        imgEl.style.width      = imgDispW  + 'px';
        imgEl.style.height     = imgDispH  + 'px';
        imgEl.style.marginLeft = offsetX   + 'px';
        imgEl.style.marginTop  = offsetY   + 'px';
    }

    // -------------------------------------------------------------------------
    // Preview grid helpers
    // -------------------------------------------------------------------------

    var MAX_PREVIEW_DIM = 120;

    function previewDims(targetW, targetH) {
        if (targetW >= targetH) {
            return { w: MAX_PREVIEW_DIM, h: Math.round(MAX_PREVIEW_DIM * targetH / targetW) };
        }
        return { w: Math.round(MAX_PREVIEW_DIM * targetW / targetH), h: MAX_PREVIEW_DIM };
    }

    // -------------------------------------------------------------------------
    // Crop tool DOM — built once per editor, reused across sizes.
    // -------------------------------------------------------------------------

    /**
     * Create the crop tool DOM inside `container` and return a handle object.
     */
    function buildCropTool(container) {
        var el = document.createElement('div');
        el.className = 'wpir-crop-tool';
        el.style.display = 'none';

        // Title (updated per size when opened).
        var titleEl = document.createElement('p');
        titleEl.className = 'wpir-crop-tool-title';
        el.appendChild(titleEl);

        var hintEl = document.createElement('p');
        hintEl.className = 'wpir-crop-tool-hint';
        hintEl.textContent = wpirL10n.dragHint || 'Drag to reposition:';
        el.appendChild(hintEl);

        // Image workspace.
        var imgWrap = document.createElement('div');
        imgWrap.className = 'wpir-crop-tool-img-wrap';

        var img = document.createElement('img');
        img.alt = '';
        img.draggable = false;
        imgWrap.appendChild(img);

        // Crop rectangle with rule-of-thirds grid and handles.
        var cropRect = document.createElement('div');
        cropRect.className = 'wpir-crop-rect';

        // Rule-of-thirds grid (4 dashed lines).
        ['h1', 'h2', 'v1', 'v2'].forEach(function (cls) {
            var line = document.createElement('div');
            line.className = 'wpir-crop-grid-line wpir-crop-grid-' + cls;
            cropRect.appendChild(line);
        });

        // Corner and midpoint handles (8 total, visual only).
        ['tl', 'tc', 'tr', 'ml', 'mr', 'bl', 'bc', 'br'].forEach(function (pos) {
            var h = document.createElement('div');
            h.className = 'wpir-crop-handle';
            h.setAttribute('data-pos', pos);
            cropRect.appendChild(h);
        });

        imgWrap.appendChild(cropRect);
        el.appendChild(imgWrap);

        // Action buttons.
        var actionsEl = document.createElement('div');
        actionsEl.className = 'wpir-crop-tool-actions';

        var saveBtn = document.createElement('button');
        saveBtn.type = 'button';
        saveBtn.className = 'button button-primary wpir-crop-save-btn';
        saveBtn.textContent = wpirL10n.saveCrop || 'Save Crop';

        var clearBtn = document.createElement('button');
        clearBtn.type = 'button';
        clearBtn.className = 'button wpir-crop-clear-btn';
        clearBtn.textContent = wpirL10n.clearCrop || 'Clear Override';

        var cancelBtn = document.createElement('button');
        cancelBtn.type = 'button';
        cancelBtn.className = 'button wpir-crop-cancel-btn';
        cancelBtn.textContent = wpirL10n.cancelCrop || 'Cancel';

        var savedMsg = document.createElement('span');
        savedMsg.className = 'wpir-crop-saved-msg';
        savedMsg.textContent = wpirL10n.cropSaved || 'Crop saved!';

        actionsEl.appendChild(saveBtn);
        actionsEl.appendChild(clearBtn);
        actionsEl.appendChild(cancelBtn);
        actionsEl.appendChild(savedMsg);
        el.appendChild(actionsEl);

        container.appendChild(el);

        return {
            el:        el,
            titleEl:   titleEl,
            imgWrap:   imgWrap,
            img:       img,
            cropRect:  cropRect,
            saveBtn:   saveBtn,
            clearBtn:  clearBtn,
            cancelBtn: cancelBtn,
            savedMsg:  savedMsg
        };
    }

    // -------------------------------------------------------------------------
    // Crop tool interaction
    // -------------------------------------------------------------------------

    /**
     * Open the crop tool for a specific image size.
     *
     * @param {Object} opts
     * @param {Object}   opts.size          { name, label, width, height }
     * @param {number}   opts.attachmentId
     * @param {string}   opts.imageUrl      Full or large image URL for the workspace
     * @param {number}   opts.origW         Original image width
     * @param {number}   opts.origH         Original image height
     * @param {Object}   opts.sizeFocals    Mutable map of per-size focals { name: {x,y} }
     * @param {Object}   opts.tool          Handle returned by buildCropTool()
     * @param {Element}  opts.grid          Preview grid element (to hide while tool open)
     * @param {Element}  opts.smartNote     Smart-mode notice element
     * @param {Element}  opts.badge         Override badge for this size
     * @param {Function} opts.onSaveOrClear Called after a successful save or clear
     */
    // Handle metadata for ratio-locked resize.
    // driver: which axis drives the new size ('x' or 'y')
    // xDir/yDir: sign of delta's contribution to width/height
    // rx/ry: fraction of rect size at which the anchor point sits
    var HANDLE_META = {
        tl: { driver: 'x', xDir: -1, yDir: -1, rx: 1,   ry: 1   },
        tc: { driver: 'y', xDir:  0, yDir: -1, rx: 0.5, ry: 1   },
        tr: { driver: 'x', xDir: +1, yDir: -1, rx: 0,   ry: 1   },
        ml: { driver: 'x', xDir: -1, yDir:  0, rx: 1,   ry: 0.5 },
        mr: { driver: 'x', xDir: +1, yDir:  0, rx: 0,   ry: 0.5 },
        bl: { driver: 'x', xDir: -1, yDir: +1, rx: 1,   ry: 0   },
        bc: { driver: 'y', xDir:  0, yDir: +1, rx: 0.5, ry: 0   },
        br: { driver: 'x', xDir: +1, yDir: +1, rx: 0,   ry: 0   }
    };
    var MIN_RECT_PX = 30; // minimum shorter-side pixels in display space

        function openCropTool(opts) {
        var size          = opts.size;
        var attachmentId  = opts.attachmentId;
        var imageUrl      = opts.imageUrl;
        var origW         = opts.origW;
        var origH         = opts.origH;
        var sizeFocals    = opts.sizeFocals;
        var tool          = opts.tool;
        var grid          = opts.grid;
        var smartNote     = opts.smartNote;
        var badge         = opts.badge;
        var onSaveOrClear = opts.onSaveOrClear;

        var tw = size.width;
        var th = size.height;
        var backendScale = Math.max(tw / origW, th / origH);

        // Show tool; hide grid.
        tool.el.style.display    = 'block';
        grid.style.display       = 'none';
        smartNote.style.display  = 'none';

        tool.titleEl.textContent = size.label + ' – ' + tw + '×' + th;
        tool.img.src = imageUrl;

        /**
         * Position the crop rect based on current image dimensions and stored focal.
         * Called on image load and immediately if image is already cached.
         */
        function positionRect() {
            var dW = tool.imgWrap.offsetWidth || 300;
            // Compute display height proportionally from origW/H.
            var dH = Math.round(origH * dW / origW);

            // Set explicit height so the overlay is contained correctly.
            tool.imgWrap.style.height = dH + 'px';

            // Crop rect dimensions in display space (clamped to min and image bounds).
            var rectW = Math.min(Math.max(MIN_RECT_PX, Math.round(tw / backendScale / origW * dW)), dW);
            var rectH = Math.min(Math.max(MIN_RECT_PX, Math.round(th / backendScale / origH * dH)), dH);

            tool.cropRect.style.width  = rectW + 'px';
            tool.cropRect.style.height = rectH + 'px';

            // Initial position from stored per-size focal, or center.
            var sf = sizeFocals[size.name];
            var fx = sf ? sf.x : 0.5;
            var fy = sf ? sf.y : 0.5;

            var left = clamp(Math.round(fx * dW - rectW / 2), 0, Math.max(0, dW - rectW));
            var top  = clamp(Math.round(fy * dH - rectH / 2), 0, Math.max(0, dH - rectH));

            tool.cropRect.style.left = left + 'px';
            tool.cropRect.style.top  = top  + 'px';
        }

        // Position immediately if cached, always re-position on load.
        tool.img.onload = positionRect;
        if (tool.img.complete && tool.img.naturalWidth) {
            positionRect();
        }

        // ---- Drag (move) and Resize ----
        // resizePos is the data-pos of the handle being dragged, or null for a body drag.
        var dragging  = false;
        var resizePos = null;
        var dragStartX, dragStartY;
        var startW, startH, startLeft, startTop;

        function onMouseMove(e) {
            if (!dragging) return;
            var dW = tool.imgWrap.offsetWidth  || 300;
            var dH = tool.imgWrap.offsetHeight || Math.round(origH * dW / origW);

            if (resizePos) {
                // ---- Resize mode: ratio-locked ----
                var meta  = HANDLE_META[resizePos];
                var ratio = tw / th;

                var newW, newH;
                if (meta.driver === 'x') {
                    newW = Math.max(MIN_RECT_PX, startW + (e.clientX - dragStartX) * meta.xDir);
                    newH = newW / ratio;
                } else {
                    newH = Math.max(MIN_RECT_PX, startH + (e.clientY - dragStartY) * meta.yDir);
                    newW = newH * ratio;
                }

                // Clamp to image bounds while maintaining ratio.
                newW = Math.min(newW, dW);
                newH = Math.min(newH, dH);
                if (newW / ratio > dH) { newH = dH; newW = newH * ratio; }
                if (newH * ratio > dW) { newW = dW; newH = newW / ratio; }

                // Anchor point (fixed corner/edge during this drag).
                var anchorX = startLeft + meta.rx * startW;
                var anchorY = startTop  + meta.ry * startH;

                var newLeft = anchorX - meta.rx * newW;
                var newTop  = anchorY - meta.ry * newH;

                // Clamp position so rect stays inside image.
                newLeft = clamp(newLeft, 0, dW - newW);
                newTop  = clamp(newTop,  0, dH - newH);

                tool.cropRect.style.width  = Math.round(newW)    + 'px';
                tool.cropRect.style.height = Math.round(newH)    + 'px';
                tool.cropRect.style.left   = Math.round(newLeft) + 'px';
                tool.cropRect.style.top    = Math.round(newTop)  + 'px';
            } else {
                // ---- Move mode: reposition rect ----
                var rectW = parseInt(tool.cropRect.style.width,  10) || 0;
                var rectH = parseInt(tool.cropRect.style.height, 10) || 0;
                var left = clamp(startLeft + (e.clientX - dragStartX), 0, dW - rectW);
                var top  = clamp(startTop  + (e.clientY - dragStartY), 0, dH - rectH);
                tool.cropRect.style.left = left + 'px';
                tool.cropRect.style.top  = top  + 'px';
            }
        }

        function onMouseUp() {
            dragging  = false;
            resizePos = null;
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup',   onMouseUp);
        }

        tool.cropRect.onmousedown = function (e) {
            e.preventDefault();
            e.stopPropagation();
            dragStartX = e.clientX;
            dragStartY = e.clientY;
            startLeft  = parseInt(tool.cropRect.style.left,   10) || 0;
            startTop   = parseInt(tool.cropRect.style.top,    10) || 0;
            startW     = parseInt(tool.cropRect.style.width,  10) || 0;
            startH     = parseInt(tool.cropRect.style.height, 10) || 0;
            // data-pos is set on handle elements; absent on grid lines and rect body.
            resizePos  = (e.target.dataset && e.target.dataset.pos) ? e.target.dataset.pos : null;
            dragging   = true;
            document.addEventListener('mousemove', onMouseMove);
            document.addEventListener('mouseup',   onMouseUp);
        };

        // ---- Derive focal from current rect position ----
        function getFocal() {
            var dW = tool.imgWrap.offsetWidth || 300;
            var dH = tool.imgWrap.offsetHeight || Math.round(origH * dW / origW);
            var rectW = parseInt(tool.cropRect.style.width,  10) || 0;
            var rectH = parseInt(tool.cropRect.style.height, 10) || 0;
            var left  = parseInt(tool.cropRect.style.left,   10) || 0;
            var top   = parseInt(tool.cropRect.style.top,    10) || 0;
            return {
                x: clamp((left + rectW / 2) / dW, 0, 1),
                y: clamp((top  + rectH / 2) / dH, 0, 1)
            };
        }

        // ---- Close ----
        function closeTool() {
            tool.el.style.display = 'none';
            grid.style.display    = 'flex';
        }

        // ---- AJAX helper ----
        function postSizeFocal(data, onSuccess) {
            tool.saveBtn.disabled   = true;
            tool.clearBtn.disabled  = true;
            tool.cancelBtn.disabled = true;

            var fd = new FormData();
            fd.append('action',        'wpir_save_size_focal');
            fd.append('nonce',         wpirL10n.nonce);
            fd.append('attachment_id', attachmentId);
            fd.append('size_name',     size.name);
            Object.keys(data).forEach(function (k) { fd.append(k, data[k]); });

            fetch(ajaxurl, { method: 'POST', body: fd })
                .then(function (r) { return r.json(); })
                .then(function (resp) {
                    tool.saveBtn.disabled   = false;
                    tool.clearBtn.disabled  = false;
                    tool.cancelBtn.disabled = false;
                    if (resp.success) {
                        onSuccess(resp);
                    } else {
                        alert(resp.data || 'Error saving crop');
                    }
                })
                .catch(function () {
                    tool.saveBtn.disabled   = false;
                    tool.clearBtn.disabled  = false;
                    tool.cancelBtn.disabled = false;
                    alert('Network error');
                });
        }

        // Replace button nodes to clear previous listeners (avoids duplicates).
        ['saveBtn', 'clearBtn', 'cancelBtn'].forEach(function (key) {
            var clone = tool[key].cloneNode(true);
            tool[key].parentNode.replaceChild(clone, tool[key]);
            tool[key] = clone;
        });
        // Re-query savedMsg (still in DOM, not cloned).
        tool.savedMsg = tool.el.querySelector('.wpir-crop-saved-msg');

        tool.saveBtn.addEventListener('click', function () {
            var focal = getFocal();
            postSizeFocal(
                { focal_x: focal.x.toFixed(4), focal_y: focal.y.toFixed(4), clear: '0' },
                function () {
                    sizeFocals[size.name] = focal;
                    badge.style.display = 'inline-block';
                    tool.savedMsg.classList.add('visible');
                    setTimeout(function () { tool.savedMsg.classList.remove('visible'); }, 2000);
                    onSaveOrClear();
                    setTimeout(closeTool, 1500);
                }
            );
        });

        tool.clearBtn.addEventListener('click', function () {
            postSizeFocal({ clear: '1' }, function () {
                delete sizeFocals[size.name];
                badge.style.display = 'none';
                onSaveOrClear();
                closeTool();
            });
        });

        tool.cancelBtn.addEventListener('click', closeTool);
    }

    // -------------------------------------------------------------------------
    // Preview section — grid of per-size thumbnails + crop tool
    // -------------------------------------------------------------------------

    /**
     * Build the preview section and return an updateAll(fx, fy, mode) function.
     *
     * @param {Element} wrap           Parent element
     * @param {string}  previewUrl     Image URL for small preview thumbnails (medium)
     * @param {string}  fullUrl        Image URL for the crop tool workspace (large/full)
     * @param {number}  origW          Full-size image width
     * @param {number}  origH          Full-size image height
     * @param {number}  attachmentId
     * @param {Object}  sizeFocals     Mutable map { sizeName: {x, y} }
     * @returns {function(fx, fy, mode): void}
     */
    function buildPreviewSection(wrap, previewUrl, fullUrl, origW, origH, attachmentId, sizeFocals) {
        var sizes = (wpirL10n.imageSizes || []);
        if (!sizes.length || !origW || !origH) {
            return function () {}; // no-op
        }

        var section = document.createElement('div');
        section.className = 'wpir-preview-sizes';

        var heading = document.createElement('h4');
        heading.textContent = wpirL10n.cropPreviewsTitle || 'Crop Previews';
        section.appendChild(heading);

        // Smart-mode notice (hidden unless mode === 'smart').
        var smartNote = document.createElement('p');
        smartNote.className = 'wpir-smart-preview-note';
        smartNote.textContent = wpirL10n.smartPreviewNote || 'Smart crop — preview not available';
        smartNote.style.display = 'none';
        section.appendChild(smartNote);

        // Preview grid.
        var grid = document.createElement('div');
        grid.className = 'wpir-preview-grid';
        section.appendChild(grid);

        // Crop tool (single shared instance, shown/hidden per size).
        var tool = buildCropTool(section);

        wrap.appendChild(section);

        // Track last known global focal for calling onSaveOrClear.
        var lastFx = 0.5, lastFy = 0.5, lastMode = 'center';

        // Per-size updater functions: { name, fn(fx, fy), badge }
        var sizeUpdaters = [];

        sizes.forEach(function (size) {
            var dims = previewDims(size.width, size.height);

            var item = document.createElement('div');
            item.className = 'wpir-preview-item';

            // Overflow-hidden container for the cropped preview.
            var cropContainer = document.createElement('div');
            cropContainer.className = 'wpir-preview-crop-container';
            cropContainer.style.width  = dims.w + 'px';
            cropContainer.style.height = dims.h + 'px';

            var previewImg = document.createElement('img');
            previewImg.src = previewUrl;
            previewImg.alt = '';
            cropContainer.appendChild(previewImg);

            var labelEl = document.createElement('div');
            labelEl.className = 'wpir-size-label';
            labelEl.textContent = size.label + ' – ' + size.width + '×' + size.height;

            // "Custom" badge — shown when a per-size override exists.
            var badge = document.createElement('span');
            badge.className = 'wpir-override-badge';
            badge.textContent = wpirL10n.customBadge || 'Custom';
            badge.style.display = sizeFocals[size.name] ? 'inline-block' : 'none';

            // "Adjust Crop" button — opens the crop tool for this size.
            var adjustBtn = document.createElement('button');
            adjustBtn.type = 'button';
            adjustBtn.className = 'button-link wpir-crop-adjust-btn';
            adjustBtn.textContent = wpirL10n.adjustCrop || 'Adjust Crop';

            item.appendChild(cropContainer);
            item.appendChild(labelEl);
            item.appendChild(badge);
            item.appendChild(adjustBtn);
            grid.appendChild(item);

            // Register preview updater.
            sizeUpdaters.push({
                name:  size.name,
                badge: badge,
                fn:    function (fx, fy) {
                    var rect = calcCropRect(origW, origH, size.width, size.height, fx, fy);
                    applyPreviewCrop(previewImg, origW, origH, rect, dims.w, dims.h);
                }
            });

            // Wire up the Adjust Crop button.
            adjustBtn.addEventListener('click', function () {
                openCropTool({
                    size:          size,
                    attachmentId:  attachmentId,
                    imageUrl:      fullUrl,
                    origW:         origW,
                    origH:         origH,
                    sizeFocals:    sizeFocals,
                    tool:          tool,
                    grid:          grid,
                    smartNote:     smartNote,
                    badge:         badge,
                    onSaveOrClear: function () {
                        updateAll(lastFx, lastFy, lastMode);
                    }
                });
            });
        });

        /**
         * Update all size previews.
         * Uses per-size focal when available; falls back to global focal.
         */
        function updateAll(fx, fy, mode) {
            lastFx   = fx;
            lastFy   = fy;
            lastMode = mode;

            var isSmart = (mode === 'smart');
            smartNote.style.display = isSmart ? 'block' : 'none';
            grid.style.display      = isSmart ? 'none'  : 'flex';

            if (isSmart) return;

            sizeUpdaters.forEach(function (u) {
                var sf  = sizeFocals[u.name];
                var sfx = sf ? sf.x : (mode === 'center' ? 0.5 : fx);
                var sfy = sf ? sf.y : (mode === 'center' ? 0.5 : fy);
                u.fn(sfx, sfy);
            });
        }

        return updateAll;
    }

    // -------------------------------------------------------------------------
    // Main focal-point editor
    // -------------------------------------------------------------------------

    /**
     * Initialize the full focal-point / crop editor on an attachment detail panel.
     *
     * @param {Element}       container
     * @param {number}        attachmentId
     * @param {string}        imageUrl        Medium-size URL (for focal picker)
     * @param {number|string} currentFocalX
     * @param {number|string} currentFocalY
     * @param {string}        currentCropMode
     * @param {number}        origW           Full-size original width
     * @param {number}        origH           Full-size original height
     * @param {Object}        sizeFocals      Per-size focal overrides { name: {x,y} }
     * @param {string}        fullImageUrl    Large/full URL (for crop tool workspace)
     */
    WPIR.initFocalPointEditor = function (
            container, attachmentId, imageUrl,
            currentFocalX, currentFocalY, currentCropMode,
            origW, origH, sizeFocals, fullImageUrl) {

        var focalX   = parseFloat(currentFocalX) || 0.5;
        var focalY   = parseFloat(currentFocalY) || 0.5;
        var cropMode = currentCropMode || 'center';

        sizeFocals   = (sizeFocals && !Array.isArray(sizeFocals)) ? sizeFocals : {};
        fullImageUrl = fullImageUrl || imageUrl;
        origW = parseInt(origW, 10) || 0;
        origH = parseInt(origH, 10) || 0;

        // ---- Wrapper ----
        var wrap = document.createElement('div');
        wrap.className = 'wpir-focal-point-wrap';

        // ---- Crop mode selector ----
        var modeWrap = document.createElement('div');
        modeWrap.className = 'wpir-crop-mode-wrap';

        var modeLabel = document.createElement('label');
        modeLabel.textContent = wpirL10n.cropModeLabel || 'Crop Mode';
        modeWrap.appendChild(modeLabel);

        var modeOptions = document.createElement('div');
        modeOptions.className = 'wpir-crop-mode-options';

        [
            { value: 'center', label: wpirL10n.center     || 'Center' },
            { value: 'smart',  label: wpirL10n.smart      || 'Smart'  },
            { value: 'focal',  label: wpirL10n.focalPoint || 'Focal Point' }
        ].forEach(function (mode) {
            var btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'button' + (cropMode === mode.value ? ' active' : '');
            btn.textContent = mode.label;
            btn.dataset.mode = mode.value;
            btn.addEventListener('click', function () {
                cropMode = mode.value;
                modeOptions.querySelectorAll('.button').forEach(function (b) {
                    b.classList.remove('active');
                });
                btn.classList.add('active');
                focalContainer.style.display = (cropMode === 'focal') ? 'block' : 'none';
                updateAllPreviews(focalX, focalY, cropMode);
            });
            modeOptions.appendChild(btn);
        });

        modeWrap.appendChild(modeOptions);
        wrap.appendChild(modeWrap);

        // ---- Focal point picker ----
        var focalContainer = document.createElement('div');
        focalContainer.style.display = (cropMode === 'focal') ? 'block' : 'none';

        var focalLabel = document.createElement('label');
        focalLabel.textContent = wpirL10n.clickToSetFocal || 'Click on the image to set the focal point:';
        focalContainer.appendChild(focalLabel);

        var imgContainer = document.createElement('div');
        imgContainer.className = 'wpir-focal-point-container';

        var pickerImg = document.createElement('img');
        pickerImg.src = imageUrl;
        pickerImg.alt = '';
        imgContainer.appendChild(pickerImg);

        var marker = document.createElement('div');
        marker.className = 'wpir-focal-point-marker';
        imgContainer.appendChild(marker);

        focalContainer.appendChild(imgContainer);

        var coordsWrap = document.createElement('div');
        coordsWrap.className = 'wpir-focal-coords';
        var coordsText = document.createElement('span');
        coordsWrap.appendChild(coordsText);
        focalContainer.appendChild(coordsWrap);

        wrap.appendChild(focalContainer);

        // ---- Save / Regenerate buttons ----
        var actionsWrap = document.createElement('div');
        actionsWrap.className = 'wpir-focal-actions';

        var saveBtn = document.createElement('button');
        saveBtn.type = 'button';
        saveBtn.className = 'button button-primary';
        saveBtn.textContent = wpirL10n.save || 'Save';
        actionsWrap.appendChild(saveBtn);

        var regenBtn = document.createElement('button');
        regenBtn.type = 'button';
        regenBtn.className = 'button';
        regenBtn.textContent = wpirL10n.saveAndRegenerate || 'Save & Regenerate';
        actionsWrap.appendChild(regenBtn);

        var savedMsg = document.createElement('span');
        savedMsg.className = 'wpir-focal-saved';
        savedMsg.textContent = wpirL10n.saved || 'Saved!';
        actionsWrap.appendChild(savedMsg);

        wrap.appendChild(actionsWrap);

        container.appendChild(wrap);

        // ---- Preview section (includes crop tool) ----
        var updateAllPreviews = buildPreviewSection(
            wrap, imageUrl, fullImageUrl, origW, origH, attachmentId, sizeFocals
        );

        // ---- Marker positioning ----
        function updateMarker() {
            marker.style.left = (focalX * 100) + '%';
            marker.style.top  = (focalY * 100) + '%';
            coordsText.textContent = 'X: ' + focalX.toFixed(2) + '  Y: ' + focalY.toFixed(2);
        }

        pickerImg.addEventListener('load', function () {
            updateMarker();
            updateAllPreviews(focalX, focalY, cropMode);
        });
        if (pickerImg.complete) {
            updateMarker();
            updateAllPreviews(focalX, focalY, cropMode);
        }

        // Click to set focal point.
        imgContainer.addEventListener('click', function (e) {
            var rect = imgContainer.getBoundingClientRect();
            focalX = clamp((e.clientX - rect.left)  / rect.width,  0, 1);
            focalY = clamp((e.clientY - rect.top)   / rect.height, 0, 1);
            updateMarker();
            updateAllPreviews(focalX, focalY, cropMode);
        });

        // ---- Save global focal point ----
        function saveFocalPoint(regenerate) {
            saveBtn.disabled  = true;
            regenBtn.disabled = true;

            var fd = new FormData();
            fd.append('action',        'wpir_save_focal_point');
            fd.append('nonce',         wpirL10n.nonce);
            fd.append('attachment_id', attachmentId);
            fd.append('crop_mode',     cropMode);
            fd.append('focal_x',       focalX.toFixed(4));
            fd.append('focal_y',       focalY.toFixed(4));
            fd.append('regenerate',    regenerate ? '1' : '0');

            fetch(ajaxurl, { method: 'POST', body: fd })
                .then(function (r) { return r.json(); })
                .then(function (resp) {
                    saveBtn.disabled  = false;
                    regenBtn.disabled = false;
                    if (resp.success) {
                        savedMsg.classList.add('visible');
                        setTimeout(function () { savedMsg.classList.remove('visible'); }, 2000);
                    } else {
                        alert(resp.data || 'Error saving focal point');
                    }
                })
                .catch(function () {
                    saveBtn.disabled  = false;
                    regenBtn.disabled = false;
                    alert('Network error');
                });
        }

        saveBtn.addEventListener('click',  function () { saveFocalPoint(false); });
        regenBtn.addEventListener('click', function () { saveFocalPoint(true);  });
    };

    // -------------------------------------------------------------------------
    // Hook into WordPress media library attachment details.
    // -------------------------------------------------------------------------

    if (typeof wp !== 'undefined' && wp.media) {
        var origTwoColumn = wp.media.view.Attachment.Details.TwoColumn;
        if (origTwoColumn) {
            wp.media.view.Attachment.Details.TwoColumn = origTwoColumn.extend({
                render: function () {
                    origTwoColumn.prototype.render.apply(this, arguments);

                    var model = this.model;
                    if (model.get('type') !== 'image') return this;

                    var detailsEl = this.el.querySelector('.attachment-info .details');
                    if (!detailsEl) return this;

                    // Remove any previously injected editor (re-renders).
                    var existing = detailsEl.querySelector('.wpir-focal-point-wrap');
                    if (existing) existing.remove();

                    var meta       = model.get('wpir_meta') || {};
                    var sizeFocals = (meta.size_focals && !Array.isArray(meta.size_focals))
                        ? meta.size_focals
                        : {};

                    var sizes        = model.get('sizes') || {};
                    var imageUrl     = model.get('url');   // fallback: full URL
                    var fullImageUrl = model.get('url');

                    // Medium for focal picker; large for crop tool workspace.
                    if (sizes.medium) imageUrl     = sizes.medium.url;
                    if (sizes.large)  fullImageUrl = sizes.large.url;

                    WPIR.initFocalPointEditor(
                        detailsEl,
                        model.get('id'),
                        imageUrl,
                        meta.focal_x   || '0.5',
                        meta.focal_y   || '0.5',
                        meta.crop_mode || 'center',
                        model.get('width'),
                        model.get('height'),
                        sizeFocals,
                        fullImageUrl
                    );

                    return this;
                }
            });
        }
    }

    window.WPIR = WPIR;
})();
