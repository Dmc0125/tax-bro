package utils

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func GetEnvVar(name string) string {
	v := os.Getenv(name)
	if v == "" {
		err := godotenv.Load()
		Assert(err == nil, fmt.Sprintf("Unable to load env from .env %s", err))
		v = os.Getenv(name)
		if v == "" {
			log.Fatalf("Env variable \"%s\" missing", name)
		}
	}
	return v
}
