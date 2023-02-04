package main

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

var smartColTests = map[string]struct {
	col column
	err error
}{
	"#id": {
		col: column{
			Name:       "id",
			Type:       "INTEGER",
			Constraint: "PRIMARY KEY",
			ID:         true,
		},
	},
	"col_id#id": {
		col: column{
			Name:       "col_id",
			Type:       "INTEGER",
			Constraint: "PRIMARY KEY",
			ID:         true,
		},
	},
	"primary_key#text#id": {
		col: column{
			Name:       "primary_key",
			Type:       "TEXT",
			Constraint: "NOT NULL PRIMARY KEY",
			ID:         true,
		},
	},
	"col#text": {
		col: column{
			Name:       "col",
			Type:       "TEXT",
			Constraint: "NOT NULL",
			ID:         false,
		},
	},
	"col#text#null": {
		col: column{
			Name:       "col",
			Type:       "TEXT",
			Constraint: "",
			ID:         false,
		},
	},
	"col#text#unique": {
		col: column{
			Name:       "col",
			Type:       "TEXT",
			Constraint: "NOT NULL UNIQUE",
			ID:         false,
		},
	},
	"col#int": {
		col: column{
			Name:       "col",
			Type:       "INTEGER",
			Constraint: "NOT NULL",
			ID:         false,
		},
	},
	"col#datetime": {
		col: column{
			Name:       "col",
			Type:       "DATETIME",
			Constraint: "NOT NULL",
			ID:         false,
		},
		err: nil,
	},
}

func TestParseSmartColumnDefinition(t *testing.T) {
	for def, want := range smartColTests {
		t.Run(def, func(t *testing.T) {
			got, err := parseSmartColumnDefinition(def)
			if diff := cmp.Diff(want.err, err); diff != "" {
				t.Errorf("parseSmartColumnDefinition(\"%s\") returned wrong error: diff -want +got\n%s", def, diff)
			}
			if diff := cmp.Diff(want.col, got); diff != "" {
				t.Errorf("parseSmartColumnDefinition(\"%s\") returned wrong column: diff -want +got\n%s", def, diff)
			}
		})
	}
}
