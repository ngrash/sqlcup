# sqlcup

[sqlc](https://github.com/kyleconroy/sqlc) is great, but setting up `schema.sql` and `query.sql` could be a lot faster. Well, now it is.

## Installation

```
$ go install github.com/ngrash/sqlcup/cmd/sqlcup@v0.4.1
```

## Usage

```
$ sqlcup -help
sqlcup - generate SQL statements for sqlc (https://sqlc.dev)

Synopsis:
  sqlcup [options] <entity-name> <column> ...

Description:
  sqlcup prints SQL statements to stdout. The <entity-name> argument must be
  of the form <singular-name>/<plural-name>. sqlcup capitalizes those names
  where necessary.

  Each column argument given to sqlcup defines a database column and must
  be either a <plain-column> or a <smart-column>:

  A <plain-column> must be of the form <name>:<type>[:<constraint>]. <name>,
  <type> and the optional <constraint> are used to generate a CREATE TABLE
  statement. In addition, <name> also appears in SQL queries. sqlcup never
  capitalizes those names. To use <tag> you need to define a <smart-column>.

  A <smart-column> is a shortcut for common column definitions. It must be of
  the form [<name>]<tag>... where <name> is only optional for the special case
  when the <smart-column> consists of the single <tag> @id. A <smart-column> is
  not nullable unless @null is present.

  A <tag> adds either a data type or a constraint to a <smart-column>.

      @id
          Make this column the primary key. Omitting <type> and <name>
          for an @id column creates an INTEGER PRIMARY KEY named 'id'.

      @text, @int, @float, @double, @datetime, @blob
          Set the column type.

      @unique
          Add a UNIQUE constraint.

      @null
          Omit the default NOT NULL constraint.

  If any part of a <column> contains a space, it may be necessary to add        
  quotes or otherwise escape those spaces, depending on the user's shell.       

Example:
  sqlcup author/authors "id:INTEGER:PRIMARY KEY" "name:text:NOT NULL" bio:text  
  sqlcup --order-by name user/users "id:INTEGER:PRIMARY KEY" name:text
  sqlcup author/authors @id name@text@unique bio@text@null

Options:
  -id-column string
        Name of the column that identifies a row (default "id")
  -no-exists-clause
        Omit IF NOT EXISTS in CREATE TABLE statements
  -no-returning-clause
        Omit 'RETURNING *' in UPDATE statement
  -only string
        Limit output to 'schema' or 'queries'
  -order-by string
        Include ORDER BY in 'SELECT *' statement
```

## Example

```
$ sqlcup --order-by name author/authors "id:INTEGER:PRIMARY KEY" "name:text:NOT NULL" bio:text
#############################################
# Add the following to your SQL schema file #
#############################################

CREATE TABLE IF NOT EXISTS authors (
  id   INTEGER PRIMARY KEY,
  name text    NOT NULL,
  bio  text
);

##############################################
# Add the following to your SQL queries file #
##############################################

-- name: GetAuthor :one
SELECT * FROM authors
WHERE id = ? LIMIT 1;

-- name: ListAuthors :many
SELECT * FROM authors
ORDER BY name;

-- name: CreateAuthor :one
INSERT INTO authors (
  name, bio
) VALUES (
  ?, ?
)
RETURNING *;

-- name: DeleteAuthor :exec
DELETE FROM authors
WHERE id = ?;

-- name: UpdateAuthor :one
UPDATE authors
SET
  name = ?,
  bio = ?
WHERE id = ?
RETURNING *;
```