package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/secrets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// brokerSecretsFixture is an MQTT service over a real database and a real
// secret store, whose repository can be swapped to make writes fail.
type brokerSecretsFixture struct {
	service *MQTTService
	brokers *database.MQTTBrokerRepository
	store   *secrets.Store
}

// failingSecretRepository stores nothing: every write fails.
type failingSecretRepository struct{ secrets.Repository }

func (failingSecretRepository) Put(context.Context, database.SealedSecret) error {
	return errors.New("disk full")
}

func newBrokerSecretsFixture(t *testing.T, failWrites bool) brokerSecretsFixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handles, err := database.Open(&appProps.ApplicationConfiguration{
		DatabasePath:              filepath.Join(t.TempDir(), "brokers.db"),
		DatabaseReaderConnections: 2,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { handles.Close() })

	var repo secrets.Repository = database.NewSecretRepository(handles)
	if failWrites {
		repo = failingSecretRepository{repo}
	}
	key, err := secrets.GenerateKey()
	require.NoError(t, err)
	store, err := secrets.NewStore(repo, key, logger)
	require.NoError(t, err)

	brokers := database.NewMQTTBrokerRepository(handles, logger)
	return brokerSecretsFixture{
		service: NewMQTTService(brokers, database.NewMQTTSubscriptionRepository(handles, logger), store, logger),
		brokers: brokers,
		store:   store,
	}
}

func externalBroker(name string, password *string) gen.MQTTBroker {
	return gen.MQTTBroker{Name: name, Type: "external", Host: ptrStr(name + ".lan"), Port: ptrInt(1883), Username: ptrStr("hub"), Password: password}
}

func TestMQTTService_AddBroker_StoresNoPasswordForAnEmptyOneOrThePlaceholder(t *testing.T) {
	f := newBrokerSecretsFixture(t, false)

	for name, password := range map[string]*string{"empty": ptrStr(""), "placeholder": ptrStr("****")} {
		_, err := f.service.AddBroker(context.Background(), externalBroker(name, password))
		require.NoError(t, err, name)
	}

	assert.Empty(t, f.store.StatusAll())
}

func TestMQTTService_AddBroker_LeavesNoBrokerWhenItsPasswordCannotBeStored(t *testing.T) {
	f := newBrokerSecretsFixture(t, true)

	_, err := f.service.AddBroker(context.Background(), externalBroker("home", ptrStr("s3cret")))

	require.Error(t, err)
	stored, err := f.brokers.GetByName(context.Background(), "home")
	require.NoError(t, err)
	assert.Nil(t, stored, "a broker without the password it was created with is removed again")
}

func TestMQTTService_DeleteBroker_DropsThePasswordStatus(t *testing.T) {
	f := newBrokerSecretsFixture(t, false)
	ctx := context.Background()
	id, err := f.service.AddBroker(ctx, externalBroker("home", ptrStr("s3cret")))
	require.NoError(t, err)
	require.Equal(t, secrets.StatusSet, f.store.Status(database.BrokerSecretOwner(id), database.BrokerPasswordSecret))

	require.NoError(t, f.service.DeleteBroker(ctx, id))

	assert.Empty(t, f.store.StatusAll())
}
