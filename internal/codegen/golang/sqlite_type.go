package golang

import (
	"log"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/codegen/golang/opts"
	"github.com/sqlc-dev/sqlc/internal/codegen/sdk"
	"github.com/sqlc-dev/sqlc/internal/debug"
	"github.com/sqlc-dev/sqlc/internal/plugin"
)

func sqliteType(_req *plugin.GenerateRequest, _options *opts.Options, col *plugin.Column) string {
	dt := strings.ToLower(sdk.DataType(col.Type))
	notNull := col.NotNull || col.IsArray

	switch dt {
	case "int", "integer", "tinyint", "smallint", "mediumint", "int2":
		if notNull {
			return "uint32"
		}
		return "types.NullUint32"
	case "bigint", "unsignedbigint", "int8":
		if notNull {
			return "int64"
		}
		return "types.NullInt64"
	case "blob":
		return "[]byte"
	case "real", "double", "doubleprecision", "float":
		if notNull {
			return "float64"
		}
		return "sql.NullFloat64"
	case "boolean", "bool":
		if notNull {
			return "bool"
		}
		return "sql.NullBool"
	case "date", "datetime", "timestamp":
		if notNull {
			return "time.Time"
		}
		return "sql.NullTime"
	case "any":
		return "interface{}"
	}

	switch {
	case dt == "text",
		strings.HasPrefix(dt, "character"),
		strings.HasPrefix(dt, "varchar"),
		strings.HasPrefix(dt, "varyingcharacter"),
		strings.HasPrefix(dt, "nchar"),
		strings.HasPrefix(dt, "nativecharacter"),
		strings.HasPrefix(dt, "nvarchar"),
		dt == "clob":
		return "string"
	case strings.HasPrefix(dt, "decimal"), dt == "numeric":
		if notNull {
			return "float64"
		}
		return "types.NullFloat64"
	default:
		if debug.Active {
			log.Printf("unknown SQLite type: %s\n", dt)
		}
		return "interface{}"
	}
}
