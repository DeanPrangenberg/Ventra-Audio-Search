from typing import Any
from datetime import datetime, timezone

def to_iso(dt_val) -> str:
    if dt_val is None or dt_val == "":
        return ""
    if isinstance(dt_val, (int, float)):
        return datetime.fromtimestamp(dt_val, tz=timezone.utc).isoformat().replace("+00:00", "Z")
    if isinstance(dt_val, str):
        return dt_val

    return str(dt_val)

def iso_to_timestamp(val) -> int | str:
    if val is None or val == "":
        return ""
    if isinstance(val, (int, float)):
        return int(val)
    if isinstance(val, str):
        s = val.replace("Z", "+00:00")
        dt = datetime.fromisoformat(s)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return int(dt.timestamp())
    return ""

def update_meta(value: str, files_state: list[dict[str, Any]], idx: int, field: str):
    if not files_state or idx < 0 or idx >= len(files_state):
        return files_state
    files_state[idx][field] = value
    return files_state

def update_meta_single(value: str, state: dict[str, Any], field: str):
    state[field] = value
    return state