package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration38_SendsSecretFailuresInAppButNotByEmail(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(38))

	var email, inApp bool
	require.NoError(t, db.QueryRow(
		"SELECT email_enabled, inapp_enabled FROM notification_channel_defaults WHERE category = 'secret_failure'",
	).Scan(&email, &inApp))
	assert.False(t, email)
	assert.True(t, inApp)

	require.NoError(t, m.Migrate(37))
	var defaults int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM notification_channel_defaults WHERE category = 'secret_failure'").Scan(&defaults))
	assert.Zero(t, defaults)
}
