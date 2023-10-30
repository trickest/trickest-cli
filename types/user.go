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
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	HiveType     string    `json:"hive_type"`
	Vault        uuid.UUID `json:"vault"`
	Cluster      string    `json:"cluster"`
	State        string    `json:"state"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Machines     []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Mem         string `json:"mem"`
		CPU         string `json:"cpu"`
		Total       int    `json:"total"`
		Running     int    `json:"running"`
		Up          int    `json:"up"`
		Down        int    `json:"down"`
		Error       int    `json:"error"`
	} `json:"machines"`
}
