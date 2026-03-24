import json
from pathlib import Path
from typing import Any, Literal

from payload import SearchPayload
from search_request import search_request

SearchMode = Literal["semantic", "lexical", "hybrid"]
TESTS_FILE = Path("test_data/tests.json")


def normalize_text(text: str | None) -> str:
    return " ".join((text or "").lower().split())


def load_cases(path: Path = TESTS_FILE) -> list[dict[str, Any]]:
    with path.open("r", encoding="utf-8") as file:
        return json.load(file)


def filter_cases(
    cases: list[dict[str, Any]],
    category: str,
    difficulty: int,
) -> list[dict[str, Any]]:
    return [
        case
        for case in cases
        if case.get("difficulty") == difficulty and case.get("category") == category
    ]


def build_payload(case: dict[str, Any], mode: SearchMode, return_window: int) -> SearchPayload:
    ts_query = ""
    semantic_query = ""

    if mode == "lexical":
        ts_query = case.get("ts_query", "")
    elif mode == "semantic":
        semantic_query = case.get("semantic_query", "")
    elif mode == "hybrid":
        ts_query = case.get("ts_query", "")
        semantic_query = case.get("semantic_query", "")
    else:
        raise ValueError(f"Unsupported search mode: {mode}")

    return SearchPayload(
        ts_query,
        semantic_query,
        case.get("category", ""),
        return_window,
    )


def excerpt_found_in_response(
    response: dict[str, Any],
    expected_excerpt: str | None,
) -> tuple[bool, int | None, float, float]:
    excerpt = normalize_text(expected_excerpt)

    if not excerpt:
        return False, None, 0.0, 0.0

    top_k_segments = response.get("top_k_segments", [])
    if not isinstance(top_k_segments, list) or not top_k_segments:
        return False, None, 0.0, 0.0

    best_index: int | None = None
    total_hits = 0

    for idx, segment in enumerate(top_k_segments, start=1):
        transcript = normalize_text((segment or {}).get("transcript"))
        if excerpt in transcript:
            total_hits += 1
            if best_index is None:
                best_index = idx

    found = best_index is not None

    precision = total_hits / len(top_k_segments)
    recall = total_hits / 3

    return found, best_index, precision, recall


def reciprocal_rank(rank: int | None) -> float:
    """
    Reciprocal Rank:
        rank 1 -> 1.0
        rank 2 -> 0.5
        rank 3 -> 0.333...
        None   -> 0.0
    """
    if rank is None:
        return 0.0

    if rank <= 0:
        raise ValueError(f"Rank must be >= 1 or None, got: {rank}")

    return 1.0 / rank


def run_search_test(
    mode: SearchMode,
    category: str,
    difficulty: int,
    return_window: int,
    url: str,
    tests_path: Path = TESTS_FILE,
) -> tuple[int, int, float, float, float, float]:
    cases = load_cases(tests_path)
    cases = filter_cases(cases, category, difficulty)

    if not cases:
        return 0, 0, 0.0, 0.0, 0.0, 0.0

    results: list[dict[str, Any]] = []

    for case in cases:
        case_id = case.get("id", "unknown")
        print(f"\n--- Running {mode} case: {case_id} ---")

        payload = build_payload(case, mode, return_window)

        ok, response, respond_time = search_request(payload, url)

        if not ok:
            print(f"FAILED: {response}")
            results.append({
                "id": case_id,
                "success": False,
                "rank": None,
                "rr": 0.0,
                "respond_time": float(respond_time),
                "reason": str(response),
            })
            continue

        found, rank, precision, recall = excerpt_found_in_response(response, case.get("expected_excerpt"))
        rr = reciprocal_rank(rank)

        if found:
            print(f"PASSED (rank={rank}, rr={rr:.4f})")
        else:
            print("FAILED")

        results.append({
            "id": case_id,
            "success": found,
            "rank": rank,
            "rr": rr,
            "precision": precision,
            "recall": recall,
            "respond_time": float(respond_time),
            "reason": None if found else "expected excerpt not found",
        })

    passed = sum(1 for result in results if result["success"])
    failed = len(results) - passed

    average_mrr = sum(result["rr"] for result in results) / len(results)
    average_respond_time = sum(result["respond_time"] for result in results) / len(results)
    average_precision = sum(result["precision"] for result in results) / len(results)
    average_recall = sum(result["recall"] for result in results) / len(results)

    assert 0.0 <= average_mrr <= 1.0, f"Invalid MRR detected: {average_mrr}"
    assert passed + failed == len(results)

    return passed, failed, average_mrr, average_respond_time, average_precision, average_recall


def run_search_test_semantic(category: str, difficulty: int, return_window: int, url: str) -> tuple[int, int, float, float, float, float]:
    return run_search_test("semantic", category, difficulty, return_window, url)


def run_search_test_lexical(category: str, difficulty: int, return_window: int, url: str) -> tuple[int, int, float, float, float, float]:
    return run_search_test("lexical", category, difficulty, return_window, url)


def run_search_test_hybrid(category: str, difficulty: int, return_window: int, url: str) -> tuple[int, int, float, float, float, float]:
    return run_search_test("hybrid", category, difficulty, return_window, url)