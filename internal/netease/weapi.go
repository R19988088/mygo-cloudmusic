package netease

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
)

// The web player's API, the "weapi": its requests are encrypted twice with
// AES, the key itself sealed with RSA, and they carry the site's cookies.
// What a podcast is answers only here: the client's own API serves its
// programs, but not the show they belong to.

const (
	weapiNonce    = "0CoJUm6Qyw8W8jud"
	weapiIV       = "0102030405060708"
	weapiModulus  = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7"
	weapiExponent = 65537
	weapiAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// weapiPost calls path (as "/weapi/dj/sublist") with data, and decodes the
// JSON it answers into out.
func weapiPost(path string, data map[string]any, out any) error {
	key := weapiKey()
	text, err := json.Marshal(withCSRF(data))
	if err != nil {
		return err
	}
	once, err := weapiSeal([]byte(weapiNonce), text)
	if err != nil {
		return err
	}
	params, err := weapiSeal([]byte(key), []byte(once))
	if err != nil {
		return err
	}
	form := url.Values{"params": {params}, "encSecKey": {weapiSealedKey(key)}}
	req, err := http.NewRequest("POST", apiBase+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(req, out)
}

// withCSRF adds the site's token to data, as the web player sends it.
func withCSRF(data map[string]any) map[string]any {
	if _, ok := data["csrf_token"]; ok {
		return data
	}
	for _, c := range httpClient.Jar.Cookies(siteURL) {
		if c.Name == "__csrf" {
			data["csrf_token"] = c.Value
			return data
		}
	}
	data["csrf_token"] = ""
	return data
}

// weapiKey is the AES key the second seal uses: 16 letters of the alphabet.
func weapiKey() string {
	b := make([]byte, 16)
	var src [1]byte
	for i := range b {
		rand.Read(src[:])
		b[i] = weapiAlphabet[int(src[0])%len(weapiAlphabet)]
	}
	return string(b)
}

// weapiSeal seals plain with AES-128-CBC, the IV the web player's, in base64.
func weapiSeal(key, plain []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)
	sealed := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, []byte(weapiIV)).CryptBlocks(sealed, plain)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// weapiSealedKey seals the key itself: its reversed letters, as a number,
// raised to the web player's exponent, in 128 bytes of hex.
func weapiSealedKey(key string) string {
	rev := []byte(key)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	text := hex.EncodeToString(rev)
	m, _ := new(big.Int).SetString(text, 16)
	n, _ := new(big.Int).SetString(weapiModulus, 16)
	c := new(big.Int).Exp(m, big.NewInt(weapiExponent), n)
	return fmt.Sprintf("%0256x", c)
}
