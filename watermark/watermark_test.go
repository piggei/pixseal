package watermark

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
	"time"
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
	input := make([]byte, maxFrameBits)
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

func TestProfileSelectionThresholds(t *testing.T) {
	tests := []struct {
		bytes int
		want  Profile
	}{
		{16, ProfileRobust},
		{17, ProfileBalanced},
		{32, ProfileBalanced},
		{33, ProfileCapacity},
		{64, ProfileCapacity},
	}
	for _, tc := range tests {
		info, err := SelectProfile(tc.bytes, ProfileAuto)
		if err != nil {
			t.Fatalf("SelectProfile(%d): %v", tc.bytes, err)
		}
		if info.Name != tc.want {
			t.Errorf("SelectProfile(%d) = %s, want %s", tc.bytes, info.Name, tc.want)
		}
	}
	if _, err := SelectProfile(65, ProfileAuto); err == nil {
		t.Fatal("65-byte payload was accepted")
	}
}

func TestExplicitProfileCapacityErrors(t *testing.T) {
	if _, err := SelectProfile(17, ProfileRobust); err == nil {
		t.Fatal("robust accepted 17 bytes")
	}
	if _, err := SelectProfile(33, ProfileBalanced); err == nil {
		t.Fatal("balanced accepted 33 bytes")
	}
	if _, err := SelectProfile(64, ProfileCapacity); err != nil {
		t.Fatalf("capacity rejected 64 bytes: %v", err)
	}
}

func TestV3ProfileRoundTrips(t *testing.T) {
	key := []byte("correct horse battery staple")
	tests := []struct {
		profile Profile
		size    int
	}{
		{ProfileRobust, 16},
		{ProfileBalanced, 32},
		{ProfileCapacity, 64},
	}
	for _, tc := range tests {
		t.Run(string(tc.profile), func(t *testing.T) {
			message := bytes.Repeat([]byte{byte('A' + tc.size%20)}, tc.size)
			options := DefaultOptions()
			options.Profile = tc.profile
			marked, embedInfo, err := EmbedWithInfo(testImage(560, 512), message, key, options)
			if err != nil {
				t.Fatal(err)
			}
			if embedInfo.Profile != tc.profile {
				t.Fatalf("embedded profile %s, want %s", embedInfo.Profile, tc.profile)
			}
			got, extractInfo, err := ExtractWithInfo(marked, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, message) {
				t.Fatalf("got %d bytes, want %d", len(got), len(message))
			}
			if extractInfo.Version != 3 || extractInfo.Profile != tc.profile {
				t.Fatalf("extracted v%d/%s, want v3/%s", extractInfo.Version, extractInfo.Profile, tc.profile)
			}
		})
	}
}

func TestV3TransformsByProfile(t *testing.T) {
	key := []byte("profile transform test key")
	tests := []struct {
		profile Profile
		message []byte
	}{
		{ProfileRobust, []byte("robust payload")},
		{ProfileBalanced, []byte("balanced profile payload")},
		{ProfileCapacity, []byte("capacity profile payload exercising full frame 1234567890")},
	}
	for _, tc := range tests {
		t.Run(string(tc.profile), func(t *testing.T) {
			options := DefaultOptions()
			options.Profile = tc.profile
			marked, err := Embed(testImage(560, 512), tc.message, key, options)
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
			transforms := map[string]image.Image{
				"jpeg":      jpegImage,
				"resize-75": resizeBilinear(marked, 420, 384),
				"crop-75":   cropCopy(marked, image.Rect(70, 64, 490, 448)),
			}
			for name, transformed := range transforms {
				t.Run(name, func(t *testing.T) {
					got, info, err := ExtractWithInfo(transformed, key)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, tc.message) || info.Profile != tc.profile {
						t.Fatalf("recovered %q with %s; want %q with %s", got, info.Profile, tc.message, tc.profile)
					}
				})
			}
		})
	}
}

func TestV3AutoProfileExtraction(t *testing.T) {
	key := []byte("automatic profile key")
	message := bytes.Repeat([]byte("x"), 17)
	marked, info, err := EmbedWithInfo(testImage(560, 512), message, key, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if info.Profile != ProfileBalanced {
		t.Fatalf("auto selected %s, want balanced", info.Profile)
	}
	got, extracted, err := ExtractWithInfo(marked, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, message) || extracted.Profile != ProfileBalanced {
		t.Fatalf("automatic extraction got profile %s payload %q", extracted.Profile, got)
	}
}

func TestWrongKeyAndUnmarkedImageAreBounded(t *testing.T) {
	key := []byte("correct extraction key")
	message := []byte("short message")
	options := DefaultOptions()
	options.Profile = ProfileRobust
	marked, err := Embed(testImage(320, 288), message, key, options)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]image.Image{
		"wrong-key": marked,
		"unmarked":  testImage(320, 288),
	}
	for name, carrier := range tests {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			useKey := []byte("different extraction key")
			if name == "unmarked" {
				useKey = key
			}
			if _, _, err := Extract(carrier, useKey); err == nil {
				t.Fatal("unexpected extraction success")
			}
			if elapsed := time.Since(start); elapsed > 20*time.Second {
				t.Fatalf("failed extraction took %s; bounded-search regression suspected", elapsed)
			}
		})
	}
}

