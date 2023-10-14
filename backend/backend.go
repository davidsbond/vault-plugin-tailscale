// Package backend provides the Vault plugin backend that is used to generate authentication keys for Tailscale
// devices.
package backend

import (
	"context"
	"errors"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/tailscale/tailscale-client-go/tailscale"
)

type (
	// The Backend type is responsible for handling inbound requests from Vault to serve Tailscale authentication
	// keys.
	Backend struct {
		*framework.Backend
	}

	// The Config type describes the configuration fields used by the Backend
	Config struct {
		Tailnet           string   `json:"tailnet"`
		APIKey            string   `json:"api_key"`
		APIUrl            string   `json:"api_url"`
		OAuthClientID     string   `json:"oauth_client_id"`
		OAuthClientSecret string   `json:"oauth_client_secret"`
		OAuthScopes       []string `json:"oauth_scopes"`
	}
)

func (c Config) Client() (*tailscale.Client, error) {
	if c.APIKey != "" {
		return tailscale.NewClient(c.APIKey, c.Tailnet, tailscale.WithBaseURL(c.APIUrl))
	}

	return tailscale.NewClient("", c.Tailnet,
		tailscale.WithBaseURL(c.APIUrl),
		tailscale.WithOAuthClientCredentials(c.OAuthClientID, c.OAuthClientSecret, c.OAuthScopes),
	)
}

const (
	backendHelp              = "The Tailscale backend is used to generate Tailscale authentication keys for a configured Tailnet"
	readKeyDescription       = "Generate a single-use authentication key for a device"
	readConfigDescription    = "Read the current Tailscale backend configuration"
	updateConfigDescription  = "Update the Tailscale backend configuration"
	apiKeyDescription        = "The API key to use for authenticating with the Tailscale API"
	tailnetDescription       = "The name of the Tailscale Tailnet"
	tagsDescription          = "Tags to apply to the device that uses the authentication key"
	preauthorizedDescription = "If true, machines added to the tailnet with this key will not required authorization"
	apiUrlDescription        = "The URL of the Tailscale API"
	ephemeralDescription     = "If true, nodes created with this key will be removed after a period of inactivity or when they disconnect from the Tailnet"
)

// Create a new logical.Backend implementation that can generate authentication keys for Tailscale devices.
func Create(ctx context.Context, config *logical.BackendConfig) (logical.Backend, error) {
	backend := &Backend{}
	backend.Backend = &framework.Backend{
		BackendType: logical.TypeLogical,
		Help:        backendHelp,
		Paths: []*framework.Path{
			{
				Pattern: "key",
				Fields: map[string]*framework.FieldSchema{
					"tags": {
						Type:        framework.TypeStringSlice,
						Description: tagsDescription,
					},
					"preauthorized": {
						Type:        framework.TypeBool,
						Description: preauthorizedDescription,
					},
					"ephemeral": {
						Type:        framework.TypeBool,
						Description: ephemeralDescription,
					},
				},
				Operations: map[logical.Operation]framework.OperationHandler{
					logical.ReadOperation: &framework.PathOperation{
						Summary:  readKeyDescription,
						Callback: backend.GenerateKey,
					},
				},
			},
			{
				Pattern: "config",
				Fields: map[string]*framework.FieldSchema{
					"api_key": {
						Type:        framework.TypeString,
						Description: apiKeyDescription,
					},
					"tailnet": {
						Type:        framework.TypeString,
						Description: tailnetDescription,
					},
					"api_url": {
						Type:        framework.TypeString,
						Description: apiUrlDescription,
						Default:     "https://api.tailscale.com",
					},
					"oauth_client_id": {
						Type: framework.TypeString,
					},
					"oauth_client_secret": {
						Type: framework.TypeString,
					},
					"oauth_scopes": {
						Type: framework.TypeStringSlice,
					},
				},
				Operations: map[logical.Operation]framework.OperationHandler{
					logical.ReadOperation: &framework.PathOperation{
						Callback: backend.ReadConfiguration,
						Summary:  readConfigDescription,
					},
					logical.UpdateOperation: &framework.PathOperation{
						Callback: backend.UpdateConfiguration,
						Summary:  updateConfigDescription,
					},
				},
			},
		},
	}

	return backend, backend.Setup(ctx, config)
}

const (
	configPath = "config"
)

// GenerateKey generates a new authentication key via the Tailscale API. This method checks the existing Backend configuration
// for the Tailnet and API key. It will return an error if the configuration does not exist.
func (b *Backend) GenerateKey(ctx context.Context, request *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	entry, err := request.Storage.Get(ctx, configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err = entry.DecodeJSON(&config); err != nil {
		return nil, err
	}

	client, err := config.Client()
	if err != nil {
		return nil, err
	}

	var capabilities tailscale.KeyCapabilities
	capabilities.Devices.Create.Tags = data.Get("tags").([]string)
	capabilities.Devices.Create.Preauthorized = data.Get("preauthorized").(bool)
	capabilities.Devices.Create.Ephemeral = data.Get("ephemeral").(bool)

	key, err := client.CreateKey(ctx, capabilities)
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"id":            key.ID,
			"key":           key.Key,
			"expires":       key.Expires,
			"tags":          key.Capabilities.Devices.Create.Tags,
			"reusable":      key.Capabilities.Devices.Create.Reusable,
			"ephemeral":     key.Capabilities.Devices.Create.Ephemeral,
			"preauthorized": key.Capabilities.Devices.Create.Preauthorized,
		},
	}, nil
}

// ReadConfiguration reads the Backend configuration and returns its values.
func (b *Backend) ReadConfiguration(ctx context.Context, request *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	entry, err := request.Storage.Get(ctx, configPath)
	switch {
	case err != nil:
		return nil, err
	case entry == nil:
		return nil, errors.New("configuration has not been set")
	}

	var config Config
	if err = entry.DecodeJSON(&config); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"tailnet":             config.Tailnet,
			"api_key":             config.APIKey,
			"api_url":             config.APIUrl,
			"oauth_client_id":     config.OAuthClientID,
			"oauth_client_secret": config.OAuthClientSecret,
			"oauth_scopes":        config.OAuthScopes,
		},
	}, nil
}

// UpdateConfiguration modifies the Backend configuration. Returns an error if any required fields are missing.
func (b *Backend) UpdateConfiguration(ctx context.Context, request *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config := Config{
		Tailnet:           data.Get("tailnet").(string),
		APIKey:            data.Get("api_key").(string),
		APIUrl:            data.Get("api_url").(string),
		OAuthScopes:       data.Get("oauth_scopes").([]string),
		OAuthClientSecret: data.Get("oauth_client_secret").(string),
		OAuthClientID:     data.Get("oauth_client_id").(string),
	}

	switch {
	case config.Tailnet == "":
		return nil, errors.New("provided tailnet cannot be empty")
	case config.APIKey == "" && config.OAuthClientID == "":
		return nil, errors.New("one of api_key or oauth_client_id cannot be empty")
	case config.APIUrl == "":
		return nil, errors.New("provided api_url cannot be empty")
	}

	entry, err := logical.StorageEntryJSON(configPath, config)
	if err != nil {
		return nil, err
	}

	if err = request.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	return &logical.Response{}, nil
}
