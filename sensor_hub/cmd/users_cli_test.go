package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersDisableAndEnable_CallTheDisabledEndpoint(t *testing.T) {
	for verb, want := range map[string]bool{"disable": true, "enable": false} {
		t.Run(verb, func(t *testing.T) {
			var got *gen.SetUserDisabledJSONRequestBody
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/api/users/7/disabled", r.URL.Path)
				var body gen.SetUserDisabledJSONRequestBody
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				got = &body
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"message":"ok"}`))
			}))
			defer server.Close()

			_, _, err := executeRootCommand(t, "--server", server.URL, "users", verb, "7")

			require.NoError(t, err)
			require.NotNil(t, got, "the endpoint was called")
			assert.Equal(t, want, got.Disabled)
		})
	}
}
