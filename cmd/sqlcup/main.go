// Command sqlcup implements an SQL statement generator for sqlc (https://sqlc.dev).
package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

const (
	plainColumnSep = ":"
	smartColumnSep = "@"
)

var (
	noExistsClauseFlag    = flag.Bool("no-exists-clause", false, "Omit IF NOT EXISTS in CREATE TABLE statements")
	idColumnFlag          = flag.String("id-column", "id", "Name of the column that identifies a row")
	orderByFlag           = flag.String("order-by", "", "Include ORDER BY in 'SELECT *' statement")
	noReturningClauseFlag = flag.Bool("no-returning-clause", false, "Omit 'RETURNING *' in UPDATE statement")
	onlyFlag              = flag.String("only", "", "Limit output to 'schema' or 'queries'")
)

var (
	errBadArgument        = errors.New("bad argument")
	errInvalidSmartColumn = fmt.Errorf("%w: invalid <smart-column>", errBadArgument)
)

func main() {
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = printUsage
	flag.Parse()

	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		if errors.Is(err, errBadArgument) {
			flag.Usage()
		}
		os.Exit(1)
	}
}

//go:embed usage.txt
var usage string

//goland:noinspection GoUnhandledErrorResult
func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, usage)
	flag.PrintDefaults()
}

func run() error {
	sca, err := parseScaffoldCommandArgs(flag.Args())
	if err != nil {
		return err
	}
	return scaffoldCommand(sca)
}

type column struct {
	Name       string
	Type       string
	Constraint string
	ID         bool
}

type outputMode uint8

const (
	outputSchema outputMode = 1 << iota
	outputQueries

	outputAll = outputSchema | outputQueries
)

type scaffoldCommandArgs struct {
	Table             string
	SingularEntity    string
	PluralEntity      string
	IDColumn          *column
	Columns           []column
	NonIDColumns      []column
	LongestName       int
	LongestType       int
	NoExistsClause    bool
	OrderBy           string
	NoReturningClause bool
	Output            outputMode
}

func parseColumnDefinition(s string) (column, error) {
	var (
		plainColumn = strings.Contains(s, plainColumnSep)
		smartColumn = strings.Contains(s, smartColumnSep)
	)
	if plainColumn && smartColumn {
		return column{}, fmt.Errorf("%w: invalid <column>: '%s' contains both plain and smart separators", errBadArgument, s)
	}
	if plainColumn {
		return parsePlainColumnDefinition(s)
	} else if smartColumn {
		return parseSmartColumnDefinition(s)
	}
	return column{}, fmt.Errorf("%w: invalid <column>: '%s', expected <smart-column> or <plain-column>", errBadArgument, s)
}

func parseSmartColumnDefinition(s string) (column, error) {
	if s == "@id" {
		return column{
			ID:         true,
			Name:       "id",
			Type:       "INTEGER",
			Constraint: "PRIMARY KEY",
		}, nil
	}

	name, rest, _ := strings.Cut(s, smartColumnSep)
	if name == "" {
		return column{}, fmt.Errorf("%w: '%s', missing <name>", errInvalidSmartColumn, s)
	}

	var (
		colType string
		id      bool
		null    bool
		unique  bool
	)
	tags := strings.Split(rest, smartColumnSep)
	for _, tag := range tags {
		switch tag {
		case "id":
			id = true
		case "null":
			null = true
		case "unique":
			unique = true
		case "float":
			colType = "FLOAT"
		case "double":
			colType = "DOUBLE"
		case "datetime":
			colType = "DATETIME"
		case "text":
			colType = "TEXT"
		case "int":
			colType = "INTEGER"
		case "blob":
			colType = "BLOB"
		default:
			return column{}, fmt.Errorf("%w: '%s': unknown <tag> #%s", errInvalidSmartColumn, s, tag)
		}
	}
	if id {
		if unique || null {
			return column{}, fmt.Errorf("%w: '%s', cannot combine @id with @unique or @null", errInvalidSmartColumn, s)
		}
		if colType == "" {
			colType = "INTEGER"
		}
		// sqlite special case
		var constraint = "PRIMARY KEY"
		if colType != "INTEGER" {
			constraint = "NOT NULL " + constraint
		}
		return column{
			Name:       name,
			Type:       colType,
			Constraint: constraint,
			ID:         true,
		}, nil
	}

	if colType == "" {
		return column{}, fmt.Errorf("%w: '%s' missing column type", errInvalidSmartColumn, s)
	}
	constraint := ""
	if !null {
		constraint += " NOT NULL"
	}
	if unique {
		constraint += " UNIQUE"
	}
	return column{
		Name:       name,
		Type:       colType,
		Constraint: strings.TrimSpace(constraint),
		ID:         false,
	}, nil
}

