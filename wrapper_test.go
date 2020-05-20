package leopard

import (
	"bytes"
	"crypto/md5"
	"math/rand"
	"testing"
	"unsafe"

	"github.com/liamsi/go-leopard/leopard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLeo(t *testing.T) {
	assert.NoError(t, Init())
}

func TestEncodeSimple(t *testing.T) {
	const originalCount = 64
	const bufferBytes = 640

	originalData := make([][]byte, originalCount)
	for i := 0; i < originalCount; i++ {
		originalData[i] = make([]byte, bufferBytes)
		checkedRandBytes(originalData[i])
	}
	encoded, err := Encode(originalData)
	assert.NoError(t, err)
	assert.NotNil(t, encoded)
	// due to leo_encode_work_count this has to be 2*origCount
	// see: https://github.com/catid/leopard/issues/15#issuecomment-631391392
	assert.Equal(t, 2*originalCount, len(encoded))
}

func TestEncodeDecodeRoundtripSimple(t *testing.T) {
	const originalCount = 1024
	const bufferBytes = 6400
	originalData := make([][]byte, originalCount)
	for i := 0; i < originalCount; i++ {
		originalData[i] = make([]byte, bufferBytes)
		checkedRandBytes(originalData[i])
	}
	encoded, err := Encode(originalData)
	require.NoError(t, err)
	assert.EqualValues(t, 2*originalCount, len(encoded))

	// lose all orig data:
	for i := 0; i < originalCount; i++ {
		originalData[i] = nil
	}

	dec, err := Decode(originalData, encoded)
	require.NoError(t, err)
	assert.Equal(t, 2*originalCount, len(dec))
	for i := 0; i < originalCount; i++ {
		if originalData[i] == nil {
			// see if we recovered that missing data:
			assert.Equal(t, true, checkBytes(dec[i]))
		}
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	const originalCount = 32768
	const lossCount = 32768 // lose exactly originalCount of total data
	const bufferBytes = 640

	originalData := make([][]byte, originalCount)
	for i := 0; i < originalCount; i++ {
		originalData[i] = make([]byte, bufferBytes)
		checkedRandBytes(originalData[i])
	}

	encoded, err := Encode(originalData)
	require.NoError(t, err)

	// lose lossCount data:
	lostIdxs := map[int32]struct{}{}
	for len(lostIdxs) < lossCount {
		loseIdx := rand.Int31n(lossCount)
		if _, alreadyLost := lostIdxs[loseIdx]; !alreadyLost {
			encoded[loseIdx] = nil
			lostIdxs[loseIdx] = struct{}{}
		}
	}

	dec, err := Decode(originalData, encoded)
	require.NoError(t, err)
	for i := 0; i < originalCount; i++ {
		if originalData[i] == nil {
			// see if we recovered that missing data:
			assert.Equal(t, true, checkBytes(dec[i]))
		}
	}
}

func TestEncodeDecodeRoundtripRandomized(t *testing.T) {
	t.Skip("Skip time consuming randomized test")
	rounds := 100
	maxOrig := 1000
	maxBufferBytes := 1000
	for i := 0; i < rounds; i++ {
		originalCount := rand.Intn(maxOrig-1) + 1
		bufferBytes := (rand.Intn(maxBufferBytes) + 17) * 64
		decodeWorkCount := leopard.LeoDecodeWorkCount(uint32(originalCount), uint32(originalCount))
		lossCount := rand.Int31n(int32(decodeWorkCount)) + 1%int32(originalCount)

		originalData := make([][]byte, originalCount)
		for i := 0; i < originalCount; i++ {
			originalData[i] = make([]byte, bufferBytes)
			checkedRandBytes(originalData[i])
		}

		encoded, err := Encode(originalData)
		require.NoError(t, err)

		// lose lossCount data:
		lostIdxs := map[int]struct{}{}
		for len(lostIdxs) < int(lossCount) {
			loseIdx := rand.Intn(int(lossCount))
			if _, alreadyLost := lostIdxs[loseIdx]; !alreadyLost {
				encoded[loseIdx] = nil
				lostIdxs[loseIdx] = struct{}{}
			}
		}

		dec, err := Decode(originalData, encoded)
		require.NoError(t, err)
		for i := 0; i < originalCount; i++ {
			if originalData[i] == nil {
				// see if we recovered that missing data:
				assert.Equal(t, true, checkBytes(dec[i]))
			}
		}
	}
}

func TestMemRoundTrip(t *testing.T) {
	t.Skip("Skip testing private memory helper: freeAll")
	const originalCount = 128
	const bufferBytes = 640

	originalData := make([][]byte, originalCount)
	for i := 0; i < originalCount; i++ {
		originalData[i] = make([]byte, bufferBytes)
		checkedRandBytes(originalData[i])
	}
	ptrs := mockScopeFunc1(originalData)
	result := mockScopeFunc2(originalCount, ptrs, bufferBytes)
	// freeAll pointers and see if we run into any problem:
	freeAll(ptrs)
	assert.EqualValues(t, originalData, result)
}

func mockScopeFunc2(originalCount int, ptrs []unsafe.Pointer, bufferBytes int) [][]byte {
	result := make([][]byte, originalCount)
	toGoByte(ptrs, result, bufferBytes)
	return result
}

func mockScopeFunc1(originalData [][]byte) []unsafe.Pointer {
	ptrs := copyToCmallocedPtrs(originalData)
	return ptrs
}

// Helper functions for checking we can recover original data without bytes.Equal():
func checkedRandBytes(p []byte) {
	if len(p) <= md5.Size {
		panic("provided slice is too small")
	}
	raw := make([]byte, len(p)-md5.Size)
	rand.Read(raw)
	chksm := md5.Sum(raw)
	copy(p, raw)
	copy(p[len(p)-md5.Size:], chksm[:])
}

func checkBytes(p []byte) bool {
	if len(p) <= md5.Size {
		panic("provided slice is too small")
	}
	data := p[:len(p)-md5.Size]
	readChksm := p[len(p)-md5.Size:]
	chksm := md5.Sum(data)
	return bytes.Equal(readChksm, chksm[:])
}
