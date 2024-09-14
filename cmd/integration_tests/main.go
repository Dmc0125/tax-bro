package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
	"github.com/logrusorgru/aurora"
)

type TestSuite interface {
	name() string
	execute() error
}

var tests = []TestSuite{
	&TestTransactionsGet{},
}

var rpcClient, db = setup()

func main() {
	info := aurora.Yellow("[i]").String()
	success := aurora.Green("[✓]").String()
	fail := aurora.Red("[✘]").String()

	results := make([]string, len(tests))

	for _, t := range tests {
		fmt.Printf("%s Running test \"%s\"\n", info, t.name())
		err := t.execute()
		if err == nil {
			results = append(
				results,
				fmt.Sprintf("%s Test \"%s\" succeeded\n\n", success, t.name()),
			)
		} else {
			results = append(results, fmt.Sprintf("%s Test \"%s\" failed\nerr: %s\n\n", fail, t.name(), err))
		}
	}

	for _, res := range results {
		fmt.Println(res)
	}
}

func expectErrMsg(expected any, got any, msg string) error {
	return fmt.Errorf("%s\nexpected: %v\ngot: %v", msg, expected, got)
}

func loadEnvVar(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("env var \"%s\" not provided", name)
	}
	return v
}

func setup() (*rpc.Client, *sqlx.DB) {
	dbUrl := loadEnvVar("DB_URL")
	db, err := sqlx.Open("postgres", dbUrl)
	if err != nil {
		log.Fatalf("unable to connect to db: %s", err)
	}

	rpcUrl := loadEnvVar("RPC_URL")
	rpcClient := rpc.New(rpcUrl)

	return rpcClient, db
}
