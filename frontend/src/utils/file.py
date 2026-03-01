import base64
import hashlib
import logging
import os
import shutil
import time
from pathlib import Path
from typing import Any

from mutagen.mp3 import MP3

import src.config_manager as config_manager


def file_sha256(path: str, chunk_size: int = 1024 * 1024) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(chunk_size), b""):
            h.update(chunk)
    return h.hexdigest()

def cleanup_upload_dir_ttl() -> None:
    upload_dir = Path(os.environ.get("DATA_DIR", "/app/data")).resolve() / "uploads"
    ttl_seconds = int(os.environ.get("FILE_CLEAN_UP", 30*60))

    now = time.time()
    if not upload_dir.exists():
        return

    for p in upload_dir.glob("*"):
        if not p.is_file():
            continue
        try:
            age = now - p.stat().st_mtime
            if age > ttl_seconds:
                p.unlink()
        except Exception as e:
            logging.warning(f"TTL cleanup failed for {p}: {e}")

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
        unique_name = f"{file_sha256(src_path)}{ext}"

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
