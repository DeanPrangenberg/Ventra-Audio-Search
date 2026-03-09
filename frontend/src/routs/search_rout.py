import logging
import os
from datetime import datetime, timezone

import gradio as gr
from sqlalchemy import create_engine, text

import config_manager
from src.api import api
from src.api.payloads.search_payload import SearchPayload

def load_postgres_url():
    return os.environ.get(
        "POSTGRES_URL",
        "user:password@localhost:5432/audio_transcript_db"
    )


def fetch_categories() -> list[str]:
    engine = create_engine("postgresql+psycopg://" + load_postgres_url(), pool_pre_ping=True)

    with engine.connect() as conn:
        return list(conn.execute(text("""
            SELECT DISTINCT category
            FROM audiofiles
            ORDER BY category
        """)).scalars().all())

def get_default_date_range():
    oldest = datetime(1970, 1, 1, 0, 0, 0, tzinfo=timezone.utc)
    now = datetime.now(timezone.utc)
    return oldest, now


def do_backend_request(
    ts_query: str,
    semantic_search_query: str,
    category: str,
    start_time_period,
    end_time_period,
    max_segment_return,
):
    logging.info(
        "Creating search payload: ts_query=%s, semantic_search_query=%s, category=%s",
        ts_query,
        semantic_search_query,
        category,
    )

    payload = SearchPayload(
        ts_query=ts_query or "",
        semantic_search_query=semantic_search_query or "",
        category=category or "",
        start_time_period=start_time_period,
        end_time_period=end_time_period,
        max_segment_return=int(max_segment_return) if max_segment_return else None,
    )

    successful, res = api.API().search_request(payload)

    if successful:
        result_count = len(res.get("results", [])) if isinstance(res, dict) else 0
        return (
            gr.update(value=f"Search successful. Results: {result_count}", visible=True),
            gr.update(value=res, visible=True),
        )

    return (
        gr.update(value=f"Search failed: {res}", visible=True),
        gr.update(value=None, visible=False),
    )


def reset_search_form(default_category, oldest, now):
    return (
        "",
        "",
        default_category,
        oldest,
        now,
        1,
        gr.update(value="", visible=False),
        gr.update(value=None, visible=False),
    )


def mount_search_routes(app: gr.Blocks):
    oldest, now = get_default_date_range()

    with app.route("Search"):
        gr.Markdown("# Search Audio Files")
        gr.Markdown(
            "Search by exact keywords, semantic meaning, category, and date range."
        )

        choices = fetch_categories()
        default_category = choices[0] if choices else "Standard"

        with gr.Row():
            with gr.Column(scale=2):
                ts_query = gr.Textbox(
                    label="Keyword Search",
                    placeholder="e.g. keyword (names, persons, etc.)",
                    value="",
                    interactive=True,
                )

                semantic_search_query = gr.Textbox(
                    label="Semantic Search",
                    placeholder="e.g. When is the release deadline?",
                    value="",
                    interactive=True,
                )

                category = gr.Dropdown(
                    label="Category",
                    choices=choices,
                    value=default_category,
                    interactive=True,
                )

            with gr.Column(scale=1):
                start_time_period = gr.DateTime(
                    label="Start Date",
                    value=oldest,
                    interactive=True,
                )

                end_time_period = gr.DateTime(
                    label="End Date",
                    value=now,
                    interactive=True,
                )

                max_segment_return = gr.Number(
                    label="Maximum Results",
                    value=10,
                    minimum=1,
                    precision=0,
                    interactive=True,
                )

        with gr.Row():
            send_btn = gr.Button("Search", variant="primary")
            reset_btn = gr.Button("Reset")

        status = gr.Markdown(visible=False)
        json_view = gr.JSON(visible=False)

        send_btn.click(
            fn=do_backend_request,
            inputs=[
                ts_query,
                semantic_search_query,
                category,
                start_time_period,
                end_time_period,
                max_segment_return,
            ],
            outputs=[status, json_view],
            show_progress="full",
            show_progress_on=[status, json_view],
            api_visibility="private",
        )

        reset_btn.click(
            fn=lambda: reset_search_form(default_category, oldest, now),
            outputs=[
                ts_query,
                semantic_search_query,
                category,
                start_time_period,
                end_time_period,
                max_segment_return,
                status,
                json_view,
            ],
            api_visibility="private",
        )