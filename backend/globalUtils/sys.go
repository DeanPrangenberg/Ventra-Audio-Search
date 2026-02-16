package globalUtils

import (
	"fmt"
	"os"
)

func LoadEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", key))
	}
	return v
}
