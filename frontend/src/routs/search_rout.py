import logging
import os
from typing import Any

import gradio as gr
import src.utils.file as file_utils
import src.utils.state as state_utils
from src.api import api
from src.api.payloads import import_payload

def do_backend_request(files_state: list[dict[str, Any]]):
    payload_list: list[import_payload.ImportPayload] = []

    for idx, file in enumerate(files_state):
        title = file.get("title", "")
        category = file.get("category", "")
        audio_type = file.get("audio_type", "")
        user_summary = file.get("summary", "")
        recording_date_raw = file.get("time", "")
        recording_date = state_utils.to_iso(recording_date_raw)

        logging.info("Creating payload for file idx=%s, title=%s, recording_date=%s, user_summary=%s", idx, title, recording_date, user_summary)

        payload = import_payload.ImportPayload(
            title=title,
            recording_date=recording_date,
            user_summary=user_summary,
            base64_data=file_utils.file_to_base64_str(file["download_path"]),
            duration_in_sec=file_utils.mp3_duration_seconds(file["download_path"])
        )
        payload_list.append(payload)

        api.API().import_request(payload_list)

def mount_import_routes(app: gr.Blocks):
    with app.route("Import"):
        gr.Markdown("# Import Audio Files")
        gr.Markdown(
            """
        This page uploads new audio files.
        For each file you can set metadata.
        Later you send file URLs + metadata to the backend. The backend downloads the MP3 via that URL.
        """
            )

        # Holds [{orig_name, stored_path, file_url, title, summary}, ...]
        files_state = gr.State([])

        file_upload = gr.File(
            label="Upload MP3 Audio Files",
            file_types=[".mp3"],
            file_count="multiple",
            type="filepath",
        )

        # When upload changes, persist files + create urls
        file_upload.change(
            fn=file_utils.persist_and_make_state,
            inputs=[file_upload],
            outputs=[files_state],
        )

        send_btn = gr.Button("Send configured files to backend", variant="primary")
        send_result = gr.Markdown()

        send_btn.click(
            fn=do_backend_request,
            inputs=[files_state],
            outputs=[send_result],
        )

        @gr.render(inputs=files_state)
        def show_audio(state):
            if not state:
                gr.Markdown("No files uploaded.")
                return

            for idx, f in enumerate(state):
                label = f"{f['orig_name']}"

                with gr.Accordion(label=label, open=True):
                    title = gr.Textbox(label="Set a Title", value=f.get("title", ""))
                    category = gr.Textbox(label="Set a Category (Team Name....)", value=f.get("category", ""))
                    audio_type = gr.Dropdown(value="Meeting", choices=["Meeting", "Media", "Generic"])
                    summary = gr.Textbox(
                        label="Write a little summary",
                        value=f.get("summary", ""),
                        lines=3,
                    )

                    record_time = gr.DateTime(
                        label="Enter Recording date & time",
                    )

                    # Update state when fields change
                    title.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "title"),
                        inputs=[title, files_state],
                        outputs=[files_state],
                        queue=False,
                    )
                    category.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "category"),
                        inputs=[category, files_state],
                        outputs=[files_state],
                        queue=False,
                    )
                    audio_type.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "audio_type"),
                        inputs=[audio_type, files_state],
                        outputs=[files_state],
                        queue=False,
                    )
                    summary.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "summary"),
                        inputs=[summary, files_state],
                        outputs=[files_state],
                        queue=False,
                    )
                    record_time.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta(v, s, i, "time"),
                        inputs=[record_time, files_state],
                        outputs=[files_state],
                        queue=False,
                    )

                    gr.Audio(f["stored_path"], label=os.path.basename(f["stored_path"]))
