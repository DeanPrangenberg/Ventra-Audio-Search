import logging
from typing import Any

import gradio as gr

import config_manager
import src.utils.state as state_utils
from src.api import api
from src.api.payloads import search_payload


def do_backend_request(state: dict[str, Any]) -> str:
    fts5_query: str = state.get("fts5_query", "")
    semantic_search_query: str = state.get("semantic_search_query", "")
    category: str = state.get("category", "")
    start_time_period = state.get("start_time_period", None)
    end_time_period = state.get("end_time_period", None)
    max_segment_return = state.get("max_segment_return", None)

    logging.info(
        "Creating Search payload: fts5_query=%s, semantic_search_query=%s, category=%s",
        fts5_query, semantic_search_query, category
    )

    payload = search_payload.SearchPayload(
        fts5_query=fts5_query,
        semantic_search_query=semantic_search_query,
        category=category,
        start_time_period=start_time_period,
        end_time_period=end_time_period,
        max_segment_return=max_segment_return,
    )

    res = api.API().search_request(payload)


    return res


def mount_search_routes(app: gr.Blocks):
    with app.route("Search"):
        gr.Markdown("# Search Audio Files")
        gr.Markdown("This page is for semantic search — results are matched by meaning, not just exact keywords.")

        state = gr.State({})

        fts5_query = gr.Text(
            label="Keywords",
            placeholder="Enter exact keywords like (Deadline, Project x)...",
            value="",
            interactive=True,
        )

        semantic_search_query = gr.Text(
            label="Question",
            placeholder="Enter a question like (When is the deadline for project x)",
            value="",
            interactive=True,
        )

        choices = config_manager.ConfigManager().get_category_list()
        category = gr.Dropdown(
            label="Choose a Category",
            choices=choices,
            value=(choices[0] if choices else None),
            interactive=True,
        )

        start_time_period = gr.DateTime(
            label="Search range start",
            value=None,
            interactive=True,
        )

        end_time_period = gr.DateTime(
            label="Search range end",
            value=None,
            interactive=True,
        )

        fts5_query.change(
            fn=lambda v, s: state_utils.update_meta_single(v, s, "fts5_query"),
            inputs=[fts5_query, state],
            outputs=[state],
            api_visibility="private",
            queue=False,
        )

        semantic_search_query.change(
            fn=lambda v, s: state_utils.update_meta_single(v, s, "semantic_search_query"),
            inputs=[semantic_search_query, state],
            outputs=[state],
            api_visibility="private",
            queue=False,
        )

        category.change(
            fn=lambda v, s: state_utils.update_meta_single(v, s, "category"),
            inputs=[category, state],
            outputs=[state],
            api_visibility="private",
            queue=False,
        )

        start_time_period.change(
            fn=lambda v, s: state_utils.update_meta_single(v, s, "start_time_period"),
            inputs=[start_time_period, state],
            outputs=[state],
            api_visibility="private",
            queue=False,
        )

        end_time_period.change(
            fn=lambda v, s: state_utils.update_meta_single(v, s, "end_time_period"),
            inputs=[end_time_period, state],
            outputs=[state],
            api_visibility="private",
            queue=False,
        )

        send_btn = gr.Button("Send Search Request", variant="primary")
        send_result = gr.Markdown()

        send_btn.click(
            fn=do_backend_request,
            inputs=[state],
            outputs=[send_result],
            api_visibility="private",
        )