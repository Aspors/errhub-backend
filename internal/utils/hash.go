package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func GenerateIssueHash(projectID, errType, errMessage string) string {
	data := fmt.Sprintf("%s:%s:%s", projectID, errType, errMessage)
	
	h := sha256.Sum256([]byte(data))
	
	return hex.EncodeToString(h[:])
}
