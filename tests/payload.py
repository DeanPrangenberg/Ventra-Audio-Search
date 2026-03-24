from datetime import datetime, timezone


class SearchPayload:
    def __init__(
            self,
            ts_query: str,
            semantic_search_query: str,
            category: str,
            max_segment_return: int
    ):
        self.ts_query = ts_query
        self.semantic_search_query = semantic_search_query
        self.category = category
        self.start_time_period_iso = datetime(1970, 1, 1, 0, 0, 0, tzinfo=timezone.utc).isoformat()
        self.end_time_period_iso = datetime.now(timezone.utc).isoformat()
        self.max_segment_return = max_segment_return

    def to_dict(self) -> dict:
        payload = {
            "ts_query": self.ts_query,
            "semantic_search_query": self.semantic_search_query,
            "category": self.category,
            "start_time_period_iso": self.start_time_period_iso,
            "end_time_period_iso": self.end_time_period_iso,
            "max_segment_return": self.max_segment_return
        }

        return payload