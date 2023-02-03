// Command sqlcup implements an SQL statement generator for sqlc (https://sqlc.dev).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

var (
	noExistsClauseFlag    = flag.Bool("no-exists-clause", false, "Omit IF NOT EXISTS in CREATE TABLE statements")
	idColumnFlag          = flag.String("id-column", "id", "Name of the column that identifies a row")
	orderByFlag           = flag.String("order-by", "", "Include ORDER BY in 'SELECT *' statement")
	noReturningClauseFlag = flag.Bool("no-returning-clause", false, "Omit 'RETURNING *' in UPDATE statement")
)

var errBadArgument = errors.New("bad argument")

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

//goland:noinspection GoUnhandledErrorResult
func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, "sqlcup - generate SQL statements for sqlc (https://sqlc.dev)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Synopsis:")
	fmt.Fprintln(w, "  sqlcup [options] <name> <column> ...")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  sqlcup prints SQL statements to stdout. The <name> argument given to sqlcup")
	fmt.Fprintln(w, "  must be of the form <singular>/<plural> where <singular> is the name of the")
	fmt.Fprintln(w, "  Go struct and <plural> is the name of the database table.")
	fmt.Fprintln(w, "  sqlcup capitalizes those names where required.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Each <column> arguments given to sqlcup defines a database column and must")
	fmt.Fprintln(w, "  be of the form <name>:<type>[:<constraint>]. <name>, <type> and the")
	fmt.Fprintln(w, "  optional <constraint> are used to generate a CREATE TABLE statement.")
	fmt.Fprintln(w, "  In addition, <name> also appears in the SQL queries. sqlcup never")
	fmt.Fprintln(w, "  capitalizes those names.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  If any part of a <column> contains a space, it may be necessary to add")
	fmt.Fprintln(w, "  quotes or escape those spaces, depending on the user's shell.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Example:")
	fmt.Fprintln(w, "  sqlcup author/authors \"id:INTEGER:PRIMARY KEY\" \"name:text:NOT NULL\" bio:text")
	fmt.Fprintln(w, "  sqlcup --order-by name user/users \"id:INTEGER:PRIMARY KEY\" name:text")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
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
}

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
	for _, arg := range args[1:] {
		parts := strings.Split(arg, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("%w: invalid <column>: '%s', expected '<name>:<type>' or '<name>:<type>:<constraint>'", errBadArgument, arg)
		}
		col := column{
			Name: parts[0],
			Type: parts[1],
		}
		if len(col.Name) > sca.LongestName {
			sca.LongestName = len(col.Name)
		}
		if len(col.Type) > sca.LongestType {
			sca.LongestType = len(col.Type)
		}
		if len(parts) == 3 {
			col.Constraint = parts[2]
		}

		sca.Columns = append(sca.Columns, col)
		if strings.ToLower(parts[0]) == *idColumnFlag {
			sca.IDColumn = &col
		} else {
			sca.NonIDColumns = append(sca.NonIDColumns, col)
		}
	}
	return sca, nil
}

func scaffoldCommand(args *scaffoldCommandArgs) error {
	b := &strings.Builder{}

	b.WriteString("#############################################\n")
	b.WriteString("# Add the following to your SQL schema file #\n")
	b.WriteString("#############################################\n\n")
	writeSchema(b, args)
	b.WriteString("\n\n")

	b.WriteString("##############################################\n")
	b.WriteString("# Add the following to your SQL queries file #\n")
	b.WriteString("##############################################\n\n")
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
	fmt.Fprintf(w, "-- name: Delete%s\n", args.SingularEntity)
	fmt.Fprintf(w, "DELETE FROM %s\n", args.Table)
	fmt.Fprintf(w, "WHERE %s = ?;", args.IDColumn.Name)
}

//goland:noinspection GoUnhandledErrorResult
func writeUpdateQuery(w io.Writer, args *scaffoldCommandArgs) {
	fmt.Fprintf(w, "-- name: Update%s\n", args.SingularEntity)
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
