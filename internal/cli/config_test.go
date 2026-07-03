package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/thalesfp/dbridge/internal/cli/form"
	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
)

// fakeCredStore is an in-memory credentials.Store whose Save/Load/Delete can be
// made to fail per connection name. Its Delete mirrors the real KeyringStore:
// deleting a missing key returns credentials.ErrNotFound.
type fakeCredStore struct {
	data       map[string]credentials.Credentials
	failSave   map[string]bool
	failLoad   map[string]bool
	failDelete map[string]bool
}

func newFakeCredStore() *fakeCredStore {
	return &fakeCredStore{
		data:       map[string]credentials.Credentials{},
		failSave:   map[string]bool{},
		failLoad:   map[string]bool{},
		failDelete: map[string]bool{},
	}
}

func (f *fakeCredStore) Save(ctx context.Context, connection string, creds credentials.Credentials) error {
	if f.failSave[connection] {
		return errors.New("save failed")
	}
	f.data[connection] = creds
	return nil
}

func (f *fakeCredStore) Load(ctx context.Context, connection string) (credentials.Credentials, error) {
	if f.failLoad[connection] {
		return credentials.Credentials{}, errors.New("load failed")
	}
	c, ok := f.data[connection]
	if !ok {
		return credentials.Credentials{}, credentials.ErrNotFound
	}
	return c, nil
}

func (f *fakeCredStore) Delete(ctx context.Context, connection string) error {
	if f.failDelete[connection] {
		return errors.New("delete failed")
	}
	if _, ok := f.data[connection]; !ok {
		return credentials.ErrNotFound
	}
	delete(f.data, connection)
	return nil
}

func (f *fakeCredStore) List(ctx context.Context) ([]string, error) {
	names := make([]string, 0, len(f.data))
	for n := range f.data {
		names = append(names, n)
	}
	return names, nil
}

func (f *fakeCredStore) Available() bool { return true }
func (f *fakeCredStore) Type() string    { return "fake" }

func okSave() error   { return nil }
func failSave() error { return errors.New("config save failed") }

// nameFree is a nameTaken predicate for tests where the destination name is
// not used by any other connection.
func nameFree(string) bool { return false }

