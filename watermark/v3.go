package watermark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"math"
)

const (
	v3Version       = 3
	v3VersionNibble = 0x30
	v3TileStride    = 251 // coprime with 448, 672 and 1120
	v3WhitenLabel   = "pixseal-whiten-v3"
)

// EmbedInfo reports deterministic properties of the v3 embed that was selected.
type EmbedInfo struct {
	Version         int
	Profile         Profile
	PayloadBytes    int
	ProfileCapacity int
	ProtectedBits   int
	TileRedundancy  float64
}

// EmbedWithInfo hides a v3 payload and reports the resolved adaptive profile.
func EmbedWithInfo(src image.Image, payload, key []byte, options Options) (*image.NRGBA, EmbedInfo, error) {
	options, err := normalizeEmbedOptions(options)
	if err != nil {
		return nil, EmbedInfo{}, err
	}
	if len(key) < 8 {
		return nil, EmbedInfo{}, errors.New("key must contain at least 8 bytes")
	}
	selected, err := SelectProfile(len(payload), options.Profile)
	if err != nil {
		return nil, EmbedInfo{}, err
	}
	if Capacity(src) == 0 {
		return nil, EmbedInfo{}, errors.New("image must be at least 280x256 pixels for a v3 hidden payload")
	}
	spec, _ := profileSpecFor(selected.Name)
	frame := makeV3Frame(payload, key, spec)
	protected := hammingEncode(whiten(bytesToBits(frame), key, v3WhitenLabel))
	if len(protected) != spec.codedBits {
		return nil, EmbedInfo{}, errors.New("internal v3 protected-frame size mismatch")
	}

	out := toNRGBA(src)
	bounds := out.Bounds()
	blocksWide := bounds.Dx() / blockSize
	blocksHigh := bounds.Dy() / blockSize
	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			tilePosition := (blockY%tileHeight)*tileWidth + blockX%tileWidth
			codeIndex := v3CodeIndex(tilePosition, spec.codedBits)
			embedBlock(out, point{blockX * blockSize, blockY * blockSize}, protected[codeIndex], options.Strength)
		}
	}

	return out, EmbedInfo{
		Version:         v3Version,
		Profile:         spec.profile,
		PayloadBytes:    len(payload),
		ProfileCapacity: spec.maxPayload,
		ProtectedBits:   spec.codedBits,
		TileRedundancy:  spec.redundancy,
	}, nil
}

func v3HeaderByte(spec profileSpec) byte {
	return v3VersionNibble | spec.id
}

func makeV3Frame(payload, key []byte, spec profileSpec) []byte {
	frame := make([]byte, spec.frameBytes)
	frame[0], frame[1] = magic[0], magic[1]
	frame[2] = v3HeaderByte(spec)
	frame[3] = byte(len(payload))
	binary.BigEndian.PutUint32(frame[4:8], crc32.ChecksumIEEE(payload))
	copy(frame[headerSize:], payload)
	mac := hmac.New(sha256.New, key)
	mac.Write(frame[:headerSize+len(payload)])
	copy(frame[headerSize+len(payload):], mac.Sum(nil)[:tagSize])
	return frame
}

func parseV3Frame(frame, key []byte, spec profileSpec) ([]byte, error) {
	if len(frame) < spec.frameBytes || frame[0] != magic[0] || frame[1] != magic[1] || frame[2] != v3HeaderByte(spec) {
		return nil, errors.New("v3 hidden payload header mismatch")
	}
	payloadLength := int(frame[3])
	if payloadLength > spec.maxPayload {
		return nil, errors.New("invalid v3 hidden payload length")
	}
	payload := frame[headerSize : headerSize+payloadLength]
	if crc32.ChecksumIEEE(payload) != binary.BigEndian.Uint32(frame[4:8]) {
		return nil, errors.New("v3 hidden payload damaged: CRC mismatch")
	}
	tagOffset := headerSize + payloadLength
	mac := hmac.New(sha256.New, key)
	mac.Write(frame[:tagOffset])
	if !hmac.Equal(frame[tagOffset:tagOffset+tagSize], mac.Sum(nil)[:tagSize]) {
		return nil, errors.New("v3 hidden payload authentication failed")
	}
	return append([]byte(nil), payload...), nil
}

