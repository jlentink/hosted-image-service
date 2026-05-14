package image

import (
	"fmt"
	"math"
	"strings"

	"github.com/davidbyttow/govips/v2/vips"
	log "github.com/jlentink/yaglogger"
)

// CropMode defines how images are cropped.
type CropMode int

const (
	CropCenter     CropMode = iota // Center crop
	CropSmart                      // Attention-based smart crop (libvips)
	CropFocalPoint                 // Custom focal point crop
	CropRect                       // Explicit source rectangle (CropX/Y/W/H as fractions of source)
)

// OutputFormat defines the output image format.
type OutputFormat string

const (
	FormatJPEG OutputFormat = "jpeg"
	FormatPNG  OutputFormat = "png"
	FormatWebP OutputFormat = "webp"
	FormatAVIF OutputFormat = "avif"
)

// ProcessRequest contains all parameters for image processing.
type ProcessRequest struct {
	ImageData []byte       // Raw image bytes
	Width     int          // Target width
	Height    int          // Target height
	Crop      CropMode     // Crop strategy
	FocalX    float64      // Focal point X (0.0-1.0), only used with CropFocalPoint
	FocalY    float64      // Focal point Y (0.0-1.0), only used with CropFocalPoint
	CropX     float64      // Source rect left (0.0-1.0), only used with CropRect
	CropY     float64      // Source rect top  (0.0-1.0), only used with CropRect
	CropW     float64      // Source rect width  (0.0-1.0), only used with CropRect
	CropH     float64      // Source rect height (0.0-1.0), only used with CropRect
	Format    OutputFormat // Output format
	Quality   int          // Quality/compression (0 = use default)
}

// ProcessResult contains the processed image output.
type ProcessResult struct {
	Data        []byte
	ContentType string
	Width       int
	Height      int
}

// Processor handles image processing via govips.
type Processor struct {
	defaultQualityJPEG int
	defaultQualityWebP int
	defaultQualityAVIF int
	defaultQualityPNG  int
	maxWidth           int
	maxHeight          int
}

// NewProcessor creates a new image processor.
func NewProcessor(maxWidth, maxHeight, defaultJPEG, defaultWebP, defaultAVIF, defaultPNG int) *Processor {
	return &Processor{
		defaultQualityJPEG: defaultJPEG,
		defaultQualityWebP: defaultWebP,
		defaultQualityAVIF: defaultAVIF,
		defaultQualityPNG:  defaultPNG,
		maxWidth:           maxWidth,
		maxHeight:          maxHeight,
	}
}

// Process processes an image according to the given request.
func (p *Processor) Process(req *ProcessRequest) (*ProcessResult, error) {
	if err := p.validateRequest(req); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	img, err := vips.NewImageFromBuffer(req.ImageData)
	if err != nil {
		return nil, fmt.Errorf("loading image: %w", err)
	}
	defer img.Close()

	if err := img.AutoRotate(); err != nil {
		log.Debug("AutoRotate failed (non-fatal): %s", err.Error())
	}

	if err := p.resizeAndCrop(img, req); err != nil {
		return nil, fmt.Errorf("resize/crop: %w", err)
	}

	data, contentType, err := p.export(img, req.Format, req.Quality)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}

	return &ProcessResult{
		Data:        data,
		ContentType: contentType,
		Width:       img.Width(),
		Height:      img.PageHeight(),
	}, nil
}

func (p *Processor) validateRequest(req *ProcessRequest) error {
	if len(req.ImageData) == 0 {
		return fmt.Errorf("image data is empty")
	}
	if req.Width <= 0 || req.Height <= 0 {
		return fmt.Errorf("width and height must be positive")
	}
	if req.Width > p.maxWidth {
		return fmt.Errorf("width exceeds maximum (%d)", p.maxWidth)
	}
	if req.Height > p.maxHeight {
		return fmt.Errorf("height exceeds maximum (%d)", p.maxHeight)
	}
	if req.Crop == CropFocalPoint {
		if req.FocalX < 0 || req.FocalX > 1 || req.FocalY < 0 || req.FocalY > 1 {
			return fmt.Errorf("focal point coordinates must be between 0.0 and 1.0")
		}
	}
	if req.Crop == CropRect {
		if req.CropX < 0 || req.CropX > 1 || req.CropY < 0 || req.CropY > 1 {
			return fmt.Errorf("rect origin must be between 0.0 and 1.0")
		}
		if req.CropW <= 0 || req.CropH <= 0 || req.CropX+req.CropW > 1.0001 || req.CropY+req.CropH > 1.0001 {
			return fmt.Errorf("rect size must be positive and stay inside the source image")
		}
	}
	if !isValidFormat(req.Format) {
		return fmt.Errorf("unsupported format: %s", req.Format)
	}
	if req.Quality < 0 || req.Quality > 100 {
		return fmt.Errorf("quality must be between 0 and 100")
	}
	return nil
}

