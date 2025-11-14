package main

import (
	"strings"
	"unicode"
)

// UTF8ToASCII converts UTF-8 text to ASCII, replacing unknown characters
func UTF8ToASCII(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		if r < 128 {
			// Standard ASCII character
			result.WriteRune(r)
		} else {
			// Try to find ASCII equivalent
			ascii := unicodeToASCII(r)
			result.WriteString(ascii)
		}
	}

	return result.String()
}

// unicodeToASCII attempts to convert a Unicode rune to ASCII equivalent
func unicodeToASCII(r rune) string {
	// Common mappings for European characters
	replacements := map[rune]string{
		// Latin-1 Supplement
		'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a",
		'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
		'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
		'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o",
		'ù': "u", 'ú': "u", 'û': "u", 'ü': "u",
		'ý': "y", 'ÿ': "y",
		'ñ': "n", 'ç': "c",
		'À': "A", 'Á': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A",
		'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E",
		'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I",
		'Ò': "O", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O",
		'Ù': "U", 'Ú': "U", 'Û': "U", 'Ü': "U",
		'Ý': "Y",
		'Ñ': "N", 'Ç': "C",
		// Quotes
		'\u2018': "'", '\u2019': "'", '\u201c': "\"", '\u201d': "\"",
		'`': "'", '\u00b4': "'",
		// Dashes
		'—': "-", '–': "-", '−': "-",
		// Other common symbols
		'•': "*", '·': ".", '…': "...",
		'©': "(c)", '®': "(R)", '™': "(TM)",
		'°': " deg", '±': "+/-",
		'×': "x", '÷': "/",
		'€': "EUR", '£': "GBP", '¥': "YEN",
	}

	if ascii, ok := replacements[r]; ok {
		return ascii
	}

	// For other characters, check if they're whitespace
	if unicode.IsSpace(r) {
		return " "
	}

	// Default to "?"
	return "?"
}
