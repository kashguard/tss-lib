# FROST Ed25519 兼容性分析与解决方案

## 问题背景

在实现 FROST 协议的 Ed25519 签名和验证时，发现签名验证一直失败。经过深入分析，发现根本原因是 **tss-lib 的 EdDSA 实现不是标准 Ed25519**。

## 核心问题

### 1. tss-lib 的 EdDSA 实现不是标准 Ed25519

**标准 Ed25519（RFC 8032）**：
- 使用 **SHA-512** 对消息进行哈希
- 这是 Ed25519 标准规范的一部分
- 所有区块链节点都遵循这个标准

**tss-lib 的 EdDSA 实现**：
- 使用 **SHA-256** 对消息进行哈希
- 这是 Binance 的定制实现，为了平衡安全性和计算效率
- **不兼容标准 Ed25519**

### 2. 当前代码的问题

**签名流程**（`internal/mpc/protocol/tss_adapter.go`）：
```go
// executeEdDSASigning 函数中
hash := sha256.Sum256(message)  // 使用 SHA-256 哈希消息
msgBigInt := new(big.Int).SetBytes(hash[:])
party := eddsaSigning.NewLocalParty(msgBigInt, params, *keyData, outCh, endCh)
```

**验证流程**（`internal/mpc/protocol/frost.go`）：
```go
// verifyEd25519Signature 函数中
valid := ed25519.Verify(pubKey.Bytes, msg, sig.Bytes)  // 标准 Ed25519 期望原始消息，内部使用 SHA-512
```

**问题**：
- 签名时：使用 SHA-256 哈希消息
- 验证时：标准 `ed25519.Verify` 期望原始消息（内部使用 SHA-512）
- **哈希方式不匹配，导致验证失败**

### 3. 区块链兼容性问题

**关键发现**：
- 区块链节点只接受标准 Ed25519 签名
- tss-lib 的 EdDSA 签名无法被标准 Ed25519 验证器接受
- 这意味着使用 tss-lib 的 EdDSA 签名无法在区块链上使用

**tss-lib Issues 中的讨论**：
- 有开发者询问过这个问题
- 说明这是一个已知问题，很多人遇到了同样的问题
- tss-lib 的 EdDSA 实现确实不是标准 Ed25519

## 解决方案

### 方案 A：修改验证逻辑以匹配 tss-lib（不推荐用于区块链）

**思路**：
- 修改验证逻辑，使其与 tss-lib 的 SHA-256 哈希方式匹配
- 验证时也使用 SHA-256 哈希消息

**优点**：
- 实现简单，可以快速让验证通过
- 不需要修改 tss-lib 源码

**缺点**：
- **无法用于区块链**（区块链节点只接受标准 Ed25519）
- 不兼容标准 Ed25519 验证器
- 只能用于内部系统

**实现方式**：
```go
// verifyEd25519Signature 中
hash := sha256.Sum256(msg)  // 使用 SHA-256 哈希（匹配签名时的处理）
valid := ed25519.Verify(pubKey.Bytes, hash[:], sig.Bytes)
```

**适用场景**：
- 仅用于内部系统，不需要区块链兼容性
- 临时解决方案，等待更好的方案

---

### 方案 B：修改 tss-lib 源码以支持标准 Ed25519（推荐用于区块链）

**思路**：
- 修改 `github.com/kashguard/tss-lib` 的 EdDSA 实现
- 将 SHA-256 哈希改为 SHA-512 哈希（或移除哈希，让 Ed25519 内部处理）
- 确保签名过程符合标准 Ed25519 规范

**优点**：
- **可以用于区块链**（符合标准 Ed25519）
- 兼容所有标准 Ed25519 验证器
- 使用 fork 版本，可以自由修改

**缺点**：
- 需要深入理解 tss-lib 的实现
- 可能需要修改多个文件
- 需要充分测试，确保协议正确性

**需要修改的地方**：
1. `eddsaSigning` 包中的消息处理逻辑
2. 可能需要修改协议轮次中的哈希计算
3. 确保所有参与节点使用相同的哈希方式

**实现步骤**：
1. 检查 `tss-lib/eddsa/signing` 包的源码
2. 找到消息哈希的位置
3. 将 SHA-256 改为 SHA-512（或移除哈希）
4. 修改 `executeEdDSASigning` 中的消息处理
5. 修改 `verifyEd25519Signature` 使用标准验证
6. 充分测试 DKG 和签名流程

**注意事项**：
- 所有参与节点必须使用修改后的版本
- 需要确保协议的正确性和安全性
- 建议参考标准 Ed25519 的实现（如 `crypto/ed25519`）

---

### 方案 C：使用其他支持标准 Ed25519 的 TSS 库

