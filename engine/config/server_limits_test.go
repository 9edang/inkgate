package config

import (
	"testing"
	"time"
)

func TestServerLimitDefaults(t *testing.T) {
	s := ServiceConfig{}
	s.applyServerLimitDefaults()
	if s.ReadHeaderTimeout != DefaultReadHeaderTimeout || s.ReadTimeout != DefaultReadTimeout ||
		s.IdleTimeout != DefaultIdleTimeout || s.MaxBodyBytes != DefaultMaxBodyBytes {
		t.Errorf("defaults not applied: %+v", s)
	}
	if s.WriteTimeout != 0 {
		t.Errorf("write_timeout must stay unlimited (it would cut streams), got %v", s.WriteTimeout)
	}
}

func TestServerLimitsExplicitAndDisabled(t *testing.T) {
	s := ServiceConfig{ReadHeaderTimeout: 3 * time.Second, ReadTimeout: -1, IdleTimeout: -1, MaxBodyBytes: -1}
	s.applyServerLimitDefaults()
	if s.ReadHeaderTimeout != 3*time.Second {
		t.Errorf("explicit value overwritten: %v", s.ReadHeaderTimeout)
	}
	if s.ReadTimeout != 0 || s.IdleTimeout != 0 {
		t.Errorf("a negative timeout means no limit (0 for net/http), got %v %v", s.ReadTimeout, s.IdleTimeout)
	}
	if s.MaxBodyBytes != -1 {
		t.Errorf("a negative max_body_bytes means no limit, got %d", s.MaxBodyBytes)
	}
}
