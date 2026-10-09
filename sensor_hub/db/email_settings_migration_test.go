package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration39_RenamesManageOAuthKeepingItsGrants(t *testing.T) {
	db := newTempFileDB(t)
	m := newTestMigrator(t, db)
	require.NoError(t, m.Migrate(38))
	_, err := db.Exec(`INSERT INTO roles (name) VALUES ('mailer')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'mailer' AND p.name = 'manage_oauth'`)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(39))

	rolesWith := func(permission string) []string {
		rows, err := db.Query(`SELECT r.name FROM roles r
			JOIN role_permissions rp ON rp.role_id = r.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE p.name = ? ORDER BY r.name`, permission)
		require.NoError(t, err)
		defer rows.Close()
		var names []string
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			names = append(names, name)
		}
		return names
	}
	assert.Equal(t, []string{"admin", "mailer"}, rolesWith("manage_email"))
	assert.Empty(t, rolesWith("manage_oauth"))

	require.NoError(t, m.Migrate(38))
	assert.Equal(t, []string{"admin", "mailer"}, rolesWith("manage_oauth"))
}

func TestMigration39_StartsTheEmailSettingsOnGmail(t *testing.T) {
	h := newMigratedHandles(t)

	settings, err := NewEmailSettingsRepository(h).Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, EmailSettings{Host: "smtp.gmail.com", Port: 587, Security: "starttls"}, settings)
}

func TestEmailSettings_SeedFromSMTPUserRunsOnceOnAnUntouchedRow(t *testing.T) {
	ctx := context.Background()
	repo := NewEmailSettingsRepository(newMigratedHandles(t))

	seeded, err := repo.SeedFromSMTPUser(ctx, "alerts@example.com")
	require.NoError(t, err)
	assert.True(t, seeded)
	settings, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "alerts@example.com", settings.Username)
	assert.Equal(t, "alerts@example.com", settings.FromAddress)

	seeded, err = repo.SeedFromSMTPUser(ctx, "someone-else@example.com")
	require.NoError(t, err)
	assert.False(t, seeded)
	settings, err = repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "alerts@example.com", settings.Username)
}

func TestEmailSettings_SeedNeverOverwritesSavedSettings(t *testing.T) {
	ctx := context.Background()
	repo := NewEmailSettingsRepository(newMigratedHandles(t))
	saved := EmailSettings{Host: "smtp.example.com", Port: 465, Security: "implicit_tls", Username: "hub", FromAddress: "hub@example.com"}
	require.NoError(t, repo.Save(ctx, saved))

	seeded, err := repo.SeedFromSMTPUser(ctx, "alerts@example.com")

	require.NoError(t, err)
	assert.False(t, seeded)
	settings, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, saved, settings)
}