func TestApplyConnectionEdit(t *testing.T) {
	ctx := context.Background()

	t.Run("rename without password change migrates the secret", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "pw"}

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u"}, nameFree, okSave)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := s.data["new"]; got.Password != "pw" {
			t.Errorf("migrated password = %q, want pw", got.Password)
		}
		if _, ok := s.data["old"]; ok {
			t.Error("old credential should be removed after rename")
		}
	})

	// Regression: a Save failure while migrating must not remove the old secret,
	// and config must not be saved.
	t.Run("rename migrate Save failure preserves the old secret and skips config save", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "pw"}
		s.failSave["new"] = true
		saved := false

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u"}, nameFree, func() error {
			saved = true
			return nil
		})
		if err == nil {
			t.Fatal("expected an error when the migrating Save fails")
		}
		if saved {
			t.Error("config must not be saved after a credential failure")
		}
		if got, ok := s.data["old"]; !ok || got.Password != "pw" {
			t.Errorf("old secret must survive a Save failure; got %+v ok=%v", got, ok)
		}
	})

	// Regression for finding A: if config save fails during a rename, the old
	// secret must remain intact (not orphaned under the new name).
	t.Run("rename config-save failure keeps the old secret usable", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "pw"}

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u"}, nameFree, failSave)
		if err == nil {
			t.Fatal("expected an error when config save fails")
		}
		if got, ok := s.data["old"]; !ok || got.Password != "pw" {
			t.Errorf("old secret must remain usable after config-save failure; got %+v ok=%v", got, ok)
		}
	})

	t.Run("rename and clear removes the secret everywhere on success", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "pw"}

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u", PasswordChanged: true, Password: ""}, nameFree, okSave)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := s.data["old"]; ok {
			t.Error("old secret should be gone")
		}
		if _, ok := s.data["new"]; ok {
			t.Error("new secret should be absent after clearing")
		}
	})

	t.Run("rename and clear with config-save failure keeps the old secret", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "pw"}

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u", PasswordChanged: true, Password: ""}, nameFree, failSave)
		if err == nil {
			t.Fatal("expected an error when config save fails")
		}
		if got, ok := s.data["old"]; !ok || got.Password != "pw" {
			t.Errorf("old secret must survive a rejected clear; got %+v ok=%v", got, ok)
		}
	})

	t.Run("rename with a new password stores it and removes the old key", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "old-pw"}

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u", PasswordChanged: true, Password: "new-pw"}, nameFree, okSave)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := s.data["new"]; got.Password != "new-pw" {
			t.Errorf("new password = %q, want new-pw", got.Password)
		}
		if _, ok := s.data["old"]; ok {
			t.Error("old secret should be removed")
		}
	})

	t.Run("same-name password update applies after config save", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["c"] = credentials.Credentials{Username: "u", Password: "old"}

		err := applyConnectionEdit(ctx, s, "c", &form.ConnectionData{Name: "c", Username: "u", PasswordChanged: true, Password: "new"}, nameFree, okSave)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := s.data["c"]; got.Password != "new" {
			t.Errorf("password = %q, want new", got.Password)
		}
	})

	// Same-name password change must not touch the keychain if config save fails.
	t.Run("same-name password update is skipped when config save fails", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["c"] = credentials.Credentials{Username: "u", Password: "old"}

		err := applyConnectionEdit(ctx, s, "c", &form.ConnectionData{Name: "c", Username: "u", PasswordChanged: true, Password: "new"}, nameFree, failSave)
		if err == nil {
			t.Fatal("expected an error when config save fails")
		}
		if got := s.data["c"]; got.Password != "old" {
			t.Errorf("password must be unchanged after rejected edit; got %q", got.Password)
		}
	})

	t.Run("no rename and no password change leaves the keychain untouched", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["c"] = credentials.Credentials{Username: "u", Password: "pw"}
		s.failSave["c"] = true   // would error if Save were called
		s.failDelete["c"] = true // would error if Delete were called

		err := applyConnectionEdit(ctx, s, "c", &form.ConnectionData{Name: "c", Username: "u"}, nameFree, okSave)
		if err != nil {
			t.Fatalf("keychain should be untouched, got error: %v", err)
		}
		if got := s.data["c"]; got.Password != "pw" {
			t.Errorf("secret should be unchanged, got %+v", got)
		}
	})

	// Passwordless rename: deleting the missing old key returns ErrNotFound,
	// which must be tolerated (mirrors the real KeyringStore.Delete).
	t.Run("rename of a passwordless connection succeeds", func(t *testing.T) {
		s := newFakeCredStore()

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u"}, nameFree, okSave)
		if err != nil {
			t.Fatalf("passwordless rename should succeed, got: %v", err)
		}
		if len(s.data) != 0 {
			t.Errorf("no credential should exist, got %v", s.data)
		}
	})

	// A config-save failure during a rename must remove the credential staged
	// under the new name, so no orphaned secret can later be picked up by a
	// connection created with that name.
	t.Run("rename config-save failure cleans up the staged new credential", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "pw"}

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u", PasswordChanged: true, Password: "new-pw"}, nameFree, failSave)
		if err == nil {
			t.Fatal("expected an error when config save fails")
		}
		if _, ok := s.data["new"]; ok {
			t.Error("staged credential under the new name must be cleaned up on failure")
		}
		if got, ok := s.data["old"]; !ok || got.Password != "pw" {
			t.Errorf("old secret must remain intact; got %+v ok=%v", got, ok)
		}
	})

	// Renaming a passwordless connection onto a name that still has a stale
	// orphan credential must clear it, so the renamed connection stays
	// passwordless instead of silently authenticating with the orphan.
	t.Run("passwordless rename clears a stale destination credential", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["new"] = credentials.Credentials{Username: "orphan", Password: "stale"}

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "new", Username: "u"}, nameFree, okSave)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, ok := s.data["new"]; ok {
			t.Errorf("stale destination credential must be cleared; got %+v", got)
		}
	})

	// Renaming onto a name another connection already uses must be rejected
	// before any credential or config mutation, so it cannot clobber that
	// connection's config and stored secret.
	t.Run("rename onto an existing connection name is rejected and touches nothing", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["old"] = credentials.Credentials{Username: "u", Password: "pw"}
		s.data["taken"] = credentials.Credentials{Username: "other", Password: "other-pw"}
		saved := false
		nameTaken := func(name string) bool { return name == "taken" }

		err := applyConnectionEdit(ctx, s, "old", &form.ConnectionData{Name: "taken", Username: "u", PasswordChanged: true, Password: "new"}, nameTaken, func() error {
			saved = true
			return nil
		})
		if err == nil {
			t.Fatal("expected an error when renaming onto an existing name")
		}
		if saved {
			t.Error("config must not be saved when the rename is rejected")
		}
		if got := s.data["taken"]; got.Password != "other-pw" || got.Username != "other" {
			t.Errorf("the existing connection's secret must be untouched; got %+v", got)
		}
		if got := s.data["old"]; got.Password != "pw" {
			t.Errorf("the source secret must be untouched; got %+v", got)
		}
	})
}

