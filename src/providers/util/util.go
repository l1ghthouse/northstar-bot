package util

import (
	"github.com/lucasepe/codename"
	"log"
)

// CreateFunnyName generates a docker like name
func CreateFunnyName() string {
	rng, err := codename.DefaultRNG()
	if err != nil {
		log.Fatalf("Error creating random number generator: %v", err)
	}
	return codename.Generate(rng, 0)
}
