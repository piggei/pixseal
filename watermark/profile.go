package watermark

import (
	"fmt"
	"image"
	"math"
)

// Profile controls how much of the fixed 1120-position tile is spent on
// payload capacity versus repeated observations of the protected frame.
type Profile string

const (
	ProfileAuto     Profile = "auto"
	ProfileRobust   Profile = "robust"
	ProfileBalanced Profile = "balanced"
	ProfileCapacity Profile = "capacity"
)

type profileSpec struct {
	profile    Profile
	id         byte
	maxPayload int
	frameBytes int
	codedBits  int
	redundancy float64
}

var v3Profiles = [...]profileSpec{
	{profile: ProfileRobust, id: 1, maxPayload: 16, frameBytes: 32, codedBits: 448, redundancy: 2.5},
	{profile: ProfileBalanced, id: 2, maxPayload: 32, frameBytes: 48, codedBits: 672, redundancy: float64(eccBits) / 672},
	{profile: ProfileCapacity, id: 3, maxPayload: 64, frameBytes: 80, codedBits: 1120, redundancy: 1},
}

// ProfileInfo exposes deterministic properties of one v3 profile.
type ProfileInfo struct {
	Name           Profile
	MaximumPayload int
	FrameBytes     int
	ProtectedBits  int
	TileRedundancy float64
}

// ParseProfile validates a CLI/API profile name.
func ParseProfile(value string) (Profile, error) {
	profile := Profile(value)
	switch profile {
	case ProfileAuto, ProfileRobust, ProfileBalanced, ProfileCapacity:
		return profile, nil
	default:
		return "", fmt.Errorf("invalid profile %q; expected auto, robust, balanced or capacity", value)
	}
}

func profileSpecFor(profile Profile) (profileSpec, bool) {
	for _, spec := range v3Profiles {
		if spec.profile == profile {
			return spec, true
		}
	}
	return profileSpec{}, false
}

func profileInfo(spec profileSpec) ProfileInfo {
	return ProfileInfo{
		Name:           spec.profile,
		MaximumPayload: spec.maxPayload,
		FrameBytes:     spec.frameBytes,
		ProtectedBits:  spec.codedBits,
		TileRedundancy: spec.redundancy,
	}
}

// Profiles returns the three concrete v3 profiles from most robust to largest.
func Profiles() []ProfileInfo {
	result := make([]ProfileInfo, 0, len(v3Profiles))
	for _, spec := range v3Profiles {
		result = append(result, profileInfo(spec))
	}
	return result
}

// SelectProfile resolves auto or validates an explicit profile against a
// requested payload size.
func SelectProfile(payloadBytes int, requested Profile) (ProfileInfo, error) {
	if payloadBytes < 0 {
		return ProfileInfo{}, errorsPayloadSize(payloadBytes)
	}
	minimum, ok := minimumProfile(payloadBytes)
	if !ok {
		return ProfileInfo{}, fmt.Errorf("payload is %d bytes; PixSeal v3 maximum is %d bytes", payloadBytes, maxPayload)
	}
	if requested == "" {
		requested = ProfileAuto
	}
	if requested == ProfileAuto {
		return profileInfo(minimum), nil
	}
	spec, ok := profileSpecFor(requested)
	if !ok {
		return ProfileInfo{}, fmt.Errorf("invalid profile %q; expected auto, robust, balanced or capacity", requested)
	}
	if payloadBytes > spec.maxPayload {
		return ProfileInfo{}, fmt.Errorf(
			"payload is %d bytes; profile %s capacity is %d bytes; minimum compatible profile is %s",
			payloadBytes, requested, spec.maxPayload, minimum.profile,
		)
	}
	return profileInfo(spec), nil
}

func errorsPayloadSize(payloadBytes int) error {
	return fmt.Errorf("payload size must not be negative: %d", payloadBytes)
}

func minimumProfile(payloadBytes int) (profileSpec, bool) {
	for _, spec := range v3Profiles {
		if payloadBytes <= spec.maxPayload {
			return spec, true
		}
	}
	return profileSpec{}, false
}

