package signing

import (
	"crypto/ed25519"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kashguard/tss-lib/common"
	"github.com/kashguard/tss-lib/eddsa/keygen"
	"github.com/kashguard/tss-lib/tss"
)

func TestStandardEd25519Compatibility(t *testing.T) {
	// 使用现有的测试fixture，设置阈值等于参与者数减1（允许单参与者签名）
	threshold := 0 // 阈值为0，意味着只需要1个参与者签名
	keys, signPIDs, err := keygen.LoadKeygenTestFixtures(1)
	assert.NoError(t, err, "should load keygen fixtures")

	// 准备要签名的原始消息（不预哈希）
	originalMessage := []byte("Hello, FROST Ed25519 Standard Compatibility Test!")
	msgBigInt := new(big.Int).SetBytes(originalMessage)

	// 设置单参与者上下文
	pIDs := signPIDs[:1] // 只使用第一个参与者
	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*LocalParty, 0, len(pIDs))

	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *common.SignatureData, len(pIDs))

	// 创建签名参与者
	for i := 0; i < len(pIDs); i++ {
		params := tss.NewParameters(tss.Edwards(), p2pCtx, pIDs[i], len(pIDs), threshold)
		P := NewLocalParty(msgBigInt, params, keys[i], outCh, endCh).(*LocalParty)
		parties = append(parties, P)
	}

	// 开始签名协议
	for _, P := range parties {
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				t.Errorf("party %s failed to start: %v", P.PartyID(), err)
			}
		}(P)
	}

	// 处理消息直到完成
	done := make(chan bool, 1)
	go func() {
		for {
			select {
			case msg := <-outCh:
				// 单参与者模式下不需要转发消息
				_ = msg
			case <-endCh:
				done <- true
				return
			}
		}
	}()

	// 等待签名完成
	<-done

	// 获取签名结果
	var signatureData *common.SignatureData
	select {
	case signatureData = <-endCh:
		t.Log("✅ Signature completed successfully")
	default:
		t.Fatal("❌ Signing did not complete")
	}

	// 从密钥中提取公钥
	firstKey := keys[0]
	pubKeyX := firstKey.EDDSAPub.X()
	pubKeyY := firstKey.EDDSAPub.Y()

	// 转换tss-lib公钥为标准Ed25519格式
	// Ed25519公钥是32字节的Y坐标，符号位在最高位
	pubKeyBytes := make([]byte, 32)
	yBytes := pubKeyY.Bytes()

	// 复制Y坐标的字节（注意字节序转换）
	for i, b := range yBytes {
		if i < 32 {
			pubKeyBytes[i] = b
		}
	}

	// 如果X坐标的最低位是1，设置符号位
	if pubKeyX.Bit(0) == 1 {
		pubKeyBytes[31] |= 0x80
	}

	// 使用标准crypto/ed25519.Verify验证签名
	valid := ed25519.Verify(ed25519.PublicKey(pubKeyBytes), originalMessage, signatureData.Signature)

	// 断言验证成功
	assert.True(t, valid, "tss-lib EdDSA signature should be verifiable with standard Ed25519")

	if valid {
		t.Log("✅ SUCCESS: tss-lib EdDSA signature is compatible with standard Ed25519 verification!")
		t.Logf("Original message: %s", originalMessage)
		t.Logf("Signature length: %d bytes", len(signatureData.Signature))
		t.Logf("Public key (first 8 bytes): %x", pubKeyBytes[:8])
		t.Logf("Message in signature data: %s", signatureData.M)
		t.Logf("Signature R component: %x", signatureData.R)
		t.Logf("Signature S component: %x", signatureData.S)
	} else {
		t.Error("❌ FAILURE: tss-lib EdDSA signature is NOT compatible with standard Ed25519")

		// 调试信息
		t.Logf("Signature bytes: %x", signatureData.Signature)
		t.Logf("Public key bytes: %x", pubKeyBytes)
		t.Logf("Message bytes: %x", originalMessage)

		// 尝试用标准库生成签名进行对比
		stdPubKey, stdPrivKey, _ := ed25519.GenerateKey(nil)
		stdSignature := ed25519.Sign(stdPrivKey, originalMessage)
		stdValid := ed25519.Verify(stdPubKey, originalMessage, stdSignature)
		t.Logf("Standard Ed25519 test (should pass): %v", stdValid)
		t.Logf("Standard signature length: %d bytes", len(stdSignature))
	}
}

func TestEd25519SignatureFormat(t *testing.T) {
	// 测试签名格式是否符合Ed25519标准
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(1, 1)
	assert.NoError(t, err)

	message := []byte("Test Ed25519 signature format")
	msgBigInt := new(big.Int).SetBytes(message)

	pID := signPIDs[0]
	p2pCtx := tss.NewPeerContext([]*tss.PartyID{pID})
	params := tss.NewParameters(tss.Edwards(), p2pCtx, pID, 1, 0)

	out := make(chan tss.Message, 1)
	end := make(chan *common.SignatureData, 1)

	party := NewLocalParty(msgBigInt, params, keys[0], out, end)

	go func() {
		if err := party.Start(); err != nil {
			t.Errorf("party failed to start: %v", err)
		}
	}()

	party.Update(nil)

	var sigData *common.SignatureData
	select {
	case sigData = <-end:
		// Ed25519签名应该是64字节
		assert.Equal(t, 64, len(sigData.Signature), "Ed25519 signature should be 64 bytes")

		// 验证R和S组件
		assert.NotEmpty(t, sigData.R, "R component should not be empty")
		assert.NotEmpty(t, sigData.S, "S component should not be empty")

		// 验证消息被正确保存
		assert.Equal(t, message, sigData.M, "Original message should be preserved in signature data")

		t.Logf("✅ Signature format verified: length=%d, R=%x..., S=%x...",
			len(sigData.Signature), sigData.R[:8], sigData.S[:8])

	default:
		t.Fatal("Signing did not complete")
	}
}
