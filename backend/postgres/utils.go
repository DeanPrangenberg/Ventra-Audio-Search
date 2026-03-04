package postgres

import (
	"database/sql"
	"encoding/json"
	"strings"
)

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func jsonOrNilFromStringSlice(v []string) (any, error) {
	if len(v) == 0 {
		return nil, nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return string(b), nil
}

func stringSliceFromJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}

	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}

	return out
}

func float64OrZero(v sql.NullFloat64) float64 {
	if !v.Valid {
		return 0
	}
	return v.Float64
}
