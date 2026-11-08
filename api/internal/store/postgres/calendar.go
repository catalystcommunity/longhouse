package postgres

import "gorm.io/gorm"

// CalendarDB supplies transactions for the calendar publication state machine.
func (s *PostgresStore) CalendarDB() *gorm.DB { return db }
