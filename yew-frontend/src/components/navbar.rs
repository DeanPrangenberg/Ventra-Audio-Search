use yew::prelude::*;
use yew_router::prelude::*;

use crate::router::Route;

#[component]
pub fn Navbar() -> Html {
    html! {
        <nav class="navbar">
            <Link<Route> to={Route::Home} classes="nav-logo">
                <img src="/logo-icon.png" alt=""/>
                <span class="brand">{ "Ventra Audio Search" }</span>
            </Link<Route>>

            <div class="nav-links">
                <Link<Route> to={Route::Home} classes="nav-link">
                    { "Home" }
                </Link<Route>>
                <Link<Route> to={Route::Search} classes="nav-link">
                    { "Search" }
                </Link<Route>>
                <Link<Route> to={Route::Import} classes="nav-link">
                    { "Import" }
                </Link<Route>>
                <Link<Route> to={Route::Statistics} classes="nav-link">
                    { "Statistics" }
                </Link<Route>>
                <Link<Route> to={Route::Settings} classes="nav-link">
                    { "Settings" }
                </Link<Route>>
            </div>

            <div class="nav-control">
                <a
                    class="github-button"
                    href="https://github.com/DeanPrangenberg/Ventra-Audio-Search"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    <div class="github-button-icon"></div>
                </a>
            </div>
        </nav>
    }
}