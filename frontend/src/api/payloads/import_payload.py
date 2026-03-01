import src.utils.state as state_utils

class ImportPayload:
    def __init__(
        self,
        title: str,
        recording_date: str,
        category: str,
        audio_type: str,
        duration_in_sec: float,
        user_summary: str,
        base64_data: str = False,
        file_url: str = False,
    ):

        self.title = title
        self.recording_date = state_utils.to_iso(recording_date)
        self.duration_in_sec = duration_in_sec
        self.user_summary = user_summary
        self.base64_data = base64_data
        self.file_url = file_url
        self.category = category
        self.audio_type = audio_type

    def to_dict(self) -> dict:
        payload = {
            "title": self.title,
            "recording_date": self.recording_date,
            "duration_in_sec": self.duration_in_sec,
            "user_summary": self.user_summary,
            "category": self.category,
            "audio_type": self.audio_type,
        }
        if self.base64_data:
            payload["base64_data"] = self.base64_data
        else:
            payload["file_url"] = self.file_url
        return payload