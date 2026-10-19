package core

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestConnectionPortRange(t *testing.T) {
	for _, makeConfig := range []func() Config{validMySQL, validPostgres} {
		for _, port := range []int{-1, 0, 1, 65535, 65536} {
			cfg := makeConfig()
			if cfg.MySQL != nil {
				cfg.MySQL.Port = port
			} else {
				cfg.Postgres.Port = port
			}
			err := cfg.validate()
			invalid := port < 0 || port > 65535
			if errors.Is(err, ErrInvalidConfig) != invalid {
				t.Errorf("type=%s port=%d invalid=%v", cfg.Type, port, invalid)
			}
		}
	}
}

func TestMySQLDSN(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		for _, port := range []int{0, 1, 65535} {
			cfg := validMySQL()
			cfg.MySQL.Host = host
			cfg.MySQL.Port = port
			cfg.MySQL.Password = "test:@/% password"
			cfg.MySQL.DBName = "test% name"
			dsn := buildDSN(cfg)
			prefix := cfg.MySQL.User + ":" + cfg.MySQL.Password + "@tcp("
			if !strings.HasPrefix(dsn, prefix) {
				t.Fatal("credentials were changed")
			}
			rest := strings.TrimPrefix(dsn, prefix)
			address, name, ok := strings.Cut(rest, ")/")
			if !ok {
				t.Fatal("invalid address separator")
			}
			wantPort := port
			if wantPort == 0 {
				wantPort = 3306
			}
			if address != net.JoinHostPort(host, strconv.Itoa(wantPort)) {
				t.Fatal("invalid address")
			}
			decoded, err := url.PathUnescape(name)
			if err != nil || decoded != cfg.MySQL.DBName || strings.Contains(name, " ") {
				t.Fatal("database identifier did not round-trip")
			}
		}
	}
}

func TestPostgresDSN(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		for _, port := range []int{0, 1, 65535} {
			cfg := validPostgres()
			p := cfg.Postgres
			p.Host = host
			p.Port = port
			p.User = "test user@"
			p.Password = "test:@/% password"
			p.DBName = "test% name?#"
			u, err := url.Parse(buildDSN(cfg))
			if err != nil {
				t.Fatal("invalid URL")
			}
			password, _ := u.User.Password()
			wantPort := port
			if wantPort == 0 {
				wantPort = 5432
			}
			if u.Scheme != "postgres" || u.User.Username() != p.User || password != p.Password || u.Hostname() != host || u.Port() != strconv.Itoa(wantPort) || u.Path != "/"+p.DBName || u.RawQuery != "" || u.Fragment != "" {
				t.Fatal("connection fields did not round-trip")
			}
		}
	}
}