func v3CodeIndex(tilePosition, codedBits int) int {
	return (tilePosition * v3TileStride) % codedBits
}

type v3SyncPoint struct {
	tilePosition int
	expected     byte
}

type v3SyncPattern struct {
	spec   profileSpec
	points []v3SyncPoint
}

func newV3SyncPattern(key []byte, spec profileSpec) v3SyncPattern {
	known := []byte{magic[0], magic[1], v3HeaderByte(spec)}
	syncBits := hammingEncode(whiten(bytesToBits(known), key, v3WhitenLabel))
	points := make([]v3SyncPoint, 0, int(math.Ceil(float64(len(syncBits))*spec.redundancy)))
	for tilePosition := 0; tilePosition < eccBits; tilePosition++ {
		codeIndex := v3CodeIndex(tilePosition, spec.codedBits)
		if codeIndex < len(syncBits) {
			points = append(points, v3SyncPoint{tilePosition: tilePosition, expected: syncBits[codeIndex]})
		}
	}
	return v3SyncPattern{spec: spec, points: points}
}

type v3PhaseCandidate struct {
	score int
	x     int
	y     int
	total int
}

func strongestV3Phases(grid []float64, pattern v3SyncPattern, count int) []v3PhaseCandidate {
	if count <= 0 {
		return nil
	}
	best := make([]v3PhaseCandidate, 0, count)
	for phaseY := 0; phaseY < tileHeight; phaseY++ {
		for phaseX := 0; phaseX < tileWidth; phaseX++ {
			score := 0
			for _, syncPoint := range pattern.points {
				logicalX := syncPoint.tilePosition % tileWidth
				logicalY := syncPoint.tilePosition / tileWidth
				observedX := positiveMod(logicalX-phaseX, tileWidth)
				observedY := positiveMod(logicalY-phaseY, tileHeight)
				observed := byte(0)
				if grid[observedY*tileWidth+observedX] >= 0 {
					observed = 1
				}
				if observed == syncPoint.expected {
					score++
				}
			}
			candidate := v3PhaseCandidate{
				score: score,
				x:     phaseX,
				y:     phaseY,
				total: len(pattern.points),
			}
			insertAt := len(best)
			for i, existing := range best {
				if candidate.score > existing.score {
					insertAt = i
					break
				}
			}
			if insertAt < count {
				best = append(best, v3PhaseCandidate{})
				copy(best[insertAt+1:], best[insertAt:])
				best[insertAt] = candidate
				if len(best) > count {
					best = best[:count]
				}
			}
		}
	}
	return best
}

func decodeV3WithPattern(grid []float64, key []byte, pattern v3SyncPattern) ([]byte, float64, int, bool) {
	phases := strongestV3Phases(grid, pattern, 4)
	bestScore := 0
	if len(phases) > 0 && phases[0].total > 0 {
		bestScore = phases[0].score * 1000 / phases[0].total
	}
	for _, phase := range phases {
		if phase.total == 0 || phase.score*4 < phase.total*3 {
			continue
		}
		coded, margin := readV3ProtectedFrame(grid, phase.x, phase.y, pattern.spec.codedBits)
		decoded := hammingDecode(coded)
		raw := bitsToBytes(whiten(decoded, key, v3WhitenLabel))
		if payload, err := parseV3Frame(raw, key, pattern.spec); err == nil {
			return payload, margin, bestScore, true
		}
	}
	return nil, 0, bestScore, false
}

func readV3ProtectedFrame(grid []float64, phaseX, phaseY, codedBits int) ([]byte, float64) {
	sums := make([]float64, codedBits)
	counts := make([]int, codedBits)
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		logicalX := logicalPosition % tileWidth
		logicalY := logicalPosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		value := grid[observedY*tileWidth+observedX]
		codeIndex := v3CodeIndex(logicalPosition, codedBits)
		sums[codeIndex] += value
		counts[codeIndex]++
	}

	coded := make([]byte, codedBits)
	margin := 0.0
	for index, value := range sums {
		if value >= 0 {
			coded[index] = 1
		}
		if counts[index] > 0 {
			margin += math.Abs(value) / float64(counts[index])
		}
	}
	return coded, margin / float64(codedBits)
}
