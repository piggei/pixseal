package watermark

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"strings"
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

func TestStrengthRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		options := DefaultOptions()
		options.Strength = value
		if _, err := normalizeEmbedOptions(options); err == nil {
			t.Fatalf("normalizeEmbedOptions accepted non-finite strength %v", value)
		}
	}
}

type boundsOnlyImage struct{ rectangle image.Rectangle }

func (img boundsOnlyImage) ColorModel() color.Model { return color.NRGBAModel }
func (img boundsOnlyImage) Bounds() image.Rectangle { return img.rectangle }
func (img boundsOnlyImage) At(x, y int) color.Color { return color.NRGBA{A: 255} }

func TestWorkingImageLimitRejectsBeforePixelPlaneAllocation(t *testing.T) {
	img := boundsOnlyImage{rectangle: image.Rect(0, 0, maxWorkingPixels+1, 1)}
	if _, _, err := ExtractWithInfo(img, []byte("12345678")); err == nil || !strings.Contains(err.Error(), "working-image limit") {
		t.Fatalf("oversized ExtractWithInfo error = %v", err)
	}
	if _, _, err := EmbedWithInfo(img, []byte("x"), []byte("12345678"), DefaultOptions()); err == nil || !strings.Contains(err.Error(), "working-image limit") {
		t.Fatalf("oversized EmbedWithInfo error = %v", err)
	}
}

func TestAnalyzerUsesSameWhiteAlphaFlatteningAsEncoder(t *testing.T) {
	transparent := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	transparent.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 0})
	white := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			white.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	if got, want := pixelLuminance(transparent, 0, 0), pixelLuminance(white, 0, 0); math.Abs(got-want) > 1e-9 {
		t.Fatalf("transparent pixel luminance %.3f differs from white-flattened %.3f", got, want)
	}
}

func TestV3EncoderGoldenFingerprint(t *testing.T) {
	key := []byte("encoder golden key")
	message := []byte("golden-v3")
	expected := map[Profile]string{
		ProfileRobust:   "f78b3f97824780cbbd2a303d25b0d36c94f845cdd8213291fc76b263f14a08bb",
		ProfileBalanced: "9b548bd4da9befab4d4ab97fb0d2fe758d79cd0c7ef30c0c32079f8c03874029",
		ProfileCapacity: "cdf7007a900c05247b532e327b549607cdb7e8c2d677c71b46481b835edfc577",
	}
	for _, profile := range []Profile{ProfileRobust, ProfileBalanced, ProfileCapacity} {
		t.Run(string(profile), func(t *testing.T) {
			options := DefaultOptions()
			options.Profile = profile
			marked, err := Embed(testImage(560, 512), message, key, options)
			if err != nil {
				t.Fatal(err)
			}
			got := sha256.Sum256(marked.Pix)
			if hex := fmt.Sprintf("%x", got); hex != expected[profile] {
				t.Fatalf("encoder fingerprint changed for %s: got %s, want %s", profile, hex, expected[profile])
			}
		})
	}
}

