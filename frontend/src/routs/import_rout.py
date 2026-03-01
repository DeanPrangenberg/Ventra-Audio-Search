import os

import gradio as gr

import config_manager
import src.utils.file as file_utils
import src.utils.state as state_utils


def mount_import_routes(app: gr.Blocks):
    with app.route("Import") as import_rout:
        gr.Markdown("# Import Audio Files")
        gr.Markdown(
            """
        This page uploads new audio files. For each file you have set metadata, this metadate will be used in the search process.
        """
            )

        # Holds [{orig_name, stored_path, file_url, title, summary}, ...]
        files_state = gr.State([])

        audio_import_type = gr.Dropdown(
            label="Choose away to import your files",
            choices=["Podcast-Rss-Feed", "File-Upload", "File-Url"],
            interactive=True,
        )

        file_upload = gr.File(
            label="Upload MP3 Audio Files",
            file_types=[".mp3"],
            file_count="multiple",
            type="filepath",
        )

        # When upload changes, persist files + create urls
        file_upload.change(
            fn=file_utils.persist_and_make_state,
            inputs=[file_upload],
            outputs=[files_state],
        )

        @gr.render(inputs=files_state)
        def show_audio_files(state):
            if not state:
                gr.Markdown("No files uploaded.")
                return

            send_result = gr.Markdown()

            for idx, f in enumerate(state):
                label = f"{f['orig_name']}"

                with gr.Accordion(label=label, open=True):
                    if f.get("error"):
                        gr.Markdown(f" **Error:** {f['error']}", elem_id="error-markdown")

                    title = gr.Textbox(label="Set a Title", value=f.get("title", ""))
                    record_time = gr.DateTime(
                        label="Enter Recording date & time",
                        value=f.get("time", None),
                    )
                    category = gr.Dropdown(label="Choose a Category", choices=config_manager.ConfigManager().get_category_list(), value=f.get("category", None),
                                           interactive=True)
                    audio_type = gr.Dropdown(label="Choose a Audio Type", value=f.get("audio_type", "Meeting"),
                                             choices=["Meeting", "Media", "Generic"], interactive=True)
                    summary = gr.Textbox(label="Write a little summary", value=f.get("summary", ""), lines=3)

                    # Update state when fields change
                    title.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "title"),
                        inputs=[title, files_state],
                        outputs=[files_state],
                        queue=False,
                        api_visibility="private",
                    )
                    category.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "category"),
                        inputs=[category, files_state],
                        outputs=[files_state],
                        queue=False,
                        api_visibility="private",
                    )
                    audio_type.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "audio_type"),
                        inputs=[audio_type, files_state],
                        outputs=[files_state],
                        queue=False,
                        api_visibility="private",
                    )
                    summary.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "summary"),
                        inputs=[summary, files_state],
                        outputs=[files_state],
                        queue=False,
                        api_visibility="private",
                    )
                    record_time.change(
                        fn=lambda v, s, i=idx: state_utils.update_meta_array(v, s, i, "time"),
                        inputs=[record_time, files_state],
                        outputs=[files_state],
                        queue=False,
                        api_visibility="private",
                    )

                    gr.Audio(f["stored_path"], label=os.path.basename(f["stored_path"]))

            send_btn = gr.Button("Send configured files to backend", variant="primary")
            send_btn.click(
                fn=do_backend_request,
                inputs=[files_state],
                outputs=[files_state, file_upload, send_result],
            )