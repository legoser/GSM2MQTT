package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecipients_AddAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "recipients.json")

	mgr := NewRecipientsManager(filePath, []string{"+79001112233"})
	recipients := mgr.Get()
	if len(recipients) != 1 || recipients[0] != "+79001112233" {
		t.Fatalf("expected [+79001112233], got %v", recipients)
	}

	// Add new number with formatting
	err := mgr.Add("8 (900) 222-33-44")
	if err != nil {
		t.Fatalf("unexpected error adding number: %v", err)
	}

	recipients = mgr.Get()
	if len(recipients) != 2 {
		t.Fatalf("expected 2 recipients, got %d", len(recipients))
	}
	if recipients[1] != "+79002223344" {
		t.Errorf("expected normalized number '+79002223344', got %q", recipients[1])
	}
}

func TestRecipients_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "recipients.json")

	mgr := NewRecipientsManager(filePath, nil)
	_ = mgr.Add("+79001112233")
	_ = mgr.Add("+79001112233")
	_ = mgr.Add("89001112233")

	recipients := mgr.Get()
	if len(recipients) != 1 {
		t.Fatalf("expected 1 recipient after deduplication, got %d: %v", len(recipients), recipients)
	}
}

func TestRecipients_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "recipients.json")

	mgr := NewRecipientsManager(filePath, []string{"+79001112233", "+79002223344"})
	removed := mgr.Remove("+79001112233")
	if !removed {
		t.Fatal("expected Remove to return true")
	}

	recipients := mgr.Get()
	if len(recipients) != 1 || recipients[0] != "+79002223344" {
		t.Fatalf("expected [+79002223344], got %v", recipients)
	}

	// Remove non-existent
	if mgr.Remove("+79999999999") {
		t.Fatal("expected Remove to return false for non-existent number")
	}
}

func TestRecipients_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "recipients.json")

	mgr1 := NewRecipientsManager(filePath, []string{"+79001112233"})
	_ = mgr1.Add("+79002223344")

	// Ensure file was created on disk
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("recipients file was not written: %v", err)
	}

	// Create manager 2 pointing to the same file
	mgr2 := NewRecipientsManager(filePath, nil)
	recipients := mgr2.Get()
	if len(recipients) != 2 {
		t.Fatalf("expected 2 recipients loaded from file, got %d: %v", len(recipients), recipients)
	}
	if recipients[0] != "+79001112233" || recipients[1] != "+79002223344" {
		t.Errorf("unexpected recipients loaded: %v", recipients)
	}
}

func TestRecipients_InvalidNumber(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "recipients.json")

	mgr := NewRecipientsManager(filePath, nil)
	err := mgr.Add("invalid_number")
	if err == nil {
		t.Fatal("expected error adding invalid number")
	}

	err = mgr.Add("")
	if err == nil {
		t.Fatal("expected error adding empty number")
	}
}

func TestRecipients_Set(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "recipients.json")

	mgr := NewRecipientsManager(filePath, []string{"+79001112233"})
	err := mgr.Set([]string{"+79003334455", "+79004445566"})
	if err != nil {
		t.Fatalf("unexpected error setting recipients: %v", err)
	}

	recipients := mgr.Get()
	if len(recipients) != 2 || recipients[0] != "+79003334455" || recipients[1] != "+79004445566" {
		t.Fatalf("expected updated list, got %v", recipients)
	}
}
