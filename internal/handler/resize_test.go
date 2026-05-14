package handler

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"

	"github.com/jlentink/image-service/internal/config"
	img "github.com/jlentink/image-service/internal/image"
)

func TestMain(m *testing.M) {
	vips.Startup(nil)
	code := m.Run()
	vips.Shutdown()
	os.Exit(code)
}

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			MaxUploadSize: "50MB",
		},
		Image: config.ImageConfig{
			MaxWidth:           4096,
			MaxHeight:          4096,
			DefaultQualityJPEG: 85,
			DefaultQualityWebP: 80,
			DefaultQualityAVIF: 60,
			DefaultQualityPNG:  6,
			AllowedFormats:     []string{"jpeg", "png", "webp", "avif"},
			MaxFetchSize:       "20MB",
		},
	}
}

func testProcessor() *img.Processor {
	cfg := testConfig()
	return img.NewProcessor(
		cfg.Image.MaxWidth, cfg.Image.MaxHeight,
		cfg.Image.DefaultQualityJPEG, cfg.Image.DefaultQualityWebP,
		cfg.Image.DefaultQualityAVIF, cfg.Image.DefaultQualityPNG,
	)
}

func testHandler() *ResizeHandler {
	cfg := testConfig()
	return NewResizeHandler(testProcessor(), cfg, 50*1024*1024, img.DefaultFetchConfig())
}

func createTestJPEG(w, h int) []byte {
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rgba.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func createMultipartRequest(imageData []byte, fields map[string]string) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if imageData != nil {
		part, err := writer.CreateFormFile("image", "test.jpg")
		if err != nil {
			return nil, err
		}
		part.Write(imageData)
	}

	for k, v := range fields {
		writer.WriteField(k, v)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/resize", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestResizeHandler_Success(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "200",
		"height": "150",
		"crop":   "center",
		"format": "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		body := w.Body.String()
		t.Fatalf("expected 200, got %d: %s", w.Code, body)
	}

	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", ct)
	}
	if pt := w.Header().Get("X-Processing-Time"); pt == "" {
		t.Error("missing X-Processing-Time header")
	}
	if w.Body.Len() == 0 {
		t.Error("response body is empty")
	}

	// Verify output dimensions.
	result, err := vips.NewImageFromBuffer(w.Body.Bytes())
	if err != nil {
		t.Fatalf("failed to load result: %v", err)
	}
	defer result.Close()
	if result.Width() != 200 || result.PageHeight() != 150 {
		t.Errorf("expected 200x150, got %dx%d", result.Width(), result.PageHeight())
	}
}

func TestResizeHandler_WebPOutput(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"width":   "100",
		"height":  "100",
		"crop":    "smart",
		"format":  "webp",
		"quality": "75",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/webp" {
		t.Errorf("expected Content-Type image/webp, got %s", ct)
	}
}

func TestResizeHandler_FocalPointCrop(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "150",
		"height": "150",
		"crop":   "0.3,0.7",
		"format": "png",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", ct)
	}
}

func TestResizeHandler_DefaultParams(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	// Omit crop and format — should default to center/jpeg.
	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "200",
		"height": "150",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", ct)
	}
}

func TestResizeHandler_MissingWidth(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"height": "150",
		"format": "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestResizeHandler_MissingImage(t *testing.T) {
	h := testHandler()

	req, err := createMultipartRequest(nil, map[string]string{
		"width":  "200",
		"height": "150",
		"format": "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestResizeHandler_InvalidFormat(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "200",
		"height": "150",
		"format": "gif",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestResizeHandler_InvalidQuality(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"width":   "200",
		"height":  "150",
		"format":  "jpeg",
		"quality": "999",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestResizeHandler_InvalidCrop(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "200",
		"height": "150",
		"format": "jpeg",
		"crop":   "invalid",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestResizeHandler_URLFetch(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	// Serve the image via a test HTTP server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(imgData)
	}))
	defer srv.Close()

	// Override fetch config to allow private addrs for test server.
	h.fetchCfg = &img.FetchConfig{
		MaxSize:           20 * 1024 * 1024,
		Timeout:           10e9,
		AllowPrivateAddrs: true,
	}

	req, err := createMultipartRequest(nil, map[string]string{
		"url":    srv.URL + "/test.jpg",
		"width":  "100",
		"height": "100",
		"format": "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		t.Fatalf("expected 200, got %d: %v", w.Code, errResp)
	}
}

// -- Edge cases ---------------------------------------------------------------

func TestResizeHandler_MissingHeight(t *testing.T) {
	h := testHandler()
	imgData := createTestJPEG(400, 300)

	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "200",
		"format": "jpeg",
		// height intentionally omitted
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing height, got %d", w.Code)
	}
}

func TestResizeHandler_OversizedUpload(t *testing.T) {
	cfg := testConfig()
	// Limit to 1 byte so any real image exceeds the maximum.
	h := NewResizeHandler(testProcessor(), cfg, 1, img.DefaultFetchConfig())

	imgData := createTestJPEG(400, 300) // several KB
	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "200",
		"height": "150",
		"format": "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized upload, got %d", w.Code)
	}
}

func TestResizeHandler_DisallowedFormat(t *testing.T) {
	// Build a handler that only allows jpeg — requesting webp must be rejected.
	cfg := testConfig()
	cfg.Image.AllowedFormats = []string{"jpeg"}
	h := NewResizeHandler(testProcessor(), cfg, 50*1024*1024, img.DefaultFetchConfig())

	imgData := createTestJPEG(400, 300)
	req, err := createMultipartRequest(imgData, map[string]string{
		"width":  "200",
		"height": "150",
		"format": "webp", // valid format string but not in AllowedFormats
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for disallowed format, got %d", w.Code)
	}
}

func TestResizeHandler_URLFetch_MalformedURL(t *testing.T) {
	h := testHandler()

	req, err := createMultipartRequest(nil, map[string]string{
		"url":    "://not-a-valid-url",
		"width":  "100",
		"height": "100",
		"format": "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed URL, got %d", w.Code)
	}
}

func TestResizeHandler_URLFetch_BlockedPrivateAddr(t *testing.T) {
	h := testHandler()
	// The default FetchConfig has AllowPrivateAddrs=false, so internal test
	// server addresses (127.0.0.1) must be blocked.
	imgData := createTestJPEG(100, 100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(imgData)
	}))
	defer srv.Close()

	req, err := createMultipartRequest(nil, map[string]string{
		"url":    srv.URL + "/test.jpg",
		"width":  "50",
		"height": "50",
		"format": "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for blocked private address, got %d", w.Code)
	}
}
