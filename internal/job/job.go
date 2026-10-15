package job

// Card is a list-page listing (no description).
type Card struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Location string `json:"location"`
	Posted   string `json:"posted,omitempty"`
	URL      string `json:"url"`
}

// Detail is a fetched job ad after Luna keeps the ID.
type Detail struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	Posted      string `json:"posted,omitempty"`
	URL         string `json:"url"`
	ApplyURL    string `json:"apply_url,omitempty"`
	Description string `json:"description"`
}

// Match is a ranked result from Gemini 4 Argon.
type Match struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Location string `json:"location"`
	URL      string `json:"url"`
	Score    int    `json:"score"`
	Why      string `json:"why"`
}

// ResultFile is written to --out (default output/matches.json).
type ResultFile struct {
	Query   string  `json:"query"`
	Area    string  `json:"area,omitempty"`
	Pages   int     `json:"pages"`
	Kept    int     `json:"kept"`
	Matches []Match `json:"matches"`
	Profile string  `json:"profile_path"`
	Source  string  `json:"source"`
}
