package rpi

import (
	"regexp"
	"strconv"
)

// rpiHexNumber matches unquoted 0x-prefixed hex literals that GoldHEN RPI
// emits in JSON (e.g. "length": 0xFD4C65000). Strict JSON parsers reject them.
var rpiHexNumber = regexp.MustCompile(`(:\s*)0[xX]([0-9A-Fa-f]+)`)

func sanitizeHexNumbers(body []byte) []byte {
	return rpiHexNumber.ReplaceAllFunc(body, func(match []byte) []byte {
		groups := rpiHexNumber.FindSubmatch(match)
		if len(groups) != 3 {
			return match
		}
		value, err := strconv.ParseUint(string(groups[2]), 16, 64)
		if err != nil {
			return match
		}
		return append(groups[1], strconv.FormatUint(value, 10)...)
	})
}
