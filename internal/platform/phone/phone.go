// Package phone validates and masks E.164 phone numbers.
package phone

import "regexp"

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// Valid reports whether s is an E.164 number: a plus sign, then 8 to 15
// digits with no leading zero.
func Valid(s string) bool { return e164.MatchString(s) }

const (
	countryLen = 3 // digits shown as the country code
	nationalHd = 2 // leading national digits left visible
	tailLen    = 2 // trailing digits left visible
	minMasked  = countryLen + nationalHd + tailLen + 1
)

// Mask hides the middle of a number for display and logs:
// +24107123456 becomes "+241 07 ** ** 56". Invalid input is fully hidden.
func Mask(s string) string {
	if !Valid(s) || len(s)-1 < minMasked {
		return "***"
	}
	d := s[1:]
	head := d[:countryLen] + " " + d[countryLen:countryLen+nationalHd]
	hidden := len(d) - countryLen - nationalHd - tailLen
	return "+" + head + " " + stars(hidden) + " " + d[len(d)-tailLen:]
}

// stars returns n asterisks grouped in pairs, for example "** **" or "** *".
func stars(n int) string {
	out := make([]byte, 0, n+n/2)
	for i := 0; i < n; i++ {
		if i > 0 && i%2 == 0 {
			out = append(out, ' ')
		}
		out = append(out, '*')
	}
	return string(out)
}
