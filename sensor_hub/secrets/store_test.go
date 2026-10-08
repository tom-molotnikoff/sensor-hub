package secrets

import (
	"context"
	"encoding/base64"
	"log/slog"
	"strings"
	"testing"

	database "example/sensorHub/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryRepo struct {
	rows map[Ref]database.SealedSecret
}

func newMemoryRepo() *memoryRepo { return &memoryRepo{rows: map[Ref]database.SealedSecret{}} }

func (r *memoryRepo) Put(_ context.Context, s database.SealedSecret) error {
	r.rows[Ref{Owner: s.Owner, Name: s.Name}] = s
	return nil
}

func (r *memoryRepo) Get(_ context.Context, owner, name string) (*database.SealedSecret, error) {
	s, ok := r.rows[Ref{Owner: owner, Name: name}]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func (r *memoryRepo) Delete(_ context.Context, owner, name string) error {
	delete(r.rows, Ref{Owner: owner, Name: name})
	return nil
}

func (r *memoryRepo) All(context.Context) ([]database.SealedSecret, error) {
	var all []database.SealedSecret
	for _, s := range r.rows {
		all = append(all, s)
	}
	return all, nil
}

func newTestStore(t *testing.T, repo Repository) *Store {
	t.Helper()
	key, err := GenerateKey()
	require.NoError(t, err)
	return newTestStoreWithKey(t, repo, key)
}

func newTestStoreWithKey(t *testing.T, repo Repository, key Key) *Store {
	t.Helper()
	s, err := NewStore(repo, key, slog.Default())
	require.NoError(t, err)
	return s
}

func TestStore_SetThenGetRoundTrips(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	s := newTestStore(t, repo)

	require.NoError(t, s.Set(ctx, "mqtt_broker:1", "password", "hunter2-broker"))

	value, status, err := s.Get(ctx, "mqtt_broker:1", "password")
	require.NoError(t, err)
	assert.Equal(t, "hunter2-broker", value)
	assert.Equal(t, StatusSet, status)
	assert.Equal(t, StatusSet, s.Status("mqtt_broker:1", "password"))

	stored := repo.rows[Ref{Owner: "mqtt_broker:1", Name: "password"}]
	assert.Equal(t, 1, stored.KeyID)
	assert.Len(t, stored.Nonce, 12)
	assert.NotContains(t, string(stored.Ciphertext), "hunter2-broker")
}

func TestStore_EveryWriteUsesAFreshNonce(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	s := newTestStore(t, repo)
	ref := Ref{Owner: "mqtt_broker:1", Name: "password"}

	require.NoError(t, s.Set(ctx, ref.Owner, ref.Name, "same value"))
	first := repo.rows[ref]
	require.NoError(t, s.Set(ctx, ref.Owner, ref.Name, "same value"))
	second := repo.rows[ref]

	assert.NotEqual(t, first.Nonce, second.Nonce)
	assert.NotEqual(t, first.Ciphertext, second.Ciphertext)
}

func TestStore_ACiphertextMovedToAnotherOwnerOrNameDoesNotDecrypt(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	s := newTestStore(t, repo)
	require.NoError(t, s.Set(ctx, "mqtt_broker:1", "password", "broker one"))
	original := repo.rows[Ref{Owner: "mqtt_broker:1", Name: "password"}]

	for _, target := range []Ref{{Owner: "mqtt_broker:2", Name: "password"}, {Owner: "mqtt_broker:1", Name: "other"}} {
		copied := original
		copied.Owner, copied.Name = target.Owner, target.Name
		repo.rows[target] = copied

		value, status, err := s.Get(ctx, target.Owner, target.Name)

		require.NoError(t, err)
		assert.Empty(t, value)
		assert.Equal(t, StatusNeedsReentry, status, "%v", target)
	}
}

func TestStore_DeleteMakesTheSecretUnset(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t, newMemoryRepo())
	require.NoError(t, s.Set(ctx, "mqtt_broker:1", "password", "pw"))

	require.NoError(t, s.Delete(ctx, "mqtt_broker:1", "password"))

	value, status, err := s.Get(ctx, "mqtt_broker:1", "password")
	require.NoError(t, err)
	assert.Empty(t, value)
	assert.Equal(t, StatusUnset, status)
	assert.Empty(t, s.StatusAll())
}

