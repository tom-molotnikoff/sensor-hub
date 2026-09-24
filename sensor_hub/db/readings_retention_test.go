package database

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadingsRepository_RetentionCutoffUsesStoredUTCFormat(t *testing.T) {
	bst := time.FixedZone("BST", 60*60)
	cutoff := time.Date(2026, 6, 1, 10, 0, 0, 0, bst)

	cases := []struct {
		name string
		run  func(r *ReadingsRepositoryImpl) error
		args []driver.Value
	}{
		{"all sensors", func(r *ReadingsRepositoryImpl) error {
			return r.DeleteReadingsOlderThan(context.Background(), cutoff)
		}, []driver.Value{"2026-06-01 09:00:00"}},
		{"one sensor", func(r *ReadingsRepositoryImpl) error {
			return r.DeleteReadingsOlderThanForSensor(context.Background(), cutoff, 7)
		}, []driver.Value{7, "2026-06-01 09:00:00"}},
		{"excluding sensors", func(r *ReadingsRepositoryImpl) error {
			return r.DeleteReadingsOlderThanExcludingSensors(context.Background(), cutoff, []int{7})
		}, []driver.Value{"2026-06-01 09:00:00", 7}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			repo := &ReadingsRepositoryImpl{db: handles(db)}
			mock.ExpectExec("DELETE FROM readings").WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, tc.run(repo))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