func TestAnalyzeImageMatchesProfileMath(t *testing.T) {
	analysis := AnalyzeImage(testImage(1920, 1080), 18)
	if !analysis.ImageCompatible || analysis.RecommendedProfile != ProfileBalanced || analysis.ProfileCapacity != 32 {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}
	if math.Abs(analysis.ProfileTileRedundancy-float64(eccBits)/672) > 0.001 {
		t.Fatalf("unexpected profile redundancy %.4f", analysis.ProfileTileRedundancy)
	}
	if analysis.RecommendedStrength < 4 || analysis.RecommendedStrength > 120 {
		t.Fatalf("invalid recommended strength %.1f", analysis.RecommendedStrength)
	}
}

func TestV3FrameIgnoresTrailingPaddingButAuthenticatesHeader(t *testing.T) {
	key := []byte("v3 frame authentication key")
	message := []byte("short")
	spec, _ := profileSpecFor(ProfileRobust)
	frame := makeV3Frame(message, key, spec)

	// Bytes after header || payload || tag are intentionally non-semantic padding.
	paddingOffset := headerSize + len(message) + tagSize
	if paddingOffset >= len(frame) {
		t.Fatal("test message leaves no v3 padding")
	}
	frame[paddingOffset] ^= 0xff
	got, err := parseV3Frame(frame, key, spec)
	if err != nil || !bytes.Equal(got, message) {
		t.Fatalf("trailing padding affected payload authentication: got=%q err=%v", got, err)
	}

	frame = makeV3Frame(message, key, spec)
	frame[2] ^= 1
	if _, err := parseV3Frame(frame, key, spec); err == nil {
		t.Fatal("tampered v3 format/profile byte was accepted")
	}
}

func TestV3SyncPatternObservationCounts(t *testing.T) {
	key := []byte("sync-pattern-test-key")
	want := map[Profile]int{
		ProfileRobust:   106,
		ProfileBalanced: 69,
		ProfileCapacity: 42,
	}
	for _, spec := range v3Profiles {
		pattern := newV3SyncPattern(key, spec)
		if got := len(pattern.points); got != want[spec.profile] {
			t.Fatalf("profile %s sync observations = %d, want %d", spec.profile, got, want[spec.profile])
		}
	}
}

func TestV3TileMappingObservationCounts(t *testing.T) {
	for _, spec := range v3Profiles {
		counts := make([]int, spec.codedBits)
		for position := 0; position < eccBits; position++ {
			counts[v3CodeIndex(position, spec.codedBits)]++
		}
		total := 0
		minimum := counts[0]
		maximum := counts[0]
		for _, count := range counts {
			total += count
			if count < minimum {
				minimum = count
			}
			if count > maximum {
				maximum = count
			}
		}
		if total != eccBits || maximum-minimum > 1 {
			t.Fatalf("profile %s has uneven mapping min=%d max=%d total=%d", spec.profile, minimum, maximum, total)
		}
	}
}

func TestPixelPlaneBicubicMatchesReference(t *testing.T) {
	src := testImage(73, 61)
	optimized := resizePixelPlaneBicubic(newPixelPlane(src), 91, 77)
	reference := newPixelPlane(resizeBicubicReference(src, 91, 77))
	if !bytes.Equal(optimized.rgb, reference.rgb) {
		t.Fatal("optimized pixel-plane normalization differs from reference bicubic reconstruction")
	}
}

func resizeBicubicReference(src image.Image, width, height int) *image.NRGBA {
	bounds := src.Bounds()
	output := image.NewNRGBA(image.Rect(0, 0, width, height))
	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		sourceY := (float64(y)+.5)*scaleY - .5
		baseY := int(math.Floor(sourceY))
		for x := 0; x < width; x++ {
			sourceX := (float64(x)+.5)*scaleX - .5
			baseX := int(math.Floor(sourceX))
			red, green, blue, weightSum := 0.0, 0.0, 0.0, 0.0

			for sampleY := baseY - 1; sampleY <= baseY+2; sampleY++ {
				weightY := cubicWeight(sourceY - float64(sampleY))
				pixelY := clampCoordinate(sampleY, bounds.Dy()) + bounds.Min.Y
				for sampleX := baseX - 1; sampleX <= baseX+2; sampleX++ {
					weight := weightY * cubicWeight(sourceX-float64(sampleX))
					pixelX := clampCoordinate(sampleX, bounds.Dx()) + bounds.Min.X
					r, g, b, _ := src.At(pixelX, pixelY).RGBA()
					red += float64(r>>8) * weight
					green += float64(g>>8) * weight
					blue += float64(b>>8) * weight
					weightSum += weight
				}
			}
			output.SetNRGBA(x, y, color.NRGBA{
				R: clamp(red / weightSum),
				G: clamp(green / weightSum),
				B: clamp(blue / weightSum),
				A: 255,
			})
		}
	}
	return output
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