func TestIsotropicScaleSearchIsFixed(t *testing.T) {
	want := []int{95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25}
	if len(normalizedScales) != len(want) {
		t.Fatalf("isotropic scale search has %d candidates, want %d", len(normalizedScales), len(want))
	}
	for i, value := range want {
		if normalizedScales[i] != value {
			t.Fatalf("isotropic scale candidate %d = %d, want %d", i, normalizedScales[i], value)
		}
	}
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

func TestV3QuarterTurnRecovery(t *testing.T) {
	key := []byte("quarter-turn recovery key")
	tests := []struct {
		profile Profile
		turns   int
	}{
		{ProfileRobust, 1},
		{ProfileBalanced, 2},
		{ProfileCapacity, 3},
	}
	for _, tc := range tests {
		t.Run(string(tc.profile), func(t *testing.T) {
			options := DefaultOptions()
			options.Profile = tc.profile
			message := []byte("quarter turn")
			marked, err := Embed(testImage(400, 360), message, key, options)
			if err != nil {
				t.Fatal(err)
			}
			rotated := rotateQuarterForTest(marked, tc.turns)
			got, info, err := ExtractWithInfo(rotated, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, message) || info.Profile != tc.profile || info.RotationCorrectionDegrees == 0 {
				t.Fatalf("quarter-turn recovery got payload=%q profile=%s correction=%.2f", got, info.Profile, info.RotationCorrectionDegrees)
			}
		})
	}
}

func TestV3ArbitraryRotationRecovery(t *testing.T) {
	key := []byte("arbitrary rotation recovery key")
	tests := []struct {
		profile Profile
		angle   float64
	}{
		{ProfileRobust, 7.5},
		{ProfileBalanced, 12.3},
		{ProfileCapacity, 22.7},
	}
	for _, tc := range tests {
		t.Run(string(tc.profile), func(t *testing.T) {
			options := DefaultOptions()
			options.Profile = tc.profile
			message := []byte("rotation payload")
			marked, err := Embed(testImage(400, 360), message, key, options)
			if err != nil {
				t.Fatal(err)
			}
			rotated := rotateBilinearForTest(marked, tc.angle)
			got, info, err := ExtractWithInfo(rotated, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, message) || info.Profile != tc.profile {
				t.Fatalf("rotated recovery got payload=%q profile=%s", got, info.Profile)
			}
			wantCorrection := -tc.angle
			if math.Abs(normalizeDegrees(info.RotationCorrectionDegrees-wantCorrection)) > 0.15 {
				t.Fatalf("rotation correction %.2f, want approximately %.2f", info.RotationCorrectionDegrees, wantCorrection)
			}
		})
	}
}

func TestV3CombinedGeometryRecovery(t *testing.T) {
	key := []byte("combined geometry recovery key")
	tests := []struct {
		name      string
		profile   Profile
		message   []byte
		transform func(image.Image) image.Image
	}{
		{
			name:    "robust-rotate-resize75",
			profile: ProfileRobust,
			message: []byte("robust payload"),
			transform: func(src image.Image) image.Image {
				rotated := rotateBilinearForTest(src, 12.3)
				return resizeBilinear(rotated, rotated.Bounds().Dx()*3/4, rotated.Bounds().Dy()*3/4)
			},
		},
		{
			name:    "balanced-rotate-resize75",
			profile: ProfileBalanced,
			message: []byte("balanced geometry payload"),
			transform: func(src image.Image) image.Image {
				rotated := rotateBilinearForTest(src, 12.3)
				return resizeBilinear(rotated, rotated.Bounds().Dx()*3/4, rotated.Bounds().Dy()*3/4)
			},
		},
		{
			name:    "capacity-rotate-crop80",
			profile: ProfileCapacity,
			message: []byte("capacity geometry payload"),
			transform: func(src image.Image) image.Image {
				rotated := rotateBilinearForTest(src, 12.3)
				b := rotated.Bounds()
				w, h := b.Dx()*4/5, b.Dy()*4/5
				x0, y0 := (b.Dx()-w)/2, (b.Dy()-h)/2
				return cropCopy(rotated, image.Rect(x0, y0, x0+w, y0+h))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			options := DefaultOptions()
			options.Profile = tc.profile
			marked, err := Embed(testImage(900, 700), tc.message, key, options)
			if err != nil {
				t.Fatal(err)
			}
			transformed := tc.transform(marked)
			got, info, err := ExtractWithInfo(transformed, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.message) || info.Profile != tc.profile {
				t.Fatalf("combined recovery got payload=%q profile=%s", got, info.Profile)
			}
			if math.Abs(info.RotationCorrectionDegrees) < 1 {
				t.Fatalf("combined recovery did not report rotation correction: %.2f", info.RotationCorrectionDegrees)
			}
		})
	}
}

func TestV3DirectLatticeBasisRecovery(t *testing.T) {
	key := []byte("direct lattice basis key")
	tests := []struct {
		name   string
		scaleX float64
		scaleY float64
	}{
		{name: "110x90", scaleX: 1.10, scaleY: 0.90},
		{name: "90x110", scaleX: 0.90, scaleY: 1.10},
		{name: "105x95", scaleX: 1.05, scaleY: 0.95},
		{name: "95x105", scaleX: 0.95, scaleY: 1.05},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			message := []byte("direct lattice")
			options := DefaultOptions()
			options.Profile = ProfileRobust
			marked, err := Embed(testImage(700, 600), message, key, options)
			if err != nil {
				t.Fatal(err)
			}
			scaled := resizeBilinear(marked,
				int(math.Round(float64(marked.Bounds().Dx())*tc.scaleX)),
				int(math.Round(float64(marked.Bounds().Dy())*tc.scaleY)))
			transformed := rotateBilinearForTest(scaled, 12.3)

			got, info, err := ExtractWithInfo(transformed, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, message) || info.Profile != ProfileRobust {
				t.Fatalf("direct-lattice basis got payload=%q profile=%s", got, info.Profile)
			}
			if math.Abs(normalizeDegrees(info.RotationCorrectionDegrees+12.3)) > 0.20 {
				t.Fatalf("rotation correction %.2f, want approximately -12.3", info.RotationCorrectionDegrees)
			}
			if math.Abs(info.ScaleXCorrection-1/tc.scaleX) > 0.001 || math.Abs(info.ScaleYCorrection-1/tc.scaleY) > 0.001 {
				t.Fatalf("scale correction x=%.4f y=%.4f", info.ScaleXCorrection, info.ScaleYCorrection)
			}
		})
	}
}

