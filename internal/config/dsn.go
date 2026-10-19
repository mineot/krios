package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// BuildDSN returns the driver-specific data source name. The driver named in
// Config.Driver must accept the format produced for Config.Type.
func BuildDSN(cfg Config) string {
	switch cfg.Type {
	case DBTypeSQLite:
		return cfg.SQLite.DBPath
	case DBTypeMySQL:
		m := cfg.MySQL
		return fmt.Sprintf("%s:%s@tcp(%s)/%s", m.User, m.Password, net.JoinHostPort(m.Host, strconv.Itoa(m.port())), url.PathEscape(m.DBName))
	case DBTypePostgres:
		p := cfg.Postgres
		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(p.User, p.Password),
			Host:   net.JoinHostPort(p.Host, strconv.Itoa(p.port())),
			Path:   "/" + p.DBName,
		}
		return u.String()
	default:
		return ""
	}
}
