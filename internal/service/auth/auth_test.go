package auth

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vedhavyas/go-subkey/v2"
	"github.com/vedhavyas/go-subkey/v2/sr25519"
)

func TestSs58Decode(t *testing.T) {
	network, pubkey, err := subkey.SS58Decode("cXkgLrxnGGaApUFqPH51nMgvWgmxC2sJeRAqZMerWq4czeh2v")
	assert.NoError(t, err)
	fmt.Println(network)
	fmt.Println(hex.EncodeToString(pubkey))
}

func TestSign(t *testing.T) {
	kr, err := sr25519.Scheme{}.FromPhrase("barely aim fringe nest flush peace settle base inhale phrase vote brand", "")
	assert.NoError(t, err)
	msg := []byte("cXhA1Tp6ypNApG2menDRGtjixXkabugAcgCqE1EyAekAU6Aew1710836494")
	s, err := kr.Sign(msg)
	fmt.Println(hex.EncodeToString(s))
	assert.NoError(t, err)
	assert.True(t, kr.Verify(msg, s))
}

func TestSign1(t *testing.T) {
	kr, err := sr25519.Scheme{}.FromPhrase("barely aim fringe nest flush peace settle base inhale phrase vote brand", "")
	assert.NoError(t, err)
	var sb strings.Builder
	sb.WriteString("<Bytes>")
	sb.WriteString(kr.SS58Address(11330))
	sb.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	sb.WriteString("</Bytes>")
	ss := sb.String()
	fmt.Println(ss)
	msg := []byte(ss)
	s, err := kr.Sign(msg)
	fmt.Println(hex.EncodeToString(s))
	assert.NoError(t, err)
	assert.True(t, kr.Verify(msg, s))
}
