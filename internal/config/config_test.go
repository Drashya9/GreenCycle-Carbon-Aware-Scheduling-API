package config

import "testing"

func TestRedisURLFallsBackToKVURL(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("KV_URL", "rediss://default:token@upstash.example:6379")
	if got := Load().RedisURL; got != "rediss://default:token@upstash.example:6379" {
		t.Errorf("expected KV_URL to be used when REDIS_URL is unset, got %q", got)
	}

	t.Setenv("REDIS_URL", "redis://explicit:6379/0")
	if got := Load().RedisURL; got != "redis://explicit:6379/0" {
		t.Errorf("expected REDIS_URL to win over KV_URL, got %q", got)
	}
}

func TestServerlessSwitchesDefaultOn(t *testing.T) {
	t.Setenv("RUN_MIGRATIONS", "")
	t.Setenv("SCHEDULER_ENABLED", "")
	cfg := Load()
	if !cfg.RunMigrations || !cfg.SchedulerEnabled {
		t.Errorf("both switches must default to true so Docker behavior is unchanged, got %+v", cfg)
	}

	t.Setenv("RUN_MIGRATIONS", "false")
	t.Setenv("SCHEDULER_ENABLED", "false")
	cfg = Load()
	if cfg.RunMigrations || cfg.SchedulerEnabled {
		t.Errorf("expected both switches off, got RunMigrations=%v SchedulerEnabled=%v", cfg.RunMigrations, cfg.SchedulerEnabled)
	}
}
