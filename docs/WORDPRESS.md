# WordPress Plugin Guide

The `wp-image-resizer` plugin offloads WordPress's thumbnail generation to a running [image-service](../README.md). Once configured, WordPress will call out to the service whenever it would normally invoke GD or Imagick.

## Prerequisites

- A running image-service instance reachable from your WordPress server (see the [Deployment Guide](DEPLOYMENT.md))
- The JWT shared secret from `auth.jwt_secret` in your image-service config
- WordPress 5.8+ and PHP 7.4+

## Installation

1. Copy the `wordpress-plugin/wp-image-resizer/` directory into your site's `wp-content/plugins/` folder, or upload the release zip via **Plugins → Add New → Upload Plugin**.
2. Activate **WP Image Resizer** under **Plugins**.
3. Navigate to **Settings → Image Resizer**.

## Configuration

The settings page exposes:

| Setting | Description |
|---------|-------------|
| **Service URL** | Full URL where image-service is reachable, e.g. `https://images.example.com` |
| **JWT Shared Secret** | Must match `auth.jwt_secret` in the service config. Used to sign HMAC-SHA256 tokens (no PHP `jwt` library required). |
| **Enabled** | Master switch — when disabled WordPress falls back to GD/Imagick |
| **Default Format** | Output format used for resized thumbnails (`jpeg`, `png`, `webp`, `avif`) |
| **Quality (per format)** | 1–100 for JPEG/WebP/AVIF, 0–9 for PNG compression |
| **Test Connection** | Sends a signed request to the service and reports the result |

After saving, use **Test Connection** to confirm:

- The Service URL is reachable
- The shared secret matches
- The IP whitelist (if enabled on the service) permits this host

## How it Works

The plugin registers a custom `WP_Image_Editor` subclass via the `wp_image_editors` filter, placing it ahead of `WP_Image_Editor_GD` and `WP_Image_Editor_Imagick`. Whenever WordPress generates a thumbnail size (during upload, regenerate, theme image size registration, etc.), the original image is uploaded to image-service, the resized result is fetched back, and the file is written to the uploads directory.

JWT tokens are generated per request with a short expiry (typically 5 minutes) using pure-PHP HMAC-SHA256 — there is no external Composer dependency.

## Regenerating Thumbnails

There are two ways to regenerate existing thumbnails through the service:

### Bulk Regenerate Page

**Tools → Regenerate Thumbnails** (or **Media → Regenerate**). The page processes attachments in batches via AJAX with a progress bar so large libraries don't time out. Each attachment is regenerated through the image-service.

### Bulk Action in Media Library

1. Open **Media → Library** in list view
2. Select one or more attachments
3. From **Bulk actions** choose **Regenerate via image-service** and click **Apply**

An admin notice reports how many attachments were regenerated.

## Focal Point and Crop Editor

For finer control over how each image is cropped at different sizes:

1. Edit an attachment from the Media Library
2. The plugin adds a **Focal Point / Crop** panel showing the original image
3. Choose a crop mode:
   - **Center** — middle crop (default)
   - **Smart** — libvips attention-based crop
   - **Focal point** — click on the image to set the point of interest
4. Click **Save Focal Point** or **Save & Regenerate** to apply

The chosen mode and focal point are stored as attachment post meta and used on every subsequent resize.

## Troubleshooting

### Connection test fails

- Confirm the **Service URL** is correct (no trailing slash needed, both work)
- Verify the secret matches `auth.jwt_secret` on the service exactly — leading/trailing whitespace counts
- If `whitelist.enabled = true` on the service, ensure the WordPress server's outbound IP is in `whitelist.ips`
- Check the service logs (`/var/log/...` or `docker logs`) for the rejected request

### Thumbnails fall back to GD/Imagick

The plugin returns control to WordPress when:

- The plugin is disabled in settings
- The service responds with a non-2xx status (e.g. 401 if secret is wrong, 403 if IP is blocked)
- The service is unreachable

Enable debug logging in WordPress (`define( 'WP_DEBUG', true );` and `WP_DEBUG_LOG`) — the plugin writes failure reasons there.

### "Format not enabled" errors

The service's `image.allowed_formats` must include the format the plugin requests. If you set the plugin to output AVIF, confirm `"avif"` is in `allowed_formats` on the service.

### Large images time out

- Increase `server.read_timeout` / `server.write_timeout` on the service
- Increase `server.max_upload_size` on the service if uploads are rejected with 413
- Increase your reverse proxy timeouts (`proxy_read_timeout` for nginx)
- Increase PHP's `max_execution_time` and `upload_max_filesize`

## Uninstall

Deactivating the plugin restores standard WordPress image processing. Existing regenerated thumbnails remain in place — they are normal files on disk and indistinguishable from GD/Imagick output. Removing the plugin also deletes the stored settings; focal-point post meta on attachments persists unless explicitly cleaned up.
