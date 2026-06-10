// utils/crypto.go
package utils

import (
    "golang.org/x/crypto/argon2"
    "crypto/rand"
)

func HashPassword(password string) (string, error) {
    salt := make([]byte, 16)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }

    hash := argon2.IDKey(
        []byte(password),
        salt,
        1,    // time cost
        64*1024, // memory cost (64MB)
        4,    // parallelism
        32,   // key length
    )

    // Format: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
    return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s",
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash)), nil
}

func ComparePassword(hash, password string) bool {
    // Parse and compare logic
}
