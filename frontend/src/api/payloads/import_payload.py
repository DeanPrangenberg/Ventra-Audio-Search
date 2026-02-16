import json

class ImportPayload:
    def __init__(
        self,
        title: str,
        recording_date: str,
        category: str,
        audio_type: str,
        duration_in_sec: float,
        user_summary: str,
        base64_data: str = None,
        file_url: str = None,
    ):
        if not base64_data and not file_url:
            raise ValueError("Either base64_data or file_url must be provided.")

        if title is None:
            raise ValueError("A Title must be provided.")

        if recording_date is None:
            raise ValueError("A Recording Date must be provided.")

        if duration_in_sec is None:
            raise ValueError("A Audio Duration must be provided.")

        if user_summary is None:
            raise ValueError("A User written Summary must be provided.")

        if category is None:
            raise ValueError("A category must be provided.")

        if audio_type is None:
            raise ValueError("A audio_type must be provided.")

        self.title = title
        self.recording_date = recording_date
        self.duration_in_sec = duration_in_sec
        self.user_summary = user_summary
        self.base64_data = base64_data
        self.file_url = file_url

    def to_dict(self) -> dict:
        payload = {
            "title": self.title,
            "recording_date": self.recording_date,
            "duration_in_sec": self.duration_in_sec,
            "user_summary": self.user_summary,
        }
        if self.base64_data:
            payload["base64_data"] = self.base64_data
        else:
            payload["file_url"] = self.file_url
        return payload