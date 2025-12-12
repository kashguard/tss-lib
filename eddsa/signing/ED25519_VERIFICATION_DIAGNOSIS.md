# Ed25519 验证失败诊断指南

## 问题描述

tss-lib 生成的签名无法通过标准 `crypto/ed25519.Verify` 验证。

## 关键发现

### RFC 8032 Ed25519 格式说明

**重要**：根据 RFC 8032，Ed25519 使用 **LITTLE-ENDIAN** 编码，不是 big-endian！

- **公钥格式**：32 字节，Y 坐标的 little-endian 编码，最高位（bit 255）表示 X 的符号
- **签名格式**：64 字节，R || S，每个都是 32 字节的 little-endian 编码

### tss-lib 输出格式

tss-lib 的内部函数已经输出 little-endian 格式：
- `bigIntToEncodedBytes()`: 返回 little-endian 格式（反转字节顺序）
- `ecPointToEncodedBytes()`: 返回 little-endian 格式的公钥

**结论**：tss-lib 的输出应该已经是标准 Ed25519 格式（little-endian）！

## 诊断步骤

### 步骤 1：检查直接验证

```go
import (
    "crypto/ed25519"
    "github.com/kashguard/tss-lib/eddsa/signing"
)

// 获取 tss-lib 签名和公钥
sigData := <-endCh
tssPubKey := signing.ecPointToEncodedBytes(keyData.EDDSAPub.X(), keyData.EDDSAPub.Y())

// 直接验证（不转换）
valid := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, sigData.Signature)
```

**如果直接验证成功**：
- ✅ tss-lib 输出已经是标准 Ed25519 格式
- ✅ 不需要转换函数
- ✅ 可以直接用于区块链

**如果直接验证失败**：继续步骤 2

### 步骤 2：检查公钥格式

标准 Ed25519 公钥编码：
- Y 坐标的 32 字节（little-endian）
- 最高位（字节 31 的第 7 位）表示 X 的符号

检查 tss-lib 公钥：
```go
// tss-lib 公钥
tssPubKey := ecPointToEncodedBytes(x, y)

// 检查格式
fmt.Printf("Public key: %x\n", tssPubKey[:])
fmt.Printf("MSB (bit 255): %d\n", (tssPubKey[31] >> 7) & 1)
```

### 步骤 3：检查签名格式

标准 Ed25519 签名：
- R: 32 字节（little-endian）
- S: 32 字节（little-endian）
- 签名 = R || S（64 字节）

检查 tss-lib 签名：
```go
fmt.Printf("Signature R: %x\n", sigData.Signature[:32])
fmt.Printf("Signature S: %x\n", sigData.Signature[32:])
fmt.Printf("R (big.Int): %s\n", new(big.Int).SetBytes(sigData.R).String())
fmt.Printf("S (big.Int): %s\n", new(big.Int).SetBytes(sigData.S).String())
```

### 步骤 4：算法层面差异检查

如果格式都正确但验证仍然失败，可能是算法层面的差异：

1. **挑战计算**：检查 `h = SHA-512(R || A || M)` 的计算是否正确
2. **标量运算**：检查 S 的计算是否符合 Ed25519 规范
3. **点编码**：检查 R 点的编码是否符合 Ed25519 规范

## 可能的原因

### 1. 公钥编码问题

**症状**：公钥格式不符合标准 Ed25519

**检查方法**：
```go
// 对比标准 Ed25519 公钥格式
stdPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
tssPubKey := ecPointToEncodedBytes(x, y)

// 检查格式差异
fmt.Printf("Standard pubkey format: %x\n", stdPubKey)
fmt.Printf("tss-lib pubkey format: %x\n", tssPubKey[:])
```

### 2. 签名格式问题

**症状**：签名格式不符合标准 Ed25519

**检查方法**：
```go
// 对比标准 Ed25519 签名格式
stdSig := ed25519.Sign(stdPrivKey, message)
fmt.Printf("Standard signature: %x\n", stdSig)
fmt.Printf("tss-lib signature: %x\n", sigData.Signature)
```

### 3. 算法不兼容

**症状**：格式正确但验证失败

**可能原因**：
- tss-lib 的 EdDSA 实现与标准 Ed25519 在算法层面有差异
- 挑战计算方式不同
- 标量运算方式不同

## 解决方案

### 方案 A：如果直接验证成功

如果 tss-lib 输出已经是标准格式，则：

```go
// 直接使用，不需要转换
valid := ed25519.Verify(
    ed25519.PublicKey(ecPointToEncodedBytes(x, y)[:]),
    message,
    sigData.Signature,
)
```

### 方案 B：如果格式需要调整

如果发现格式问题，需要修正编码函数。

### 方案 C：如果算法不兼容

如果算法层面不兼容，可能需要：
1. 修改 tss-lib 的签名生成算法
2. 或者，使用适配层进行格式转换
3. 或者，使用其他支持标准 Ed25519 的 TSS 库

## 测试代码

使用以下代码进行诊断：

```go
package main

import (
    "crypto/ed25519"
    "crypto/rand"
    "fmt"
    "math/big"
    
    "github.com/kashguard/tss-lib/eddsa/signing"
)

func diagnose(sigData *common.SignatureData, pubKeyX, pubKeyY *big.Int, message []byte) {
    // 1. 直接验证
    tssPubKey := signing.ecPointToEncodedBytes(pubKeyX, pubKeyY)
    valid := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, sigData.Signature)
    fmt.Printf("Direct verification: %v\n", valid)
    
    // 2. 使用转换函数
    standardSig, _ := signing.SignatureToStandardEd25519(sigData.Signature)
    standardPubKey := signing.PublicKeyToStandardEd25519(pubKeyX, pubKeyY)
    valid2 := ed25519.Verify(ed25519.PublicKey(standardPubKey[:]), message, standardSig)
    fmt.Printf("Converted verification: %v\n", valid2)
    
    // 3. 格式对比
    stdPubKey, stdPrivKey, _ := ed25519.GenerateKey(rand.Reader)
    stdSig := ed25519.Sign(stdPrivKey, message)
    fmt.Printf("\nStandard Ed25519 pubkey: %x\n", stdPubKey)
    fmt.Printf("tss-lib pubkey: %x\n", tssPubKey[:])
    fmt.Printf("\nStandard Ed25519 signature: %x\n", stdSig)
    fmt.Printf("tss-lib signature: %x\n", sigData.Signature)
}
```

## 下一步行动

1. **运行诊断代码**：使用上述代码检查实际格式
2. **对比格式**：对比 tss-lib 输出与标准 Ed25519 的格式差异
3. **修正问题**：根据诊断结果修正编码函数或算法

