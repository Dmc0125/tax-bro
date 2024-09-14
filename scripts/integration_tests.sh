PROJECT_DIR="$(cd "$(dirname "$(readlink -f "$0")")" && cd .. && pwd)"
SOLANA_DIR="$PROJECT_DIR/.solana"

cleanup() {
    echo "Cleaning up"
    kill $VALIDATOR_PID
    docker-compose --profile test down
    exit 0
}

trap cleanup INT

seed_account() {
    local idx=$1
    local kpath="$SOLANA_DIR/keypair$idx.json"

    solana-keygen new --no-bip39-passphrase -s --outfile $kpath
    local pubkey=$(solana-keygen pubkey $kpath)

    cat > "$SOLANA_DIR/acc$idx.json" << EOL
{
    "pubkey": "$pubkey",
    "account": {
        "lamports": 100000000000,
        "data": [
            "",
            "base64"
        ],
        "owner": "11111111111111111111111111111111",
        "executable": false,
        "rentEpoch": 18446744073709551615,
        "space": 0
    }
}
EOL

    echo $pubkey
}

RPC_URL="http://127.0.0.1:8899"

ACC1_PATH="$SOLANA_DIR/acc1.json"
ACC2_PATH="$SOLANA_DIR/acc2.json"

if [ ! -d $SOLANA_DIR ]; then
    echo "Missing .solana directory. It will be created now"
    mkdir .solana

    PUBKEY1=$(seed_account 1 | tail -n 1)  
    PUBKEY2=$(seed_account 2 | tail -n 1)
else
    PUBKEY1=$(/bin/cat "$SOLANA_DIR/acc1.json" | /usr/bin/jq -r .pubkey)
    PUBKEY2=$(/bin/cat "$SOLANA_DIR/acc2.json" | /usr/bin/jq -r .pubkey)
fi

if [ -d $SOLANA_DIR/test_ledger ]; then
    rm -rf $SOLANA_DIR/test_ledger
    rm $SOLANA_DIR/validator.log
fi

echo "Starting solana-test-validator"
solana-test-validator \
    --account $PUBKEY1 $ACC1_PATH \
    --account $PUBKEY2 $ACC2_PATH \
    -u $RPC_URL \
    -l $SOLANA_DIR/test_ledger > $SOLANA_DIR/validator.log 2>&1 &
VALIDATOR_PID=$!
# wait for solana-test-validator to start
while true
do
    sleep 1
    RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"getSlot"}' $RPC_URL)
    if echo "$RESPONSE" | grep -q "result"; then
        break
    fi
done

echo ""
echo "Starting testing DB"
docker-compose --profile test up -d >/dev/null
# Wait for the test database to be ready
until docker-compose --profile test exec test_db pg_isready; do
  echo "Waiting for test database to be ready..."
  sleep 1
done

DATABASE_URL="postgresql://user:pwd@localhost:5433/db?sslmode=disable"

echo ""
echo "Runnning migrations"
DB_URL=$DATABASE_URL go run $PROJECT_DIR/cmd/migrate --zero >/dev/null
DB_URL=$DATABASE_URL go run $PROJECT_DIR/cmd/migrate --latest >/dev/null

echo ""
echo "Running tests"
DB_URL=$DATABASE_URL RPC_URL=$RPC_URL go run $PROJECT_DIR/cmd/integration_tests/... "$SOLANA_DIR/keypair1.json" "$SOLANA_DIR/keypair2.json"

echo ""
docker-compose --profile test down
kill $VALIDATOR_PID

