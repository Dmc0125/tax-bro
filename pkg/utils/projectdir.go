package utils

import (
	"path"
	"runtime"
)

func GetProjectDir() string {
	_, fp, _, ok := runtime.Caller(0)
	Assert(ok, "unable to get current filename")
	projectDir := path.Join(fp, "../../..")
	return projectDir
}