func TestDirectLatticeBasisSearchIsFixed(t *testing.T) {
	if len(latticeBasisShapes) != 4 {
		t.Fatalf("lattice basis shape count=%d, want 4", len(latticeBasisShapes))
	}
	if latticeBasisShapes[0].scaleX != 1.10 || latticeBasisShapes[0].scaleY != 0.90 ||
		latticeBasisShapes[1].scaleX != 0.90 || latticeBasisShapes[1].scaleY != 1.10 ||
		latticeBasisShapes[2].scaleX != 1.05 || latticeBasisShapes[2].scaleY != 0.95 ||
		latticeBasisShapes[3].scaleX != 0.95 || latticeBasisShapes[3].scaleY != 1.05 {
		t.Fatalf("unexpected lattice basis bank: %+v", latticeBasisShapes)
	}
	count := 0
	for range latticeBasisShapes {
		for angle := -45.0; angle <= 45.000001; angle += 0.25 {
			count++
		}
	}
	if count != 1444 {
		t.Fatalf("direct lattice probe count=%d, want 1444", count)
	}
}

func TestRotationProbeRejectsUnmarkedSyntheticImage(t *testing.T) {
	if candidates := detectRotationCandidates(newPixelPlane(testImage(400, 360))); len(candidates) != 0 {
		t.Fatalf("unmarked image produced rotation candidates: %+v", candidates)
	}
}

func TestWrongKeyOnRotatedCarrierIsBounded(t *testing.T) {
	key := []byte("rotated correct key")
	options := DefaultOptions()
	options.Profile = ProfileRobust
	marked, err := Embed(testImage(320, 288), []byte("rotated key"), key, options)
	if err != nil {
		t.Fatal(err)
	}
	rotated := rotateBilinearForTest(marked, 12.3)
	start := time.Now()
	if _, _, err := Extract(rotated, []byte("rotated wrong key!")); err == nil {
		t.Fatal("rotated carrier unexpectedly decoded with wrong key")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("rotated wrong-key extraction took %s; bounded geometry regression suspected", elapsed)
	}
}

