// Package security 提供密码哈希能力，对应 Java 侧的 jBCrypt PasswordUtil。
package security

import "golang.org/x/crypto/bcrypt"

// HashPassword 对应 BCrypt.hashpw(plain, BCrypt.gensalt())，cost 用 bcrypt 默认值(10)。
func HashPassword(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyPassword 对应 BCrypt.checkpw(plain, hashed)。
func VerifyPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
