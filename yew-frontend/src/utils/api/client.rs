use reqwest::Client;

pub struct ApiClient {
	pub(crate) url: String,
	pub(crate) client: Client,
}

impl ApiClient {
	pub fn new(url: String) -> Self {
		Self {
			url: url,
			client: Client::new(),
		}
	}

	pub fn update_url(&mut self, url: String) {
		self.url = url;
	}
}