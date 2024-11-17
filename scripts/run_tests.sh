PROJECT_DIR="$(cd "$(dirname "$(readlink -f "$0")")" && cd .. && pwd)"
RPC_PORT=6942
# uint64 max value
MAX_SOL=18446744073709551615
VALIDATOR_PID=""

TEST_DIR="${PROJECT_DIR}/pkg/..."
TEST_ARGS=""
for arg in "$@"; do
    case $arg in
        args=*)
        TEST_ARGS="${arg#*=}"
        ;;
        test-dir=*)
        TEST_DIR="${PROJECT_DIR}${arg#*=}"
        ;;
    esac
done

GREEN='\033[0;32m'
RED='\033[0;31m'
GRAY='\033[0;90m'
NC='\033[0m'

coloredEcho() {
    echo -e "${1}${2}${NC}"
}

cleanup() {
    if [ ! -z "$VALIDATOR_PID" ] && kill -0 $VALIDATOR_PID 2>/dev/null; then
        coloredEcho $GRAY "\nShutting down validator..."
        kill $VALIDATOR_PID
        wait $VALIDATOR_PID 2>/dev/null
    fi
    exit 0
}

trap cleanup SIGINT SIGTERM

check_validator() {
    curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"getSlot"}' \
        http://localhost:${RPC_PORT} > /dev/null 2>&1
    return $?
}

if check_validator; then
    echo "Error: Solana validator is already running on port ${RPC_PORT}"
    exit 1
fi

solana-test-validator \
    --ledger ${PROJECT_DIR}/test-ledger \
    --rpc-port ${RPC_PORT} \
    --reset \
    --quiet \
    --faucet-sol ${MAX_SOL} >/dev/null &
VALIDATOR_PID=$!

coloredEcho $GRAY "> Waiting for validator to start..."
while ! check_validator; do
    sleep 1
done
coloredEcho $GRAY "> Validator started successfully\n"

coloredEcho $GRAY "> Running tests"
echo -e "> go test $TEST_DIR $TEST_ARGS\n"
RPC_PORT=${RPC_PORT} go test ${TEST_DIR} ${TEST_ARGS}
TEST_EXIT_CODE=$?

if [ ! -z "$TEST_EXIT_CODE" ]; then
    if [ $TEST_EXIT_CODE -eq 0 ]; then
        echo -e "\n$GREEN\u2713\u2713\u2713 GREAT SUCCESS$NC: Tests executed successfully"
    else
        echo -e "\n$RED\u2717\u2717\u2717 FAIL$NC: Tests failed with exit code"
    fi
fi

cleanup
exit ${TEST_EXIT_CODE:-0}
