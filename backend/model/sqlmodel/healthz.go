package sqlmodel

import (
	"context"
	"fmt"
)

func (s *sqlModel) HealthZ(ctx context.Context) error {
	var out int
	if err := s.db.QueryRowContext(ctx, `SELECT 1`).Scan(&out); err != nil {
		return fmt.Errorf("failed to select from database: %w", err)
	}
	return nil
}
