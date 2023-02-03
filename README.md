# sqlcup

[sqlc](https://github.com/kyleconroy/sqlc) is great, but setting up `schema.sql` and `query.sql` could be a lot faster. Well, now it is.

## Installation

```
$ go install github.com/ngrash/sqlcup/cmd/sqlcup@v0.1.0
```

## Usage

```
$ sqlcup -help
sqlcup - generate SQL statements for sqlc (https://sqlc.dev)
                  
Synopsis:
  sqlcup [options] <name> <column> ...

Description:
  sqlcup prints SQL statements to stdout. The <name> argument given to sqlcup
  must be of the form <singular>/<plural> where <singular> is the name of the
  Go struct and <plural> is the name of the database table. 
  sqlcup capitalizes those names where required.                   
  
  Each <column> arguments given to sqlcup defines a database column and must
  be of the form <name>:<type>[:<constraint>]. <name>, <type> and the
  optional <constraint> are used to generate a CREATE TABLE statement.
  In addition, <name> also appears in the SQL queries. sqlcup never        
  capitalizes those names.           
                                                    
  If any part of a <column> contains a space, it may be necessary to add
  quotes or escape those spaces, depending on the user's shell.      
               
Example:
  sqlcup author/authors "id:INTEGER:PRIMARY KEY" "name:text:NOT NULL" bio:text
  sqlcup --order-by name user/users "id:INTEGER:PRIMARY KEY" name:text

Options:
  -id-column string
        Name of the column that identifies a row (default "id")
  -no-exists-clause
        Omit IF NOT EXISTS in CREATE TABLE statements
  -no-returning-clause
        Omit 'RETURNING *' in UPDATE statement
  -order-by string
        Include ORDER BY in 'SELECT *' statement
```

## Example

```
$ sqlcup author/authors "id:INTEGER:PRIMARY KEY" "name:text:NOT NULL" bio:text
#############################################
# Add the following to your SQL schema file #
#############################################

CREATE TABLE IF NOT EXISTS "authors" (
  "id"   INTEGER PRIMARY KEY,
  "name" text    NOT NULL,
  "bio"  text
);

##############################################
# Add the following to your SQL queries file #
##############################################

-- name: GetAuthor :one
SELECT * FROM "authors"
WHERE "id" = ? LIMIT 1;

-- name: ListAuthors :many
SELECT * FROM "authors"
ORDER BY "";

-- name: CreateAuthor :one
INSERT INTO "authors" (
  "name", "bio"
) VALUES (
  ?, ?
)
RETURNING *;

-- name: DeleteAuthor
DELETE FROM "authors"
WHERE "id" = ?;

-- name: UpdateAuthor
UPDATE "authors"
SET
  "name" = ?,
  "bio" = ?
WHERE "id" = ?
RETURNING *;
```