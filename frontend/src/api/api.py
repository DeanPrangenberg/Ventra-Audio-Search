import requests
import logging

from api.payloads.search_payload import SearchPayload
from src.api.payloads.import_payload import ImportPayload
import src.config_manager as config_manager


class API:
    def __init__(self):
        self.base_url = config_manager.ConfigManager().get_api_base_url()
        self.import_url = f"{self.base_url}/import"
        self.health_url = f"{self.base_url}/health"
        self.search_url = f"{self.base_url}/search"
        self.status_url = f"{self.base_url}/status"

    def import_request(self, payload_list: list[ImportPayload]) -> None | list[dict] | str:
        request_payloads = [p.to_dict() for p in payload_list]
        r = requests.post(self.import_url, json=request_payloads, timeout=60)

        ct = (r.headers.get("content-type") or "").lower()
        if "application/json" in ct:
            response_json = r.json()
        else:
            body = (r.text or "")[:800]
            logging.error("Backend returned non-JSON response (%s, %s): %r", r.status_code, ct, body)
            raise ValueError(f"Backend returned non-JSON response ({r.status_code}): {body}")

        response_code = r.status_code

        error_str = ""
        warning_str = ""

        all_invalid = False

        match response_code:
            case 200:
                logging.info("Import successful. Imported %s audio files.",
                             response_json.get("imported", {}).get("count", 0))
                return None
            case 413:
                error_str = f"Payload too large. The limit is {response_json.get('limit')} bytes."
                all_invalid = True
            case 415:
                error_str = ("Request content type is not supported. Content-Type must be application/json. "
                             f"Got {response_json.get('got')}")
            case 400:
                error_str = f"Payload Json is invalid. Response: {response_json}"
            case 422:
                warning_str = "Every audio file in the payload is invalid."
                all_invalid = True
            case 207:
                inv = response_json.get("invalid", {})
                warning_str = f"{inv.get('count', 0)} of {len(payload_list)} audio files in the payload are invalid."
            case _:
                error_str = f"Unexpected status code {response_code}. Response: {response_json}"

        if error_str:
            logging.error(error_str)
            return error_str

        if warning_str:
            logging.warning(warning_str)

        invalid = response_json.get("invalid", {}) or {}
        idxs = invalid.get("indexes") or []
        errs = invalid.get("errors") or []

        if all_invalid and not idxs:
            return [{"index": i, "error": "invalid (backend returned 422 without per-item details)"} for i in
                    range(len(payload_list))]

        result: list[dict] = []
        for k, original_idx in enumerate(idxs):
            reason = errs[k] if k < len(errs) else "unknown"
            if isinstance(original_idx, int) and 0 <= original_idx < len(payload_list):
                result.append({"index": original_idx, "error": str(reason)})
            else:
                logging.error("Backend returned invalid index out of range: %r", original_idx)

        return result

    def health_check(self) -> bool:
        r = requests.get(self.health_url, timeout=10)
        return r.status_code == 200

    def search_request(self, payload: SearchPayload):
        request_payloads = payload.to_dict()
        r = requests.get(self.import_url, json=request_payloads, timeout=600)

        ct = (r.headers.get("content-type") or "").lower()
        if "application/json" in ct:
            response_json = r.json()
        else:
            body = (r.text or "")[:800]
            logging.error("Backend returned non-JSON response (%s, %s): %r", r.status_code, ct, body)
            raise ValueError(f"Backend returned non-JSON response ({r.status_code}): {body}")

        response_code = r.status_code

        error_str = ""

        match response_code:
            case 200:
                logging.info("Search Request successful")

                return None
            case 413:
                error_str = f"Payload too large. The limit is {response_json.get('limit')} bytes."
            case 415:
                error_str = ("Request content type is not supported. \"application/json\" or \"application/json; charset=utf-8\""
                             f"Got {response_json.get('got')}")
            case 400:
                error_str = f"Payload Json is invalid. Response: {response_json}"
            case 422:
                error_str = f"{response_json.get('error')}"
            case _:
                error_str = f"Unexpected status code {response_code}. Response: {response_json}"

        logging.error(error_str)
        return error_str

