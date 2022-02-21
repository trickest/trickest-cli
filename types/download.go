package types

type OutputsConfig struct {
	Outputs []string `yaml:"outputs"`
}

type SignedURL struct {
	Url        string `json:"url"`
	Size       int    `json:"size"`
	PrettySize string `json:"pretty_size"`
}

type SubJobOutputs struct {
	Next     string         `json:"next"`
	Previous string         `json:"previous"`
	Page     int            `json:"page"`
	Last     int            `json:"last"`
	Count    int            `json:"count"`
	Results  []SubJobOutput `json:"results"`
}

type SubJobOutput struct {
	ID         string `json:"id"`
	FileName   string `json:"file_name"`
	Size       int    `json:"size"`
	PrettySize string `json:"pretty_size"`
	Format     string `json:"format"`
	Path       string `json:"path"`
	SignedURL  string `json:"signed_url,omitempty"`
}
