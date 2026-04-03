package response

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// textRender is the intermediate representation produced by renderText.
// It separates key-value fields from body content so that plain-text and
// ANSI renderers can decorate the same structure differently.
type textRender struct {
	Fields []textField
	Body   string
}

type textField struct {
	Key   string
	Value string
}

// bodyKeys are field names whose values go into the body section rather
// than appearing as key-value header fields.
var bodyKeys = map[string]bool{
	"body":    true,
	"content": true,
	"message": true,
}

// renderText inspects data and produces a structured textRender.
func renderText(data any) textRender {
	if data == nil {
		return textRender{}
	}

	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return textRender{}
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		return renderMap(v)
	case reflect.Struct:
		return renderStruct(v)
	case reflect.Slice, reflect.Array:
		return renderSlice(v)
	default:
		return textRender{Body: fmt.Sprint(data)}
	}
}

func renderMap(v reflect.Value) textRender {
	// Sort keys for deterministic output.
	keys := make([]string, 0, v.Len())
	for _, k := range v.MapKeys() {
		keys = append(keys, fmt.Sprint(k.Interface()))
	}
	sort.Strings(keys)

	var r textRender
	var bodyParts []string

	for _, key := range keys {
		val := v.MapIndex(reflect.ValueOf(key)).Interface()

		if bodyKeys[strings.ToLower(key)] {
			bodyParts = append(bodyParts, fmt.Sprint(val))
			continue
		}

		r.Fields = append(r.Fields, textField{
			Key:   key,
			Value: formatValue(val, 0),
		})
	}

	if len(bodyParts) > 0 {
		r.Body = strings.Join(bodyParts, "\n")
	}

	return r
}

func renderStruct(v reflect.Value) textRender {
	t := v.Type()

	var r textRender
	var bodyParts []string

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		name := f.Name
		if tag, ok := f.Tag.Lookup("json"); ok {
			parts := strings.SplitN(tag, ",", 2)
			if parts[0] != "" && parts[0] != "-" {
				name = parts[0]
			}
		}

		val := v.Field(i).Interface()

		if bodyKeys[strings.ToLower(name)] {
			bodyParts = append(bodyParts, fmt.Sprint(val))
			continue
		}

		r.Fields = append(r.Fields, textField{
			Key:   name,
			Value: formatValue(val, 0),
		})
	}

	if len(bodyParts) > 0 {
		r.Body = strings.Join(bodyParts, "\n")
	}

	return r
}

func renderSlice(v reflect.Value) textRender {
	var r textRender
	r.Fields = append(r.Fields, textField{Key: "Count", Value: fmt.Sprint(v.Len())})

	if v.Len() == 0 {
		return r
	}

	var body strings.Builder
	for i := range v.Len() {
		if i > 0 {
			body.WriteString("\n")
		}

		elem := v.Index(i).Interface()
		ev := reflect.ValueOf(elem)
		for ev.Kind() == reflect.Pointer {
			if ev.IsNil() {
				break
			}
			ev = ev.Elem()
		}

		prefix := fmt.Sprintf("[%d]", i+1)

		switch ev.Kind() {
		case reflect.Map:
			sub := renderMap(ev)
			for j, f := range sub.Fields {
				if j == 0 {
					body.WriteString(fmt.Sprintf("%s %s: %s\n", prefix, f.Key, f.Value))
				} else {
					body.WriteString(fmt.Sprintf("%s %s: %s\n", strings.Repeat(" ", len(prefix)), f.Key, f.Value))
				}
			}
			if sub.Body != "" {
				for _, line := range strings.Split(sub.Body, "\n") {
					body.WriteString(fmt.Sprintf("%s %s\n", strings.Repeat(" ", len(prefix)), line))
				}
			}
		case reflect.Struct:
			sub := renderStruct(ev)
			for j, f := range sub.Fields {
				if j == 0 {
					body.WriteString(fmt.Sprintf("%s %s: %s\n", prefix, f.Key, f.Value))
				} else {
					body.WriteString(fmt.Sprintf("%s %s: %s\n", strings.Repeat(" ", len(prefix)), f.Key, f.Value))
				}
			}
		default:
			body.WriteString(fmt.Sprintf("%s %s\n", prefix, formatValue(elem, 0)))
		}
	}

	r.Body = strings.TrimRight(body.String(), "\n")
	return r
}

// formatValue renders a single value as a string. The indent parameter
// controls indentation depth for nested structures.
func formatValue(val any, indent int) string {
	if val == nil {
		return ""
	}

	v := reflect.ValueOf(val)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		return formatMapInline(v)
	case reflect.Slice, reflect.Array:
		return formatSliceInline(v)
	default:
		return fmt.Sprint(val)
	}
}

func formatMapInline(v reflect.Value) string {
	keys := make([]string, 0, v.Len())
	for _, k := range v.MapKeys() {
		keys = append(keys, fmt.Sprint(k.Interface()))
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		val := v.MapIndex(reflect.ValueOf(key)).Interface()
		parts = append(parts, fmt.Sprintf("%s=%v", key, val))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatSliceInline(v reflect.Value) string {
	parts := make([]string, 0, v.Len())
	for i := range v.Len() {
		parts = append(parts, fmt.Sprint(v.Index(i).Interface()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// renderPlain writes the textRender as plain text with no decoration.
func renderPlain(r textRender) []byte {
	var b strings.Builder

	for _, f := range r.Fields {
		b.WriteString(f.Key)
		b.WriteString(": ")
		b.WriteString(f.Value)
		b.WriteString("\n")
	}

	if r.Body != "" {
		if len(r.Fields) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(r.Body)
		b.WriteString("\n")
	}

	return []byte(b.String())
}
