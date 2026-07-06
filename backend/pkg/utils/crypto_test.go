package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	passphrase := "test-passphrase-for-encryption"
	plaintext := "hello world"

	ciphertext, err := Encrypt(plaintext, passphrase)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := Decrypt(ciphertext, passphrase)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptDifferentPassphrases(t *testing.T) {
	plaintext := "secret data"

	ciphertext, err := Encrypt(plaintext, "key1")
	require.NoError(t, err)

	_, err = Decrypt(ciphertext, "key2")
	require.Error(t, err)
}

func TestEncryptEmptyString(t *testing.T) {
	ciphertext, err := Encrypt("", "key")
	require.NoError(t, err)

	decrypted, err := Decrypt(ciphertext, "key")
	require.NoError(t, err)
	assert.Equal(t, "", decrypted)
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	_, err := Decrypt("not-valid-base64!!!", "key")
	require.Error(t, err)
}

func TestDecryptWrongKey(t *testing.T) {
	ciphertext, err := Encrypt("data", "correct-key")
	require.NoError(t, err)

	_, err = Decrypt(ciphertext, "wrong-key")
	require.Error(t, err)
}

func TestEncryptLongString(t *testing.T) {
	long := ""
	for i := 0; i < 1000; i++ {
		long += "a"
	}

	ciphertext, err := Encrypt(long, "key")
	require.NoError(t, err)

	decrypted, err := Decrypt(ciphertext, "key")
	require.NoError(t, err)
	assert.Equal(t, long, decrypted)
}

func TestEncryptSpecialCharacters(t *testing.T) {
	special := "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"

	ciphertext, err := Encrypt(special, "key")
	require.NoError(t, err)

	decrypted, err := Decrypt(ciphertext, "key")
	require.NoError(t, err)
	assert.Equal(t, special, decrypted)
}
