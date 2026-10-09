package email

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinishUpgrade_CarriesSMTPUserAndDeletesTheOAuthFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"smtp.properties":  "smtp.user=alerts@example.com\nsmtp.recipient=\n",
		"credentials.json": `{"installed":{"client_secret":"x"}}`,
		"token.json":       `{"refresh_token":"y"}`,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o640))
	}
	repo, logs := &fakeRepo{}, &bytes.Buffer{}

	require.NoError(t, FinishUpgrade(context.Background(), repo, dir, slog.New(slog.NewTextHandler(logs, nil))))

	assert.Equal(t, "alerts@example.com", repo.settings.Username)
	assert.Equal(t, "alerts@example.com", repo.settings.FromAddress)
	for _, name := range []string{"credentials.json", "token.json"} {
		assert.NoFileExists(t, filepath.Join(dir, name))
		assert.Equal(t, 1, strings.Count(logs.String(), filepath.Join(dir, name)), "one log line names %s", name)
	}
	assert.FileExists(t, filepath.Join(dir, "smtp.properties"), "the old file is left on disk")
}

func TestFinishUpgrade_NeedsNoLegacyFiles(t *testing.T) {
	repo, logs := &fakeRepo{}, &bytes.Buffer{}

	require.NoError(t, FinishUpgrade(context.Background(), repo, t.TempDir(), slog.New(slog.NewTextHandler(logs, nil))))

	assert.True(t, repo.seeded)
	assert.Empty(t, repo.settings.Username)
	assert.Empty(t, logs.String())
}

// 1.5.x could keep the OAuth files elsewhere, named by oauth.*.file.path,
// which an upgraded application.properties still holds. The hub deletes
// nothing at a path an operator chose; it warns at every start instead.
func TestFinishUpgrade_WarnsOfOAuthFilesThe15xPropertiesPutElsewhere(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	token := filepath.Join(elsewhere, "gmail-token.json")
	require.NoError(t, os.WriteFile(token, []byte(`{"refresh_token":"y"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "client.json"), []byte(`{}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"),
		[]byte("oauth.credentials.file.path=client.json\noauth.token.file.path="+token+"\n"), 0o640))

	for range 2 {
		logs := &bytes.Buffer{}
		require.NoError(t, FinishUpgrade(context.Background(), &fakeRepo{}, dir, slog.New(slog.NewTextHandler(logs, nil))))

		assert.FileExists(t, token)
		assert.FileExists(t, filepath.Join(dir, "client.json"))
		assert.Equal(t, 2, strings.Count(logs.String(), "level=WARN"), logs.String())
		assert.Contains(t, logs.String(), token)
		assert.Contains(t, logs.String(), filepath.Join(dir, "client.json"))
		assert.Contains(t, logs.String(), "revoke")
	}
}

// oauth.* properties that name the configuration directory's own files, as
// the 1.5.x defaults did, need no warning: those files are deleted.
func TestFinishUpgrade_SaysNothingOfOAuthPropertiesThatNameTheDefaults(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"),
		[]byte("oauth.credentials.file.path=credentials.json\noauth.token.file.path=token.json\nsensor.collection.interval=300\n"), 0o640))
	logs := &bytes.Buffer{}

	require.NoError(t, FinishUpgrade(context.Background(), &fakeRepo{}, dir, slog.New(slog.NewTextHandler(logs, nil))))

	assert.Empty(t, logs.String())
}
