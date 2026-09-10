package watermark

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func syntheticPrintBoundaryImage(width, height int, rect image.Rectangle) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	paper := color.NRGBA{R: 238, G: 236, B: 226, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, paper)
		}
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(30 + (x*3+y)%120),
				G: uint8(45 + (x+y*5)%100),
				B: uint8(60 + (x*7+y*2)%110),
				A: 255,
			})
		}
	}
	return img
}

func TestPrintBoundaryEstimatorFindsSyntheticPrint(t *testing.T) {
	rect := image.Rect(90, 75, 1110, 825)
	img := syntheticPrintBoundaryImage(1200, 900, rect)
	got := estimatePrintBoundary(img)
	if !got.Detected {
		t.Fatalf("print boundary not detected: %+v", got)
	}
	if got.Confidence < 0.45 {
		t.Fatalf("print boundary confidence %.3f; want >= 0.45", got.Confidence)
	}
	checks := []struct {
		name string
		got  ImagePoint
		x, y float64
	}{
		{"TL", got.TopLeft, float64(rect.Min.X), float64(rect.Min.Y)},
		{"TR", got.TopRight, float64(rect.Max.X), float64(rect.Min.Y)},
		{"BR", got.BottomRight, float64(rect.Max.X), float64(rect.Max.Y)},
		{"BL", got.BottomLeft, float64(rect.Min.X), float64(rect.Max.Y)},
	}
	for _, check := range checks {
		if math.Hypot(check.got.X-check.x, check.got.Y-check.y) > 30 {
			t.Fatalf("%s boundary=(%.1f,%.1f), want near (%.1f,%.1f)", check.name, check.got.X, check.got.Y, check.x, check.y)
		}
	}
}

func TestHomographyForPrintBoundaryMapsCorners(t *testing.T) {
	boundary := PrintBoundaryEstimate{
		Detected: true,
		TopLeft:  ImagePoint{X: 20, Y: 30}, TopRight: ImagePoint{X: 820, Y: 12},
		BottomRight: ImagePoint{X: 850, Y: 650}, BottomLeft: ImagePoint{X: 35, Y: 675},
	}
	h, ok := homographyForPrintBoundary(640, 480, boundary)
	if !ok {
		t.Fatal("homographyForPrintBoundary failed")
	}
	cases := []struct {
		x, y float64
		want ImagePoint
	}{
		{0, 0, boundary.TopLeft}, {639, 0, boundary.TopRight},
		{639, 479, boundary.BottomRight}, {0, 479, boundary.BottomLeft},
	}
	for _, tc := range cases {
		x, y, ok := h.mapPoint(tc.x, tc.y)
		if !ok || math.Hypot(x-tc.want.X, y-tc.want.Y) > 1e-6 {
			t.Fatalf("map(%.0f,%.0f)=(%.6f,%.6f,%t), want (%.6f,%.6f)", tc.x, tc.y, x, y, ok, tc.want.X, tc.want.Y)
		}
	}
}
