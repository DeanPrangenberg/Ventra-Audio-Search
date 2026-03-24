use yew::prelude::*;
use yew_router::prelude::*;
use crate::routes::search::SearchPage;

#[derive(Clone, Routable, PartialEq)]
pub enum Route {
	#[at("/")]
	Home,
	#[at("/search")]
	Search,
	#[at("/import")]
	Import,
	#[at("/statistics")]
	Statistics,
	#[at("/settings")]
	Settings,

	#[not_found]
	#[at("/404")]
	NotFound,
}

#[function_component(HomePage)]
fn home_page() -> Html {
	html! {
		<section class="zen-hero">
        <div class="zen-hero-inner">
            <h1>{ "Welcome to" } <b>{ " Ventra" }</b>{" Audio Search"}</h1>
            <p class="zen-copy">
                {"a local containerized audio ingestion and hybrid search platform."}
            </p>
        </div>
    </section>
	}
}

#[function_component(ImportPage)]
fn import_page() -> Html {
	html! { <h1 class="page-header">{ "Import " }<b>{ "Audio" }</b> { " Data" }</h1> }
}

#[function_component(StatisticsPage)]
fn statistics_page() -> Html {
	html! { <h1 class="page-header">{ "Backend " }<b>{ "Statistic" }</b> { " Dashboard" }</h1> }
}

#[function_component(SettingsPage)]
fn settings_page() -> Html {
	html! { <h1 class="page-header">{ "Frontend " }<b>{ "Config" }</b> { " Menu" }</h1> }
}

#[function_component(NotFoundPage)]
fn not_found_page() -> Html {
	html! { <h1>{ "404" }</h1> }
}

pub fn switch(route: Route) -> Html {
	match route {
		Route::Home => html! { <HomePage /> },
		Route::Search => html! { <SearchPage /> },
		Route::Import => html! { <ImportPage /> },
		Route::Statistics => html! { <StatisticsPage /> },
		Route::Settings => html! { <SettingsPage /> },
		Route::NotFound => html! { <NotFoundPage /> },
	}
}