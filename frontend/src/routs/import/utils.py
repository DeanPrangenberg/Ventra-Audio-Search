import logging
import os
from typing import Any

import gradio as gr

import api
from api.payloads import import_payload
from src.utils import file as file_utils


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