func parsePlainColumnDefinition(s string) (column, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return column{}, fmt.Errorf("%w: invalid <plain-column>: '%s', expected '<name>:<type>' or '<name>:<type>:<constraint>'", errBadArgument, s)
	}
	col := column{
		ID:   strings.ToLower(parts[0]) == *idColumnFlag,
		Name: parts[0],
		Type: parts[1],
	}
	if len(parts) == 3 {
		col.Constraint = parts[2]
	}
	return col, nil
}

func parseScaffoldCommandArgs(args []string) (*scaffoldCommandArgs, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%w: missing <name> and <column>", errBadArgument)
	}

	tableParts := strings.Split(args[0], "/")
	if len(tableParts) != 2 || len(tableParts[0]) == 0 || len(tableParts[1]) == 0 {
		return nil, fmt.Errorf("%w: invalid <name>: '%s', expected '<singular>/<plural>'", errBadArgument, tableParts)
	}

	sca := &scaffoldCommandArgs{
		Table:             tableParts[1],
		SingularEntity:    capitalize(tableParts[0]),
		PluralEntity:      capitalize(tableParts[1]),
		NoExistsClause:    *noExistsClauseFlag,
		NoReturningClause: *noReturningClauseFlag,
		OrderBy:           *orderByFlag,
	}
	switch *onlyFlag {
	case "schema":
		sca.Output = sca.Output | outputSchema
	case "queries":
		sca.Output = sca.Output | outputQueries
	case "":
		sca.Output = sca.Output | outputAll
	default:
		return nil, fmt.Errorf("%w: '-only %s', expected 'schema' or 'queries'", errBadArgument, *onlyFlag)
	}

	for _, arg := range args[1:] {
		col, err := parseColumnDefinition(arg)
		if err != nil {
			return nil, err
		}
		if len(col.Name) > sca.LongestName {
			sca.LongestName = len(col.Name)
		}
		if len(col.Type) > sca.LongestType {
			sca.LongestType = len(col.Type)
		}
		sca.Columns = append(sca.Columns, col)
		if col.ID {
			sca.IDColumn = &col
		} else {
			sca.NonIDColumns = append(sca.NonIDColumns, col)
		}
	}
	return sca, nil
}

func scaffoldCommand(args *scaffoldCommandArgs) error {
	b := &strings.Builder{}

	if args.Output&outputAll == outputAll {
		b.WriteString("#############################################\n")
		b.WriteString("# Add the following to your SQL schema file #\n")
		b.WriteString("#############################################\n\n")
	}
	if args.Output&outputSchema != 0 {
		writeSchema(b, args)
		b.WriteString("\n\n")
	}
	if args.Output&outputAll == outputAll {
		b.WriteString("##############################################\n")
		b.WriteString("# Add the following to your SQL queries file #\n")
		b.WriteString("##############################################\n\n")
	}
	if args.Output&outputQueries != 0 {
		if args.IDColumn != nil {
			writeGetQuery(b, args)
			b.WriteString("\n\n")
		}

		writeListQuery(b, args)
		b.WriteString("\n\n")

		writeCreateQuery(b, args)
		b.WriteString("\n")

		if args.IDColumn != nil {
			b.WriteString("\n")
			writeDeleteQuery(b, args)
			b.WriteString("\n\n")
			writeUpdateQuery(b, args)
			b.WriteString("\n\n")
		}
	}
	fmt.Print(b)
	return nil
}

