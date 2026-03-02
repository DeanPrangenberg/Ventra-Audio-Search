import gradio as gr
import os

from pprint import pprint

from src.utils.rss import rss_feed_to_import_payloads
from src.routs.import_rout.utils import is_valid_url, do_backend_request
import config_manager
import utils.state as state_utils


def handle_podcast_input(rss_feed):
    if not is_valid_url(rss_feed):
        return gr.update(
            value=f"**Error:** Please enter a valid Url",
            elem_id="error-markdown",
            visible=True
        ), []

    out = rss_feed_to_import_payloads(rss_feed)

    pprint(out)

    if isinstance(out, str):
        return gr.update(
            value=f"**Error:** {out}",
            elem_id="error-markdown",
            visible=True
        ), []

    return None, out


def mount_rss_renderer():
    found_episodes_state = gr.State([])

    rss_feed_input = gr.Text(
        label="Podcast RSS Feed URL",
        placeholder="https://podcast.feed.rss",
        interactive=True
    )

    input_error = gr.Markdown()

    rss_feed_input.change(
        fn=handle_podcast_input,
        inputs=[rss_feed_input],
        outputs=[input_error, found_episodes_state],
    )

    send_result = gr.Markdown()

    @gr.render(inputs=found_episodes_state)
    def render_episodes(state):
        for idx, f in enumerate(state):
            label = f"{f.get('title', f'Episode {idx}')}"

            if not f.get("category") in config_manager.ConfigManager().get_category_list():
                config_manager.ConfigManager().extend_categories(f.get("category"))

            with gr.Accordion(label=label, open=True):
                if f.get("error"):
                    gr.Markdown(f"**Error:** {f['error']}", elem_id="error-markdown")

                title = gr.Textbox(label="Set a Title", value=f.get("title", ""))
                record_time = gr.DateTime(
                    label="Enter Recording date & time",
                    value=f.get("time", None),
                )
                category = gr.Dropdown(
                    label="Choose a Category",
                    choices=config_manager.ConfigManager().get_category_list(),
                    value=f.get("category", None),
                    interactive=True,
                )
                audio_type = gr.Dropdown(
                    label="Choose an Audio Type",
                    value=f.get("audio_type", "Media"),
                    choices=["Meeting", "Media", "Generic"],
                    interactive=True,
                )
                summary = gr.Textbox(
                    label="Write a little summary",
                    value=f.get("summary", ""),
                    lines=3,
                )

                title.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "title"),
                    inputs=[title, found_episodes_state],
                    outputs=[found_episodes_state],
                    queue=False,
                    api_visibility="private",
                )
                category.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "category"),
                    inputs=[category, found_episodes_state],
                    outputs=[found_episodes_state],
                    queue=False,
                    api_visibility="private",
                )
                audio_type.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "audio_type"),
                    inputs=[audio_type, found_episodes_state],
                    outputs=[found_episodes_state],
                    queue=False,
                    api_visibility="private",
                )
                summary.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "summary"),
                    inputs=[summary, found_episodes_state],
                    outputs=[found_episodes_state],
                    queue=False,
                    api_visibility="private",
                )
                record_time.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "time"),
                    inputs=[record_time, found_episodes_state],
                    outputs=[found_episodes_state],
                    queue=False,
                    api_visibility="private",
                )

    send_btn = gr.Button("Send configured episodes to backend", variant="primary")
    send_btn.click(
        fn=do_backend_request,
        inputs=[found_episodes_state],
        outputs=[found_episodes_state, gr.State(), send_result],
    )