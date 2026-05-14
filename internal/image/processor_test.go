package image

import (
	"testing"
)

func TestParseCropMode(t *testing.T) {
	tests := []struct {
		input    string
		wantMode CropMode
		wantX    float64
		wantY    float64
		wantErr  bool
	}{
		{"center", CropCenter, 0, 0, false},
		{"centre", CropCenter, 0, 0, false},
		{"", CropCenter, 0, 0, false},
		{"smart", CropSmart, 0, 0, false},
		{"attention", CropSmart, 0, 0, false},
		{"0.5,0.3", CropFocalPoint, 0.5, 0.3, false},
		{"0.0,1.0", CropFocalPoint, 0.0, 1.0, false},
		{"1.0,0.0", CropFocalPoint, 1.0, 0.0, false},
		{"1.1,0.5", 0, 0, 0, true},  // Out of range
		{"0.5,1.1", 0, 0, 0, true},  // Out of range
		{"-0.1,0.5", 0, 0, 0, true}, // Negative
		{"invalid", 0, 0, 0, true},
		{"abc,def", 0, 0, 0, true},
	}

	for _, tt := range tests {
		mode, x, y, err := ParseCropMode(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseCropMode(%q): expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCropMode(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if mode != tt.wantMode {
			t.Errorf("ParseCropMode(%q): mode = %v, want %v", tt.input, mode, tt.wantMode)
		}
		if x != tt.wantX || y != tt.wantY {
			t.Errorf("ParseCropMode(%q): focal = %f,%f, want %f,%f", tt.input, x, y, tt.wantX, tt.wantY)
		}
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    OutputFormat
		wantErr bool
	}{
		{"jpeg", FormatJPEG, false},
		{"jpg", FormatJPEG, false},
		{"JPEG", FormatJPEG, false},
		{"png", FormatPNG, false},
		{"PNG", FormatPNG, false},
		{"webp", FormatWebP, false},
		{"WebP", FormatWebP, false},
		{"avif", FormatAVIF, false},
		{"AVIF", FormatAVIF, false},
		{"gif", "", true},
		{"bmp", "", true},
		{"tiff", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := ParseFormat(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseFormat(%q): expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFormat(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseFormat(%q): got %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestContentTypeForFormat(t *testing.T) {
	tests := []struct {
		format OutputFormat
		want   string
	}{
		{FormatJPEG, "image/jpeg"},
		{FormatPNG, "image/png"},
		{FormatWebP, "image/webp"},
		{FormatAVIF, "image/avif"},
		{"unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := ContentTypeForFormat(tt.format)
		if got != tt.want {
			t.Errorf("ContentTypeForFormat(%v): got %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestValidateRequest(t *testing.T) {
	p := NewProcessor(4096, 4096, 85, 80, 60, 6)

	tests := []struct {
		name    string
		req     *ProcessRequest
		wantErr bool
	}{
		{
			name:    "empty image data",
			req:     &ProcessRequest{ImageData: nil, Width: 100, Height: 100, Format: FormatJPEG},
			wantErr: true,
		},
		{
			name:    "zero width",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 0, Height: 100, Format: FormatJPEG},
			wantErr: true,
		},
		{
			name:    "negative height",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 100, Height: -1, Format: FormatJPEG},
			wantErr: true,
		},
		{
			name:    "exceeds max width",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 5000, Height: 100, Format: FormatJPEG},
			wantErr: true,
		},
		{
			name:    "exceeds max height",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 100, Height: 5000, Format: FormatJPEG},
			wantErr: true,
		},
		{
			name:    "invalid format",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 100, Height: 100, Format: "gif"},
			wantErr: true,
		},
		{
			name:    "quality out of range",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 100, Height: 100, Format: FormatJPEG, Quality: 101},
			wantErr: true,
		},
		{
			name:    "focal point out of range",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 100, Height: 100, Format: FormatJPEG, Crop: CropFocalPoint, FocalX: 1.5, FocalY: 0.5},
			wantErr: true,
		},
		{
			name:    "valid request",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 100, Height: 100, Format: FormatJPEG},
			wantErr: false,
		},
		{
			name:    "valid focal point",
			req:     &ProcessRequest{ImageData: []byte{1}, Width: 100, Height: 100, Format: FormatJPEG, Crop: CropFocalPoint, FocalX: 0.5, FocalY: 0.5},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validateRequest(tt.req)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, min, max, want int
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 0, 0},
	}

	for _, tt := range tests {
		got := clamp(tt.v, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.v, tt.min, tt.max, got, tt.want)
		}
	}
}
