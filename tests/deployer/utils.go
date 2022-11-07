package deployer

import (
	"math/rand"
	"time"
)

var runeSet = []rune("0123456789abcdefghijklmnop")

func randomPostfix(length int) string {
	rand.Seed(time.Now().Unix())
	runes := make([]rune, length)

	var newRune rune
	for i := range runes {
		newRune = runeSet[rand.Intn(len(runeSet))]

		runes[i] = newRune
	}

	return string(runes)
}
