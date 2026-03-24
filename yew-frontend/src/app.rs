use yew::prelude::*;
use yew_router::prelude::*;

use crate::components::navbar::Navbar;
use crate::router::{switch, Route};

#[component]
pub fn App() -> Html {
	html! {
        <BrowserRouter>
            <Navbar />
            <main class="page-content">
                <Switch<Route> render={switch} />
            </main>
        </BrowserRouter>
    }
}