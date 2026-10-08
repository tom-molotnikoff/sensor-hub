package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"

	database "example/sensorHub/db"
)

// Status is what may be said about a secret without revealing it.
type Status string

const (
	StatusUnset Status = "unset"
	StatusSet   Status = "set"
	// StatusNeedsReentry marks a stored secret that does not decrypt under the
	// current key, such as after the key was replaced.
	StatusNeedsReentry Status = "needs_reentry"
)

// currentKeyID is stored with every secret written, so a later key rotation
// can tell which key sealed it.
const currentKeyID = 1

const nonceSize = 12

// Ref names a secret: what it belongs to and which of its secrets it is.
type Ref struct {
	Owner string
	Name  string
}

// Repository is where the sealed secrets are kept.
type Repository interface {
	Put(ctx context.Context, s database.SealedSecret) error
	Get(ctx context.Context, owner, name string) (*database.SealedSecret, error)
	Delete(ctx context.Context, owner, name string) error
	All(ctx context.Context) ([]database.SealedSecret, error)
}

// Store encrypts secrets into the repository and decrypts them back. It keeps
// each secret's status in memory, learnt by decrypting it at startup and
// after every write, and never keeps a plaintext.
type Store struct {
	repo   Repository
	aead   cipher.AEAD
	logger *slog.Logger

	mu     sync.RWMutex
	status map[Ref]Status // only stored secrets; absent means unset
}

func NewStore(repo Repository, key Key, logger *slog.Logger) (*Store, error) {
	block, err := aes.NewCipher(key.bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to set up AES-256: %w", err)
	}
	aead, err := cipher.NewGCMWithNonceSize(block, nonceSize)
	if err != nil {
		return nil, fmt.Errorf("failed to set up AES-GCM: %w", err)
	}
	return &Store{
		repo:   repo,
		aead:   aead,
		logger: logger.With("component", "secrets"),
		status: make(map[Ref]Status),
	}, nil
}

// Open readies the store for a starting hub. It finishes the 2.0 upgrade by
// moving any plaintext broker passwords into the store, then decrypts every
// stored secret to learn its status.
func Open(ctx context.Context, db *database.Handles, key Key, logger *slog.Logger) (*Store, error) {
	s, err := NewStore(database.NewSecretRepository(db), key, logger)
	if err != nil {
		return nil, err
	}
	moved, err := database.MoveBrokerPasswordsToSecrets(ctx, db, s.seal, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to move broker passwords into the secret store: %w", err)
	}
	if moved > 0 {
		s.logger.Info("encrypted the outbound broker passwords into the secret store and scrubbed the plaintext", "count", moved)
	}
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load(ctx context.Context) error {
	stored, err := s.repo.All(ctx)
	if err != nil {
		return err
	}
	status := make(map[Ref]Status, len(stored))
	for _, sealed := range stored {
		ref := Ref{Owner: sealed.Owner, Name: sealed.Name}
		if _, err := s.open(sealed); err != nil {
			s.logger.Warn("a stored secret does not decrypt under the current key", "owner", ref.Owner, "name", ref.Name)
			status[ref] = StatusNeedsReentry
			continue
		}
		status[ref] = StatusSet
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	return nil
}

// Set encrypts value and stores it, replacing any secret held under the same
// owner and name. An empty value is refused: Delete removes a secret.
func (s *Store) Set(ctx context.Context, owner, name, value string) error {
	if value == "" {
		return errors.New("a secret cannot be empty")
	}
	sealed, err := s.seal(owner, name, []byte(value))
	if err != nil {
		return err
	}
	if err := s.repo.Put(ctx, sealed); err != nil {
		return err
	}
	s.setStatus(Ref{Owner: owner, Name: name}, StatusSet)
	return nil
}

// Get decrypts the secret. The value is empty unless the status is set.
func (s *Store) Get(ctx context.Context, owner, name string) (string, Status, error) {
	ref := Ref{Owner: owner, Name: name}
	sealed, err := s.repo.Get(ctx, owner, name)
	if err != nil {
		return "", "", err
	}
	if sealed == nil {
		s.setStatus(ref, StatusUnset)
		return "", StatusUnset, nil
	}
	value, err := s.open(*sealed)
	if err != nil {
		s.setStatus(ref, StatusNeedsReentry)
		return "", StatusNeedsReentry, nil
	}
	s.setStatus(ref, StatusSet)
	return string(value), StatusSet, nil
}

func (s *Store) Delete(ctx context.Context, owner, name string) error {
	if err := s.repo.Delete(ctx, owner, name); err != nil {
		return err
	}
	s.setStatus(Ref{Owner: owner, Name: name}, StatusUnset)
	return nil
}

// Forget drops what the store knows about an owner whose secrets were
// deleted along with it, such as a deleted broker.
func (s *Store) Forget(owner string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ref := range s.status {
		if ref.Owner == owner {
			delete(s.status, ref)
		}
	}
}

func (s *Store) Status(owner, name string) Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if status, ok := s.status[Ref{Owner: owner, Name: name}]; ok {
		return status
	}
	return StatusUnset
}

// StatusAll gives the status of every stored secret. A secret not stored is
// unset and absent from the map.
func (s *Store) StatusAll() map[Ref]Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return maps.Clone(s.status)
}

func (s *Store) setStatus(ref Ref, status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status == StatusUnset {
		delete(s.status, ref)
		return
	}
	s.status[ref] = status
}

// associatedData binds a ciphertext to its owner and name, so a ciphertext
// copied onto another row fails to decrypt.
func associatedData(owner, name string) []byte {
	return []byte(owner + "\x00" + name)
}

func (s *Store) seal(owner, name string, value []byte) (database.SealedSecret, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return database.SealedSecret{}, fmt.Errorf("failed to generate a nonce: %w", err)
	}
	return database.SealedSecret{
		Owner:      owner,
		Name:       name,
		KeyID:      currentKeyID,
		Nonce:      nonce,
		Ciphertext: s.aead.Seal(nil, nonce, value, associatedData(owner, name)),
	}, nil
}

func (s *Store) open(sealed database.SealedSecret) ([]byte, error) {
	if sealed.KeyID != currentKeyID {
		return nil, fmt.Errorf("sealed with key %d, not the current key", sealed.KeyID)
	}
	if len(sealed.Nonce) != nonceSize {
		return nil, fmt.Errorf("nonce is %d bytes, want %d", len(sealed.Nonce), nonceSize)
	}
	return s.aead.Open(nil, sealed.Nonce, sealed.Ciphertext, associatedData(sealed.Owner, sealed.Name))
}
