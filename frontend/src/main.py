import logging
import os
import sys
import threading
import time
from pathlib import Path
import gradio as gr
import utils.file
import utils.rss
from routs import config_rout, search_rout, statistics_rout
from routs.import_rout.render import mount_import_routes

# Clean files every 5 min
def start_ttl_cleanup_thread():
    def loop():
        while True:
            utils.file.cleanup_upload_dir_ttl()
            time.sleep(300)  # alle 5 min

    t = threading.Thread(target=loop, daemon=True)
    t.start()


def setup_logging(level: str = "INFO") -> None:
    logging.basicConfig(
        level=getattr(logging, level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        handlers=[logging.StreamHandler(sys.stdout)],
        force=True,
    )


def load_css_with_font_paths() -> tuple[str, Path]:
    base_dir = Path(__file__).resolve().parent
    css_path = base_dir / "theme.css"
    fonts_dir = (base_dir.parent / "res" / "fonts").resolve()
    logo_path = (base_dir.parent / "res" / "logo.png").resolve()

    try:
        css = css_path.read_text(encoding="utf-8")
    except FileNotFoundError:
        logging.error("CSS file %s not found", css_path)
        return "", fonts_dir

    font_files = {
        "__FONT_JUNICODE_REGULAR__": fonts_dir / "Junicode.woff2",
        "__FONT_JUNICODE_BOLD__": fonts_dir / "Junicode-Bold.woff2",
        "__FONT_JUNICODE_ITALIC__": fonts_dir / "Junicode-Italic.woff2",
        "__FONT_JUNICODE_BOLD_ITALIC__": fonts_dir / "Junicode-BoldItalic.woff2",
        "__LOGO__": logo_path,
    }

    for placeholder, font_path in font_files.items():
        css = css.replace(placeholder, f"/gradio_api/file={font_path.as_posix()}")

    return css, fonts_dir



if __name__ == "__main__":
    setup_logging("INFO")
    log = logging.getLogger("frontend")
    log.info("logging ready")

    with gr.Blocks() as demo:
        gr.HTML("""
           <section class="zen-hero">
                <div class="zen-hero-inner">
                    <h1>Welcome to <b>Ventra</b> Audio Search</h1>
                    <p class="zen-copy">
                        a local containerized audio ingestion and hybrid search platform.
                    </p>
                </div>
            </section>
           """, elem_classes=["page-header"])

    search_rout.mount_search_routes(demo)
    mount_import_routes(demo)
    statistics_rout.mount_statistics_routes(demo)
    config_rout.mount_config_routes(demo)

    start_ttl_cleanup_thread()

    port = int(os.environ.get("PORT", "7861"))

    css, fonts_dir = load_css_with_font_paths()

    demo.launch(allowed_paths=[str(fonts_dir)],
                server_name="0.0.0.0",
                server_port=port,
                css=css,
    )
