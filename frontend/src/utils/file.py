import base64
import logging
import os
import uuid
import shutil
from mutagen.mp3 import MP3
from pathlib import Path
from typing import Any
import src.config_manager as config_manager


def persist_and_make_state(paths: list[str] | None) -> list[dict[str, Any]]:
    if not paths:
        logging.warning("inputted paths in persist_and_make_state() are empty")
        return []

    upload_dir = Path(config_manager.ConfigManager().get_upload_dir()).resolve()
    upload_dir.mkdir(parents=True, exist_ok=True)
    logging.info("upload_dir: %s", upload_dir.__str__())

    out: list[dict[str, Any]] = []

    for src_path in paths:
        src = Path(src_path)
        ext = src.suffix.lower() or ".mp3"
        unique_name = f"{uuid.uuid4().hex}{ext}"

        dst = upload_dir / unique_name
        shutil.copy2(src, dst)

        out.append(
            {
                "orig_name": src.name,
                "stored_path": str(dst),
                "download_path": dst,
                "title": "",
                "summary": "",
            }
        )

    return out

def file_to_base64_str(path: str | Path) -> str:
    data = Path(path).read_bytes()
    return base64.b64encode(data).decode("ascii")

def mp3_duration_seconds(path: str) -> float:
    audio = MP3(path)
    return float(audio.info.length)
