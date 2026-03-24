import os
from datetime import datetime, timezone

import gradio as gr

import config_manager
import utils.file as file_utils
import utils.state as state_utils
from src.routs.import_rout.utils import do_backend_request


def mount_uploaded_files_renderer():
    # Holds [{orig_name, stored_path, file_url, title, summary}, ...]
    files_state = gr.State([])

    file_upload = gr.File(label="Upload MP3 Audio Files", file_types=[".mp3"], file_count="multiple", type="filepath",
        elem_classes="almost-no-bg", )

    file_upload.change(fn=file_utils.persist_and_make_state, inputs=[file_upload], outputs=[files_state], )

    @gr.render(inputs=files_state)
    def show_audio(state):
        if not state:
            return

        send_result = gr.Markdown()

        for idx, f in enumerate(state):
            label = f"{f['orig_name']}"

            with gr.Accordion(label=label, open=True, elem_classes="almost-no-bg"):
                if f.get("error"):
                    gr.Markdown(f" **Error:** {f['error']}", elem_id="error-markdown")

                title = gr.Textbox(label="Set a Title", value=f.get("title", ""), elem_classes="almost-no-bg")
                record_time = gr.DateTime(label="Enter Recording date & time",
                    value=f.get("time", datetime.now(timezone.utc)), elem_classes="almost-no-bg")
                category = gr.Dropdown(label="Choose a Category",
                                       choices=config_manager.ConfigManager().get_category_list(),
                                       value=f.get("category", None), interactive=True, elem_classes="almost-no-bg")
                audio_type = gr.Dropdown(label="Choose a Audio Type", value=f.get("audio_type", "Meeting"),
                                         choices=["Meeting", "Media", "Generic"], interactive=True,
                                         elem_classes="almost-no-bg")
                summary = gr.Textbox(label="Write a little summary", value=f.get("summary", ""), lines=5,
                                     elem_classes="almost-no-bg")

                # Update state when fields change
                title.change(fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "title"),
                    inputs=[title, files_state], outputs=[files_state], queue=False, )
                category.change(fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "category"),
                    inputs=[category, files_state], outputs=[files_state], queue=False, )
                audio_type.change(fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "audio_type"),
                    inputs=[audio_type, files_state], outputs=[files_state], queue=False, )
                summary.change(fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "summary"),
                    inputs=[summary, files_state], outputs=[files_state], queue=False, )
                record_time.change(fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "time"),
                    inputs=[record_time, files_state], outputs=[files_state], queue=False, )

        send_btn = gr.Button("Send configured files to Backend", variant="primary")
        send_btn.click(fn=do_backend_request, inputs=[files_state], outputs=[files_state, file_upload, send_result], )
