package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePasswordComplexityValid(t *testing.T) {
	tests := []string{
		"Abcd1234!",
		"P@ssw0rd",
		"Strong#Pass1",
		"MyP@ssw0rd",
		"Test1234#",
	}
	for _, pw := range tests {
		err := ValidatePasswordComplexity(pw)
		require.NoError(t, err, "expected %q to be valid", pw)
	}
}

func TestValidatePasswordComplexityTooShort(t *testing.T) {
	err := ValidatePasswordComplexity("Ab1!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "8")
}

func TestValidatePasswordComplexityNoUppercase(t *testing.T) {
	err := ValidatePasswordComplexity("abcd1234!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uppercase")
}

func TestValidatePasswordComplexityNoLowercase(t *testing.T) {
	err := ValidatePasswordComplexity("ABCD1234!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lowercase")
}

func TestValidatePasswordComplexityNoDigit(t *testing.T) {
	err := ValidatePasswordComplexity("Abcdefgh!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "digit")
}

func TestValidatePasswordComplexityNoSpecialChar(t *testing.T) {
	err := ValidatePasswordComplexity("Abcd1234")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "special")
}