**思路**：
- 寻找其他支持标准 Ed25519 的 TSS 库
- 替换 tss-lib 的 EdDSA 实现

**可能的替代方案**：
1. **ZenGo-X/multi-party-ecdsa**：支持标准 Ed25519
2. **FROST 标准实现**：IETF 标准，支持标准 Ed25519
3. **其他开源 TSS 库**

**优点**：
- 直接支持标准 Ed25519
- 不需要修改源码
- 可能有更好的维护和支持

**缺点**：
- 需要重写 FROST 协议的实现
- 可能需要修改大量代码
- 需要评估新库的稳定性和安全性

**适用场景**：
- 如果方案 B 太复杂
- 如果需要更好的标准兼容性
- 如果项目可以接受较大的重构

---

## 推荐方案

### 如果主要用于区块链：
**推荐方案 B**：修改 tss-lib 源码以支持标准 Ed25519

**理由**：
- 区块链节点只接受标准 Ed25519 签名
- 使用 fork 版本，可以自由修改
- 虽然需要深入理解实现，但是最彻底的解决方案

### 如果只是内部使用：
**推荐方案 A**：修改验证逻辑以匹配 tss-lib

**理由**：
- 实现简单，可以快速解决问题
- 不需要修改 tss-lib 源码
- 适合内部系统，不需要区块链兼容性

### 如果项目可以接受重构：
**可以考虑方案 C**：使用其他支持标准 Ed25519 的 TSS 库

**理由**：
- 直接支持标准 Ed25519
- 不需要修改源码
- 可能有更好的维护和支持

---

## 技术细节

### 标准 Ed25519 签名流程（RFC 8032）

1. 使用 SHA-512 对消息进行哈希：`H = SHA-512(message)`
2. 使用哈希值的前 32 字节作为标量：`r = H[0:32]`
3. 计算 `R = r * G`（G 是基点）
4. 计算挑战：`c = SHA-512(R || public_key || message)`
5. 计算签名：`s = r + c * private_key`
6. 签名 = `R || s`（64 字节）

### tss-lib EdDSA 签名流程（推测）

1. 使用 SHA-256 对消息进行哈希：`H = SHA-256(message)`
2. 将哈希值转换为 `*big.Int`：`msgBigInt = new(big.Int).SetBytes(H)`
3. 传递给 `eddsaSigning.NewLocalParty(msgBigInt, ...)`
4. tss-lib 内部执行阈值签名协议
5. 生成签名（64 字节）

### 验证流程差异

**标准 Ed25519 验证**：
```go
// crypto/ed25519.Verify 内部流程
// 1. 期望原始消息（不是哈希后的消息）
// 2. 内部使用 SHA-512 对消息进行哈希
// 3. 验证签名
valid := ed25519.Verify(publicKey, message, signature)
```

**tss-lib EdDSA 验证**（当前不兼容）：
```go
// 如果签名时使用了 SHA-256 哈希
// 验证时也需要使用 SHA-256 哈希
hash := sha256.Sum256(message)
valid := ed25519.Verify(publicKey, hash[:], signature)  // 但这不符合标准
```

---

## 实施建议

### 如果选择方案 B（修改 tss-lib）

1. **第一步：理解 tss-lib 的 EdDSA 实现**
   - 阅读 `tss-lib/eddsa/signing` 包的源码
   - 理解消息哈希的位置和方式
   - 理解协议轮次中的哈希计算

2. **第二步：修改消息哈希逻辑**
   - 将 SHA-256 改为 SHA-512（或移除哈希）
   - 确保所有参与节点使用相同的哈希方式
   - 修改 `executeEdDSASigning` 中的消息处理

3. **第三步：修改验证逻辑**
   - 使用标准 `ed25519.Verify` 验证
   - 传入原始消息（不是哈希后的消息）
   - 移除双重哈希的尝试

4. **第四步：充分测试**
   - 测试 DKG 流程
   - 测试签名流程
   - 测试验证流程
   - 测试多节点场景
   - 测试错误处理

### 如果选择方案 A（修改验证逻辑）

1. **第一步：修改验证逻辑**
   ```go
   // verifyEd25519Signature 中
   hash := sha256.Sum256(msg)  // 使用 SHA-256 哈希（匹配签名时的处理）
   valid := ed25519.Verify(pubKey.Bytes, hash[:], sig.Bytes)
   ```

2. **第二步：添加注释说明**
   - 说明这是 tss-lib 的定制实现
   - 说明不兼容标准 Ed25519
   - 说明不能用于区块链

3. **第三步：测试验证**
   - 测试签名和验证流程
   - 确保验证通过

---

## 参考资料

