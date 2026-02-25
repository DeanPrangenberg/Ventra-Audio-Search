import logging
from typing import Any

import gradio as gr

import config_manager
from src.api import api
import src.utils.state as state_utils
from src.api.payloads import search_payload


# TODO: Update this to display and send search not import payloads

def do_backend_request(state: dict[str, Any]):
    fts5_query: str = state.get("fts5_query", "")
    semantic_search_query: str = state.get("semantic_search_query", "")
    category: str = state.get("category", "")
    start_time_period: int | float = state.get("start_time_period", "")
    end_time_period: int | float = state.get("end_time_period", "")
    max_segment_return: int = state.get("max_segment_return", "")

    logging.info("Creating Search payload payload: fts5_query=%s, semantic_search_query=%s, category=%s", fts5_query,
                 semantic_search_query, category)

    payload = search_payload.SearchPayload(
        fts5_query=fts5_query,
        semantic_search_query=semantic_search_query,
        category=category,
        start_time_period=start_time_period,
        end_time_period=end_time_period,
        max_segment_return=max_segment_return
    )

    api.API().search_request(payload)


def mount_import_routes(app: gr.Blocks):
    with app.route("Search"):
        gr.Markdown("# Search Audio Files")
        gr.Markdown("""
            This page is for semantic search — results are matched by meaning, not just exact keywords.
            """)

        input_state = gr.State()

        @gr.render(inputs=input_state)
        def show_search_mask(state):
            fts5_query = gr.Text(
                label="Keywords",
                placeholder="Enter exact keywords like (Deadline, Project x), some words you remember that was talked about",
                value=state.get("fts5_query", None),
                interactive=True
            )

            semantic_search_query = gr.Text(
                label="Question",
                placeholder="Enter a question like (When is the deadline for project x)",
                value=state.get("semantic_search_query", None),
                interactive=True
            )

            category = gr.Dropdown(
                label="Choose a Category",
                choices=config_manager.ConfigManager().get_category_list(),
                value=state.get("category", None),
                interactive=True
            )

            start_time_period = gr.DateTime(
                label="Search range start",
                value=state.get("start_time_period", None),
                interactive=True
            )

            end_time_period = gr.DateTime(
                label="Search range end",
                value=state.get("end_time_period", None),
                interactive=True
            )

            fts5_query.change(
                fn=state_utils.update_meta_single,
                inputs=[fts5_query, state, "fts5_query"],
                outputs=[state]
            )

            semantic_search_query.change(
                fn=state_utils.update_meta_single,
                inputs=[semantic_search_query, state, "semantic_search_query"],
                outputs=[state]
            )

            category.change(
                fn=state_utils.update_meta_single,
                inputs=[category, state, "category"],
                outputs=[state]
            )

            start_time_period.change(
                fn=state_utils.update_meta_single,
                inputs=[start_time_period, state, "start_time_period"],
                outputs=[state]
            )

            end_time_period.change(
                fn=state_utils.update_meta_single,
                inputs=[end_time_period, state, "end_time_period"],
                outputs=[state]
            )

            send_btn = gr.Button("Send Search Request", variant="primary")
            send_result = gr.Markdown()

            send_btn.click(
                fn=do_backend_request,
                inputs=[state],
                outputs=[send_result]
            )
