package backend_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/davidsbond/tailscale-client-go/tailscale"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidsbond/vault-plugin-tailscale/backend"
)

func TestBackend_GenerateKey(t *testing.T) {
	ctx, b := setup(t)

	requestSchema := map[string]*framework.FieldSchema{
		"tags": {
			Type: framework.TypeStringSlice,
		},
		"preauthorized": {
			Type: framework.TypeBool,
		},
	}

	tt := []struct {
		Name          string
		Config        backend.Config
		Request       *logical.Request
		APIResponse   interface{}
		APIStatusCode int
		Data          *framework.FieldData
		Expected      map[string]interface{}
		ExpectsError  bool
	}{
		{
			Name: "It should call the API to generate a key and return it",
			Config: backend.Config{
				Tailnet: "example",
				APIUrl:  "http://localhost:1337",
			},
			Data: &framework.FieldData{
				Schema: requestSchema,
			},
			Request: logical.TestRequest(t, logical.ReadOperation, "config"),
			APIResponse: tailscale.Key{
				ID:  "12345",
				Key: "test",
			},
			APIStatusCode: http.StatusOK,
			Expected: map[string]interface{}{
				"ephemeral":     false,
				"expires":       time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
				"id":            "12345",
				"key":           "test",
				"reusable":      false,
				"tags":          []string(nil),
				"preauthorized": false,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			entry, err := logical.StorageEntryJSON("config", tc.Config)
			require.NoError(t, err)
			require.NoError(t, tc.Request.Storage.Put(ctx, entry))

			respondWith(t, tc.APIStatusCode, tc.APIResponse)
			response, err := b.GenerateKey(ctx, tc.Request, tc.Data)

			if tc.ExpectsError {
				assert.Error(t, response.Error())
				return
			}

			assert.EqualValues(t, tc.Expected, response.Data)
		})
	}
}

func TestBackend_ReadConfiguration(t *testing.T) {
	ctx, b := setup(t)

	tt := []struct {
		Name         string
		Config       *backend.Config
		Request      *logical.Request
		Data         *framework.FieldData
		Expected     map[string]interface{}
		ExpectsError bool
	}{
		{
			Name:    "It should read the backend configuration",
			Request: logical.TestRequest(t, logical.ReadOperation, "config"),
			Config: &backend.Config{
				Tailnet: "example.com",
				APIKey:  "1234",
				APIUrl:  "example.com",
			},
			Expected: map[string]interface{}{
				"tailnet": "example.com",
				"api_key": "1234",
				"api_url": "example.com",
			},
		},
		{
			Name:         "It should return an error if no configuration is set",
			Request:      logical.TestRequest(t, logical.ReadOperation, "config"),
			ExpectsError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Config != nil {
				entry, err := logical.StorageEntryJSON("config", tc.Config)
				require.NoError(t, err)
				require.NoError(t, tc.Request.Storage.Put(ctx, entry))
			}

			response, err := b.ReadConfiguration(ctx, tc.Request, tc.Data)
			assert.NoError(t, err)

			if tc.ExpectsError {
				assert.Error(t, response.Error())
				return
			}

			assert.EqualValues(t, tc.Expected, response.Data)
		})
	}
}

func TestBackend_UpdateConfiguration(t *testing.T) {
	ctx, b := setup(t)

	requestSchema := map[string]*framework.FieldSchema{
		"api_key": {
			Type: framework.TypeString,
		},
		"tailnet": {
			Type: framework.TypeString,
		},
		"api_url": {
			Type:    framework.TypeString,
			Default: "https://api.tailscale.com",
		},
	}

	tt := []struct {
		Name         string
		Request      *logical.Request
		Data         *framework.FieldData
		Expected     backend.Config
		ExpectsError bool
	}{
		{
			Name:    "It should update the backend configuration",
			Request: logical.TestRequest(t, logical.UpdateOperation, "config"),
			Data: &framework.FieldData{
				Schema: requestSchema,
				Raw: map[string]interface{}{
					"api_key": "12345",
					"tailnet": "example.com",
				},
			},
			Expected: backend.Config{
				Tailnet: "example.com",
				APIKey:  "12345",
				APIUrl:  "https://api.tailscale.com",
			},
		},
		{
			Name:    "It should return an error if the api key is missing",
			Request: logical.TestRequest(t, logical.UpdateOperation, "config"),
			Data: &framework.FieldData{
				Schema: requestSchema,
				Raw: map[string]interface{}{
					"tailnet": "example.com",
				},
			},
			ExpectsError: true,
		},
		{
			Name:    "It should return an error if the tailnet is missing",
			Request: logical.TestRequest(t, logical.UpdateOperation, "config"),
			Data: &framework.FieldData{
				Schema: requestSchema,
				Raw: map[string]interface{}{
					"api_key": "12345",
				},
			},
			ExpectsError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			response, err := b.UpdateConfiguration(ctx, tc.Request, tc.Data)
			assert.NoError(t, err)

			if tc.ExpectsError {
				assert.Error(t, response.Error())
				return
			}

			assert.EqualValues(t, tc.Expected, getConfig(t, ctx, tc.Request))
		})
	}
}

func setup(t *testing.T) (context.Context, *backend.Backend) {
	t.Helper()

	ctx := context.Background()

	b, err := backend.Create(ctx, logical.TestBackendConfig())
	require.NoError(t, err)

	return ctx, b.(*backend.Backend)
}

func getConfig(t *testing.T, ctx context.Context, request *logical.Request) backend.Config {
	t.Helper()

	entry, err := request.Storage.Get(ctx, "config")
	require.NoError(t, err)

	var config backend.Config
	require.NoError(t, entry.DecodeJSON(&config))

	return config
}

func respondWith(t *testing.T, code int, body interface{}) {
	t.Helper()

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		assert.NoError(t, json.NewEncoder(w).Encode(body))
	}))

	svr := &http.Server{
		Addr:    ":1337",
		Handler: mux,
	}

	go func() {
		_ = svr.ListenAndServe()
	}()

	t.Cleanup(func() {
		assert.NoError(t, svr.Close())
	})
}
