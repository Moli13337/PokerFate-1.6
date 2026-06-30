package ws

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

func VerifySign(random, ts, sign, salt string) bool {
	raw := fmt.Sprintf("random=%s&ts=%s&salt=%s", random, ts, salt)
	h := md5.Sum([]byte(raw))
	return hex.EncodeToString(h[:]) == sign
}

func VerifyGuest(os, imei, verify, guestSalt string) bool {
	raw := os + imei + guestSalt
	h := md5.Sum([]byte(raw))
	return hex.EncodeToString(h[:]) == verify
}

func DoubleMD5(password string) string {
	h1 := md5.Sum([]byte(password))
	h2 := md5.Sum(h1[:])
	return hex.EncodeToString(h2[:])
}

func MakeSign(random, ts, salt string) string {
	raw := fmt.Sprintf("random=%s&ts=%s&salt=%s", random, ts, salt)
	h := md5.Sum([]byte(raw))
	return hex.EncodeToString(h[:])
}

func PackTypeToMessageName(packType string) string {
	return strings.TrimPrefix(packType, "pb.")
}

func MessageNameToPackType(msgName string) string {
	return "pb." + msgName
}
