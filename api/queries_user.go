package api

import (
	"fmt"
	"net/http"
)

// User represents a GitCode user
type User struct {
	ID        interface{} `json:"id"`
	Login     string      `json:"login"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	AvatarURL string      `json:"avatar_url"`
	HTMLURL   string      `json:"html_url"`
	CreatedAt string      `json:"created_at"`
	Bio       string      `json:"bio"`
	Blog      string      `json:"blog"`
	Company   string      `json:"company"`
	Followers int         `json:"followers"`
	Following int         `json:"following"`
	Type      string      `json:"type"`
}

// Username returns the user's login name
func (u *User) Username() string {
	return u.Login
}

// CurrentUser fetches the current authenticated user
func CurrentUser(client *Client) (*User, error) {
	var user User
	err := client.Get("/user", &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUser fetches a user by username
func GetUser(client *Client, username string) (*User, error) {
	var user User
	err := client.Get(fmt.Sprintf("/users/%s", username), &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUserRequest is the request body for updating the current user.
type UpdateUserRequest struct {
	Nickname      string `json:"nickname,omitempty"`
	Company       string `json:"company,omitempty"`
	Description   string `json:"description,omitempty"`
	Email         string `json:"email,omitempty"`
	GithubAccount string `json:"github_account,omitempty"`
	Website       string `json:"website,omitempty"`
	Location      string `json:"location,omitempty"`
}

// UpdateUser updates the current authenticated user's profile.
// After updating, it re-reads the full user profile via GET /user
// to return a complete User object with all fields.
func UpdateUser(client *Client, req *UpdateUserRequest) (*User, error) {
	err := client.Patch("/user", req, nil)
	if err != nil {
		return nil, err
	}
	return CurrentUser(client)
}

// VerifyToken verifies that a token is valid by fetching the current user
func VerifyToken(httpClient *http.Client, host, token string) (*User, error) {
	client := NewClient(httpClient, host, token)
	return CurrentUser(client)
}
