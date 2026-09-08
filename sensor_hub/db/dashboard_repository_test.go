package database

import (
	"context"
	"log/slog"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestDashboardRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	repo := NewDashboardRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT INTO dashboards").
		WithArgs(1, "My Dashboard", `{"widgets":[]}`, false, false).
		WillReturnResult(sqlmock.NewResult(42, 1))

	dashboard := &gen.Dashboard{UserId: 1, Name: "My Dashboard", Config: `{"widgets":[]}`, Shared: false, IsDefault: false}
	id, err := repo.Create(context.Background(), dashboard)

	assert.NoError(t, err)
	assert.Equal(t, 42, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardRepository_GetById(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	repo := NewDashboardRepository(handles(db), slog.Default())

	rows := sqlmock.NewRows([]string{"id", "user_id", "name", "config", "shared", "is_default", "created_at", "updated_at"}).
		AddRow(1, 1, "Test", `{"widgets":[]}`, false, true, "2026-03-31 00:00:00", "2026-03-31 00:00:00")
	mock.ExpectQuery("SELECT .+ FROM dashboards WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(rows)

	d, err := repo.GetById(context.Background(), 1)

	assert.NoError(t, err)
	assert.NotNil(t, d)
	assert.Equal(t, "Test", d.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardRepository_GetById_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	repo := NewDashboardRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT .+ FROM dashboards WHERE id = \\?").
		WithArgs(999).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "config", "shared", "is_default", "created_at", "updated_at"}))

	d, err := repo.GetById(context.Background(), 999)

	assert.NoError(t, err)
	assert.Nil(t, d)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardRepository_GetByUserId(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	repo := NewDashboardRepository(handles(db), slog.Default())

	rows := sqlmock.NewRows([]string{"id", "user_id", "name", "config", "shared", "is_default", "created_at", "updated_at"}).
		AddRow(1, 1, "Default", `{"widgets":[]}`, false, true, "2026-03-31 00:00:00", "2026-03-31 00:00:00").
		AddRow(2, 1, "Custom", `{"widgets":[]}`, false, false, "2026-03-31 00:00:00", "2026-03-31 00:00:00")
	mock.ExpectQuery("SELECT .+ FROM dashboards WHERE user_id = \\? OR shared = 1").
		WithArgs(1).
		WillReturnRows(rows)

	dashboards, err := repo.GetByUserId(context.Background(), 1)

	assert.NoError(t, err)
	assert.Len(t, dashboards, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	repo := NewDashboardRepository(handles(db), slog.Default())

	mock.ExpectExec("UPDATE dashboards SET").
		WithArgs("Updated", `{"widgets":[]}`, false, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Update(context.Background(), &gen.Dashboard{Id: 1, Name: "Updated", Config: `{"widgets":[]}`, Shared: false})

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	repo := NewDashboardRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM dashboards WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(context.Background(), 1)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardRepository_SetDefault(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	repo := NewDashboardRepository(handles(db), slog.Default())

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE dashboards SET is_default = 0 WHERE user_id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE dashboards SET is_default = 1").
		WithArgs(5, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.SetDefault(context.Background(), 1, 5)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