func (p *Processor) resizeAndCrop(img *vips.ImageRef, req *ProcessRequest) error {
	switch req.Crop {
	case CropSmart:
		return p.smartCrop(img, req.Width, req.Height)
	case CropFocalPoint:
		return p.focalPointCrop(img, req.Width, req.Height, req.FocalX, req.FocalY)
	case CropRect:
		return p.rectCrop(img, req.Width, req.Height, req.CropX, req.CropY, req.CropW, req.CropH)
	default:
		return p.centerCrop(img, req.Width, req.Height)
	}
}

// centerCrop resizes and crops from the center using Thumbnail.
func (p *Processor) centerCrop(img *vips.ImageRef, width, height int) error {
	return img.Thumbnail(width, height, vips.InterestingCentre)
}

// smartCrop uses libvips attention-based smart crop.
func (p *Processor) smartCrop(img *vips.ImageRef, width, height int) error {
	return img.Thumbnail(width, height, vips.InterestingAttention)
}

// rectCrop extracts the given source rectangle (fractions of original
// dimensions) and resizes it to width × height. Used for user-defined crop
// rectangles where both position AND size matter (e.g. zoom-in via the manual
// crop editor).
func (p *Processor) rectCrop(img *vips.ImageRef, width, height int, fx, fy, fw, fh float64) error {
	origW := float64(img.Width())
	origH := float64(img.PageHeight())

	// Convert fractional rect to pixel coordinates in the source image.
	srcLeft := int(math.Round(fx * origW))
	srcTop := int(math.Round(fy * origH))
	srcW := int(math.Round(fw * origW))
	srcH := int(math.Round(fh * origH))

	// Clamp to image bounds.
	if srcLeft < 0 {
		srcLeft = 0
	}
	if srcTop < 0 {
		srcTop = 0
	}
	if srcLeft+srcW > int(origW) {
		srcW = int(origW) - srcLeft
	}
	if srcTop+srcH > int(origH) {
		srcH = int(origH) - srcTop
	}
	if srcW <= 0 || srcH <= 0 {
		return fmt.Errorf("rect crop produced empty area")
	}

	if err := img.ExtractArea(srcLeft, srcTop, srcW, srcH); err != nil {
		return fmt.Errorf("rect crop extract: %w", err)
	}

	// Resize the extracted rect to the target dimensions. Use independent
	// horizontal/vertical scale since the source rect already matches the
	// target aspect ratio (or close to it — the editor enforces ratio).
	hScale := float64(width) / float64(img.Width())
	vScale := float64(height) / float64(img.PageHeight())
	if err := img.ResizeWithVScale(hScale, vScale, vips.KernelLanczos3); err != nil {
		return fmt.Errorf("rect crop resize: %w", err)
	}
	return nil
}

// focalPointCrop resizes then crops around a focal point.
func (p *Processor) focalPointCrop(img *vips.ImageRef, width, height int, focalX, focalY float64) error {
	// First resize to cover the target dimensions while maintaining aspect ratio.
	origW := float64(img.Width())
	origH := float64(img.PageHeight())
	targetW := float64(width)
	targetH := float64(height)

	scale := math.Max(targetW/origW, targetH/origH)
	if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
		return fmt.Errorf("resize for focal crop: %w", err)
	}

	// Calculate crop position based on focal point.
	resizedW := float64(img.Width())
	resizedH := float64(img.PageHeight())

	// Focal point in pixel coordinates of the resized image.
	focalPxX := focalX * resizedW
	focalPxY := focalY * resizedH

	// Calculate top-left of crop area, centered on focal point.
	left := int(math.Round(focalPxX - targetW/2))
	top := int(math.Round(focalPxY - targetH/2))

	// Clamp to image bounds.
	left = clamp(left, 0, int(resizedW)-width)
	top = clamp(top, 0, int(resizedH)-height)

	return img.ExtractArea(left, top, width, height)
}

func (p *Processor) export(img *vips.ImageRef, format OutputFormat, quality int) ([]byte, string, error) {
	switch format {
	case FormatJPEG:
		q := p.resolveQuality(quality, p.defaultQualityJPEG)
		buf, _, err := img.ExportJpeg(&vips.JpegExportParams{
			Quality:        q,
			StripMetadata:  true,
			Interlace:      true,
			OptimizeCoding: true,
		})
		return buf, "image/jpeg", err

	case FormatPNG:
		compression := p.resolveQuality(quality, p.defaultQualityPNG)
		if compression > 9 {
			compression = 9
		}
		buf, _, err := img.ExportPng(&vips.PngExportParams{
			StripMetadata: true,
			Compression:   compression,
			Filter:        vips.PngFilterAll,
			Interlace:     false,
		})
		return buf, "image/png", err

	case FormatWebP:
		q := p.resolveQuality(quality, p.defaultQualityWebP)
		buf, _, err := img.ExportWebp(&vips.WebpExportParams{
			Quality:         q,
			StripMetadata:   true,
			Lossless:        false,
			ReductionEffort: 4,
		})
		return buf, "image/webp", err

	case FormatAVIF:
		q := p.resolveQuality(quality, p.defaultQualityAVIF)
		buf, _, err := img.ExportAvif(&vips.AvifExportParams{
			Quality:       q,
			StripMetadata: true,
			Effort:        5,
			Lossless:      false,
		})
		return buf, "image/avif", err

	default:
		return nil, "", fmt.Errorf("unsupported output format: %s", format)
	}
}

