package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"vhome/internal/model"
)

type DashboardStorageStat struct {
	StorageType   model.StorageType
	LocationCount uint64
	ItemCount     uint64
}

type DashboardActivity struct {
	Type       string
	ItemName   string
	ActorName  string
	Quantity   *float64
	Unit       string
	HappenedAt time.Time
}

func (r *Repository) DashboardStorageStats(ctx context.Context) ([]DashboardStorageStat, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT l.storage_type, COUNT(DISTINCT l.id), COUNT(i.id)
		FROM storage_locations l
		LEFT JOIN inventory_items i
		  ON i.storage_location_id = l.id
		 AND i.status = 'ACTIVE'
		WHERE l.enabled = TRUE
		GROUP BY l.storage_type
		ORDER BY FIELD(l.storage_type, 'COLD', 'AMBIENT')
	`)
	if err != nil {
		return nil, fmt.Errorf("dashboard storage stats: %w", err)
	}
	defer rows.Close()

	out := make([]DashboardStorageStat, 0, 2)
	for rows.Next() {
		var stat DashboardStorageStat
		if err := rows.Scan(&stat.StorageType, &stat.LocationCount, &stat.ItemCount); err != nil {
			return nil, fmt.Errorf("scan dashboard storage stat: %w", err)
		}
		out = append(out, stat)
	}
	return out, rows.Err()
}

func (r *Repository) DashboardActivities(ctx context.Context, limit int) ([]DashboardActivity, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT event_type, item_name, actor_name, quantity, unit, happened_at
		FROM (
			SELECT 'INVENTORY_ADDED' AS event_type,
			       i.name AS item_name,
			       m.display_name AS actor_name,
			       i.quantity,
			       i.unit,
			       i.created_at AS happened_at
			FROM inventory_items i
			JOIN members m ON m.id = i.created_by

			UNION ALL

			SELECT 'INVENTORY_DISCARDED',
			       i.name,
			       m.display_name,
			       NULL,
			       NULL,
			       i.discarded_at
			FROM inventory_items i
			JOIN members m ON m.id = i.discarded_by
			WHERE i.discarded_at IS NOT NULL

			UNION ALL

			SELECT 'MEMBER_JOINED',
			       '',
			       m.display_name,
			       NULL,
			       NULL,
			       COALESCE(m.reviewed_at, m.created_at)
			FROM members m
			WHERE m.status = 'ACTIVE' AND m.role <> 'OWNER'
		) activity
		ORDER BY happened_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("dashboard activities: %w", err)
	}
	defer rows.Close()

	out := make([]DashboardActivity, 0, limit)
	for rows.Next() {
		var itemName, unit sql.NullString
		var quantity sql.NullFloat64
		var activity DashboardActivity
		if err := rows.Scan(
			&activity.Type,
			&itemName,
			&activity.ActorName,
			&quantity,
			&unit,
			&activity.HappenedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dashboard activity: %w", err)
		}
		if itemName.Valid {
			activity.ItemName = itemName.String
		}
		if quantity.Valid {
			value := quantity.Float64
			activity.Quantity = &value
		}
		if unit.Valid {
			activity.Unit = unit.String
		}
		out = append(out, activity)
	}
	return out, rows.Err()
}
