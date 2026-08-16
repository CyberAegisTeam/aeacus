package main

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// These values are intentionally generated alongside the matching Aeacus and
// Phocus binaries by misc/dev/gen-crypto.sh for release builds.
const studioRandomString = "HASH_HERE"

var studioByteKey = []byte{0x01}

func encryptScoringData(plainText string) (string, error) {
	key := studioRandomString
	if len(key) < 64 {
		sum := sha256.Sum256([]byte(key))
		key = string(sum[:])
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write([]byte(plainText)); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	xored := xorBytes(key, compressed.String())
	return studioEncryptString(string(studioByteKey), xored)
}

func studioEncryptString(password, plainText string) (string, error) {
	key := sha256.Sum256([]byte(password))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plainText), nil)
	return string(append(nonce, sealed...)), nil
}

func xorBytes(key, input string) string {
	if len(key) == 0 {
		return input
	}
	result := make([]byte, len(input))
	for i := range result {
		result[i] = input[i] ^ key[i%len(key)]
	}
	return string(result)
}

func scoringDataFilename(config ScoringConfig) string {
	name := slug(config.Name)
	if name == "" {
		name = "scoring"
	}
	return fmt.Sprintf("%s-scoring.dat", name)
}
