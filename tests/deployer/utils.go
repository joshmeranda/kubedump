package deployer

import "math/rand"

var runeSet = []rune("0123456789-.abcdefghijklmnop")

func randomPostfix(length int) string {
	runes := make([]rune, length)

	for i := range runes {
		runes[i] = runeSet[rand.Intn(len(runeSet))]
	}

	return string(runes)
}