// ImageAnalysis combines deterministic format/geometry facts with a lightweight
// texture heuristic. Detail and recommended strength are advisory, not recovery
// guarantees.
type ImageAnalysis struct {
	Width                 int
	Height                int
	RequestedBytes        int
	ImageCompatible       bool
	RecommendedProfile    Profile
	ProfileCapacity       int
	ProfileTileRedundancy float64
	AverageObservations   float64
	Detail                string
	DetailScore           float64
	RecommendedStrength   float64
	Status                string
	Warnings              []string
}

// AnalyzeImage evaluates whether an image can carry a v3 payload of the given
// size and estimates local visual detail from a bounded sample of pixel gradients.
func AnalyzeImage(src image.Image, payloadBytes int) ImageAnalysis {
	bounds := src.Bounds()
	analysis := ImageAnalysis{
		Width:          bounds.Dx(),
		Height:         bounds.Dy(),
		RequestedBytes: payloadBytes,
	}

	detail, score, strength := estimateImageDetail(src)
	analysis.Detail = detail
	analysis.DetailScore = score
	analysis.RecommendedStrength = strength

	geometryOK := bounds.Dx()/blockSize >= tileWidth && bounds.Dy()/blockSize >= tileHeight
	profile, profileOK := minimumProfile(payloadBytes)
	if payloadBytes < 0 {
		profileOK = false
		analysis.Warnings = append(analysis.Warnings, "requested payload size must not be negative")
	} else if payloadBytes > maxPayload {
		analysis.Warnings = append(analysis.Warnings, fmt.Sprintf("requested payload exceeds the v3 maximum of %d bytes", maxPayload))
	}
	if !geometryOK {
		analysis.Warnings = append(analysis.Warnings,
			fmt.Sprintf("image is smaller than the %dx%d minimum v3 carrier geometry", tileWidth*blockSize, tileHeight*blockSize))
	}
	if exceedsPixelLimit(bounds.Dx(), bounds.Dy(), maxSearchPixels) {
		analysis.Warnings = append(analysis.Warnings,
			"large carrier: some inverse-resize candidates may exceed the decoder's 50-million-pixel normalization bound")
	}
	if detail == "low" {
		analysis.Warnings = append(analysis.Warnings,
			"low-detail imagery can make DCT changes more visible; the strength recommendation is conservative")
	}

	if profileOK {
		analysis.RecommendedProfile = profile.profile
		analysis.ProfileCapacity = profile.maxPayload
		analysis.ProfileTileRedundancy = profile.redundancy
		blocks := (bounds.Dx() / blockSize) * (bounds.Dy() / blockSize)
		if profile.codedBits > 0 {
			analysis.AverageObservations = float64(blocks) / float64(profile.codedBits)
		}
	}

	analysis.ImageCompatible = geometryOK && profileOK
	if analysis.ImageCompatible {
		analysis.Status = "suitable"
	} else {
		analysis.Status = "unsuitable"
	}
	return analysis
}

func estimateImageDetail(src image.Image) (string, float64, float64) {
	bounds := src.Bounds()
	if bounds.Dx() < 2 || bounds.Dy() < 2 {
		return "low", 0, 20
	}

	// At most roughly 256x256 sample anchors. Each anchor compares immediate
	// horizontal and vertical neighbours so the metric remains pixel-local even
	// when large images are sparsely sampled.
	stepX := int(math.Ceil(float64(bounds.Dx()-1) / 256.0))
	stepY := int(math.Ceil(float64(bounds.Dy()-1) / 256.0))
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}

	total := 0.0
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y-1; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X-1; x += stepX {
			base := pixelLuminance(src, x, y)
			total += math.Abs(base - pixelLuminance(src, x+1, y))
			total += math.Abs(base - pixelLuminance(src, x, y+1))
			count += 2
		}
	}
	if count == 0 {
		return "low", 0, 20
	}
	score := total / float64(count)
	switch {
	case score < 4:
		return "low", score, 20
	case score < 12:
		return "medium", score, 24
	default:
		return "high", score, 28
	}
}

func pixelLuminance(src image.Image, x, y int) float64 {
	r, g, b, _ := src.At(x, y).RGBA()
	return .299*float64(r>>8) + .587*float64(g>>8) + .114*float64(b>>8)
}
