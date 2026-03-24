import time
from typing import Any

from payload import SearchPayload
import requests

def search_request(payload: SearchPayload, url: str) -> tuple[bool, Any, float]:
    request_payload = payload.to_dict()

    start = time.perf_counter()
    r = requests.post(url, json=request_payload, timeout=600)
    end = time.perf_counter()

    respond_time = end - start

    ct = (r.headers.get("content-type") or "").lower()
    if "application/json" in ct:
        response_json = r.json()
    else:
        body = (r.text or "")[:800]
        raise ValueError(f"Backend returned non-JSON response ({r.status_code}): {body}")

    response_code = r.status_code

    match response_code:
        case 200:
            return True, response_json, respond_time
        case 413:
            return False, f"Payload too large. The limit is {response_json.get('limit')} bytes.", respond_time
        case 415:
            return False, (
                'Request content type is not supported. '
                '"application/json" or "application/json; charset=utf-8". '
                f'Got {response_json.get("got")}'
            ), respond_time
        case 400:
            return False, f"Payload JSON is invalid. Response: {response_json}", respond_time
        case 422:
            return False, f'{response_json.get("error")}', respond_time
        case _:
            return False, f"Unexpected status code {response_code}. Response: {response_json}", respond_time