package postgres

type Counter string

const (
	ImportRequestsFailed     Counter = "import_requests_failed"
	ImportRequestsSuccessful Counter = "import_requests_successful"

	SearchRequestsFailed     Counter = "search_requests_failed"
	SearchRequestsSuccessful Counter = "search_requests_successful"
)
