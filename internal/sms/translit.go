// Package sms handles SMS encoding, decoding, assembly, and tracking.
package sms

import "strings"

// translitMap maps Cyrillic characters to Latin transliteration equivalents.
var translitMap = map[rune]string{
	'А': "A", 'Б': "B", 'В': "V", 'Г': "G", 'Д': "D",
	'Е': "E", 'Ё': "Yo", 'Ж': "Zh", 'З': "Z", 'И': "I",
	'Й': "J", 'К': "K", 'Л': "L", 'М': "M", 'Н': "N",
	'О': "O", 'П': "P", 'Р': "R", 'С': "S", 'Т': "T",
	'У': "U", 'Ф': "F", 'Х': "Kh", 'Ц': "Ts", 'Ч': "Ch",
	'Ш': "Sh", 'Щ': "Shch", 'Ъ': "'", 'Ы': "Y", 'Ь': "'",
	'Э': "E", 'Ю': "Yu", 'Я': "Ya",

	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d",
	'е': "e", 'ё': "yo", 'ж': "zh", 'з': "z", 'и': "i",
	'й': "j", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
	'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t",
	'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch",
	'ш': "sh", 'щ': "shch", 'ъ': "'", 'ы': "y", 'ь': "'",
	'э': "e", 'ю': "yu", 'я': "ya",
}

// Transliterate converts Cyrillic text to Latin text according to practical SMS Translit standard.
// Characters not present in the transliteration table are preserved as-is.
func Transliterate(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 2)

	for _, r := range text {
		if repl, ok := translitMap[r]; ok {
			b.WriteString(repl)
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}
