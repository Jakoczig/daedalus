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

//go:embed schemas/commodity-v3.0.json
var commoditySchema []byte

//go:embed schemas/shipyard-v2.0.json
var shipyardSchema []byte

//go:embed schemas/outfitting-v2.0.json
var outfittingSchema []byte
var stationOnce sync.Once
var stationSchemas map[string]*jsonschema.Schema
var stationSchemaError error

func validateEDDNStation(kind string, value any) error {
	stationOnce.Do(func() {
		stationSchemas = map[string]*jsonschema.Schema{}
		for name, raw := range map[string][]byte{"commodity/3": commoditySchema, "shipyard/2": shipyardSchema, "outfitting/2": outfittingSchema} {
			c := jsonschema.NewCompiler()
			c.Draft = jsonschema.Draft4
			c.LoadURL = func(string) (io.ReadCloser, error) { return nil, errors.New("remote schema loading forbidden") }
			ref := "https://eddn.edcd.io/schemas/" + name
			if err := c.AddResource(ref, bytes.NewReader(raw)); err != nil {
				stationSchemaError = err
				return
			}
			compiled, err := c.Compile(ref)
			if err != nil {
				stationSchemaError = err
				return
			}
			stationSchemas[name] = compiled
		}
	})
	if stationSchemaError != nil {
		return stationSchemaError
	}
	s := stationSchemas[kind]
	if s == nil {
		return errors.New("Unsupported local EDDN schema")
	}
	return s.Validate(value)
}

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