func (p *Processor) resolveQuality(requested, defaultVal int) int {
	if requested > 0 {
		return requested
	}
	return defaultVal
}

// CropSpec is the fully-parsed crop directive: mode + any associated
// coordinates (focal point for CropFocalPoint, source rectangle for CropRect).
type CropSpec struct {
	Mode   CropMode
	FocalX float64
	FocalY float64
	RectX  float64
	RectY  float64
	RectW  float64
	RectH  float64
}

// ParseCropSpec parses a crop string from the API.
// Accepted values:
//   - "center" / "centre" / ""  → CropCenter
//   - "smart" / "attention"     → CropSmart
//   - "x,y"                     → CropFocalPoint (focal point, fractions 0..1)
//   - "rect:x,y,w,h"            → CropRect      (source rectangle, fractions 0..1)
func ParseCropSpec(s string) (CropSpec, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	switch s {
	case "", "center", "centre":
		return CropSpec{Mode: CropCenter}, nil
	case "smart", "attention":
		return CropSpec{Mode: CropSmart}, nil
	}

	if strings.HasPrefix(s, "rect:") {
		var x, y, w, h float64
		n, err := fmt.Sscanf(strings.TrimPrefix(s, "rect:"), "%f,%f,%f,%f", &x, &y, &w, &h)
		if err != nil || n != 4 {
			return CropSpec{}, fmt.Errorf("invalid rect crop: %q (use 'rect:x,y,w,h' with fractions 0..1)", s)
		}
		if x < 0 || x > 1 || y < 0 || y > 1 || w <= 0 || h <= 0 || x+w > 1.0001 || y+h > 1.0001 {
			return CropSpec{}, fmt.Errorf("rect crop coordinates out of range: %f,%f,%f,%f", x, y, w, h)
		}
		return CropSpec{Mode: CropRect, RectX: x, RectY: y, RectW: w, RectH: h}, nil
	}

	var x, y float64
	n, err := fmt.Sscanf(s, "%f,%f", &x, &y)
	if err != nil || n != 2 {
		return CropSpec{}, fmt.Errorf("invalid crop mode: %q (use 'center', 'smart', 'x,y' or 'rect:x,y,w,h')", s)
	}
	if x < 0 || x > 1 || y < 0 || y > 1 {
		return CropSpec{}, fmt.Errorf("focal point coordinates must be between 0.0 and 1.0, got %f,%f", x, y)
	}
	return CropSpec{Mode: CropFocalPoint, FocalX: x, FocalY: y}, nil
}

// ParseCropMode is the legacy API kept for backwards compatibility. It only
// returns mode + focal point (no rect data). Prefer ParseCropSpec for full
// support.
func ParseCropMode(s string) (CropMode, float64, float64, error) {
	spec, err := ParseCropSpec(s)
	if err != nil {
		return 0, 0, 0, err
	}
	return spec.Mode, spec.FocalX, spec.FocalY, nil
}

// ParseFormat parses and validates an output format string.
func ParseFormat(s string) (OutputFormat, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "jpeg", "jpg":
		return FormatJPEG, nil
	case "png":
		return FormatPNG, nil
	case "webp":
		return FormatWebP, nil
	case "avif":
		return FormatAVIF, nil
	default:
		return "", fmt.Errorf("unsupported format: %q (supported: jpeg, png, webp, avif)", s)
	}
}

// ContentTypeForFormat returns the HTTP Content-Type for a format.
func ContentTypeForFormat(f OutputFormat) string {
	switch f {
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	case FormatWebP:
		return "image/webp"
	case FormatAVIF:
		return "image/avif"
	default:
		return "application/octet-stream"
	}
}

func isValidFormat(f OutputFormat) bool {
	switch f {
	case FormatJPEG, FormatPNG, FormatWebP, FormatAVIF:
		return true
	default:
		return false
	}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Startup initializes the govips library. Call once at application startup.
// It routes libvips log output through yaglogger so all messages appear in a
// consistent format alongside the rest of the service logs.
func Startup() error {
	if err := vips.Startup(nil); err != nil {
		return fmt.Errorf("govips startup: %w", err)
	}
	vips.LoggingSettings(func(domain string, level vips.LogLevel, message string) {
		switch level {
		case vips.LogLevelError, vips.LogLevelCritical:
			log.Error("[vips] %s: %s", domain, message)
		case vips.LogLevelWarning:
			log.Warn("[vips] %s: %s", domain, message)
		case vips.LogLevelMessage, vips.LogLevelInfo:
			log.Info("[vips] %s: %s", domain, message)
		default:
			log.Debug("[vips] %s: %s", domain, message)
		}
	}, vips.LogLevelDebug)
	return nil
}

// Shutdown cleans up govips resources. Call at application shutdown.
func Shutdown() {
	vips.Shutdown()
}
