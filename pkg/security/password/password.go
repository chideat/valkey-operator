/*
Copyright 2024 chideat.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package security

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// RandInt generate random number in range [0, max)
func RandInt(max int) (int, error) {
	bigNum, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(bigNum.Int64()), nil
}

// MustRandInt
func MustRandInt(max int) int {
	val, err := RandInt(max)
	if err != nil {
		panic(err)
	}
	return val
}

const (
	lowerCharDict = "abcdefghijklmnopqrstuvwxyz"
	upperCharDict = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitDict     = "0123456789"
	specialDict   = "~!@#$%^&*()-_=+?"
	all           = lowerCharDict + upperCharDict + digitDict + specialDict

	MaxPasswordLen = 128
)

// GeneratePassword return a password with at least one digit char and special char
func GeneratePassword(size int) (string, error) {
	if size < 8 {
		return "", errors.New("insecure password length")
	}

	buf := make([]byte, size)
	buf[0] = digitDict[MustRandInt(len(digitDict))]
	buf[1] = lowerCharDict[MustRandInt(len(lowerCharDict))]
	buf[2] = upperCharDict[MustRandInt(len(upperCharDict))]
	buf[3] = specialDict[MustRandInt(len(specialDict))]

	for i := 4; i < size; i++ {
		buf[i] = all[MustRandInt(len(all))]
	}
	for i := 0; i < size; i++ {
		j := MustRandInt(size)
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf), nil
}

// PasswordValidate verify password strength
func PasswordValidate(pwd string, minLen, maxLen int) error {
	if len(pwd) < minLen {
		return fmt.Errorf("password length should at least %d characters", minLen)
	}
	if len(pwd) > maxLen {
		return fmt.Errorf("password length should at most %d characters", maxLen)
	}
	var (
		letterCount  int
		upperCount   int
		numberCount  int
		specialCount int
	)
	for _, w := range pwd {
		switch {
		case unicode.IsNumber(w):
			numberCount += 1
		case unicode.IsUpper(w):
			upperCount += 1
			letterCount += 1
		case strings.Contains(specialDict, string(w)):
			specialCount += 1
		case strings.Contains(lowerCharDict, string(w)):
			letterCount += 1
		default:
			return errors.New("unsupported characters in password")
		}
	}

	if numberCount == 0 || specialCount == 0 || letterCount == 0 {
		return errors.New("password should consists of letters, number and special characters")
	}
	return nil
}
