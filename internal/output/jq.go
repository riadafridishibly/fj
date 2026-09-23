package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"
)

// ParseJQ compiles a jq expression for WriteJQ. Callers compile before
// sending a request, so a typo fails the command instead of discarding a
// response the server has already acted on.
func ParseJQ(expr string) (*gojq.Code, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jq expression: %w", err)
	}
	code, err := gojq.Compile(query, gojq.WithEnvironLoader(os.Environ))
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression: %w", err)
	}
	return code, nil
}

// WriteJQ runs code over the JSON document in input and writes one line per
// result, the way gh's --jq does: strings raw, null as an empty line, and
// everything else as compact JSON. Numbers decode as json.Number, so IDs
// past 2^53 come out exactly as the server sent them.
func WriteJQ(w io.Writer, code *gojq.Code, input []byte) error {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("jq: response is not JSON: %w", err)
	}

	iter := code.Run(v)
	for {
		result, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, ok := result.(error); ok {
			var halt *gojq.HaltError
			if errors.As(err, &halt) && halt.Value() == nil {
				return nil
			}
			return fmt.Errorf("jq: %w", err)
		}

		var line []byte
		switch r := result.(type) {
		case string:
			line = []byte(r)
		case nil:
		default:
			b, err := gojq.Marshal(r)
			if err != nil {
				return fmt.Errorf("jq: %w", err)
			}
			line = b
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return err
		}
	}
}
