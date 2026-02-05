package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateNodeID creates a deterministic hash for a node based on file path and symbol name.
func GenerateNodeID(filePath, symbolName string) string {
	input := fmt.Sprintf("%s:%s", filePath, symbolName)
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}
