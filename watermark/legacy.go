package watermark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"math"
	"math/rand"
)

// extractLegacy preserves read compatibility with the original PixSeal v1 format.
func extractLegacy(src image.Image, key []byte, options Options) ([]byte, float64, error) {
	order := legacyBlockOrder(src.Bounds(), key)
	plane := newPixelPlane(src)
	minimumBits := (headerSize + tagSize) * 8
	if len(order) < minimumBits*options.Repetition {
		return nil, 0, errors.New("image is too small")
	}

	bitCount := len(order) / options.Repetition
	maximumBits := (headerSize + maxPayload + tagSize) * 8
	if bitCount > maximumBits {
		bitCount = maximumBits
	}
	bits := make([]byte, bitCount)
	confidenceSum := 0.0
	for i := 0; i < bitCount; i++ {
		votes := 0
		margin := 0.0
		for repetition := 0; repetition < options.Repetition; repetition++ {
			value := readBlockSized(plane, order[i*options.Repetition+repetition], blockSize)
			if value >= 0 {
				votes++
			} else {
				votes--
			}
			margin += math.Abs(value)
		}
		if votes > 0 {
			bits[i] = 1
		}
		confidenceSum += margin / float64(options.Repetition)
	}

	raw := bitsToBytes(whiten(bits, key, "pixseal-whiten-v1"))
	if len(raw) < headerSize+tagSize || raw[0] != magic[0] || raw[1] != magic[1] || raw[2] != 1 {
		return nil, confidenceSum / float64(bitCount), errors.New("hidden payload not found or legacy key/repetition is incorrect")
	}
	payloadLength := int(raw[3])
	packetLength := headerSize + payloadLength + tagSize
	if payloadLength > maxPayload || packetLength > len(raw) {
		return nil, 0, errors.New("invalid hidden payload length")
	}
	payload := raw[headerSize : headerSize+payloadLength]
	if crc32.ChecksumIEEE(payload) != binary.BigEndian.Uint32(raw[4:8]) {
		return nil, 0, errors.New("hidden payload damaged: CRC mismatch")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(raw[:headerSize+payloadLength])
	if !hmac.Equal(raw[headerSize+payloadLength:packetLength], mac.Sum(nil)[:tagSize]) {
		return nil, 0, errors.New("hidden payload authentication failed")
	}
	return append([]byte(nil), payload...), confidenceSum / float64(bitCount), nil
}

func legacyBlockOrder(bounds image.Rectangle, key []byte) []point {
	blocksWide := bounds.Dx() / blockSize
	blocksHigh := bounds.Dy() / blockSize
	order := make([]point, 0, blocksWide*blocksHigh)
	for y := 0; y < blocksHigh; y++ {
		for x := 0; x < blocksWide; x++ {
			order = append(order, point{bounds.Min.X + x*blockSize, bounds.Min.Y + y*blockSize})
		}
	}

	seedInput := append(append([]byte("pixseal-order-v1"), key...),
		byte(blocksWide), byte(blocksWide>>8), byte(blocksHigh), byte(blocksHigh>>8))
	hash := sha256.Sum256(seedInput)
	random := rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(hash[:8]))))
	random.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	return order
}
