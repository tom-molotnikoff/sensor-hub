package email

import (
	"context"
	"sync"
	"time"

	database "example/sensorHub/db"
	"example/sensorHub/secrets"
)

type fakeRepo struct {
	mu       sync.Mutex
	settings database.EmailSettings
	seeded   bool
}

func (r *fakeRepo) Get(context.Context) (database.EmailSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settings, nil
}

func (r *fakeRepo) Save(_ context.Context, s database.EmailSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s.LastError, s.LastSentAt = r.settings.LastError, r.settings.LastSentAt
	r.settings = s
	return nil
}

func (r *fakeRepo) RecordSent(_ context.Context, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings.LastError, r.settings.LastSentAt = nil, &at
	return nil
}

func (r *fakeRepo) RecordFailure(_ context.Context, sendErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings.LastError = &sendErr
	return nil
}

func (r *fakeRepo) SeedFromSMTPUser(_ context.Context, smtpUser string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seeded {
		return false, nil
	}
	r.seeded = true
	if smtpUser != "" {
		r.settings.Username, r.settings.FromAddress = smtpUser, smtpUser
	}
	return true, nil
}

// fakeSecrets holds plaintext by owner and name, and a status that can be
// forced to needs_reentry.
type fakeSecrets struct {
	values       map[string]string
	needsReentry bool
}

func newFakeSecrets() *fakeSecrets { return &fakeSecrets{values: map[string]string{}} }

func (f *fakeSecrets) Get(_ context.Context, owner, name string) (string, secrets.Status, error) {
	status := f.Status(owner, name)
	if status != secrets.StatusSet {
		return "", status, nil
	}
	return f.values[owner+"/"+name], status, nil
}

func (f *fakeSecrets) Set(_ context.Context, owner, name, value string) error {
	f.values[owner+"/"+name] = value
	f.needsReentry = false
	return nil
}

func (f *fakeSecrets) Delete(_ context.Context, owner, name string) error {
	delete(f.values, owner+"/"+name)
	f.needsReentry = false
	return nil
}

func (f *fakeSecrets) Status(owner, name string) secrets.Status {
	if f.needsReentry {
		return secrets.StatusNeedsReentry
	}
	if _, ok := f.values[owner+"/"+name]; ok {
		return secrets.StatusSet
	}
	return secrets.StatusUnset
}
