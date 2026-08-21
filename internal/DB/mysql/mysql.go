package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"
	"vhome/internal/config"

	"github.com/go-sql-driver/mysql"
)

func Open(ctx context.Context, cfg config.DBConfig) (*sql.DB, error) {
	location, err := time.LoadLocation(cfg.Location)
	if err != nil {
		return nil, fmt.Errorf(
			"load mysql location %q: %w", cfg.Location, err,
		)
	}

	driverConfig := mysql.NewConfig()
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	driverConfig.DBName = cfg.DBName
	driverConfig.ParseTime = true
	driverConfig.Loc = location
	// DATETIME has no timezone metadata. Keep MySQL CURRENT_TIMESTAMP and the
	// driver's time.Time encoding in the same configured zone so default
	// created_at values do not drift by eight hours from explicit business
	// times such as memo.remind_at.
	driverConfig.Params = map[string]string{
		"time_zone": mysqlTimeZoneOffset(location),
	}
	driverConfig.Timeout = cfg.ConnectTimeout
	driverConfig.ReadTimeout = cfg.ReadTimeout
	driverConfig.WriteTimeout = cfg.WriteTimeout

	connector, err := mysql.NewConnector(driverConfig)
	if err != nil {
		return nil, fmt.Errorf("create mysql connector: %w", err)
	}

	db := sql.OpenDB(connector)

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return db, nil

}

func mysqlTimeZoneOffset(location *time.Location) string {
	_, offsetSeconds := time.Now().In(location).Zone()
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("'%s%02d:%02d'", sign, hours, minutes)
}
