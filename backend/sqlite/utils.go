package sqlite

import (
	"strings"
	"time"
)

func UserTextToFTS(userText string) string {
	toks := strings.Fields(strings.ToLower(userText))
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		t = strings.Trim(t, `"'.,:;!?()[]{}<>`)
		if len(t) < 2 {
			continue
		}
		out = append(out, t)
	}
	// join with AND
	return strings.Join(out, " AND ")
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
