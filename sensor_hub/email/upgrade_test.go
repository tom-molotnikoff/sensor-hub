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
// which an upgraded application.properties still holds.
func TestFinishUpgrade_DeletesTheOAuthFilesWhereThe15xPropertiesPutThem(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	token := filepath.Join(elsewhere, "gmail-token.json")
	require.NoError(t, os.WriteFile(token, []byte(`{"refresh_token":"y"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "client.json"), []byte(`{}`), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "token.json"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.properties"),
		[]byte("oauth.credentials.file.path=client.json\noauth.token.file.path="+token+"\n"), 0o640))
	logs := &bytes.Buffer{}

	require.NoError(t, FinishUpgrade(context.Background(), &fakeRepo{}, dir, slog.New(slog.NewTextHandler(logs, nil))))

	assert.NoFileExists(t, token)
	assert.NoFileExists(t, filepath.Join(dir, "client.json"))
	assert.DirExists(t, filepath.Join(dir, "token.json"), "only regular files are deleted")
	assert.Contains(t, logs.String(), "not a regular file")
}
