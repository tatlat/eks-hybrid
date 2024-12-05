package os

import (
	"fmt"
	"math/rand"

	"github.com/tredoe/osutil/user/crypt"
	"github.com/tredoe/osutil/user/crypt/sha512_crypt"
)

func GenerateOSPassword() (string, string, error) {
	// Generate a random string for use in the salt
	const letters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const length = 8
	password := make([]byte, length)
	for i := range password {
		password[i] = letters[rand.Intn(len(letters))]
	}
	c := crypt.New(crypt.SHA512)
	s := sha512_crypt.GetSalt()
	salt := s.GenerateWRounds(s.SaltLenMax, 4096)
	hash, err := c.Generate(password, salt)
	if err != nil {
		return "", "", fmt.Errorf("generating root password: %s", err)
	}
	return string(password), string(hash), nil
}