func rotateQuarterForTest(src image.Image, turns int) *image.NRGBA {
	turns = positiveMod(turns, 4)
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	outWidth, outHeight := width, height
	if turns%2 == 1 {
		outWidth, outHeight = height, width
	}
	out := image.NewNRGBA(image.Rect(0, 0, outWidth, outHeight))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			destinationX, destinationY := x, y
			switch turns {
			case 1:
				destinationX, destinationY = y, width-1-x
			case 2:
				destinationX, destinationY = width-1-x, height-1-y
			case 3:
				destinationX, destinationY = height-1-y, x
			}
			out.Set(destinationX, destinationY, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return out
}

func rotateBilinearForTest(src image.Image, degrees float64) *image.NRGBA {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	radians := degrees * math.Pi / 180
	cosine, sine := math.Cos(radians), math.Sin(radians)
	outWidth := int(math.Ceil(math.Abs(float64(width)*cosine) + math.Abs(float64(height)*sine)))
	outHeight := int(math.Ceil(math.Abs(float64(width)*sine) + math.Abs(float64(height)*cosine)))
	out := image.NewNRGBA(image.Rect(0, 0, outWidth, outHeight))
	for i := 0; i < len(out.Pix); i += 4 {
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = 255, 255, 255, 255
	}

	sourceCenterX := float64(width-1) / 2
	sourceCenterY := float64(height-1) / 2
	outCenterX := float64(outWidth-1) / 2
	outCenterY := float64(outHeight-1) / 2
	for y := 0; y < outHeight; y++ {
		for x := 0; x < outWidth; x++ {
			dx := float64(x) - outCenterX
			dy := float64(y) - outCenterY
			sourceX := cosine*dx + sine*dy + sourceCenterX
			sourceY := -sine*dx + cosine*dy + sourceCenterY
			if sourceX < 0 || sourceY < 0 || sourceX > float64(width-1) || sourceY > float64(height-1) {
				continue
			}
			x0, y0 := int(math.Floor(sourceX)), int(math.Floor(sourceY))
			x1, y1 := x0+1, y0+1
			if x1 >= width {
				x1 = width - 1
			}
			if y1 >= height {
				y1 = height - 1
			}
			xFraction := sourceX - float64(x0)
			yFraction := sourceY - float64(y0)
			c00 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x0, bounds.Min.Y+y0)).(color.NRGBA)
			c10 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x1, bounds.Min.Y+y0)).(color.NRGBA)
			c01 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x0, bounds.Min.Y+y1)).(color.NRGBA)
			c11 := color.NRGBAModel.Convert(src.At(bounds.Min.X+x1, bounds.Min.Y+y1)).(color.NRGBA)
			out.SetNRGBA(x, y, color.NRGBA{
				R: bilinearChannel(c00.R, c10.R, c01.R, c11.R, xFraction, yFraction),
				G: bilinearChannel(c00.G, c10.G, c01.G, c11.G, xFraction, yFraction),
				B: bilinearChannel(c00.B, c10.B, c01.B, c11.B, xFraction, yFraction),
				A: 255,
			})
		}
	}
	return out
}

func TestV3AxisAlignedAffineScaleRecovery(t *testing.T) {
	key := []byte("axis aligned affine scale key")
	tests := []struct {
		name    string
		profile Profile
		message []byte
		scaleX  float64
		scaleY  float64
	}{
		{name: "robust-110x90", profile: ProfileRobust, message: []byte("affine robust"), scaleX: 1.10, scaleY: 0.90},
		{name: "balanced-90x110", profile: ProfileBalanced, message: []byte("affine balanced payload"), scaleX: 0.90, scaleY: 1.10},
		{name: "capacity-110x90", profile: ProfileCapacity, message: []byte("affine capacity payload for regression"), scaleX: 1.10, scaleY: 0.90},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			options := DefaultOptions()
			options.Profile = tc.profile
			marked, err := Embed(testImage(700, 600), tc.message, key, options)
			if err != nil {
				t.Fatal(err)
			}
			transformed := resizeBilinear(marked,
				int(math.Round(float64(marked.Bounds().Dx())*tc.scaleX)),
				int(math.Round(float64(marked.Bounds().Dy())*tc.scaleY)))
			got, info, err := ExtractWithInfo(transformed, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.message) || info.Profile != tc.profile {
				t.Fatalf("affine scale recovery got payload=%q profile=%s", got, info.Profile)
			}
			if info.ScaleXCorrection == 0 || info.ScaleYCorrection == 0 {
				t.Fatalf("affine scale recovery did not report correction: x=%.4f y=%.4f", info.ScaleXCorrection, info.ScaleYCorrection)
			}
		})
	}
}

func TestAxisAlignedAffineSearchIsFixed(t *testing.T) {
	hypotheses := axisAlignedAffineHypotheses()
	if len(hypotheses) != 36 {
		t.Fatalf("affine hypothesis count=%d, want 36", len(hypotheses))
	}
}

