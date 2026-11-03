package check

import "reflect"

// CheckerLens are the lengths of every map and slice field of the session's checker, by field name.
func CheckerLens(s *Session) map[string]int {
	lens := map[string]int{}
	v := reflect.ValueOf(s.c).Elem()
	for i := range v.NumField() {
		if f := v.Field(i); f.Kind() == reflect.Map || f.Kind() == reflect.Slice {
			lens[v.Type().Field(i).Name] = f.Len()
		}
	}
	return lens
}
