package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEd25519ByteOrder 测试标准 Ed25519 的字节序
func TestEd25519ByteOrder(t *testing.T) {
	// 生成标准 Ed25519 密钥对
	stdPubKey, stdPrivKey, err := ed25519.GenerateKey(rand.Reader)
	assert.NoError(t, err)

	message := []byte("Test")
	stdSig := ed25519.Sign(stdPrivKey, message)

	// 验证
	valid := ed25519.Verify(stdPubKey, message, stdSig)
	assert.True(t, valid)

	t.Logf("\n📊 Standard Ed25519 Format:")
	t.Logf("Public key (32 bytes): %x", stdPubKey)
	t.Logf("Signature (64 bytes): %x", stdSig)
	t.Logf("Signature R (first 32 bytes): %x", stdSig[:32])
	t.Logf("Signature S (last 32 bytes): %x", stdSig[32:])

	// 检查字节序：创建一个已知值
	testValue := big.NewInt(0x01020304)
	testBytes := testValue.Bytes()
	t.Logf("\n📊 Byte Order Test:")
	t.Logf("big.Int value: 0x%x", testValue)
	t.Logf("big.Int.Bytes() (big-endian): %x", testBytes)
	
	// 如果 big.Int.Bytes() 是 big-endian，那么：
	// 0x01020304 应该表示为 [01 02 03 04]
	// 如果是 little-endian，应该是 [04 03 02 01]
	
	if len(testBytes) > 0 {
		if testBytes[0] == 0x01 {
			t.Logf("✅ big.Int.Bytes() uses BIG-ENDIAN (most significant byte first)")
		} else if testBytes[len(testBytes)-1] == 0x01 {
			t.Logf("✅ big.Int.Bytes() uses LITTLE-ENDIAN (least significant byte first)")
		}
	}
}

// TestTssLibSignatureByteOrder 测试 tss-lib 签名的字节序
func TestTssLibSignatureByteOrder(t *testing.T) {
	// 创建一个测试值
	testR := big.NewInt(0x0102030405060708)
	testS := big.NewInt(0x0807060504030201)

	// tss-lib 格式（little-endian）
	rLE := bigIntToEncodedBytes(testR)
	sLE := bigIntToEncodedBytes(testS)

	t.Logf("\n📊 tss-lib Format (little-endian):")
	t.Logf("R value: 0x%x", testR)
	t.Logf("R encoded (little-endian): %x", rLE)
	t.Logf("S value: 0x%x", testS)
	t.Logf("S encoded (little-endian): %x", sLE)

	// 转换为 big-endian
	rBE := littleEndianToBigEndian(rLE)
	sBE := littleEndianToBigEndian(sLE)

	t.Logf("\n📊 Converted Format (big-endian):")
	t.Logf("R converted (big-endian): %x", rBE)
	t.Logf("S converted (big-endian): %x", sBE)

	// 检查 big.Int.Bytes() 的格式
	rBigIntBytes := testR.Bytes()
	sBigIntBytes := testS.Bytes()

	t.Logf("\n📊 big.Int.Bytes() Format:")
	t.Logf("R big.Int.Bytes(): %x", rBigIntBytes)
	t.Logf("S big.Int.Bytes(): %x", sBigIntBytes)

	// 对比
	t.Logf("\n📊 Comparison:")
	t.Logf("rLE vs rBE: %v", *rLE != *rBE)
	t.Logf("rBE vs rBigIntBytes (padded): need to check")
}

