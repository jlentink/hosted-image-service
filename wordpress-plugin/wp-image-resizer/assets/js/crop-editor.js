/**
 * WP Image Resizer — Focal Point & Crop Editor
 *
 * Adds a visual focal point picker to the media library attachment detail view.
 * Users click on the image to set the focal point used for smart cropping.
 */
(function () {
    'use strict';

    var WPIR = window.WPIR || {};

    /**
     * Initialize the focal point editor on an attachment detail panel.
     */
    WPIR.initFocalPointEditor = function (container, attachmentId, imageUrl, currentFocalX, currentFocalY, currentCropMode) {
        var focalX = parseFloat(currentFocalX) || 0.5;
        var focalY = parseFloat(currentFocalY) || 0.5;
        var cropMode = currentCropMode || 'center';

        // Build DOM.
        var wrap = document.createElement('div');
        wrap.className = 'wpir-focal-point-wrap';

        // Crop mode selector.
        var modeWrap = document.createElement('div');
        modeWrap.className = 'wpir-crop-mode-wrap';
        modeWrap.innerHTML = '<label>' + wpirL10n.cropModeLabel + '</label>';

        var modeOptions = document.createElement('div');
        modeOptions.className = 'wpir-crop-mode-options';

        var modes = [
            { value: 'center', label: wpirL10n.center },
            { value: 'smart', label: wpirL10n.smart },
            { value: 'focal', label: wpirL10n.focalPoint }
        ];

        modes.forEach(function (mode) {
            var btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'button' + (cropMode === mode.value ? ' active' : '');
            btn.textContent = mode.label;
            btn.dataset.mode = mode.value;
            btn.addEventListener('click', function () {
                cropMode = mode.value;
                modeOptions.querySelectorAll('.button').forEach(function (b) { b.classList.remove('active'); });
                btn.classList.add('active');
                focalContainer.style.display = (cropMode === 'focal') ? 'block' : 'none';
            });
            modeOptions.appendChild(btn);
        });
        modeWrap.appendChild(modeOptions);
        wrap.appendChild(modeWrap);

        // Focal point picker.
        var focalLabel = document.createElement('label');
        focalLabel.textContent = wpirL10n.clickToSetFocal;

        var focalContainer = document.createElement('div');
        focalContainer.style.display = (cropMode === 'focal') ? 'block' : 'none';
        focalContainer.appendChild(focalLabel);

        var imgContainer = document.createElement('div');
        imgContainer.className = 'wpir-focal-point-container';

        var img = document.createElement('img');
        img.src = imageUrl;
        img.alt = '';
        imgContainer.appendChild(img);

        var marker = document.createElement('div');
        marker.className = 'wpir-focal-point-marker';
        imgContainer.appendChild(marker);

        focalContainer.appendChild(imgContainer);

        // Coordinates display.
        var coordsWrap = document.createElement('div');
        coordsWrap.className = 'wpir-focal-coords';
        var coordsText = document.createElement('span');
        coordsText.textContent = 'X: ' + focalX.toFixed(2) + '  Y: ' + focalY.toFixed(2);
        coordsWrap.appendChild(coordsText);
        focalContainer.appendChild(coordsWrap);

        wrap.appendChild(focalContainer);

        // Action buttons.
        var actionsWrap = document.createElement('div');
        actionsWrap.className = 'wpir-focal-actions';

        var saveBtn = document.createElement('button');
        saveBtn.type = 'button';
        saveBtn.className = 'button button-primary';
        saveBtn.textContent = wpirL10n.save;
        actionsWrap.appendChild(saveBtn);

        var regenBtn = document.createElement('button');
        regenBtn.type = 'button';
        regenBtn.className = 'button';
        regenBtn.textContent = wpirL10n.saveAndRegenerate;
        actionsWrap.appendChild(regenBtn);

        var savedMsg = document.createElement('span');
        savedMsg.className = 'wpir-focal-saved';
        savedMsg.textContent = wpirL10n.saved;
        actionsWrap.appendChild(savedMsg);

        wrap.appendChild(actionsWrap);

        container.appendChild(wrap);

        // Position marker after image loads.
        function updateMarker() {
            marker.style.left = (focalX * 100) + '%';
            marker.style.top = (focalY * 100) + '%';
            coordsText.textContent = 'X: ' + focalX.toFixed(2) + '  Y: ' + focalY.toFixed(2);
        }

        img.addEventListener('load', updateMarker);
        if (img.complete) updateMarker();

        // Click to set focal point.
        imgContainer.addEventListener('click', function (e) {
            var rect = imgContainer.getBoundingClientRect();
            focalX = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
            focalY = Math.max(0, Math.min(1, (e.clientY - rect.top) / rect.height));
            updateMarker();
        });

        // Save focal point.
        function saveFocalPoint(regenerate) {
            saveBtn.disabled = true;
            regenBtn.disabled = true;

            var data = new FormData();
            data.append('action', 'wpir_save_focal_point');
            data.append('nonce', wpirL10n.nonce);
            data.append('attachment_id', attachmentId);
            data.append('crop_mode', cropMode);
            data.append('focal_x', focalX.toFixed(4));
            data.append('focal_y', focalY.toFixed(4));
            data.append('regenerate', regenerate ? '1' : '0');

            fetch(ajaxurl, { method: 'POST', body: data })
                .then(function (r) { return r.json(); })
                .then(function (resp) {
                    saveBtn.disabled = false;
                    regenBtn.disabled = false;
                    if (resp.success) {
                        savedMsg.classList.add('visible');
                        setTimeout(function () { savedMsg.classList.remove('visible'); }, 2000);
                    } else {
                        alert(resp.data || 'Error saving focal point');
                    }
                })
                .catch(function () {
                    saveBtn.disabled = false;
                    regenBtn.disabled = false;
                    alert('Network error');
                });
        }

        saveBtn.addEventListener('click', function () { saveFocalPoint(false); });
        regenBtn.addEventListener('click', function () { saveFocalPoint(true); });
    };

    /**
     * Hook into WordPress media library attachment details.
     * This runs when an attachment modal is opened.
     */
    if (typeof wp !== 'undefined' && wp.media) {
        var origAttachmentDetailsTwoColumn = wp.media.view.Attachment.Details.TwoColumn;
        if (origAttachmentDetailsTwoColumn) {
            wp.media.view.Attachment.Details.TwoColumn = origAttachmentDetailsTwoColumn.extend({
                render: function () {
                    origAttachmentDetailsTwoColumn.prototype.render.apply(this, arguments);

                    var model = this.model;
                    var type = model.get('type');
                    if (type !== 'image') return this;

                    // Find or create a container for our editor.
                    var detailsEl = this.el.querySelector('.attachment-info .details');
                    if (!detailsEl) return this;

                    var existing = detailsEl.querySelector('.wpir-focal-point-wrap');
                    if (existing) existing.remove();

                    var meta = model.get('wpir_meta') || {};
                    var imageUrl = model.get('url');

                    // Use medium size for the picker if available.
                    var sizes = model.get('sizes');
                    if (sizes && sizes.medium) {
                        imageUrl = sizes.medium.url;
                    }

                    WPIR.initFocalPointEditor(
                        detailsEl,
                        model.get('id'),
                        imageUrl,
                        meta.focal_x || '0.5',
                        meta.focal_y || '0.5',
                        meta.crop_mode || 'center'
                    );

                    return this;
                }
            });
        }
    }

    window.WPIR = WPIR;
})();
