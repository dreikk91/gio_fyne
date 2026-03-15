package core

import (
	"bytes"
	"errors"
	"strconv"

	"cid_fyne/internal/config"
)

const (
	accountStart = 7
	accountEnd   = 11
	codeStart    = 11
	codeEnd      = 15
	minLength    = 20
	terminator   = byte(0x14)
)

var heartbeatTail = []byte("           @   ")

func IsMessageValid(message string, rules config.CidRulesConfig) bool {
	return IsMessageValidBytes([]byte(message), rules)
}

func IsHeartbeat(message string) bool {
	return IsHeartbeatBytes([]byte(message))
}

func IsMessageValidBytes(message []byte, rules config.CidRulesConfig) bool {
	if len(message) != rules.ValidLength {
		return false
	}
	return hasPrefixBytes(message, rules.RequiredPrefix)
}

func IsHeartbeatBytes(message []byte) bool {
	if len(message) != minLength {
		return false
	}
	code, ok := parseFourDigits(message[:4])
	if !ok {
		return false
	}
	return code >= 1000 && code <= 1999 && bytes.Equal(message[4:19], heartbeatTail)
}

func ChangeAccountNumber(message []byte, rules config.CidRulesConfig) ([]byte, error) {
	if len(message) < minLength {
		return nil, errors.New("invalid message length")
	}
	acc, ok := parseFourDigits(message[accountStart:accountEnd])
	if !ok {
		return nil, errors.New("invalid account number")
	}
	if acc < 1 {
		return nil, errors.New("account number out of range [0001..9999]")
	}

	code := message[codeStart:codeEnd]
	mappedCode := code
	if mapped, ok := rules.TestCodeMap[string(code)]; ok {
		mappedCode = []byte(mapped)
	}

	applied := false
	for _, rng := range rules.AccountRanges {
		if acc >= rng.From && acc <= rng.To {
			acc += rng.Delta
			applied = true
			break
		}
	}
	if !applied && acc >= 2000 && acc <= 2200 {
		// Backward compatibility with older configs.
		acc += rules.AccNumAdd
	}
	if acc < 1 || acc > 9999 {
		return nil, errors.New("account number out of range [0001..9999]")
	}
	// Fast path: fixed-width account and event code keep message size stable.
	if len(mappedCode) != codeEnd-codeStart {
		return changeAccountNumberSlow(message, acc, mappedCode), nil
	}

	out := make([]byte, len(message)+1)
	copy(out, message)
	writeFourDigits(out[accountStart:accountEnd], acc)
	copy(out[codeStart:codeEnd], mappedCode)
	out[len(message)] = terminator
	return out, nil
}

func changeAccountNumberSlow(message []byte, acc int, mappedCode []byte) []byte {
	accText := strconv.Itoa(acc)
	outCap := accountStart + len(accText) + len(mappedCode) + len(message[codeEnd:]) + 1
	out := make([]byte, 0, outCap)
	out = append(out, message[:accountStart]...)
	if len(accText) < 4 {
		for i := len(accText); i < 4; i++ {
			out = append(out, '0')
		}
	}
	out = append(out, accText...)
	out = append(out, mappedCode...)
	out = append(out, message[codeEnd:]...)
	out = append(out, terminator)
	return out
}

func hasPrefixBytes(b []byte, prefix string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(b) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if b[i] != prefix[i] {
			return false
		}
	}
	return true
}

func parseFourDigits(b []byte) (int, bool) {
	if len(b) != 4 {
		return 0, false
	}
	n := 0
	for _, ch := range b {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return n, true
}

func writeFourDigits(dst []byte, v int) {
	if len(dst) != 4 {
		return
	}
	dst[3] = byte('0' + (v % 10))
	v /= 10
	dst[2] = byte('0' + (v % 10))
	v /= 10
	dst[1] = byte('0' + (v % 10))
	v /= 10
	dst[0] = byte('0' + (v % 10))
}
