from typing import Any

import gradio as gr

import src.routs.import_rout.import_file_upload as file_upload
import src.routs.import_rout.import_rss_url as rss_upload


def render_selected_colum(dropdown) -> list[dict[str, Any]]:
    return [
        gr.update(visible=dropdown == "Podcast-Rss-Feed"),
        gr.update(visible=dropdown == "File-Upload"),
    ]


def mount_import_routes(app: gr.Blocks):
    with app.route("Import"):
        gr.HTML("<h1>Import <b>Audio</b> Data</h1>", elem_classes=["page-header"])
        gr.Markdown(
            """
            This page uploads new audio files. For each file you have set metadata,
            this metadata will be used in the search process.
            """
        )

        audio_import_type = gr.Dropdown(
            label="Choose a way to import your files",
            choices=["Podcast-Rss-Feed", "File-Upload"],
            value="File-Upload",
            interactive=True,
            elem_classes = "no-bg",
        )

        with gr.Column(visible=False) as rss_col:
            rss_upload.mount_rss_renderer()

        with gr.Column(visible=True) as upload_col:
            file_upload.mount_uploaded_files_renderer()

        audio_import_type.change(
            fn=render_selected_colum,
            inputs=[audio_import_type],
            outputs=[rss_col, upload_col],
        )
