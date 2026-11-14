package providers

import (
	"bytes"
	_ "embed"
	"errors"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"io"
	"sync"
)

//go:embed schemas/journal-v1.0.json
var eddnJournal []byte
var schemaOnce sync.Once
var journalSchema *jsonschema.Schema
var schemaError error

func validateEDDN(value any) error {
	schemaOnce.Do(func() {
		c := jsonschema.NewCompiler()
		c.Draft = jsonschema.Draft4
		c.LoadURL = func(string) (io.ReadCloser, error) { return nil, errors.New("remote schema loading forbidden") }
		schemaError = c.AddResource("https://eddn.edcd.io/schemas/journal/1", bytes.NewReader(eddnJournal))
		if schemaError == nil {
			journalSchema, schemaError = c.Compile("https://eddn.edcd.io/schemas/journal/1")
		}
	})
	if schemaError != nil {
		return schemaError
	}
	return journalSchema.Validate(value)
}
