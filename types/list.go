package types

import "time"

type Spaces struct {
	Next     string  `json:"next"`
	Previous string  `json:"previous"`
	Page     int     `json:"page"`
	Last     int     `json:"last"`
	Count    int     `json:"count"`
	Results  []Space `json:"results"`
}

type Space struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	VaultInfo    string    `json:"vault_info"`
	Metadata     string    `json:"metadata"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Playground   bool      `json:"playground"`
}
