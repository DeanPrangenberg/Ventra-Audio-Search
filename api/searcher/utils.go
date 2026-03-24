package searcher

import (
	"context"
)

func (w *Worker) opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(w.stopCtx, opTimeout)
}
