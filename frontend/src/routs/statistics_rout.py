import os

import gradio as gr
import pandas as pd
from sqlalchemy import create_engine, text


def load_postgres_url():
    return os.environ.get("POSTGRES_URL", "user:password@localhost:5432/audio_transcript_db")


engine = create_engine(
    "postgresql+psycopg://" + load_postgres_url(),
    pool_pre_ping=True,
)


def make_card(title: str, value: str, subtext: str = "", tone: str = "neutral") -> str:
    return f"""
    <div class="stat-card stat-{tone}">
        <div class="stat-card-accent"></div>
        <div class="stat-title">{title}</div>
        <div class="stat-value">{value}</div>
        <div class="stat-sub">{subtext}</div>
    </div>
    """


def make_stage_strip(stages: list[tuple[str, int]]) -> str:
    parts = []
    last_idx = len(stages) - 1

    for idx, (label, value) in enumerate(stages):
        parts.append(
            f"""
            <div class="stage-node">
                <div class="stage-label">{label}</div>
                <div class="stage-value">{value}</div>
            </div>
            """
        )
        if idx != last_idx:
            parts.append('<div class="stage-arrow">→</div>')

    return f'<div class="stage-strip">{"".join(parts)}</div>'


def fetch_scalar(sql: str) -> int:
    with engine.connect() as conn:
        value = conn.execute(text(sql)).scalar_one()
        return int(value)


def fetch_timeseries(ts_column: str, value_name: str, hours: int = 24) -> pd.DataFrame:
    allowed_columns = {"created_at", "updated_at"}
    if ts_column not in allowed_columns:
        raise ValueError(f"unsupported timestamp column: {ts_column}")

    sql = text(
        f"""
        WITH series AS (
            SELECT generate_series(
                date_trunc('hour', now() - make_interval(hours => :hours)),
                date_trunc('hour', now()),
                interval '1 hour'
            ) AS bucket
        ),
        counts AS (
            SELECT
                date_trunc('hour', {ts_column}) AS bucket,
                COUNT(*)::int AS count
            FROM audiofiles
            WHERE {ts_column} IS NOT NULL
              AND {ts_column} >= now() - make_interval(hours => :hours)
            GROUP BY 1
        )
        SELECT
            series.bucket AS "Time",
            COALESCE(counts.count, 0) AS "{value_name}"
        FROM series
        LEFT JOIN counts
            ON series.bucket = counts.bucket
        ORDER BY series.bucket
        """
    )

    df = pd.read_sql_query(sql, engine, params={"hours": hours})
    df["Time"] = pd.to_datetime(df["Time"])
    return df


def load_stats():
    # -----------------------------
    # echte KPIs aus audiofiles
    # -----------------------------
    audio_amount = fetch_scalar("SELECT COUNT(*) FROM audiofiles")
    segment_amount = fetch_scalar("SELECT COUNT(*) FROM segments")
    search_requests = fetch_scalar("""
                                   SELECT SUM(counter_value) AS total
                                   FROM counters
                                   WHERE counter_name IN ('search_requests_failed', 'search_requests_successful');
                                   """)

    import_requests = fetch_scalar("""
                                   SELECT SUM(counter_value) AS total
                                   FROM counters
                                   WHERE counter_name IN ('import_requests_failed', 'import_requests_successful');
                                   """)

    audio_files_card = make_card("Audio Files", f"{audio_amount:,}", "rows in audiofiles")
    audio_segments_card = make_card("Segment Amount", f"{segment_amount:,}", "rows in segments")
    search_requests_card = make_card("Search Requests", f"{search_requests:,}", "search requests send")
    import_requests_card = make_card("Import Requests", f"{import_requests:,}", "import requests send")

    # -----------------------------
    # grobe Statussicht aus aktuellem Zustand
    # -----------------------------
    successful_imports = fetch_scalar(
        "SELECT COUNT(*) FROM audiofiles WHERE last_successful_stage = 5"
    )
    in_processing_queue = fetch_scalar(
        """
        SELECT COUNT(*)
        FROM audiofiles
        WHERE gets_processed = true
        """
    )
    awaits_processing = fetch_scalar(
        """
        SELECT COUNT(*)
        FROM audiofiles
        WHERE COALESCE(gets_processed, false) = false
          AND last_successful_stage != 5
        """
    )
    failed_imports = fetch_scalar(
        """
        SELECT COUNT(*)
        FROM audiofiles
        WHERE last_successful_stage = -1
        """
    )

    imported_card = make_card("Successful Audio Imports", str(successful_imports), "All Import steps Completed", "ok")
    processed_card = make_card("Audio In Queue", str(in_processing_queue), "Processing Item", "neutral")
    waiting_card = make_card("Audio Waiting", str(awaits_processing), "Waiting for Processing", "warn")
    errors_card = make_card("Audio Failed", str(failed_imports), "Steps failed 10 times", "err")

    # -----------------------------
    # Pipeline aus realen Feldern
    # -----------------------------
    stage_queued = fetch_scalar(
        """
        SELECT COUNT(*)
        FROM audiofiles
        WHERE last_successful_stage = 1
        """
    )
    stage_persisted = fetch_scalar(
        """
        SELECT COUNT(*)
        FROM audiofiles
        WHERE last_successful_stage = 2
        """
    )
    stage_transcribed = fetch_scalar(
        """
        SELECT COUNT(*)
        FROM audiofiles
        WHERE last_successful_stage = 3
        """
    )
    stage_embedded = fetch_scalar(
        """
        SELECT COUNT(*)
        FROM audiofiles
        WHERE last_successful_stage = 4
        """
    )

    stage_strip = make_stage_strip([
        ("Import Stage Persisting", stage_queued),
        ("Import Stage Transcribing", stage_persisted),
        ("Import Stage Embedding", stage_transcribed),
        ("Import Stage AI Generation", stage_embedded),
    ])

    # -----------------------------
    # echte DB-Zeitreihen
    # -----------------------------
    created_df = fetch_timeseries("created_at", "Created", hours=24)

    return (
        audio_files_card,
        audio_segments_card,
        search_requests_card,
        import_requests_card,
        imported_card,
        processed_card,
        waiting_card,
        errors_card,
        stage_strip,
        created_df,
    )


def mount_statistics_routes(app: gr.Blocks):
    with app.route("Statistics") as rout:
        gr.HTML("<h1>Backend <b>Statistic</b> Dashboard</h1>", elem_classes=["page-header"])
        with gr.Row():
            audio_files_display = gr.HTML()
            audio_segments_display = gr.HTML()
            search_requests_display = gr.HTML()
            import_requests_display = gr.HTML()

        with gr.Row():
            with gr.Column(scale=7):
                with gr.Row():
                    imported_display = gr.HTML()
                    processed_display = gr.HTML()
                    waiting_display = gr.HTML()
                    errors_display = gr.HTML()

                stage_strip_display = gr.HTML()

        with gr.Column(scale=5):
            with gr.Row():
                insertions_plot = gr.LinePlot(
                    x="Time",
                    y="Created",
                    title="Created rows (last 24h)",
                )

        timer = gr.Timer(value=1.0, active=True)

        outputs = [
            audio_files_display,
            audio_segments_display,
            search_requests_display,
            import_requests_display,
            imported_display,
            processed_display,
            waiting_display,
            errors_display,
            stage_strip_display,
            insertions_plot,
        ]

        timer.tick(
            fn=load_stats,
            inputs=[],
            outputs=outputs,
            queue=False,
        )

        rout.load(
            fn=load_stats,
            inputs=[],
            outputs=outputs,
        )
