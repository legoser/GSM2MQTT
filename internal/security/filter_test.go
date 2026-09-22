package security

import "testing"

func TestFilter_All(t *testing.T) {
	f := NewFilter("all", nil, nil)
	if !f.Allowed("+79991112233") {
		t.Errorf("expected number to be allowed in 'all' mode")
	}
}

func TestFilter_Whitelist(t *testing.T) {
	f := NewFilter("whitelist", []string{"+79991112233"}, nil)

	if !f.Allowed("+79991112233") {
		t.Errorf("expected whitelisted number to be allowed")
	}
	if f.Allowed("+79999999999") {
		t.Errorf("expected non-whitelisted number to be blocked")
	}
}

func TestFilter_Blacklist(t *testing.T) {
	f := NewFilter("blacklist", nil, []string{"+79998887766"})

	if f.Allowed("+79998887766") {
		t.Errorf("expected blacklisted number to be blocked")
	}
	if !f.Allowed("+79991112233") {
		t.Errorf("expected non-blacklisted number to be allowed")
	}
}
