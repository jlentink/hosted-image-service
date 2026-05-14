package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/jlentink/yaglogger"

	"github.com/jlentink/image-service/internal/config"
	img "github.com/jlentink/image-service/internal/image"
	"github.com/jlentink/image-service/internal/middleware"
)

// ResizeHandler handles image resize requests.
type ResizeHandler struct {
	processor      *img.Processor
	cfg            *config.Config
	maxUploadBytes int64
	fetchCfg       *img.FetchConfig
}

// NewResizeHandler creates a new ResizeHandler.
func NewResizeHandler(processor *img.Processor, cfg *config.Config, maxUploadBytes int64, fetchCfg *img.FetchConfig) *ResizeHandler {
	return &ResizeHandler{
		processor:      processor,
		cfg:            cfg,
		maxUploadBytes: maxUploadBytes,
		fetchCfg:       fetchCfg,
	}
}

// Handle processes an image resize request.
func (h *ResizeHandler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse multipart form with size limit.
	if err := r.ParseMultipartForm(h.maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: %s", err.Error())
		return
	}

	// Parse and validate parameters.
	width, err := requiredIntParam(r, "width")
	if err != nil {
		writeError(w, http.StatusBadRequest, "width: %s", err.Error())
		return
	}

	height, err := requiredIntParam(r, "height")
	if err != nil {
		writeError(w, http.StatusBadRequest, "height: %s", err.Error())
		return
	}

	cropStr := r.FormValue("crop")
	if cropStr == "" {
		cropStr = "center"
	}
	cropSpec, err := img.ParseCropSpec(cropStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "crop: %s", err.Error())
		return
	}

	formatStr := r.FormValue("format")
	if formatStr == "" {
		formatStr = "jpeg"
	}
	format, err := img.ParseFormat(formatStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "format: %s", err.Error())
		return
	}

	// Validate format is in allowed list.
	if !h.isFormatAllowed(format) {
		writeError(w, http.StatusBadRequest, "format %q is not enabled in server configuration", format)
		return
	}

	quality := 0
	if qStr := r.FormValue("quality"); qStr != "" {
		q, err := strconv.Atoi(qStr)
		if err != nil || q < 1 || q > 100 {
			writeError(w, http.StatusBadRequest, "quality must be an integer between 1 and 100")
			return
		}
		quality = q
	}

	// Get image data from file upload or URL.
	imageData, err := h.getImageData(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%s", err.Error())
		return
	}

	// Process the image.
	processStart := time.Now()
	result, err := h.processor.Process(&img.ProcessRequest{
		ImageData: imageData,
		Width:     width,
		Height:    height,
		Crop:      cropSpec.Mode,
		FocalX:    cropSpec.FocalX,
		FocalY:    cropSpec.FocalY,
		CropX:     cropSpec.RectX,
		CropY:     cropSpec.RectY,
		CropW:     cropSpec.RectW,
		CropH:     cropSpec.RectH,
		Format:    format,
		Quality:   quality,
	})
	if err != nil {
		middleware.RecordError()
		log.Error("image processing failed: %s", err.Error())
		writeError(w, http.StatusInternalServerError, "image processing failed: %s", err.Error())
		return
	}

	processDuration := time.Since(processStart)
	middleware.RecordProcessingDuration(processDuration)
	middleware.RecordOutputBytes(len(result.Data))
	middleware.RecordFormat(string(format))

	elapsed := time.Since(start)
	log.Debug("Processed image: %dx%d %s in %s (processing: %s)", width, height, format, elapsed, processDuration)

	// Write response.
	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(result.Data)))
	w.Header().Set("X-Processing-Time", elapsed.String())
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.WriteHeader(http.StatusOK)
	w.Write(result.Data)
}

func (h *ResizeHandler) getImageData(r *http.Request) ([]byte, error) {
	// Try file upload first.
	file, _, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, h.maxUploadBytes+1))
		if err != nil {
			return nil, fmt.Errorf("reading uploaded file: %w", err)
		}
		if int64(len(data)) > h.maxUploadBytes {
			return nil, fmt.Errorf("uploaded file exceeds maximum size")
		}
		return data, nil
	}

	// Try URL parameter.
	rawURL := r.FormValue("url")
	if rawURL != "" {
		data, err := img.FetchImage(rawURL, h.fetchCfg)
		if err != nil {
			return nil, fmt.Errorf("fetching image from URL: %w", err)
		}
		return data, nil
	}

	return nil, fmt.Errorf("provide either an 'image' file upload or a 'url' parameter")
}

func (h *ResizeHandler) isFormatAllowed(format img.OutputFormat) bool {
	for _, f := range h.cfg.Image.AllowedFormats {
		if img.OutputFormat(f) == format {
			return true
		}
	}
	return false
}

func requiredIntParam(r *http.Request, name string) (int, error) {
	s := r.FormValue(name)
	if s == "" {
		return 0, fmt.Errorf("required parameter missing")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("must be an integer")
	}
	if v <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return v, nil
}

func writeError(w http.ResponseWriter, status int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Debug("HTTP %d: %s", status, msg)
	writeJSON(w, status, map[string]string{"error": msg})
}
