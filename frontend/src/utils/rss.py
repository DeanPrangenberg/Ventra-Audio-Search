import logging
from datetime import timezone, datetime
from email.utils import parsedate_to_datetime
import feedparser


def rss_feed_to_import_payloads(url: str) -> list[dict[str, str]] | str:
    try:
        feed = feedparser.parse(url)

        if not feed.entries:
            return "No episodes found in the RSS feed"

        cleaned_entries: list = []

        for entry in feed.entries:
            cleaned: dict = {
                "title": entry.get("title", "Unknown Episode"),
                "summary": entry.get("summary", ""),
                "category": feed.feed.get("title", "Podcast"),
                "audio_type": "Media",
            }

            try:
                cleaned["time"] = rss_pubdate_to_iso(entry.get("published", ""))
            except ValueError as e:
                cleaned["time"] = datetime.now().isoformat()
                logging.error(f"{e}, replaced it with current time")

            file_url = str(entry.get("link", ""))

            if file_url != "":
                cleaned["file_url"] = file_url
            else:
                for link in entry.get("links", []):
                    if link.get("type") == "audio/mp3":
                        cleaned["file_url"] = str(link.get("href", ""))
                        break

            cleaned_entries.append(cleaned)

        return cleaned_entries

    except Exception as e:
        return f"Couldn't load Rss Feed: {url} - {str(e)}"


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