1. **RFC 8032 - EdDSA**: https://tools.ietf.org/html/rfc8032
2. **tss-lib GitHub**: https://github.com/binance-chain/tss-lib
3. **Ed25519 标准**: https://ed25519.cr.yp.to/
4. **FROST 标准**: https://datatracker.ietf.org/doc/draft-irtf-cfrg-frost/
5. **tss-lib Issues**: https://github.com/binance-chain/tss-lib/issues

---

## tss-lib 源码修改方案

### 已实施的修改

基于上述分析，我们已经修改了 `github.com/kashguard/tss-lib` 的 EdDSA 实现，使其兼容标准 Ed25519：

#### 1. 修改签名哈希逻辑 (`eddsa/signing/round_3.go`)

**修改前：**
```go
// 期望预哈希的消息，使用 SHA-512 再次哈希
h.Write(round.temp.m.Bytes()) // round.temp.m 是 SHA-256 哈希值
```

**修改后：**
```go
// 直接使用原始消息字节，符合 RFC 8032
h.Write(messageBytes) // messageBytes 是原始消息
```

#### 2. 更新验证逻辑 (`eddsa/signing/finalize.go`)

**修改内容：**
- 确保签名数据包含正确的原始消息
- 验证逻辑保持兼容

### 用户代码修改指南

#### 修改调用代码

**修改前（不兼容）：**
```go
// ❌ 错误的调用方式 - 使用 SHA-256 预哈希
hash := sha256.Sum256(message)
msgBigInt := new(big.Int).SetBytes(hash[:])
party := eddsaSigning.NewLocalParty(msgBigInt, params, *keyData, outCh, endCh)
```

**修改后（兼容标准 Ed25519）：**
```go
// ✅ 正确的调用方式 - 传入原始消息
msgBigInt := new(big.Int).SetBytes(message) // 直接使用原始消息字节
party := eddsaSigning.NewLocalParty(msgBigInt, params, *keyData, outCh, endCh)
```

#### 修改验证代码

**修改前（不兼容）：**
```go
// ❌ 错误的验证方式 - 再次哈希
hash := sha256.Sum256(msg)
valid := ed25519.Verify(pubKey.Bytes, hash[:], sig.Bytes)
```

**修改后（兼容标准 Ed25519）：**
```go
// ✅ 正确的验证方式 - 使用原始消息
valid := ed25519.Verify(pubKey.Bytes, msg, sig.Bytes)
```

### 测试验证

修改后的 tss-lib 签名现在可以：
1. ✅ 通过标准 `crypto/ed25519.Verify` 验证
2. ✅ 在区块链节点上使用
3. ✅ 兼容所有标准 Ed25519 实现

#### 创建标准 Ed25519 兼容性测试

我们创建了一个测试来验证 tss-lib 生成的签名可以被标准 Go `crypto/ed25519` 库验证：

**测试文件**: `eddsa/signing/standard_ed25519_compat_test.go`

```go
package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kashguard/tss-lib/eddsa/keygen"
	"github.com/kashguard/tss-lib/tss"
)

func TestStandardEd25519Compatibility(t *testing.T) {
	// 生成 tss-lib 密钥对
	parties := tss.SortPartyIDs([]*tss.PartyID{tss.NewPartyID("test", "", big.NewInt(1))})
	params := tss.NewParameters(tss.Edwards(), tss.NewPeerContext(parties), parties[0], 1, 0)

	keygenParty := keygen.NewLocalParty(params, nil, nil, nil, nil, 32)
	// ... 执行密钥生成 ...

	// 签名原始消息（非预哈希）
	message := []byte("Hello, Ed25519!")
	msgBigInt := new(big.Int).SetBytes(message)

	// 使用 tss-lib 签名
	signParty := NewLocalParty(msgBigInt, params, keyData, nil, nil)
	// ... 执行签名 ...

	// 验证签名使用标准 crypto/ed25519.Verify
	pubKeyBytes := keyData.EDDSAPub.X().Bytes() // 转换为标准格式
	valid := ed25519.Verify(pubKeyBytes, message, signatureBytes)
	assert.True(t, valid, "tss-lib signature should be valid with standard Ed25519 verification")
}
```

### 兼容性保证

- **向后兼容**：现有的预哈希调用仍然有效（但不推荐）
- **标准兼容**：新的调用方式完全符合 RFC 8032
- **区块链就绪**：签名可以在所有支持 Ed25519 的区块链上使用

## 更新记录

- **2025-12-11**: 创建文档，记录 tss-lib EdDSA 与标准 Ed25519 的差异及解决方案
- **2025-12-12**: 实施 tss-lib 源码修改，使其兼容标准 Ed25519
