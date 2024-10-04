package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
)

type cache struct {
	Args     string `json:"args"`
	Accounts int    `json:"accounts"`
}

func (c *cache) save(cacheFile io.Writer) {
	cacheJson, err := json.Marshal(c)
	utils.Assert(err == nil, fmt.Sprint(err))
	fmt.Fprint(cacheFile, string(cacheJson))
}

var errNoCache = errors.New("no cache")

func loadCache(solanaDir string, cachePath string) (cache, error) {
	info, err := os.Stat(solanaDir)

	if err == nil && info.IsDir() {
		content, err := os.ReadFile(cachePath)
		utils.Assert(err == nil, fmt.Sprint(err))

		c := cache{}
		err = json.Unmarshal(content, &c)
		return c, err
	}

	return cache{}, errNoCache
}

func savePrivateKey(solanaDir string, wallet *solana.Wallet) {
	file, err := os.Create(filepath.Join(solanaDir, fmt.Sprintf("./pk_%s.json", wallet.PublicKey().String())))
	utils.Assert(err == nil, fmt.Sprint(err))

	_, err = fmt.Fprint(file, "[")
	utils.Assert(err == nil, fmt.Sprint(err))
	for i, b := range wallet.PrivateKey {
		if i > 0 && i < len(wallet.PrivateKey) {
			_, err = fmt.Fprint(file, ",")
			utils.Assert(err == nil, fmt.Sprint(err))
		}
		_, err = fmt.Fprintf(file, "%d", b)
		utils.Assert(err == nil, fmt.Sprint(err))
	}
	_, err = fmt.Fprint(file, "]")
	utils.Assert(err == nil, fmt.Sprint(err))
}

func saveData(solanaDir string, pubKey string) string {
	p := filepath.Join(solanaDir, fmt.Sprintf("./data_%s.json", pubKey))
	file, err := os.Create(p)
	utils.Assert(err == nil, fmt.Sprint(err))

	_, err = fmt.Fprintf(
		file,
		`{"pubkey": "%s","account": {"lamports": 100000000000,"data": ["", "base64"],"owner": "11111111111111111111111111111111","executable": false,"rentEpoch": 18446744073709551615,"space": 0}}`,
		pubKey,
	)
	utils.Assert(err == nil, fmt.Sprint(err))
	return p
}

func main() {
	numAccounts := flag.Uint("accounts", 0, "number of accounts to create")
	projectDir := flag.String("projectDir", "", "absolute path of project directory")
	solanaDirName := flag.String("solana", ".solana", "name of directory with solana validator data")

	flag.Parse()
	utils.Assert(*numAccounts > 0, "accounts i required")
	utils.Assert(*projectDir != "", "projectDir is required")

	solanaDir := filepath.Join(*projectDir, *solanaDirName)
	cachePath := filepath.Join(solanaDir, "cache.json")

	c, err := loadCache(solanaDir, cachePath)
	cacheMiss := false
	if err == nil {
		if c.Accounts == int(*numAccounts) {
			fmt.Print(c.Args)
			os.Exit(0)
		} else {
			cacheMiss = true
		}
	} else if !errors.Is(err, errNoCache) {
		log.Fatal(err)
	}

	if cacheMiss {
		os.RemoveAll(solanaDir)
	}

	os.Mkdir(solanaDir, 0755)
	validatorArgsBuilder := strings.Builder{}

	for i := 0; i < int(*numAccounts); i++ {
		wallet := solana.NewWallet()
		pubkey := wallet.PublicKey().String()

		savePrivateKey(solanaDir, wallet)
		p := saveData(solanaDir, pubkey)

		validatorArgsBuilder.WriteString(fmt.Sprintf("--account %s %s ", pubkey, p))
	}

	cacheFile, err := os.Create(cachePath)
	utils.Assert(err == nil, fmt.Sprint(err))

	validatorArgs := validatorArgsBuilder.String()

	newCache := cache{
		Args:     validatorArgs,
		Accounts: int(*numAccounts),
	}
	newCache.save(cacheFile)
	fmt.Print(validatorArgs)
}
