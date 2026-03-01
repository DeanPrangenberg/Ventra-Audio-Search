import logging
from datetime import timezone, datetime
from email.utils import parsedate_to_datetime
from pprint import pprint

import feedparser


def rss_feed_to_import_payloads(url: str) -> list[dict[str, str]] | str:
    feed = feedparser.parse(url)

    if feed.status == 200:
        cleaned_entries: list = []
        for entry in feed.entries:
            cleand: dict = {
                "title": str(entry.title),
                "summary": str(entry.summary),
                "category": str(feed.title),
                "audio_type": "Media",
            }

            try:
                cleand["recording_date"] = rss_pubdate_to_iso(entry.published)
            except ValueError as e:
                cleand["recording_date"] = datetime.now().isoformat()
                logging.error(f"{e}, replaced it with current time")


            for link in entry.links:
                if link["type"] == "audio/mp3":
                    cleand["file_url"] = str(link["href"])
                    break

            cleaned_entries.append(cleand)

        pprint(feed.entries[0].links)  # Entry

    return f"Couldn't load Rss Feed: {url}"


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