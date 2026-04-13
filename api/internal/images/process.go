package images

import (
	"fmt"
	"io"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/google/uuid"
)

const (
	SourceMaxEdge       = 2560
	SourceMaxEdgeSquare = 2048
	SourceQuality       = 85
	ThumbHeight         = 400
	ThumbQuality        = 75
	MaxUploadSize       = 25 << 20 // 25MB
)

// Format aspect ratios (w:h)
var formatAspect = map[string][2]int{
	"landscape": {4, 3},
	"portrait":  {3, 4},
	"square":    {1, 1},
}

// Thumbnail dimensions at 400px height
var formatThumb = map[string][2]int{
	"landscape": {533, 400},
	"portrait":  {300, 400},
	"square":    {400, 400},
}

type ProcessResult struct {
	ID        string
	Source    []byte
	Thumbnail []byte
}

func InitVips() {
	vips.LoggingSettings(nil, vips.LogLevelWarning)
	vips.Startup(nil)
}

func ShutdownVips() {
	vips.Shutdown()
}

// ProcessProductImage loads an image, strips EXIF, auto-rotates, center-crops to the given format, and produces source + thumbnail WebP.
func ProcessProductImage(r io.Reader, format string) (*ProcessResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	img, err := vips.NewImageFromBuffer(data)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	defer img.Close()

	if err := img.AutoRotate(); err != nil {
		return nil, fmt.Errorf("auto-rotate: %w", err)
	}

	aspect, ok := formatAspect[format]
	if !ok {
		aspect = formatAspect["landscape"]
	}
	if err := centerCrop(img, aspect[0], aspect[1]); err != nil {
		return nil, fmt.Errorf("crop: %w", err)
	}

	thumb := formatThumb[format]
	if thumb == [2]int{} {
		thumb = formatThumb["landscape"]
	}

	maxEdge := SourceMaxEdge
	if format == "square" {
		maxEdge = SourceMaxEdgeSquare
	}

	return encodeVariants(img, thumb[0], thumb[1], maxEdge)
}

// ProcessIssueImage loads an image, strips EXIF, auto-rotates, and produces source + thumbnail WebP. No crop — thumbnail is resized to fit within 400px height.
func ProcessIssueImage(r io.Reader) (*ProcessResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	img, err := vips.NewImageFromBuffer(data)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	defer img.Close()

	if err := img.AutoRotate(); err != nil {
		return nil, fmt.Errorf("auto-rotate: %w", err)
	}

	img.RemoveMetadata()
	id := uuid.New().String()

	// Source: resize longest edge, no crop
	sourceImg, err := img.Copy()
	if err != nil {
		return nil, fmt.Errorf("copy for source: %w", err)
	}
	defer sourceImg.Close()
	if err := resizeLongestEdge(sourceImg, SourceMaxEdge); err != nil {
		return nil, fmt.Errorf("resize source: %w", err)
	}
	sourceBytes, _, err := sourceImg.ExportWebp(&vips.WebpExportParams{Quality: SourceQuality})
	if err != nil {
		return nil, fmt.Errorf("encode source webp: %w", err)
	}

	// Thumbnail: resize longest edge to ThumbHeight, no crop
	thumbImg, err := img.Copy()
	if err != nil {
		return nil, fmt.Errorf("copy for thumb: %w", err)
	}
	defer thumbImg.Close()
	if err := resizeLongestEdge(thumbImg, ThumbHeight); err != nil {
		return nil, fmt.Errorf("resize thumb: %w", err)
	}
	thumbBytes, _, err := thumbImg.ExportWebp(&vips.WebpExportParams{Quality: ThumbQuality})
	if err != nil {
		return nil, fmt.Errorf("encode thumb webp: %w", err)
	}

	return &ProcessResult{ID: id, Source: sourceBytes, Thumbnail: thumbBytes}, nil
}

func centerCrop(img *vips.ImageRef, aspectW, aspectH int) error {
	w := img.Width()
	h := img.Height()

	targetW := w
	targetH := w * aspectH / aspectW
	if targetH > h {
		targetH = h
		targetW = h * aspectW / aspectH
	}

	left := (w - targetW) / 2
	top := (h - targetH) / 2

	return img.ExtractArea(left, top, targetW, targetH)
}

func encodeVariants(img *vips.ImageRef, thumbW, thumbH, maxEdge int) (*ProcessResult, error) {
	id := uuid.New().String()

	img.RemoveMetadata()

	// Source variant: resize longest edge
	sourceImg, err := img.Copy()
	if err != nil {
		return nil, fmt.Errorf("copy for source: %w", err)
	}
	defer sourceImg.Close()

	if err := resizeLongestEdge(sourceImg, maxEdge); err != nil {
		return nil, fmt.Errorf("resize source: %w", err)
	}

	sourceBytes, _, err := sourceImg.ExportWebp(&vips.WebpExportParams{
		Quality:  SourceQuality,
		Lossless: false,
	})
	if err != nil {
		return nil, fmt.Errorf("encode source webp: %w", err)
	}

	// Thumbnail variant
	thumbImg, err := img.Copy()
	if err != nil {
		return nil, fmt.Errorf("copy for thumb: %w", err)
	}
	defer thumbImg.Close()

	if err := thumbImg.Thumbnail(thumbW, thumbH, vips.InterestingCentre); err != nil {
		return nil, fmt.Errorf("resize thumb: %w", err)
	}

	thumbBytes, _, err := thumbImg.ExportWebp(&vips.WebpExportParams{
		Quality:  ThumbQuality,
		Lossless: false,
	})
	if err != nil {
		return nil, fmt.Errorf("encode thumb webp: %w", err)
	}

	return &ProcessResult{
		ID:        id,
		Source:    sourceBytes,
		Thumbnail: thumbBytes,
	}, nil
}

func resizeLongestEdge(img *vips.ImageRef, maxEdge int) error {
	w := img.Width()
	h := img.Height()

	if w <= maxEdge && h <= maxEdge {
		return nil
	}

	scale := float64(maxEdge) / float64(w)
	if h > w {
		scale = float64(maxEdge) / float64(h)
	}

	return img.Resize(scale, vips.KernelLanczos3)
}

// ConvertToJPEG converts WebP bytes to JPEG for download/sharing.
func ConvertToJPEG(webpData []byte) ([]byte, error) {
	img, err := vips.NewImageFromBuffer(webpData)
	if err != nil {
		return nil, fmt.Errorf("decode webp: %w", err)
	}
	defer img.Close()

	data, _, err := img.ExportJpeg(&vips.JpegExportParams{
		Quality: 85,
	})
	if err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return data, nil
}
