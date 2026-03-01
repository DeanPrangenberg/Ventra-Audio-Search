from typing import Any

import gradio as gr

import file_upload
import src.utils.file as file_utils


def render_selected_colum(
    dropdown
) -> list[dict[str, Any]]:
   return [
       gr.update(visible=dropdown == "Podcast-Rss-Feed"),
       gr.update(visible=dropdown == "File-Upload"),
       gr.update(visible=dropdown == "File-Url")
   ]

def mount_import_routes(app: gr.Blocks):
    with app.route("Import"):
        gr.Markdown("# Import Audio Files")
        gr.Markdown(
            """
            This page uploads new audio files. For each file you have set metadata,
            this metadata will be used in the search process.
            """
        )

        audio_import_type = gr.Dropdown(
            label="Choose a way to import your files",
            choices=["Podcast-Rss-Feed", "File-Upload", "File-Url"],
            value="File-Upload",
            interactive=True,
        )

        with gr.Column(visible=False) as rss_col:
            rss_url = gr.Textbox(label="Podcast RSS Feed URL")
            rss_import_btn = gr.Button("Load Podcast Feed")

        """with gr.Column(visible=False) as upload_col:
            file_upload.mount_uploaded_files_renderer()"""

        with gr.Column(visible=False) as url_col:
            file_url = gr.Textbox(label="Direct audio file URL")
            url_import_btn = gr.Button("Load from URL")


        """audio_import_type.change(
            fn=render_selected_colum,
            inputs=[audio_import_type],
            outputs=[rss_col, upload_col, url_col],
        )"""