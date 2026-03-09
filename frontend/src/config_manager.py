import json
import logging
import os
from pathlib import Path


class ConfigManager:
    def __init__(self, config_path: str = "config.json"):
        base_dir = self._resolve_base_dir()
        self._config_path = str(base_dir / config_path)
        self._default_data_dir = str(base_dir)
        self._config = {}
        self.load_config()
        logging.debug(f"Loaded Config: {self._config}")

    def _resolve_base_dir(self) -> Path:
        env_data_dir = os.environ.get("DATA_DIR")
        if env_data_dir:
            return Path(env_data_dir)

        app_data_dir = Path("/app/data")
        if app_data_dir.exists() and app_data_dir.is_dir():
            return app_data_dir

        return Path.cwd()

    def load_config(self):
        try:
            with open(self._config_path, "r") as f:
                self._config = json.load(f)
        except FileNotFoundError:
            logging.warning(f"Config file not found at {self._config_path}. Using default config.")
            self._config = {
                "api_base_url": os.environ.get("AUDIO_TRANSCRIPT_SERVER_URL", "http://audio-transcript-server:8880"),
                "upload_dir": str(Path(self._default_data_dir) / "uploads"),
                "category": ["Standard"],
            }
            self.save_config()

    def save_config(self):
        try:
            Path(self._config_path).parent.mkdir(parents=True, exist_ok=True)
            with open(self._config_path, "w") as f:
                json.dump(self._config, f, indent=4)
        except Exception as e:
            logging.error(f"Error saving config: {e}")

    def get_api_base_url(self):
        self.load_config()
        return self._config.get("api_base_url", "http://localhost:8880")

    def set_api_base_url(self, url: str):
        self._config["api_base_url"] = url
        self.save_config()

    def get_upload_dir(self):
        self.load_config()
        return self._config.get("upload_dir", "uploads")

    def get_category_list(self) -> list[str]:
        self.load_config()
        cat = self._config.get("category", ["Standard"])

        if isinstance(cat, str):
            cat = [p.strip() for p in cat.split(",") if p.strip()]
        elif isinstance(cat, list):
            cat = [str(x).strip() for x in cat if str(x).strip()]
        else:
            cat = ["Standard"]

        if not cat:
            cat = ["Standard"]

        return cat

    def get_category_csv(self) -> str:
        return ", ".join(self.get_category_list())

    def set_categories(self, category_str: str) -> None:
        categories = [part.strip() for part in (category_str or "").split(",")]
        categories = [c for c in categories if c]

        if not categories:
            categories = ["Standard"]

        self._config["category"] = categories
        self.save_config()

    def extend_categories(self, new_category_name: str) -> None:
        categories = self.get_category_list()
        new_category = new_category_name.strip().replace(",", ";")

        if new_category and new_category not in categories:
            categories.append(new_category)

        self._config["category"] = categories or ["Standard"]
        self.save_config()