func inverseHomographyForTest(h homography) (homography, bool) {
	a := h.h
	det := a[0]*(a[4]*a[8]-a[5]*a[7]) - a[1]*(a[3]*a[8]-a[5]*a[6]) + a[2]*(a[3]*a[7]-a[4]*a[6])
	if math.Abs(det) < 1e-12 {
		return homography{}, false
	}
	inv := [9]float64{
		(a[4]*a[8] - a[5]*a[7]) / det,
		(a[2]*a[7] - a[1]*a[8]) / det,
		(a[1]*a[5] - a[2]*a[4]) / det,
		(a[5]*a[6] - a[3]*a[8]) / det,
		(a[0]*a[8] - a[2]*a[6]) / det,
		(a[2]*a[3] - a[0]*a[5]) / det,
		(a[3]*a[7] - a[4]*a[6]) / det,
		(a[1]*a[6] - a[0]*a[7]) / det,
		(a[0]*a[4] - a[1]*a[3]) / det,
	}
	return homography{h: inv}, true
}

func sampleImageNRGBAForTest(src image.Image, x, y float64) color.NRGBA {
	b := src.Bounds()
	if x < 0 || y < 0 || x > float64(b.Dx()-1) || y > float64(b.Dy()-1) {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := x0+1, y0+1
	if x1 >= b.Dx() {
		x1 = b.Dx() - 1
	}
	if y1 >= b.Dy() {
		y1 = b.Dy() - 1
	}
	fx, fy := x-float64(x0), y-float64(y0)
	c00 := color.NRGBAModel.Convert(src.At(b.Min.X+x0, b.Min.Y+y0)).(color.NRGBA)
	c10 := color.NRGBAModel.Convert(src.At(b.Min.X+x1, b.Min.Y+y0)).(color.NRGBA)
	c01 := color.NRGBAModel.Convert(src.At(b.Min.X+x0, b.Min.Y+y1)).(color.NRGBA)
	c11 := color.NRGBAModel.Convert(src.At(b.Min.X+x1, b.Min.Y+y1)).(color.NRGBA)
	return color.NRGBA{
		R: bilinearChannel(c00.R, c10.R, c01.R, c11.R, fx, fy),
		G: bilinearChannel(c00.G, c10.G, c01.G, c11.G, fx, fy),
		B: bilinearChannel(c00.B, c10.B, c01.B, c11.B, fx, fy),
		A: 255,
	}
}

func projectiveWarpForTest(src image.Image, candidate projectiveCandidate) *image.NRGBA {
	b := src.Bounds()
	h, ok := homographyForQuad(b.Dx(), b.Dy(), candidate.quad)
	if !ok {
		panic("invalid test homography")
	}
	inverse, ok := inverseHomographyForTest(h)
	if !ok {
		panic("non-invertible test homography")
	}
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			sx, sy, ok := inverse.mapPoint(float64(x), float64(y))
			if !ok {
				out.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
				continue
			}
			out.SetNRGBA(x, y, sampleImageNRGBAForTest(src, sx, sy))
		}
	}
	return out
}

func TestV3MildPerspectiveRecoveryEndToEnd(t *testing.T) {
	key := []byte("perspective recovery key")
	message := []byte("perspective")
	options := DefaultOptions()
	options.Profile = ProfileRobust
	marked, err := Embed(testImage(560, 512), message, key, options)
	if err != nil {
		t.Fatal(err)
	}
	candidate := projectiveHypotheses[0]
	warped := projectiveWarpForTest(marked, candidate)
	got, info, err := ExtractWithInfo(warped, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, message) {
		t.Fatalf("perspective payload = %q; want %q", got, message)
	}
	if info.PerspectiveCorrection != candidate.name {
		t.Fatalf("perspective correction = %q; want %q", info.PerspectiveCorrection, candidate.name)
	}
}

func TestBuild11PerspectiveHypothesisBound(t *testing.T) {
	if got := len(projectiveHypotheses); got != 2 {
		t.Fatalf("perspective hypothesis count = %d, want 2", got)
	}
}
