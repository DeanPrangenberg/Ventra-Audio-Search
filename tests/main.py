import csv
import os

from search_types import (
    run_search_test_hybrid,
    run_search_test_semantic,
    run_search_test_lexical,
)

URL = "http://100.103.135.123:8880/search"


def run_mode(search_type: str, category: str):
    results = []
    results.append([
        "question_difficulty",
        "segment_return_amount",
        "failed",
        "passed",
        "average_respond_time",
        "average_mrr",
        "average_precision",
        "average_recall",
    ])

    for diff in range(1, 6):
        for return_win in range(1, 11):
            if search_type == "lexical":
                passed, failed, average_mrr, average_respond_time, average_precision, average_recall = run_search_test_lexical(
                    category, diff, return_win, URL
                )
            elif search_type == "hybrid":
                passed, failed, average_mrr, average_respond_time, average_precision, average_recall = run_search_test_hybrid(
                    category, diff, return_win, URL
                )
            elif search_type == "semantic":
                passed, failed, average_mrr, average_respond_time, average_precision, average_recall = run_search_test_semantic(
                    category, diff, return_win, URL
                )
            else:
                raise ValueError(f"Unknown search type: {search_type}")

            # Sanity check: MRR muss zwischen 0 und 1 liegen
            assert 0.0 <= average_mrr <= 1.0, (
                f"Invalid average_mrr for {search_type=} {category=} {diff=} {return_win=}: {average_mrr}"
            )

            results.append([
                diff,
                return_win,
                failed,
                passed,
                round(average_respond_time, 6),
                round(average_mrr, 6),
                round(average_precision, 6),
                round(average_recall, 6),
            ])

            print(
                f"{search_type} | {category} | diff={diff} | k={return_win} "
                f"| passed={passed} failed={failed} time={average_respond_time:.4f} mrr={average_mrr:.6f} average_precision={average_precision} average_recall={average_recall}"
            )

    os.makedirs("results", exist_ok=True)
    output_path = f"results/{search_type}_{category}_tests.csv"

    with open(output_path, "w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerows(results)

    print(f"Saved: {output_path}")


if __name__ == "__main__":
    categories = [
        "Edeltalk - mit Dominik & Kevin",
        "UNFASSBAR – ein Simplicissimus Podcast",
    ]

    for search_type in ["lexical", "hybrid", "semantic"]:
        for category in categories:
            run_mode(search_type, category)