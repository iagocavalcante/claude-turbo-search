package vector

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

func HexBlobToFloat64(hexStr string) ([]float64, error) {
	bytesVal, err := hex.DecodeString(strings.TrimSpace(hexStr))
	if err != nil {
		return nil, err
	}
	if len(bytesVal)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding blob length: %d", len(bytesVal))
	}
	out := make([]float64, 0, len(bytesVal)/4)
	for i := 0; i < len(bytesVal); i += 4 {
		bits := binary.LittleEndian.Uint32(bytesVal[i : i+4])
		out = append(out, float64(math.Float32frombits(bits)))
	}
	return out, nil
}

func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	dot := 0.0
	normA := 0.0
	normB := 0.0
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
