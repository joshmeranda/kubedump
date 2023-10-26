package deployer

import (
	"math/rand"
)

var runeSet = []rune("0123456789abcdefghijklmnop")

func randomPostfix(length int) string {
	runes := make([]rune, length)

	var newRune rune
	for i := range runes {
		newRune = runeSet[rand.Intn(len(runeSet))]

		runes[i] = newRune
	}

	return string(runes)
}
