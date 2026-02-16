import gradio as gr
import src.config_manager as config_manager


def mount_config_routes(app: gr.Blocks):
    with app.route("Config"):
        gr.Markdown("# Frontend Config / Settings")
        gr.Markdown(
            "This page displays and lets you edit settings for the frontend of **Ventra Audio Search**"
        )

        cfg = config_manager.ConfigManager()

        needs_save_state = gr.State(False)

        backend_url = gr.Text(
            label="Backend Url (with port)",
            placeholder="http://audio-transcript-server:8880",
            value=cfg.get_api_base_url(),
            interactive=True,
        )

        category_input = gr.Text(
            label="Registered Categories (comma-separated)",
            placeholder="Team Name, Meeting Type",
            value=cfg.get_category_csv(),
            interactive=True,
        )

        with gr.Row():
            save_button = gr.Button(value="Save Config", variant="primary", interactive=False)
            reset_button = gr.Button(value="Reset Config", variant="secondary")

        # ---- callbacks ----

        def mark_dirty():
            return True, gr.update(interactive=True)

        def reset():
            # Reload current config values
            new_url = cfg.get_api_base_url()
            new_cat = cfg.get_category_csv()
            return (
                new_url,
                new_cat,
                False,
                gr.update(interactive=False),
            )

        def save(url: str, cats: str):
            # Passe die Methodennamen an deine ConfigManager-API an:
            # z.B. cfg.set_api_base_url(url), cfg.set_category(cats), cfg.save()
            cfg.set_api_base_url(url)
            cfg.set_category(cats)

            return False, gr.update(interactive=False)

        # When any input changes -> dirty
        backend_url.change(
            fn=mark_dirty,
            inputs=[],
            outputs=[needs_save_state, save_button],
            queue=False,
        )
        category_input.change(
            fn=mark_dirty,
            inputs=[],
            outputs=[needs_save_state, save_button],
            queue=False,
        )

        # Reset button -> restore saved values and clear dirty
        reset_button.click(
            fn=reset,
            inputs=[],
            outputs=[backend_url, category_input, needs_save_state, save_button],
            queue=False,
        )

        # Save button -> persist and clear dirty
        save_button.click(
            fn=save,
            inputs=[backend_url, category_input],
            outputs=[needs_save_state, save_button],
            queue=False,
        )