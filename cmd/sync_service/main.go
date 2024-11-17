package main

import (
	"tax-bro/pkg/logger"
	"tax-bro/pkg/utils"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	utils.AssertNoErr(err)

	logger.NewPrettyLogger("", 0)

	// dbUrl := utils.GetEnvVar("DB_URL")
	// rpcUrl := utils.GetEnvVar("RPC_URL")

}