//goland:noinspection GoUnhandledErrorResult
func writeSchema(w io.Writer, args *scaffoldCommandArgs) {
	fmt.Fprint(w, "CREATE TABLE ")
	if !args.NoExistsClause {
		fmt.Fprint(w, "IF NOT EXISTS ")
	}
	fmt.Fprint(w, args.Table)
	fmt.Fprint(w, " (\n")

	for ci, col := range args.Columns {
		fmt.Fprintf(w, "  %s ", col.Name)
		no := args.LongestName - len(col.Name)
		for i := 0; i < no; i++ {
			fmt.Fprintf(w, " ")
		}
		fmt.Fprintf(w, "%s", col.Type)
		if col.Constraint != "" {
			to := args.LongestType - len(col.Type)
			for i := 0; i < to; i++ {
				fmt.Fprintf(w, " ")
			}
			fmt.Fprintf(w, " %s", col.Constraint)
		}
		if ci < len(args.Columns)-1 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "\n")
	}
	fmt.Fprintf(w, ");")
}

//goland:noinspection GoUnhandledErrorResult
func writeGetQuery(w io.Writer, args *scaffoldCommandArgs) {
	fmt.Fprintf(w, "-- name: Get%s :one\n", args.SingularEntity)
	fmt.Fprintf(w, "SELECT * FROM %s\n", args.Table)
	fmt.Fprintf(w, "WHERE %s = ? LIMIT 1;", args.IDColumn.Name)
}

//goland:noinspection GoUnhandledErrorResult
func writeListQuery(w io.Writer, args *scaffoldCommandArgs) {
	fmt.Fprintf(w, "-- name: List%s :many\n", args.PluralEntity)
	fmt.Fprintf(w, "SELECT * FROM %s", args.Table)
	if args.OrderBy == "" {
		fmt.Fprintf(w, ";")
	} else {
		fmt.Fprintf(w, "\nORDER BY %s;", args.OrderBy)
	}
}

//goland:noinspection GoUnhandledErrorResult
func writeCreateQuery(w io.Writer, args *scaffoldCommandArgs) {
	fmt.Fprintf(w, "-- name: Create%s :one\n", args.SingularEntity)
	fmt.Fprintf(w, "INSERT INTO %s (\n", args.Table)
	fmt.Fprintf(w, "  ")
	for i, col := range args.NonIDColumns {
		fmt.Fprint(w, col.Name)
		if i == len(args.NonIDColumns)-1 {
			fmt.Fprintf(w, "\n")
		} else {
			fmt.Fprintf(w, ", ")
		}
	}
	fmt.Fprintf(w, ") VALUES (\n")
	fmt.Fprint(w, "  ")
	for i := 0; i < len(args.NonIDColumns); i++ {
		if i < len(args.NonIDColumns)-1 {
			fmt.Fprint(w, "?, ")
		} else {
			fmt.Fprint(w, "?\n")
		}
	}
	fmt.Fprintf(w, ")\n")
	fmt.Fprintf(w, "RETURNING *;")
}

//goland:noinspection GoUnhandledErrorResult
func writeDeleteQuery(w io.Writer, args *scaffoldCommandArgs) {
	fmt.Fprintf(w, "-- name: Delete%s :exec\n", args.SingularEntity)
	fmt.Fprintf(w, "DELETE FROM %s\n", args.Table)
	fmt.Fprintf(w, "WHERE %s = ?;", args.IDColumn.Name)
}

//goland:noinspection GoUnhandledErrorResult
func writeUpdateQuery(w io.Writer, args *scaffoldCommandArgs) {
	var mode string
	if args.NoReturningClause {
		mode = ":exec"
	} else {
		mode = ":one"
	}
	fmt.Fprintf(w, "-- name: Update%s %s\n", args.SingularEntity, mode)
	fmt.Fprintf(w, "UPDATE %s\n", args.Table)
	fmt.Fprintf(w, "SET\n")
	for i, col := range args.NonIDColumns {
		if i < len(args.NonIDColumns)-1 {
			fmt.Fprintf(w, "  %s = ?,\n", col.Name)
		} else {
			fmt.Fprintf(w, "  %s = ?\n", col.Name)
		}
	}
	fmt.Fprintf(w, "WHERE %s = ?", args.IDColumn.Name)
	if !args.NoReturningClause {
		fmt.Fprintf(w, "\nRETURNING *;")
	} else {
		fmt.Fprintf(w, ";")
	}
}

func capitalize(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
