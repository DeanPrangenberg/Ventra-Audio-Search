import src.utils.state as state_utils

class SearchPayload:
    def __init__(
        self,
        fts5_query: str,
        semantic_search_query: str,
        category: str,
        start_time_period_iso: str,
        end_time_period_iso: str,
        max_segment_return: int
    ):
        self.fts5_query = fts5_query
        self.semantic_search_query = semantic_search_query
        self.category = category
        self.start_time_period_iso = state_utils.to_iso(start_time_period_iso)
        self.end_time_period_iso = state_utils.to_iso(end_time_period_iso)
        self.max_segment_return = max_segment_return

    def to_dict(self) -> dict:
        payload = {
            "fts5_query": self.fts5_query,
            "semantic_search_query": self.semantic_search_query,
            "category": self.category,
            "start_time_period_iso": self.start_time_period_iso,
            "end_time_period_iso": self.end_time_period_iso,
            "max_segment_return": self.max_segment_return
        }

        return payload