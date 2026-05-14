package image

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
)

func TestMain(m *testing.M) {
	vips.Startup(nil)
	code := m.Run()
	vips.Shutdown()
	os.Exit(code)
}

func loadTestImage(t *testing.T) []byte {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(filename)
	data, err := os.ReadFile(filepath.Join(testDir, "testdata", "test.jpg"))
	if err != nil {
		t.Fatalf("failed to load test image: %v", err)
	}
	return data
}

func TestProcessCenterCrop(t *testing.T) {
	p := NewProcessor(4096, 4096, 85, 80, 60, 6)
	imgData := loadTestImage(t)

	result, err := p.Process(&ProcessRequest{
		ImageData: imgData,
		Width:     200,
		Height:    150,
		Crop:      CropCenter,
		Format:    FormatJPEG,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if result.ContentType != "image/jpeg" {
		t.Errorf("expected content-type image/jpeg, got %s", result.ContentType)
	}
	if len(result.Data) == 0 {
		t.Error("result data is empty")
	}

	// Verify dimensions by loading the result.
	img, err := vips.NewImageFromBuffer(result.Data)
	if err != nil {
		t.Fatalf("failed to load result: %v", err)
	}
	defer img.Close()

	if img.Width() != 200 || img.PageHeight() != 150 {
		t.Errorf("expected 200x150, got %dx%d", img.Width(), img.PageHeight())
	}
}

func TestProcessSmartCrop(t *testing.T) {
	p := NewProcessor(4096, 4096, 85, 80, 60, 6)
	imgData := loadTestImage(t)

	result, err := p.Process(&ProcessRequest{
		ImageData: imgData,
		Width:     100,
		Height:    100,
		Crop:      CropSmart,
		Format:    FormatWebP,
		Quality:   75,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if result.ContentType != "image/webp" {
		t.Errorf("expected content-type image/webp, got %s", result.ContentType)
	}
	if len(result.Data) == 0 {
		t.Error("result data is empty")
	}
}

func TestProcessFocalPointCrop(t *testing.T) {
	p := NewProcessor(4096, 4096, 85, 80, 60, 6)
	imgData := loadTestImage(t)

	result, err := p.Process(&ProcessRequest{
		ImageData: imgData,
		Width:     150,
		Height:    150,
		Crop:      CropFocalPoint,
		FocalX:    0.3,
		FocalY:    0.7,
		Format:    FormatPNG,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if result.ContentType != "image/png" {
		t.Errorf("expected content-type image/png, got %s", result.ContentType)
	}

	img, err := vips.NewImageFromBuffer(result.Data)
	if err != nil {
		t.Fatalf("failed to load result: %v", err)
	}
	defer img.Close()

	if img.Width() != 150 || img.PageHeight() != 150 {
		t.Errorf("expected 150x150, got %dx%d", img.Width(), img.PageHeight())
	}
}

func TestProcessFormatConversion(t *testing.T) {
	p := NewProcessor(4096, 4096, 85, 80, 60, 6)
	imgData := loadTestImage(t)

	formats := []struct {
		format      OutputFormat
		contentType string
	}{
		{FormatJPEG, "image/jpeg"},
		{FormatPNG, "image/png"},
		{FormatWebP, "image/webp"},
		{FormatAVIF, "image/avif"},
	}

	for _, f := range formats {
		t.Run(string(f.format), func(t *testing.T) {
			result, err := p.Process(&ProcessRequest{
				ImageData: imgData,
				Width:     100,
				Height:    75,
				Crop:      CropCenter,
				Format:    f.format,
			})
			if err != nil {
				t.Fatalf("Process to %s failed: %v", f.format, err)
			}
			if result.ContentType != f.contentType {
				t.Errorf("expected %s, got %s", f.contentType, result.ContentType)
			}
			if len(result.Data) == 0 {
				t.Error("result data is empty")
			}
		})
	}
}

func TestProcessQuality(t *testing.T) {
	p := NewProcessor(4096, 4096, 85, 80, 60, 6)
	imgData := loadTestImage(t)

	// Low quality should produce smaller files.
	lowQ, err := p.Process(&ProcessRequest{
		ImageData: imgData,
		Width:     200,
		Height:    150,
		Crop:      CropCenter,
		Format:    FormatJPEG,
		Quality:   10,
	})
	if err != nil {
		t.Fatalf("low quality: %v", err)
	}

	highQ, err := p.Process(&ProcessRequest{
		ImageData: imgData,
		Width:     200,
		Height:    150,
		Crop:      CropCenter,
		Format:    FormatJPEG,
		Quality:   95,
	})
	if err != nil {
		t.Fatalf("high quality: %v", err)
	}

	if len(lowQ.Data) >= len(highQ.Data) {
		t.Errorf("low quality (%d bytes) should be smaller than high quality (%d bytes)", len(lowQ.Data), len(highQ.Data))
	}
}

func TestProcessInvalidImage(t *testing.T) {
	p := NewProcessor(4096, 4096, 85, 80, 60, 6)

	_, err := p.Process(&ProcessRequest{
		ImageData: []byte("not a valid image"),
		Width:     100,
		Height:    100,
		Crop:      CropCenter,
		Format:    FormatJPEG,
	})
	if err == nil {
		t.Fatal("expected error for invalid image data")
	}
}
