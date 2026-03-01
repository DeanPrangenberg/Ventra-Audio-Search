import logging
import os
import sys
import threading
import time
from pathlib import Path

import gradio as gr

import config_manager
import utils.file
import utils.rss
from routs import import_rout, config_rout, search_rout


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
        force=True,  # überschreibt evtl. Gradio/Lib configs
    )

if __name__ == "__main__":
    setup_logging("INFO")
    log = logging.getLogger("frontend")
    log.info("logging ready")

    with gr.Blocks() as demo:
        gr.Markdown(
            """
            # Audio Transcript Search
            
            Still in construction :)
            """
        )

    import_rout.mount_import_routes(demo)
    config_rout.mount_config_routes(demo)
    search_rout.mount_import_routes(demo)

    start_ttl_cleanup_thread()


    port = int(os.environ.get("PORT", "7860"))

    css_path = Path("src/theme.css")
    try:
        with open(css_path, "rt", encoding="utf-8") as f:
            css = f.read()
    except FileNotFoundError:
        logging.error("CSS file " + css_path.__str__() + " not found")
        css = ""

    demo.launch(allowed_paths=[config_manager.ConfigManager().get_upload_dir()], server_name="0.0.0.0", server_port=port, css=css)