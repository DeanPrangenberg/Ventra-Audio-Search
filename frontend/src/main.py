import os

import gradio as gr
import logging
import sys

from routs import import_rout, config_rout, search_rout
import config_manager

def setup_logging(level: str = "INFO") -> None:
    logging.basicConfig(
        level=getattr(logging, level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        handlers=[logging.StreamHandler(sys.stdout)],
        force=True,  # überschreibt evtl. Gradio/Lib configs
    )

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

port = int(os.environ.get("PORT", "7860"))

demo.launch(allowed_paths=[config_manager.ConfigManager().get_upload_dir()], server_name="0.0.0.0", server_port=port)