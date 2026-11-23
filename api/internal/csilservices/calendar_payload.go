package csilservices

import (
	"reflect"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/fxamacker/cbor/v2"
)

var legacyCalendarCBOR, _ = (cbor.DecOptions{
	TagsMd:         cbor.TagsForbidden,
	DefaultMapType: reflect.TypeFor[map[string]any](),
}).DecMode()

// decodeSavedEvent also reads requests saved before CatalystCal used tagged
// dates. Preserve the saved request contents and its original idempotency key.
func decodeSavedEvent(data []byte) (cal.Event, error) {
	event, err := cal.DecodeEvent(data)
	if err == nil {
		return event, nil
	}
	var fields map[string]any
	if legacyErr := legacyCalendarCBOR.Unmarshal(data, &fields); legacyErr != nil {
		return cal.Event{}, err
	}
	for _, key := range []string{"created_at", "updated_at", "start", "end", "recurrence_id"} {
		if value, ok := fields[key].(string); ok {
			if value == "" && (key == "created_at" || key == "updated_at") {
				value = time.Time{}.Format(time.RFC3339)
			}
			fields[key] = cbor.Tag{Number: 0, Content: value}
		}
	}
	for _, key := range []string{"start_date", "end_date"} {
		if value, ok := fields[key].(string); ok {
			fields[key] = cbor.Tag{Number: 1004, Content: value}
		}
	}
	if recurrence, ok := fields["recurrence"].(map[string]any); ok {
		for _, key := range []string{"exdates", "rdates"} {
			dates, _ := recurrence[key].([]any)
			for i, date := range dates {
				dates[i] = cbor.Tag{Number: 0, Content: date}
			}
		}
	}
	tagged, err := cbor.Marshal(fields)
	if err != nil {
		return cal.Event{}, err
	}
	return cal.DecodeEvent(tagged)
}
