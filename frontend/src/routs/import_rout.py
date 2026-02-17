import logging
import os
import gradio as gr

import config_manager
import src.utils.file as file_utils
import src.utils.state as state_utils
from src.api import api
from src.api.payloads import import_payload

from typing import Any

def cleanup_uploaded_files(files_state: list[dict[str, Any]]) -> None:
    for f in files_state:
        for key in ("stored_path", "download_path"):
            p = f.get(key)
            if not p:
                continue
            try:
                os.remove(p)
            except FileNotFoundError:
                pass
            except Exception as e:
                logging.warning(f"Could not delete file '{p}': {e}")

def do_backend_request(files_state: list[dict[str, Any]]):
    payload_list: list[import_payload.ImportPayload] = []

    for idx, file in enumerate(files_state):
        title = file.get("title", "")
        category = file.get("category", "")
        audio_type = file.get("audio_type", "")
        user_summary = file.get("summary", "")
        recording_date = file.get("time", "")

        payload_list.append(
            import_payload.ImportPayload(
                title=title,
                recording_date=recording_date,
                user_summary=user_summary,
                base64_data=file_utils.file_to_base64_str(file["download_path"]),
                duration_in_sec=file_utils.mp3_duration_seconds(file["download_path"]),
                category=category,
                audio_type=audio_type,
            )
        )

    return_value = api.API().import_request(payload_list)

    if not return_value:
        cleanup_uploaded_files(files_state)
        return [], gr.update(value=None), gr.update(value="All files got imported", elem_classes=["status", "ok"])

    if isinstance(return_value, str):
        return files_state, gr.update(), gr.update(value=f"Error while sending Files to Backend: {return_value}", elem_classes=["status", "err"])

    err_by_idx = {d["index"]: d.get("error", "unknown") for d in return_value if isinstance(d.get("index"), int)}

    invalid_idx = {d["index"] for d in return_value if isinstance(d.get("index"), int)}

    valid_files = [f for i, f in enumerate(files_state) if i not in invalid_idx]

    cleanup_uploaded_files(valid_files)

    new_state: list[dict[str, Any]] = []

    lines = ["**Some files are Invalid:**", ""]

    # Display Errors return from backend
    for i, f in enumerate(files_state):
        if i in err_by_idx:
            f2 = dict(f)
            f2["error"] = err_by_idx[i]
            new_state.append(f2)

            name = f2.get("orig_name") or f2.get("title") or f"Index {i}"
            lines.append(f"1. **{name}**: {f2['error']}")

    return new_state, gr.update(), gr.update(value="\n".join(lines), elem_classes=["status", "err"])

def mount_import_routes(app: gr.Blocks):
    with app.route("Import"):
        gr.Markdown("# Import Audio Files")
        gr.Markdown(
            """
        This page uploads new audio files. For each file you have set metadata, this metadate will be used in the search process.
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

        @gr.render(inputs=files_state)
        def show_audio(state):
            if not state:
                gr.Markdown("No files uploaded.")
                return

            send_result = gr.Markdown()

            for idx, f in enumerate(state):
                label = f"{f['orig_name']}"

                with gr.Accordion(label=label, open=True):
                    if f.get("error"):
                        gr.Markdown(f" **Error:** {f['error']}", elem_id="error-markdown")

                    title = gr.Textbox(label="Set a Title", value=f.get("title", ""))
                    record_time = gr.DateTime(
                        label="Enter Recording date & time",
                        value=f.get("time", None),
                    )
                    category = gr.Dropdown(label="Choose a Category", choices=config_manager.ConfigManager().get_category_list(), value=f.get("category", None),
                                           interactive=True)
                    audio_type = gr.Dropdown(label="Choose a Audio Type", value=f.get("audio_type", "Meeting"),
                                             choices=["Meeting", "Media", "Generic"], interactive=True)
                    summary = gr.Textbox(label="Write a little summary", value=f.get("summary", ""), lines=3)

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

            send_btn = gr.Button("Send configured files to backend", variant="primary")
            send_btn.click(
                fn=do_backend_request,
                inputs=[files_state],
                outputs=[files_state, file_upload, send_result],
            )