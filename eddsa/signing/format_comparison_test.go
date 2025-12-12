package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kashguard/tss-lib/common"
	"github.com/kashguard/tss-lib/eddsa/keygen"
	"github.com/kashguard/tss-lib/tss"
)

// TestFormatComparison 对比 tss-lib 输出与标准 Ed25519 的格式
func TestFormatComparison(t *testing.T) {
	// 1. 生成标准 Ed25519 密钥对和签名作为参考
	stdPubKey, stdPrivKey, err := ed25519.GenerateKey(rand.Reader)
	assert.NoError(t, err)

	message := []byte("Test message")
	stdSig := ed25519.Sign(stdPrivKey, message)
	valid := ed25519.Verify(stdPubKey, message, stdSig)
	assert.True(t, valid, "Standard Ed25519 should work")

	t.Logf("\n📊 Standard Ed25519 Format (Reference):")
	t.Logf("Public key (32 bytes): %x", stdPubKey)
	t.Logf("Signature (64 bytes): %x", stdSig)
	t.Logf("Signature R (first 32 bytes): %x", stdSig[:32])
	t.Logf("Signature S (last 32 bytes): %x", stdSig[32:])

	// 2. 获取 tss-lib 的签名和公钥
	keys, signPIDs, err := keygen.LoadKeygenTestFixtures(2)
	assert.NoError(t, err)

	msgBigInt := new(big.Int).SetBytes(message)
	pID := signPIDs[0]
	p2pCtx := tss.NewPeerContext([]*tss.PartyID{pID})
	params := tss.NewParameters(tss.Edwards(), p2pCtx, pID, 1, 0)

	outCh := make(chan tss.Message, 1)
	endCh := make(chan *common.SignatureData, 1)

	party := NewLocalParty(msgBigInt, params, keys[0], outCh, endCh)

	go func() {
		if err := party.Start(); err != nil {
			t.Errorf("party failed: %v", err)
		}
	}()

	party.Update(nil)

	var sigData *common.SignatureData
	select {
	case sigData = <-endCh:
		t.Log("✅ tss-lib signature completed")
	default:
		t.Fatal("Signing did not complete")
	}

	// 3. 获取 tss-lib 公钥（使用原始函数，little-endian）
	tssPubKey := ecPointToEncodedBytes(keys[0].EDDSAPub.X(), keys[0].EDDSAPub.Y())

	t.Logf("\n📊 tss-lib Format:")
	t.Logf("Public key (32 bytes): %x", tssPubKey[:])
	t.Logf("Signature (64 bytes): %x", sigData.Signature)
	t.Logf("Signature R (first 32 bytes): %x", sigData.Signature[:32])
	t.Logf("Signature S (last 32 bytes): %x", sigData.Signature[32:])

	// 4. 尝试直接验证（假设格式相同）
	t.Logf("\n🔍 Attempting direct verification (assuming same format):")
	validDirect := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, sigData.Signature)
	t.Logf("Direct verification result: %v", validDirect)

	if validDirect {
		t.Log("✅ SUCCESS: tss-lib output is already in standard Ed25519 format!")
		return
	}

	// 5. 如果直接验证失败，尝试各种转换
	t.Logf("\n🔍 Trying different conversion methods:")

	// 方法1：反转整个签名
	reversedSig := make([]byte, 64)
	for i := 0; i < 64; i++ {
		reversedSig[i] = sigData.Signature[63-i]
	}
	valid1 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, reversedSig)
	t.Logf("Method 1 (reversed entire signature): %v", valid1)

	// 方法2：只反转 R 和 S 部分
	reversedRS := make([]byte, 64)
	for i := 0; i < 32; i++ {
		reversedRS[i] = sigData.Signature[31-i]     // Reverse R
		reversedRS[32+i] = sigData.Signature[63-i]   // Reverse S
	}
	valid2 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, reversedRS)
	t.Logf("Method 2 (reversed R and S separately): %v", valid2)

	// 方法3：反转公钥
	reversedPubKey := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversedPubKey[i] = tssPubKey[31-i]
	}
	valid3 := ed25519.Verify(ed25519.PublicKey(reversedPubKey), message, sigData.Signature)
	t.Logf("Method 3 (reversed public key): %v", valid3)

	// 方法4：同时反转公钥和签名
	valid4 := ed25519.Verify(ed25519.PublicKey(reversedPubKey), message, reversedRS)
	t.Logf("Method 4 (reversed both): %v", valid4)

	if !valid1 && !valid2 && !valid3 && !valid4 {
		t.Log("\n❌ All conversion methods failed. This suggests:")
		t.Log("   - tss-lib's EdDSA implementation may differ from standard Ed25519 at the algorithm level")
		t.Log("   - The signature generation process may not be compatible with standard Ed25519")
		t.Log("   - Additional investigation into the algorithm differences is needed")
	}
}

