package ussd

import "testing"

// FuzzParseResponse ensures ParseResponse never panics on arbitrary network byte streams.
func FuzzParseResponse(f *testing.F) {
	seeds := []string{
		`+CUSD: 0,"Your balance is 15.50 USD",15`,
		`+CUSD: 1,"1. Tariffs\n2. Services\n0. Exit",15`,
		`+CUSD: 0,"04110430043B0430043D0441003A00200031003500300020044004430431",72`,
		`+CUSD: 2`,
		`+CUSD: 4`,
		`+CUSD: 5`,
		`+CUSD: 0,"Balance: OK"`,
		`+CUSD:`,
		`+CUSD: 999,"",-1`,
		`+CUSD: 0,""`,
		`+CUSD: 0,"invalid_hex_with_odd_len",72`,
		`+CUSD: 0,"0411ZZZZ",72`,
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = ParseResponse(input)
	})
}
