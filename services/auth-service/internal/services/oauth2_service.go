package services

import (
	"auth-service/internal/config"
	"auth-service/internal/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type OAuth2Service interface {
	GetAuthURL(provider, state string) (string, error)
	HandleCallback(provider, code, state string) (*models.OAuth2UserInfo, error)
	GetProviderConfig(provider string) (*oauth2.Config, error)
}

type oauth2Service struct {
	configs map[string]*oauth2.Config
}

func NewOAuth2Service(cfg config.OAuth2Config) OAuth2Service {
	configs := make(map[string]*oauth2.Config)

	// Google OAuth2
	if cfg.Google.Enabled {
		configs["google"] = &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			RedirectURL:  cfg.Google.RedirectURL,
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint:     google.Endpoint,
		}
	}

	// GitHub OAuth2
	if cfg.GitHub.Enabled {
		configs["github"] = &oauth2.Config{
			ClientID:     cfg.GitHub.ClientID,
			ClientSecret: cfg.GitHub.ClientSecret,
			RedirectURL:  cfg.GitHub.RedirectURL,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		}
	}

	// Facebook OAuth2
	if cfg.Facebook.Enabled {
		configs["facebook"] = &oauth2.Config{
			ClientID:     cfg.Facebook.ClientID,
			ClientSecret: cfg.Facebook.ClientSecret,
			RedirectURL:  cfg.Facebook.RedirectURL,
			Scopes:       []string{"email", "public_profile"},
			Endpoint:     facebook.Endpoint,
		}
	}

	return &oauth2Service{configs: configs}
}

func (s *oauth2Service) GetAuthURL(provider, state string) (string, error) {
	config, exists := s.configs[provider]
	if !exists {
		return "", errors.New("unsupported OAuth provider")
	}

	return config.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (s *oauth2Service) HandleCallback(provider, code, state string) (*models.OAuth2UserInfo, error) {
	config, exists := s.configs[provider]
	if !exists {
		return nil, errors.New("unsupported OAuth provider")
	}

	// Exchange code for token
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Get user info from provider
	userInfo, err := s.getUserInfo(provider, config, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	userInfo.Provider = provider
	return userInfo, nil
}

func (s *oauth2Service) GetProviderConfig(provider string) (*oauth2.Config, error) {
	config, exists := s.configs[provider]
	if !exists {
		return nil, errors.New("unsupported OAuth provider")
	}
	return config, nil
}

func (s *oauth2Service) getUserInfo(provider string, config *oauth2.Config, token *oauth2.Token) (*models.OAuth2UserInfo, error) {
	client := config.Client(context.Background(), token)

	switch provider {
	case "google":
		return s.getGoogleUserInfo(client)
	case "github":
		return s.getGitHubUserInfo(client)
	case "facebook":
		return s.getFacebookUserInfo(client)
	default:
		return nil, errors.New("unsupported provider")
	}
}

func (s *oauth2Service) getGoogleUserInfo(client *http.Client) (*models.OAuth2UserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get user info from Google")
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, err
	}

	return &models.OAuth2UserInfo{
		ID:       googleUser.ID,
		Email:    googleUser.Email,
		Username: googleUser.Name,
		Avatar:   googleUser.Picture,
	}, nil
}

func (s *oauth2Service) getGitHubUserInfo(client *http.Client) (*models.OAuth2UserInfo, error) {
	// Get user profile
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get user info from GitHub")
	}

	var githubUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, err
	}

	// If email is not public, get it from emails endpoint
	if githubUser.Email == "" {
		email, err := s.getGitHubUserEmail(client)
		if err == nil {
			githubUser.Email = email
		}
	}

	username := githubUser.Name
	if username == "" {
		username = githubUser.Login
	}

	return &models.OAuth2UserInfo{
		ID:       fmt.Sprintf("%d", githubUser.ID),
		Email:    githubUser.Email,
		Username: username,
		Avatar:   githubUser.AvatarURL,
	}, nil
}

func (s *oauth2Service) getGitHubUserEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("failed to get user emails from GitHub")
	}

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary {
			return email.Email, nil
		}
	}

	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", errors.New("no email found")
}

func (s *oauth2Service) getFacebookUserInfo(client *http.Client) (*models.OAuth2UserInfo, error) {
	resp, err := client.Get("https://graph.facebook.com/me?fields=id,name,email,picture")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get user info from Facebook")
	}

	var facebookUser struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&facebookUser); err != nil {
		return nil, err
	}

	return &models.OAuth2UserInfo{
		ID:       facebookUser.ID,
		Email:    facebookUser.Email,
		Username: facebookUser.Name,
		Avatar:   facebookUser.Picture.Data.URL,
	}, nil
}