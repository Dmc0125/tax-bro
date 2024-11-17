package utils

import "fmt"


func CallRpcWithRetries[T any](c func() (T, error), retries int8) (res T, err error) {
	i := 0
	for {
		if i > int(retries)-1 {
			err = fmt.Errorf("unable to execute rpc call: %s", err)
			return
		}
		res, err = c()
		if err == nil {
			return
		}
	}
}
