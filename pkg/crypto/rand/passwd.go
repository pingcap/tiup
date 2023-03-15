// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package rand

import (
	"github.com/sethvargo/go-password/password"
)

// charsets with some in similar shapes removed (e.g., O, o, I, l, etc.)
const (
	lowerLetters = "abcdefghijkmnpqrstuvwxyz"
	upperLetters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	digits       = "0123456789"
	symbols      = "@^*+-_"
)

// Password generates a random password
func Password(length int) (string, error) {
	if length < 8 {
		panic("password length muster be at least 8.")
	}

	gi := &password.GeneratorInput{
		LowerLetters: lowerLetters,
		UpperLetters: upperLetters,
		Digits:       digits,
		Symbols:      symbols,
		Reader:       Reader,
	}
	g, err := password.NewGenerator(gi)
	if err != nil {
		return "", err
	}

	// 1/3 of the password are digits and 1/4 of it are symbols
	numDigits := length / 3
	numSymbols := length / 4
	// allow repeat if the length is longer than the shortest charset
	allowRepeat := (numDigits > len(digits) || numSymbols > len(symbols))

	return g.Generate(length, numDigits, numSymbols, false, allowRepeat)
}
