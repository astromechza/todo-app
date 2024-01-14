package ref

import (
	"database/sql"
)

func DeRefToNullString(input *string) sql.NullString {
	if input == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *input, Valid: true}
}

func Ref[k any](desc k) *k {
	return &desc
}

func DeRefOr[k any](thing *k, defaultValue k) k {
	if thing == nil {
		return defaultValue
	}
	return *thing
}