func TestPersistConnectionEdit(t *testing.T) {
	// A failed save must not leave the destination name in the in-memory config,
	// otherwise the collision guard would block a retry with the same name.
	t.Run("rename rolls back the in-memory config when save fails", func(t *testing.T) {
		original := &config.Connection{Name: "old", Host: "h1"}
		cfg := &config.Config{Connections: map[string]*config.Connection{"old": original}}
		updated := &config.Connection{Name: "new", Host: "h2"}

		err := persistConnectionEdit(cfg, "old", updated, original, failSave)
		if err == nil {
			t.Fatal("expected the save error to propagate")
		}
		if _, exists := cfg.Connections["new"]; exists {
			t.Error("the failed destination name must not linger in the config")
		}
		got, ok := cfg.Connections["old"]
		if !ok || got.Host != "h1" {
			t.Errorf("the original connection must be restored; got %+v ok=%v", got, ok)
		}
	})

	t.Run("rename swaps the entry when save succeeds", func(t *testing.T) {
		original := &config.Connection{Name: "old", Host: "h1"}
		cfg := &config.Config{Connections: map[string]*config.Connection{"old": original}}
		updated := &config.Connection{Name: "new", Host: "h2"}

		err := persistConnectionEdit(cfg, "old", updated, original, okSave)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := cfg.Connections["old"]; exists {
			t.Error("the old entry should be gone after a successful rename")
		}
		if got := cfg.Connections["new"]; got == nil || got.Host != "h2" {
			t.Errorf("the new entry = %+v, want the updated connection", got)
		}
	})

	t.Run("same-name edit rolls back to the original when save fails", func(t *testing.T) {
		original := &config.Connection{Name: "c", Host: "h1"}
		cfg := &config.Config{Connections: map[string]*config.Connection{"c": original}}
		updated := &config.Connection{Name: "c", Host: "h2"}

		err := persistConnectionEdit(cfg, "c", updated, original, failSave)
		if err == nil {
			t.Fatal("expected the save error to propagate")
		}
		if got := cfg.Connections["c"]; got == nil || got.Host != "h1" {
			t.Errorf("the connection must be restored to the original; got %+v", got)
		}
	})
}

func TestPrefillCredentials(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the stored credential when present", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["c"] = credentials.Credentials{Username: "u", Password: "pw"}

		got, err := prefillCredentials(ctx, s, "c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Password != "pw" {
			t.Errorf("prefill = %+v, want the stored credential", got)
		}
	})

	t.Run("a missing secret yields an empty prefill and no error", func(t *testing.T) {
		s := newFakeCredStore()

		got, err := prefillCredentials(ctx, s, "c")
		if err != nil {
			t.Fatalf("a missing credential must not be an error: %v", err)
		}
		if got != (credentials.Credentials{}) {
			t.Errorf("prefill = %+v, want empty", got)
		}
	})

	// A hard load failure must not block editing: the prefill is empty and the
	// error is surfaced for the caller to warn, not returned as a credential.
	t.Run("a load failure yields an empty prefill and surfaces the error", func(t *testing.T) {
		s := newFakeCredStore()
		s.data["c"] = credentials.Credentials{Username: "u", Password: "pw"}
		s.failLoad["c"] = true

		got, err := prefillCredentials(ctx, s, "c")
		if err == nil {
			t.Fatal("expected the load error to be surfaced")
		}
		if got != (credentials.Credentials{}) {
			t.Errorf("prefill = %+v, want empty on load failure", got)
		}
	})
}
