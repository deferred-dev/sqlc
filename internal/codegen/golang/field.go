package golang

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/codegen/golang/opts"
	"github.com/sqlc-dev/sqlc/internal/plugin"
)

type Field struct {
	Name             string // CamelCased name for Go
	VariableForField string // Variable name for the field (including structVar.name, if part of a struct)
	IsKeyField       bool   // If this field is being output as the key for a map
	DBName           string // Name as used in the DB
	Type             string
	Tags             map[string]string
	Comment          string
	Column           *plugin.Column
	// EmbedFields contains the embedded fields that require scanning.
	EmbedFields []Field
}

func (gf *Field) Tag() string {
	return TagsToString(gf.Tags)
}

func (gf *Field) HasSqlcSlice() bool {
	return gf.Column != nil && gf.Column.IsSqlcSlice
}

func (gf *Field) IsNullable() bool {
	return !gf.Column.GetNotNull()
}

func (gf *Field) Serialize() bool {
	return gf.IsPointer() || (strings.IndexByte(gf.Type, '.') > 0 && !strings.HasSuffix(gf.Type, "Duration"))
}

func (gf *Field) Deserialize() bool {
	return gf.IsPointer() || (strings.IndexByte(gf.Type, '.') > 0 && !strings.HasSuffix(gf.Type, "64") && !strings.HasSuffix(gf.Type, "32") && !strings.HasSuffix(gf.Type, "Type") && !strings.HasSuffix(gf.Type, "Duration"))
}

func (gf *Field) HasLen() bool {
	return gf.Type == "[]byte" || gf.Type == "string"
}

func (gf *Field) Is64Bit() bool {
	return strings.HasSuffix(gf.Type, "64")
}

func (gf *Field) IsPointer() bool {
	return strings.HasPrefix(gf.Type, "*")
}

func (gf *Field) NeedsCast(toType string) bool {
	ty := gf.Type
	if gf.HasSqlcSlice() {
		ty = ty[2:]
	}
	return ty != toType
}

func (gf *Field) BindType() string {
	ty := gf.Type
	if gf.HasSqlcSlice() {
		ty = ty[2:]
	}
	switch {
	case ty == "uint8", ty == "int8", ty == "uint32", strings.HasSuffix(ty, "Duration"):
		return "int64"
	case ty == "float32":
		return "float64"
	case strings.HasSuffix(ty, "TimeOrderedID"):
		return "[]byte"
	case strings.HasSuffix(ty, "Bool"):
		return "bool"
	case strings.Contains(ty, "Null"):
		return "int64"
	case strings.HasSuffix(ty, "ID"), strings.HasSuffix(ty, "Type"), strings.HasSuffix(ty, "Priority"):
		return "int64"
	case strings.IndexByte(ty, '.') >= 0:
		return "[]byte"
	default:
		return ty
	}
}

func (gf *Field) BindMethod() string {
	bindType := gf.BindType()
	switch bindType {
	case "string":
		return "r.bindString"
	case "[]byte":
		return "r.bindBytes"
	case "float64":
		return "r.stmt.BindFloat"
	default:
		return "r.stmt.Bind" + toPascalCase(bindType)
	}
}

func (gf *Field) FetchMethod() string {
	bindType := gf.BindType()
	switch bindType {
	case "string":
		return "r.stmt.ColumnText"
	case "[]byte":
		if gf.Serialize() {
			// We can use zero-copy bytes to deserialize the field
			return "r.columnPeekBytes"
		}
		return "r.columnBytes"
	case "float64":
		return "r.stmt.ColumnFloat"
	default:
		return "r.stmt.Column" + toPascalCase(bindType)
	}
}

func (gf *Field) FetchInto() string {
	if gf.IsKeyField {
		return "r.Key"
	}
	if gf.Name == "" {
		return "r.Row"
	}
	return "r.Row." + gf.Name
}

func (gf *Field) DeserializeMethod() string {
	ty := gf.Type
	if gf.HasSqlcSlice() {
		ty = ty[2:]
	} else if strings.HasPrefix(ty, "*") {
		ty = ty[1:]
	}
	i := strings.IndexByte(ty, '.')
	if i >= 0 {
		return ty[:i] + ".Deserialize" + toPascalCase(ty[i+1:])
	}
	panic("not a deserializable type: " + ty)
}

func (gf *Field) WithVariable(v string) *Field {
	return &Field{
		Name:             gf.Name,
		VariableForField: v,
		DBName:           gf.DBName,
		Type:             gf.Type,
		Tags:             gf.Tags,
		Comment:          gf.Comment,
		Column:           gf.Column,
		EmbedFields:      gf.EmbedFields,
	}
}

func TagsToString(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	tagParts := make([]string, 0, len(tags))
	for key, val := range tags {
		tagParts = append(tagParts, fmt.Sprintf("%s:%q", key, val))
	}
	sort.Strings(tagParts)
	return strings.Join(tagParts, " ")
}

func JSONTagName(name string, options *opts.Options) string {
	style := options.JsonTagsCaseStyle
	idUppercase := options.JsonTagsIdUppercase
	addOmitEmpty := options.JsonTagsOmitEmpty
	if style != "" && style != "none" {
		name = SetJSONCaseStyle(name, style, idUppercase)
	}
	if addOmitEmpty {
		name = name + ",omitempty"
	}
	return name
}

func SetCaseStyle(name string, style string) string {
	switch style {
	case "camel":
		return toCamelCase(name)
	case "pascal":
		return toPascalCase(name)
	case "snake":
		return toSnakeCase(name)
	default:
		panic(fmt.Sprintf("unsupported JSON tags case style: '%s'", style))
	}
}

func SetJSONCaseStyle(name string, style string, idUppercase bool) string {
	switch style {
	case "camel":
		return toJsonCamelCase(name, idUppercase)
	case "pascal":
		return toPascalCase(name)
	case "snake":
		return toSnakeCase(name)
	default:
		panic(fmt.Sprintf("unsupported JSON tags case style: '%s'", style))
	}
}

var camelPattern = regexp.MustCompile("[^A-Z][A-Z]+")

func toSnakeCase(s string) string {
	if !strings.ContainsRune(s, '_') {
		s = camelPattern.ReplaceAllStringFunc(s, func(x string) string {
			return x[:1] + "_" + x[1:]
		})
	}
	return strings.ToLower(s)
}

func toCamelCase(s string) string {
	return toCamelInitCase(s, false)
}

func toPascalCase(s string) string {
	return toCamelInitCase(s, true)
}

func toCamelInitCase(name string, initUpper bool) string {
	out := ""
	for i, p := range strings.Split(name, "_") {
		if !initUpper && i == 0 {
			out += p
			continue
		}
		if p == "id" {
			out += "ID"
		} else {
			out += strings.Title(p)
		}
	}
	return out
}

func toJsonCamelCase(name string, idUppercase bool) string {
	out := ""
	idStr := "Id"

	if idUppercase {
		idStr = "ID"
	}

	for i, p := range strings.Split(name, "_") {
		if i == 0 {
			out += p
			continue
		}
		if p == "id" {
			out += idStr
		} else {
			out += strings.Title(p)
		}
	}
	return out
}

func toLowerCase(str string) string {
	if str == "" {
		return ""
	}

	return strings.ToLower(str[:1]) + str[1:]
}
