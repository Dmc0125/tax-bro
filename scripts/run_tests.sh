PROJECT_DIR="$(cd "$(dirname "$(readlink -f "$0")")" && cd .. && pwd)"
SOLANA_DIR_NAME=".solana"
SOLANA_DIR="$PROJECT_DIR/$SOLANA_DIR_NAME"

cleanup() {
    echo ""
    echo "Cleaning up"
    # systemctl --user stop docker
    if [ -n $VALIDATOR_PID ]; then
        kill $VALIDATOR_PID
    fi
}

trap cleanup INT

if [ $(systemctl --user is-active docker) = "inactive" ]; then
    echo "Starting docker"
    echo ""
    systemctl --user start docker
fi

echo "Run init_solana/main.go"
echo ""
cd $PROJECT_DIR/cmd/init_solana
VALIDATOR_ARGS=$(go run ./main.go --accounts 2 --projectDir $PROJECT_DIR --solana $SOLANA_DIR_NAME)

if [[ $? != 0 ]]; then
    cleanup
    echo "unable to init $SOLANA_DIR_NAME dir"
fi

RPC_URL=http://localhost:8899

echo "Run solana-test-validator $VALIDATOR_ARGS"
echo ""
rm -rf $SOLANA_DIR/test_ledger
solana-test-validator $VALIDATOR_ARGS -u $RPC_URL -l $SOLANA_DIR/test_ledger > $SOLANA_DIR/validator.log 2>&1 &
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

echo "Running tests"
echo ""
cd $PROJECT_DIR
RPC_URL=$RPC_URL SOLANA_DIR=$SOLANA_DIR go test ./... -v

cleanup
