package kodegenerator

import (
	"fmt"
	"math/rand"
)

func GenerateKodeKelas() string {
	angka := rand.Intn(900000) + 100000
	return fmt.Sprintf("%d", angka)
}