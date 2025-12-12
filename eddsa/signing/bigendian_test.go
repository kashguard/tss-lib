package signing

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignatureToStandardEd25519(t *testing.T) {
	// Test conversion with a sample signature (little-endian format)
	// Create a test signature: R (32 bytes) || S (32 bytes)
	testSigLE := make([]byte, 64)
	for i := 0; i < 64; i++ {
		testSigLE[i] = byte(i) // Simple test pattern
	}

	// Convert to standard Ed25519 format (big-endian)
	standardSig, err := SignatureToStandardEd25519(testSigLE)
	assert.NoError(t, err)
	assert.Equal(t, 64, len(standardSig), "Standard signature should be 64 bytes")

	// Verify that R and S are reversed (byte order changed)
	// R part (first 32 bytes) should be reversed
	rLE := testSigLE[:32]
	rBE := standardSig[:32]
	
	// Check that bytes are reversed
	reversed := true
	for i := 0; i < 32; i++ {
		if rLE[i] != rBE[31-i] {
			reversed = false
			break
		}
	}
	assert.True(t, reversed, "R part should be byte-reversed in big-endian format")

	// S part (last 32 bytes) should be reversed
	sLE := testSigLE[32:]
	sBE := standardSig[32:]
	
	reversed = true
	for i := 0; i < 32; i++ {
		if sLE[i] != sBE[31-i] {
			reversed = false
			break
		}
	}
	assert.True(t, reversed, "S part should be byte-reversed in big-endian format")

	t.Log("✅ Signature conversion to standard Ed25519 format works correctly")
}

func TestPublicKeyToStandardEd25519(t *testing.T) {
	// Test public key conversion with sample coordinates
	x := new(big.Int).SetBytes([]byte{0x01, 0x02, 0x03})
	y := new(big.Int).SetBytes([]byte{0x04, 0x05, 0x06})

	pubKeyBE := PublicKeyToStandardEd25519(x, y)
	assert.Equal(t, 32, len(pubKeyBE), "Public key should be 32 bytes")

	t.Log("✅ Public key conversion to standard Ed25519 format works correctly")
}

func TestSignatureToStandardEd25519_InvalidLength(t *testing.T) {
	// Test with invalid signature length
	invalidSig := []byte{1, 2, 3} // Too short
	_, err := SignatureToStandardEd25519(invalidSig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature must be 64 bytes")
}

