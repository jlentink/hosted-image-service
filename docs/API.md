# Image Service API Reference

## Authentication

All requests to `/resize` require a JWT Bearer token in the `Authorization` header.

```
Authorization: Bearer <jwt-token>
```

Tokens are generated using HMAC-SHA256 with the shared secret configured in `auth.jwt_secret`. The WordPress plugin generates these tokens automatically.

### Token structure

- **Algorithm:** HS256
- **Issuer:** `image-service`
- **Expiry:** Configurable via `auth.jwt_expiry` (default: 5 minutes)

---

## Endpoints

### POST /resize

Resize and crop an image.

The request body is limited by `server.max_upload_size` in the configuration (default `50MB`). When fetching from `url`, the response body is limited by `image.max_fetch_size` (default `20MB`).

**Request:** `multipart/form-data`

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | file | * | - | Image file upload |
| `url` | string | * | - | URL to fetch image from |
| `width` | int | yes | - | Target width in pixels |
| `height` | int | yes | - | Target height in pixels |
| `crop` | string | no | `center` | Crop mode (see below) |
| `format` | string | no | `jpeg` | Output format: `jpeg`, `png`, `webp`, `avif` |
| `quality` | int | no | per-format default | Quality 1-100 (JPEG/WebP/AVIF) or compression 0-9 (PNG) |

*Either `image` or `url` must be provided.

**Crop modes:**

| Value | Description |
|-------|-------------|
| `center` | Center crop (default) |
| `smart` | Attention-based smart crop — libvips detects the most interesting area |
| `0.5,0.3` | Focal point crop — x,y coordinates from 0.0 to 1.0 (top-left is 0,0) |

**Success response:**

```
HTTP/1.1 200 OK
Content-Type: image/jpeg
Content-Length: 12345
X-Processing-Time: 45.2ms
Cache-Control: public, max-age=31536000

<binary image data>
```

**Error responses:**

| Status | Meaning |
|--------|---------|
| 400 | Bad request (missing/invalid parameters) |
| 401 | Unauthorized (missing/invalid/expired JWT) |
| 403 | Forbidden (IP not in whitelist) |
| 413 | Request entity too large |
| 500 | Internal server error (processing failure) |

Error body:
```json
{"error": "description of the error"}
```

**Example with curl:**

```bash
# Generate a JWT token using the built-in CLI (recommended)
TOKEN=$(./image-service token --secret your-jwt-secret --expiry 5m)

# Alternatively, generate it with Python + PyJWT
# TOKEN=$(python3 -c "
# import jwt, time
# print(jwt.encode({'iss':'image-service','iat':time.time(),'exp':time.time()+300}, 'your-secret', algorithm='HS256'))
# ")

# Resize via file upload
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -F "image=@photo.jpg" \
  -F "width=800" \
  -F "height=600" \
  -F "crop=smart" \
  -F "format=webp" \
  -F "quality=80" \
  -o output.webp \
  http://localhost:8080/resize

# Resize via URL
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -F "url=https://example.com/photo.jpg" \
  -F "width=400" \
  -F "height=300" \
  -F "format=avif" \
  -o output.avif \
  http://localhost:8080/resize
```

---

### GET /health

Liveness probe. Returns 200 if the process is running.

```json
{"status": "ok"}
```

### GET /ready

Readiness probe. Returns 200 when libvips is initialized and the server is ready.

```json
{"status": "ready"}
```

Returns 503 if not yet ready:
```json
{"status": "not ready"}
```

### GET /metrics

Prometheus metrics endpoint (configurable path via `metrics.path`).

**Available metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `image_service_http_requests_total` | counter | method, path, status | Total HTTP requests |
| `image_service_http_request_duration_seconds` | histogram | method, path | Request duration |
| `image_service_processing_duration_seconds` | histogram | - | Image processing time only |
| `image_service_output_bytes` | histogram | - | Output image file size |
| `image_service_requests_by_format_total` | counter | format | Requests by output format |
| `image_service_errors_total` | counter | - | Processing errors |
