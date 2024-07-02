package types

import (
	"time"
)

type Files struct {
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Page     int    `json:"page"`
	Last     int    `json:"last"`
	Count    int    `json:"count"`
	Results  []File `json:"results"`
}

type File struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         int       `json:"size"`
	PrettySize   string    `json:"pretty_size"`
	ModifiedDate time.Time `json:"modified_date"`
}
