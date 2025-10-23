package types

import (
	"time"

	"github.com/google/uuid"
)

type Config struct {
	User struct {
		Token         string
		TokenFilePath string
		VaultId       uuid.UUID
	}
	BaseUrl    string
	Dependency string
}

type User struct {
	ID            int     `json:"id"`
	IsActive      bool    `json:"is_active"`
	Email         string  `json:"email"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	Onboarding    bool    `json:"onboarding"`
	Profile       Profile `json:"profile"`
	InitialCredit int     `json:"initial_credit"`
}

type Profile struct {
	VaultInfo  VaultInfo `json:"vault_info"`
	Bio        string    `json:"bio"`
	Type       int       `json:"type"`
	Username   string    `json:"username"`
	EntityType string    `json:"entity_type"`
}

type VaultInfo struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Type         int       `json:"type"`
	Metadata     string    `json:"metadata"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
}

type Fleets struct {
	Next     string  `json:"next"`
	Previous string  `json:"previous"`
	Page     int     `json:"page"`
	Last     int     `json:"last"`
	Count    int     `json:"count"`
	Results  []Fleet `json:"results"`
}

type Fleet struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Vault    uuid.UUID `json:"vault"`
	Cluster  string    `json:"cluster"`
	State    string    `json:"state"`
	Machines struct {
		Active   int `json:"active"`
		Deleting int `json:"deleting"`
		Inactive int `json:"inactive"`
		Max      int `json:"max"`
	} `json:"machines"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Type         string    `json:"type"`
	Default      bool      `json:"default"`
}

type IPAddresses struct {
	Next     string      `json:"next"`
	Previous string      `json:"previous"`
	Page     int         `json:"page"`
	Last     int         `json:"last"`
	Count    int         `json:"count"`
	Results  []IPAddress `json:"results"`
}

type IPAddress struct {
	IPAddress string `json:"ip_address"`
}
