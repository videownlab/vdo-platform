package account

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"vdo-platform/pkg/utils"

	"github.com/pkg/errors"
	"github.com/vedhavyas/go-subkey/v2"
	"github.com/vedhavyas/go-subkey/v2/sr25519"
)

func checkTimestamp(timestamp int64) error {
	if utils.Abs(time.Now().Unix()-timestamp) > 10 {
		return errors.New("invalid timestamp")
	}
	return nil
}

func VerifyDotWalletSign(dotWalletAddress string, timestamp int64, sign string) error {
	signBytes, err := hex.DecodeString(sign)
	if err != nil {
		return err
	}
	if err := checkTimestamp(timestamp); err != nil {
		return err
	}
	_, pubkeyBytes, err := subkey.SS58Decode(dotWalletAddress)
	if err != nil {
		return err
	}
	pubkey, err := sr25519.Scheme{}.FromPublicKey(pubkeyBytes)
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("<Bytes>")
	sb.WriteString(dotWalletAddress)
	sb.WriteString(strconv.FormatInt(timestamp, 10))
	sb.WriteString("</Bytes>")
	if !pubkey.Verify([]byte(sb.String()), signBytes) {
		return errors.New("invalid wallet sign")
	}
	return nil
}
