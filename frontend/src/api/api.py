import requests
import logging
from src.api.payloads.import_payload import ImportPayload
import src.config_manager as config_manager


class API:
    def __init__(self):
        self.base_url = config_manager.ConfigManager().get_api_base_url()
        self.import_url = f"{self.base_url}/import"
        self.health_url = f"{self.base_url}/health"
        self.search_url = f"{self.base_url}/search"
        self.status_url = f"{self.base_url}/status"

    def import_request(self, payload_list: list[ImportPayload]) -> None | list[tuple[ImportPayload, str]]:
        request_payloads = [p.to_dict() for p in payload_list]

        r = requests.post(self.import_url, json=request_payloads, timeout=60)

        # robust: nicht blind r.json() machen
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
        invalid_import_list: list[tuple[ImportPayload, str]] = []

        match response_code:
            case 200:
                logging.info(
                    "Import successful. Imported %s audio files.",
                    response_json.get("imported", {}).get("count", 0),
                )
            case 413:
                error_str = f"Payload too large. The limit is {response_json.get('limit')} bytes."
            case 415:
                error_str = (
                    "Request content type is not supported. Content-Type must be application/json. "
                    f"Got {response_json.get('got')}"
                )
            case 400:
                error_str = f"Payload Json is invalid. Response: {response_json}"
            case 422:
                warning_str = "Every audio file in the payload is invalid."
            case 207:
                inv = response_json.get("invalid", {})
                warning_str = f"{inv.get('count', 0)} of {len(payload_list)} audio files in the payload are invalid."
            case _:
                # Unknown status -> treat as error, but keep backend text for debugging
                error_str = f"Unexpected status code {response_code}. Response: {response_json}"

        if error_str:
            logging.error(error_str)
            raise ValueError(error_str)

        if warning_str:
            logging.warning(warning_str)

            invalid = response_json.get("invalid", {})
            idxs = invalid.get("indexes", []) or []
            errs = invalid.get("errors", []) or []

            # indexes[k] gehört zu errors[k]
            for k, original_idx in enumerate(idxs):
                reason = errs[k] if k < len(errs) else "unknown"

                logging.warning(
                    "Audio file at index %s is invalid. Reason: %s",
                    original_idx,
                    reason,
                )

                # payload_list indexen mit original_idx, nicht mit k
                if isinstance(original_idx, int) and 0 <= original_idx < len(payload_list):
                    invalid_import_list.append((payload_list[original_idx], reason))
                else:
                    logging.error("Backend returned invalid index out of range: %r", original_idx)

            return invalid_import_list

        return None

    def health_check(self) -> bool:
        r = requests.get(self.health_url, timeout=10)
        return r.status_code == 200