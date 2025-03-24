package trickest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// User represents the current user's information
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

// Profile represents the user's profile information
type Profile struct {
	VaultInfo  VaultInfo `json:"vault_info"`
	Bio        string    `json:"bio"`
	Type       int       `json:"type"`
	Username   string    `json:"username"`
	EntityType string    `json:"entity_type"`
}

// VaultInfo represents the user's vault information
type VaultInfo struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Type         int       `json:"type"`
	Metadata     string    `json:"metadata"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
}

type IPAddress struct {
	IPAddress string `json:"ip_address"`
}

type Fleet struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Vault    uuid.UUID `json:"vault"`
	Cluster  string    `json:"cluster"`
	State    string    `json:"state"`
	Machines []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Mem         string `json:"mem"`
		CPU         string `json:"cpu"`
		Total       int    `json:"total"`
		Running     int    `json:"running"`
		Up          int    `json:"up"`
		Down        int    `json:"down"`
	} `json:"machines"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
	Type         string    `json:"type"`
	Default      bool      `json:"default"`
}

// GetCurrentUser retrieves the current user's information
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var user User
	if err := c.doJSON(ctx, http.MethodGet, "/users/me/", nil, &user); err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	return &user, nil
}

// GetVaultIPAddresses retrieves the static IP addresses for the current user's vault
func (c *Client) GetVaultIPAddresses(ctx context.Context) ([]IPAddress, error) {
	path := fmt.Sprintf("/ip/?vault=%s", c.vaultID)

	ipAddresses, err := GetPaginated[IPAddress](c, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP addresses: %w", err)
	}

	return ipAddresses, nil
}

func (c *Client) GetFleets(ctx context.Context) ([]Fleet, error) {
	path := fmt.Sprintf("/fleet/?vault=%s", c.vaultID)

	fleets, err := GetPaginated[Fleet](c, ctx, path, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get fleets: %w", err)
	}

	return fleets, nil
}

func (c *Client) GetFleetByName(ctx context.Context, fleetName string) (*Fleet, error) {
	fleets, err := c.GetFleets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get fleets: %w", err)
	}

	if len(fleets) == 0 {
		return nil, fmt.Errorf("no fleets found")
	}

	for _, fleet := range fleets {
		if fleet.Name == fleetName {
			return &fleet, nil
		}
	}

	return nil, fmt.Errorf("fleet \"%s\" not found", fleetName)
}
