package strslice

import jsonv1 "encoding/json"

// StrSlice represents a string or an array of strings.
// We need to override the json decoder to accept both options.
type StrSlice []string

// UnmarshalJSON decodes the byte slice whether it's a string or an array of
// strings. This method is needed to implement jsonv1.Unmarshaler.
func (e *StrSlice) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		// With no input, we preserve the existing value by returning nil and
		// leaving the target alone. This allows defining default values for
		// the type.
		return nil
	}

	p := make([]string, 0, 1)
	if err := jsonv1.Unmarshal(b, &p); err != nil {
		var s string
		if err := jsonv1.Unmarshal(b, &s); err != nil {
			return err
		}
		p = append(p, s)
	}

	*e = p
	return nil
}
