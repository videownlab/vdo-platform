package account

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func VerifyEthWalletSign(ethWalletAddress string, dotWalletAddress string, timestamp int64, sign string) (*common.Address, error) {
	if err := checkTimestamp(timestamp); err != nil {
		return nil, err
	}
	var sb strings.Builder
	sb.WriteString(ethWalletAddress)
	sb.WriteString(dotWalletAddress)
	sb.WriteString(strconv.FormatInt(timestamp, 10))
	return EthVerifySignByAddress(ethWalletAddress, sb.String(), sign)
}

const EthSignStr = "Ethereum Signed Message:"

func ethSignSpec(originMsg string) string {
	return fmt.Sprintf("\x19%s\n%d%s", EthSignStr, len(originMsg), originMsg)
}

// 验证以太坊签名, 地址格式容错，签名带0x兼容
func EthVerifySignByAddress(address string, msg string, sign string) (*common.Address, error) {
	sign = strings.TrimPrefix(sign, "0x")

	if len(sign) != 130 {
		return nil, fmt.Errorf("invalid sign length")
	}

	// 计算 消息的  keccak256 hash
	msg = ethSignSpec(msg)
	dataHash := crypto.Keccak256Hash([]byte(msg))

	// 签名格式 string -> 字节
	signature, err := hex.DecodeString(sign)
	if err != nil {
		return nil, err
	}

	// the last part: V if greater and equal 0x1b (27 by decimal)
	if signature[64] >= 0x1b {
		signature[64] -= 0x1b
	}

	// 由 msgHash + 签名 ==> PublicKey
	sigPublicKeyECDSA, err := crypto.SigToPub(dataHash.Bytes(), signature)
	if err != nil {
		return nil, err
	}

	addr := crypto.PubkeyToAddress(*sigPublicKeyECDSA)
	if !strings.EqualFold(addr.Hex(), address) {
		return nil, fmt.Errorf("invalid signature or wallet address")
	}
	return &addr, nil
}

// 验证以太坊签名
func EthVerifySignByPubkey(pubKey string, msg string, sign string) bool {
	// dataHash := sha256.Sum256([]byte(msg))
	dataHash := crypto.Keccak256Hash([]byte(msg))

	sign = strings.TrimPrefix(sign, "0x")
	signature, err := hex.DecodeString(sign)
	if err != nil {
		return false
	}

	pubkey, err := hex.DecodeString(pubKey)
	if err != nil {
		return false
	}
	return crypto.VerifySignature(pubkey, dataHash[:], signature[:len(signature)-1])
}

// 以太坊签名
func EthSign(privKey string, msg string) (string, error) {
	privateKey, err := crypto.HexToECDSA(privKey)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %v", err)
	}

	msg = ethSignSpec(msg)
	dataHash := crypto.Keccak256Hash([]byte(msg))
	sig, err := crypto.Sign(dataHash.Bytes(), privateKey)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sig), nil
}
