package watermark

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
)

func testImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			noise := float64(((x*73856093)^(y*19349663))&15) - 8
			img.SetNRGBA(x, y, color.NRGBA{
				R: clamp(128 + 55*math.Sin(float64(x)/31) + 35*math.Cos(float64(y)/47) + noise),
				G: clamp(120 + 45*math.Sin(float64(x+y)/43) + 30*math.Cos(float64(y)/29) + noise),
				B: clamp(135 + 50*math.Cos(float64(x)/37) + 25*math.Sin(float64(x-2*y)/53) + noise),
				A: 255,
			})
		}
	}
	return img
}

func TestHammingCorrectsSingleBit(t *testing.T) {
	input := make([]byte, frameBits)
	for i := range input {
		input[i] = byte(i & 1)
	}
	encoded := hammingEncode(input)
	for i := 0; i < len(encoded); i += 7 {
		encoded[i] ^= 1
	}
	if decoded := hammingDecode(encoded); !bytes.Equal(decoded, input) {
		t.Fatal("Hamming decoder did not correct one error per codeword")
	}
}

func TestV2TransformRoundTrips(t *testing.T) {
	key := []byte("correct horse battery staple")
	message := []byte("geometric watermark test")
	marked, err := Embed(testImage(560, 512), message, key, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	var jpegBuffer bytes.Buffer
	if err := jpeg.Encode(&jpegBuffer, marked, &jpeg.Options{Quality: 82}); err != nil {
		t.Fatal(err)
	}
	jpegImage, err := jpeg.Decode(&jpegBuffer)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]image.Image{
		"original":  marked,
		"jpeg":      jpegImage,
		"resize-95": resizeBilinear(marked, 532, 486),
		"resize-75": resizeBilinear(marked, 420, 384),
		"resize-55": resizeBilinear(marked, 308, 282),
		"resize-50": resizeBilinear(marked, 280, 256),
		"crop-90":   cropCopy(marked, image.Rect(27, 25, 531, 486)),
		"crop-75":   cropCopy(marked, image.Rect(70, 64, 490, 448)),
	}

	for name, transformed := range tests {
		t.Run(name, func(t *testing.T) {
			got, _, err := Extract(transformed, key, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, message) {
				t.Fatalf("got %q, want %q", got, message)
			}
		})
	}
}

func cropCopy(src image.Image, rectangle image.Rectangle) *image.NRGBA {
	output := image.NewNRGBA(image.Rect(0, 0, rectangle.Dx(), rectangle.Dy()))
	for y := 0; y < rectangle.Dy(); y++ {
		for x := 0; x < rectangle.Dx(); x++ {
			output.Set(x, y, src.At(rectangle.Min.X+x, rectangle.Min.Y+y))
		}
	}
	return output
}

func resizeBilinear(src image.Image, width, height int) *image.NRGBA {
	bounds := src.Bounds()
	output := image.NewNRGBA(image.Rect(0, 0, width, height))
	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		sourceY := (float64(y)+.5)*scaleY - .5
		y0 := int(math.Floor(sourceY))
		yFraction := sourceY - float64(y0)
		if y0 < 0 {
			y0, yFraction = 0, 0
		}
		y1 := y0 + 1
		if y1 >= bounds.Dy() {
			y1 = bounds.Dy() - 1
		}

		for x := 0; x < width; x++ {
			sourceX := (float64(x)+.5)*scaleX - .5
			x0 := int(math.Floor(sourceX))
			xFraction := sourceX - float64(x0)
			if x0 < 0 {
				x0, xFraction = 0, 0
			}
			x1 := x0 + 1
			if x1 >= bounds.Dx() {
				x1 = bounds.Dx() - 1
			}

			c00 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x0, bounds.Min.Y+y0)).(color.NRGBA)
			c10 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x1, bounds.Min.Y+y0)).(color.NRGBA)
			c01 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x0, bounds.Min.Y+y1)).(color.NRGBA)
			c11 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x1, bounds.Min.Y+y1)).(color.NRGBA)
			output.SetNRGBA(x, y, color.NRGBA{
				R: bilinearChannel(c00.R, c10.R, c01.R, c11.R, xFraction, yFraction),
				G: bilinearChannel(c00.G, c10.G, c01.G, c11.G, xFraction, yFraction),
				B: bilinearChannel(c00.B, c10.B, c01.B, c11.B, xFraction, yFraction),
				A: 255,
			})
		}
	}
	return output
}

func bilinearChannel(c00, c10, c01, c11 uint8, x, y float64) uint8 {
	top := float64(c00)*(1-x) + float64(c10)*x
	bottom := float64(c01)*(1-x) + float64(c11)*x
	return uint8(math.Round(top*(1-y) + bottom*y))
}
