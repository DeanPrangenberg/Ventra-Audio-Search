import logging
from datetime import timezone, datetime
from email.utils import parsedate_to_datetime
import feedparser


import logging
import re
from datetime import datetime
import feedparser


MP3_URL_RE = re.compile(r"https?://[^\s\"'<>]+?\.mp3(?:\?[^\s\"'<>]*)?", re.IGNORECASE)


def extract_audio_url(entry) -> str:
    # 1) Beste Quelle: echte RSS/Atom-Enclosures
    for enc in entry.get("enclosures", []):
        href = str(enc.get("href", "")).strip()
        mime = str(enc.get("type", "")).lower().strip()

        if href and (
            mime.startswith("audio/") or
            ".mp3" in href.lower()
        ):
            return href

    # 2) Fallback: normale links-Liste durchsuchen
    for link in entry.get("links", []):
        href = str(link.get("href", "")).strip()
        rel = str(link.get("rel", "")).lower().strip()
        mime = str(link.get("type", "")).lower().strip()

        if href and (
            rel == "enclosure" or
            mime.startswith("audio/") or
            ".mp3" in href.lower()
        ):
            return href

    # 3) Letzter Fallback: alle String-Felder im Entry nach mp3-URL absuchen
    for _, value in entry.items():
        if isinstance(value, str):
            match = MP3_URL_RE.search(value)
            if match:
                return match.group(0)

        elif isinstance(value, list):
            for item in value:
                if isinstance(item, str):
                    match = MP3_URL_RE.search(item)
                    if match:
                        return match.group(0)
                elif isinstance(item, dict):
                    for sub_value in item.values():
                        if isinstance(sub_value, str):
                            match = MP3_URL_RE.search(sub_value)
                            if match:
                                return match.group(0)

        elif isinstance(value, dict):
            for sub_value in value.values():
                if isinstance(sub_value, str):
                    match = MP3_URL_RE.search(sub_value)
                    if match:
                        return match.group(0)

    return ""


def rss_feed_to_import_payloads(url: str) -> list[dict[str, str]] | str:
    try:
        feed = feedparser.parse(url)

        if not feed.entries:
            return "No episodes found in the RSS feed"

        cleaned_entries: list[dict[str, str]] = []

        for entry in feed.entries:
            cleaned: dict[str, str] = {
                "title": str(entry.get("title", "Unknown Episode")),
                "summary": str(entry.get("summary", "")),
                "category": str(feed.feed.get("title", "Podcast")),
                "audio_type": "Media",
            }

            try:
                cleaned["time"] = rss_pubdate_to_iso(entry.get("published", ""))
            except ValueError as e:
                cleaned["time"] = datetime.now().isoformat()
                logging.error("%s, replaced it with current time", e)

            file_url = extract_audio_url(entry)

            if not file_url:
                logging.warning(
                    "No audio URL found for entry: %s",
                    entry.get("title", "Unknown Episode"),
                )
                continue

            cleaned["file_url"] = file_url
            print(cleaned["file_url"])
            cleaned_entries.append(cleaned)

        if not cleaned_entries:
            return "No audio files found in the RSS feed"

        return cleaned_entries

    except Exception as e:
        return f"Couldn't load RSS feed: {url} - {e}"


def rss_pubdate_to_iso(pub_date: str) -> str:
    if not pub_date or not pub_date.strip():
        raise ValueError("Could not parse RSS pubDate: it was empty")

    dt = parsedate_to_datetime(pub_date.strip())

    if dt is None:
        raise ValueError(f"Could not parse RSS pubDate: {pub_date!r}")

    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)

    dt = dt.astimezone(timezone.utc)

    return dt.isoformat(timespec="seconds").replace("+00:00", "Z")