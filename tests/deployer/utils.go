package deployer

import (
	"math/rand"
	"time"
)

var runeSet = []rune("0123456789-.abcdefghijklmnop")

func randomPostfix(length int) string {
	rand.Seed(time.Now().Unix())
	runes := make([]rune, length)

	for i := range runes {
		runes[i] = runeSet[rand.Intn(len(runeSet))]
	}

	return string(runes)
}
