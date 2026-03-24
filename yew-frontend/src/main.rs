mod app;
mod components;
mod router;
mod utils;
mod routes;

use app::App;

fn main() {
    yew::Renderer::<App>::new().render();
}