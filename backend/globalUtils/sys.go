package globalUtils

import (
	"fmt"
	"os"
	"strconv"
)

func LoadEnvStr(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", key))
	}
	return v
}

func LoadEnvInt(key string) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", key))
	}

	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid env var: %s, must be a integer like \"67\" or \"-42\"", key))
	}

	return int(i)
}

func LoadEnvUInt64(key string) uint64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", key))
	}

	u, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid env var: %s, must be a positive integer like \"67\"", key))
	}

	return u
}