func TestStore_RefusesAnEmptySecret(t *testing.T) {
	s := newTestStore(t, newMemoryRepo())
	assert.Error(t, s.Set(context.Background(), "mqtt_broker:1", "password", ""))
}

func TestStore_LoadMarksSecretsFromAnotherKeyForReentry(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	require.NoError(t, newTestStore(t, repo).Set(ctx, "mqtt_broker:1", "password", "old key"))
	key, err := GenerateKey()
	require.NoError(t, err)
	require.NoError(t, newTestStoreWithKey(t, repo, key).Set(ctx, "mqtt_broker:2", "password", "current key"))

	restarted := newTestStoreWithKey(t, repo, key)
	require.NoError(t, restarted.load(ctx))

	assert.Equal(t, map[Ref]Status{
		{Owner: "mqtt_broker:1", Name: "password"}: StatusNeedsReentry,
		{Owner: "mqtt_broker:2", Name: "password"}: StatusSet,
	}, restarted.StatusAll())
}

func TestStore_SettingASecretAgainOverwritesItUnderTheCurrentKey(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	require.NoError(t, newTestStore(t, repo).Set(ctx, "mqtt_broker:1", "password", "old key"))
	require.NoError(t, newTestStore(t, repo).Set(ctx, "smtp", "password", "old key"))
	key, err := GenerateKey()
	require.NoError(t, err)
	restarted := newTestStoreWithKey(t, repo, key)
	require.NoError(t, restarted.load(ctx))
	require.Equal(t, []Ref{{Owner: "mqtt_broker:1", Name: "password"}, {Owner: "smtp", Name: "password"}}, restarted.NeedsReentry())

	require.NoError(t, restarted.Set(ctx, "mqtt_broker:1", "password", "entered again"))

	assert.Equal(t, StatusSet, restarted.Status("mqtt_broker:1", "password"))
	assert.Equal(t, []Ref{{Owner: "smtp", Name: "password"}}, restarted.NeedsReentry())
	value, status, err := newTestStoreWithKey(t, repo, key).Get(ctx, "mqtt_broker:1", "password")
	require.NoError(t, err)
	assert.Equal(t, StatusSet, status)
	assert.Equal(t, "entered again", value)
}

func TestStore_ForgetDropsAnOwnersStatuses(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t, newMemoryRepo())
	require.NoError(t, s.Set(ctx, "mqtt_broker:1", "password", "pw"))
	require.NoError(t, s.Set(ctx, "mqtt_broker:2", "password", "pw"))

	s.Forget("mqtt_broker:1")

	assert.Equal(t, StatusUnset, s.Status("mqtt_broker:1", "password"))
	assert.Equal(t, StatusSet, s.Status("mqtt_broker:2", "password"))
}

func TestKey_NeverPrintsItsBytes(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)
	encoded := key.Encode()

	var logged strings.Builder
	slog.New(slog.NewTextHandler(&logged, nil)).Info("key", "key", key)

	for _, rendered := range []string{key.String(), key.GoString(), logged.String()} {
		assert.NotContains(t, rendered, encoded)
	}
}

func TestParseKey(t *testing.T) {
	raw := make([]byte, KeySize)
	for i := range raw {
		raw[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)

	for _, text := range []string{encoded, encoded + "\n", encoded + "\r\n"} {
		key, err := ParseKey([]byte(text))
		require.NoError(t, err, "%q", text)
		assert.Equal(t, encoded, key.Encode())
	}
	for name, text := range map[string]string{
		"short":      base64.StdEncoding.EncodeToString(raw[:16]),
		"not base64": "not base64 at all!",
		"two lines":  encoded + "\n" + encoded,
		"empty":      "",
	} {
		_, err := ParseKey([]byte(text))
		assert.Error(t, err, name)
	}
}
