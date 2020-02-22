package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// ValidateSHA256 generate SHA256 checksum of a file and compare with given value
func ValidateSHA256(file, checksum string) (bool, error) {
	// always return true if validating is not needed
	if checksum == "SKIP" {
		return true, nil
	}

	hash, err := calFileSHA256(file)
	if err != nil {
		return false, err
	}
	return hash == checksum, nil
}

func calFileSHA256(file string) (string, error) {
	f, err := ReadFile(file)
	if err != nil {
		return "", err
	}

	hashWriter := sha256.New()
	if _, err := hashWriter.Write(f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hashWriter.Sum(nil)), nil
}
