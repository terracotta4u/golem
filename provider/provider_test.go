package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestErrUnsupportedFormatWrap(t *testing.T) {
	err := fmt.Errorf("%w: json_schema", ErrUnsupportedFormat)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatal("wrapped error should match ErrUnsupportedFormat")
	}
}

func TestMessageJSONRoundTrip(t *testing.T) {
	original := Message{
		Role: "assistant",
		ToolCalls: []ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: FunctionCall{
				Name:      "read",
				Arguments: `{"path":"foo.go"}`,
			},
		}},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("got %+v, want %+v", got, original)
	}
}
