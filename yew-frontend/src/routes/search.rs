use std::sync::Arc;

use wasm_bindgen_futures::spawn_local;
use web_sys::HtmlInputElement;
use yew::prelude::*;

use crate::utils::api::client::ApiClient;
use crate::utils::api::search::{SearchRequest, SearchResponse};

async fn send_search(
	client: Arc<ApiClient>,
	ts_query: String,
	semantic_search_query: String,
	category: String,
	start_time_period_iso: String,
	end_time_period_iso: String,
	max_segment_return: u64,
) -> Result<SearchResponse, reqwest::Error> {
	let req = SearchRequest {
		ts_query,
		semantic_search_query,
		category,
		start_time_period_iso,
		end_time_period_iso,
		max_segment_return,
	};

	client.search(req).await
}

#[function_component(SearchPage)]
pub fn search_page() -> Html {
	let request_url = option_env!("BASE_API_URL")
		.unwrap_or("http://100.103.135.123:8880")
		.to_string();

	let client = use_state(move || Arc::new(ApiClient::new(request_url)));

	let ts_query = use_state(String::new);
	let semantic_search_query = use_state(String::new);
	let category = use_state(String::new);
	let start_time_period_iso = use_state(String::new);
	let end_time_period_iso = use_state(String::new);
	let max_segment_return = use_state(|| 3_u64);

	let loading = use_state(|| false);
	let error_message = use_state(|| None::<String>);
	let search_result = use_state(|| None::<SearchResponse>);

	let ts_query_input = {
		let ts_query = ts_query.clone();
		Callback::from(move |e: InputEvent| {
			let input: HtmlInputElement = e.target_unchecked_into();
			ts_query.set(input.value());
		})
	};

	let semantic_search_query_input = {
		let semantic_search_query = semantic_search_query.clone();
		Callback::from(move |e: InputEvent| {
			let input: HtmlInputElement = e.target_unchecked_into();
			semantic_search_query.set(input.value());
		})
	};

	let category_input = {
		let category = category.clone();
		Callback::from(move |e: InputEvent| {
			let input: HtmlInputElement = e.target_unchecked_into();
			category.set(input.value());
		})
	};

	let start_time_period_iso_input = {
		let start_time_period_iso = start_time_period_iso.clone();
		Callback::from(move |e: InputEvent| {
			let input: HtmlInputElement = e.target_unchecked_into();
			start_time_period_iso.set(input.value());
		})
	};

	let end_time_period_iso_input = {
		let end_time_period_iso = end_time_period_iso.clone();
		Callback::from(move |e: InputEvent| {
			let input: HtmlInputElement = e.target_unchecked_into();
			end_time_period_iso.set(input.value());
		})
	};

	let max_segment_return_input = {
		let max_segment_return = max_segment_return.clone();
		Callback::from(move |e: InputEvent| {
			let input: HtmlInputElement = e.target_unchecked_into();
			let parsed = input.value().parse::<u64>().unwrap_or(3);
			max_segment_return.set(parsed);
		})
	};

	let on_search = {
		let client = (*client).clone();
		let ts_query = ts_query.clone();
		let semantic_search_query = semantic_search_query.clone();
		let category = category.clone();
		let start_time_period_iso = start_time_period_iso.clone();
		let end_time_period_iso = end_time_period_iso.clone();
		let max_segment_return = max_segment_return.clone();
		let loading = loading.clone();
		let error_message = error_message.clone();
		let search_result = search_result.clone();

		Callback::from(move |_| {
			loading.set(true);
			error_message.set(None);
			search_result.set(None);

			let client = client.clone();
			let ts_query = (*ts_query).clone();
			let semantic_search_query = (*semantic_search_query).clone();
			let category = (*category).clone();
			let start_time_period_iso = (*start_time_period_iso).clone();
			let end_time_period_iso = (*end_time_period_iso).clone();
			let max_segment_return = *max_segment_return;
			let loading = loading.clone();
			let error_message = error_message.clone();
			let search_result = search_result.clone();

			spawn_local(async move {
				match send_search(
					client,
					ts_query,
					semantic_search_query,
					category,
					start_time_period_iso,
					end_time_period_iso,
					max_segment_return,
				)
					.await
				{
					Ok(payload) => {
						search_result.set(Some(payload));
						error_message.set(None);
					}
					Err(err) => {
						search_result.set(None);
						error_message.set(Some(err.to_string()));
					}
				}

				loading.set(false);
			});
		})
	};

	let on_reset = {
		let ts_query = ts_query.clone();
		let semantic_search_query = semantic_search_query.clone();
		let category = category.clone();
		let start_time_period_iso = start_time_period_iso.clone();
		let end_time_period_iso = end_time_period_iso.clone();
		let max_segment_return = max_segment_return.clone();
		let loading = loading.clone();
		let error_message = error_message.clone();
		let search_result = search_result.clone();

		Callback::from(move |_| {
			ts_query.set(String::new());
			semantic_search_query.set(String::new());
			category.set(String::new());
			start_time_period_iso.set(String::new());
			end_time_period_iso.set(String::new());
			max_segment_return.set(3);
			loading.set(false);
			error_message.set(None);
			search_result.set(None);
		})
	};

	html! {
        <div class="search-page">
            <h1 class="page-header">
                { "Hybrid " }<b>{ "Context" }</b>{ " Search Engine" }
            </h1>

            <div class="search-ui-container">
                <div class="search-input-container">
                    <label for="ts-query-input">{ "Keywords:" }</label>
                    <input
                        id="ts-query-input"
                        type="text"
                        placeholder="TS Query"
                        value={(*ts_query).clone()}
                        oninput={ts_query_input}
                    />

                    <label for="semantic-query-input">{ "Question:" }</label>
                    <input
                        id="semantic-query-input"
                        type="text"
                        placeholder="What happened on ...?"
                        value={(*semantic_search_query).clone()}
                        oninput={semantic_search_query_input}
                    />

                    <label for="category-input">{ "Category:" }</label>
                    <input
                        id="category-input"
                        type="text"
                        placeholder="Category"
                        value={(*category).clone()}
                        oninput={category_input}
                    />

                    <label for="start-time-input">{ "Start:" }</label>
                    <input
                        id="start-time-input"
                        type="datetime-local"
                        value={(*start_time_period_iso).clone()}
                        oninput={start_time_period_iso_input}
                    />

                    <label for="end-time-input">{ "End:" }</label>
                    <input
                        id="end-time-input"
                        type="datetime-local"
                        value={(*end_time_period_iso).clone()}
                        oninput={end_time_period_iso_input}
                    />

                    <label for="max-segments-input">{ "Max segments:" }</label>
                    <input
                        id="max-segments-input"
                        type="number"
                        min="1"
                        max="100"
                        value={(*max_segment_return).to_string()}
                        oninput={max_segment_return_input}
                    />
                </div>

                <div class="search-button-container">
                    <button onclick={on_search.clone()} disabled={*loading}>
                        { if *loading { "Searching..." } else { "Search" } }
                    </button>

                    <button class="alt-button" onclick={on_reset} disabled={*loading}>
                        { "Reset" }
                    </button>
                </div>

                {
                    if *loading {
                        html! {
							<div id="wifi-loader">
							    <svg class="circle-outer" viewBox="0 0 86 86">
							        <circle class="back" cx="43" cy="43" r="40"></circle>
							        <circle class="front" cx="43" cy="43" r="40"></circle>
							        <circle class="new" cx="43" cy="43" r="40"></circle>
							    </svg>
							    <svg class="circle-middle" viewBox="0 0 60 60">
							        <circle class="back" cx="30" cy="30" r="27"></circle>
							        <circle class="front" cx="30" cy="30" r="27"></circle>
							    </svg>
							    <svg class="circle-inner" viewBox="0 0 34 34">
							        <circle class="back" cx="17" cy="17" r="14"></circle>
							        <circle class="front" cx="17" cy="17" r="14"></circle>
							    </svg>
							    <div class="text" data-text="Searching"></div>
							</div>
                        }
                    } else {
                        html! {}
                    }
                }

                {
                    if let Some(err) = &*error_message {
                        html! {
                            <div class="error-message">
                                { err }
                            </div>
                        }
                    } else {
                        html! {}
                    }
                }

                {
                    if let Some(result) = &*search_result {
                        html! {
                            <div class="search-result-container">
                                <p>{ format!("Request successful: {}", result.ok) }</p>
                                <p>{ format!("Audio results: {}", result.full_audio_data.len()) }</p>
                                <p>{ format!("Segment results: {}", result.top_k_segments.len()) }</p>
                            </div>
                        }
                    } else {
                        html! {}
                    }
                }
            </div>
        </div>
    }
}