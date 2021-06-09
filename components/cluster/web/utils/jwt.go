// Copyright 2020 PingCAP, Inc.
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

package utils

import (
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gtank/cryptopasta"
)

var hmacSampleSecret = cryptopasta.NewEncryptionKey()

// Claims is a struct that will be encoded to a JWT.
type Claims struct {
	Data string `json:"data"`
	jwt.StandardClaims
}

func newClaims(issuer string, data string, expireIn time.Duration) *Claims {
	return &Claims{
		Data: data,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(expireIn).Unix(),
			Issuer:    issuer,
		},
	}
}

// NewJWTString create a JWT string by given data, expire in 24 hours.
func NewJWTString(issuer string, data string) (string, error) {
	return NewJWTStringWithExpire(issuer, data, 24*time.Hour)
}

// NewJWTStringWithExpire create a JWT string by given data and expire time
func NewJWTStringWithExpire(issuer string, data string, expireIn time.Duration) (string, error) {
	claims := newClaims(issuer, data, expireIn)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(hmacSampleSecret[:])
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// ParseJWTString parse the JWT string and return the raw data
func ParseJWTString(requiredIssuer string, tokenStr string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return hmacSampleSecret[:], nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", fmt.Errorf("token is invalid")
	}
	if claims.Issuer != requiredIssuer {
		return "", fmt.Errorf("invalid issuer")
	}
	return claims.Data, nil
}
