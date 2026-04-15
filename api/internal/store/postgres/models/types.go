package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// StringList is a custom type for PostgreSQL text[] columns.
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "{}", nil
	}
	return "{" + strings.Join(s, ",") + "}", nil
}

func (s *StringList) Scan(src interface{}) error {
	if src == nil {
		*s = StringList{}
		return nil
	}
	var str string
	switch v := src.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into StringList", src)
	}
	str = strings.Trim(str, "{}")
	if str == "" {
		*s = StringList{}
		return nil
	}
	*s = strings.Split(str, ",")
	return nil
}
