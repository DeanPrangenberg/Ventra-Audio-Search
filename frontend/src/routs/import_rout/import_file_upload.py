import gradio as gr

import utils.file as file_utils
from src.routs.import_rout.utils import do_backend_request
import config_manager
import utils.state as state_utils


def mount_uploaded_files_renderer():
    files_state = gr.State([])

    file_upload_interface = gr.File(
        label="Upload MP3 Audio Files",
        file_types=[".mp3"],
        file_count="multiple",
        type="filepath",
    )

    file_upload_interface.change(
        fn=file_utils.persist_and_make_state,
        inputs=[file_upload_interface],
        outputs=[files_state],)

    send_result = gr.Markdown()

    @gr.render(inputs=files_state)
    def render_upload_items(state):
        for idx, f in enumerate(state):
            label = f"{f['orig_name']}"

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
                    value=f.get("audio_type", "Meeting"),
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
                    inputs=[title, state],
                    outputs=[state],
                    queue=False,
                    api_visibility="private",
                )
                category.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "category"),
                    inputs=[category, state],
                    outputs=[state],
                    queue=False,
                    api_visibility="private",
                )
                audio_type.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "audio_type"),
                    inputs=[audio_type, state],
                    outputs=[state],
                    queue=False,
                    api_visibility="private",
                )
                summary.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "summary"),
                    inputs=[summary, state],
                    outputs=[state],
                    queue=False,
                    api_visibility="private",
                )
                record_time.change(
                    fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "time"),
                    inputs=[record_time, state],
                    outputs=[state],
                    queue=False,
                    api_visibility="private",
                )

    send_btn = gr.Button("Send configured files to backend", variant="primary")
    send_btn.click(
        fn=do_backend_request,
        inputs=[files_state],
        outputs=[files_state, file_upload_interface, send_result],
    )