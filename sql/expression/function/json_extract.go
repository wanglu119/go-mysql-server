// Copyright 2020-2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package function

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oliveagle/jsonpath"

	"github.com/dolthub/go-mysql-server/sql"
)

// JSONExtract extracts data from a json document using json paths.
type JSONExtract struct {
	JSON  sql.Expression
	Paths []sql.Expression
}

var _ sql.FunctionExpression = (*JSONExtract)(nil)

// NewJSONExtract creates a new JSONExtract UDF.
func NewJSONExtract(args ...sql.Expression) (sql.Expression, error) {
	if len(args) < 2 {
		return nil, sql.ErrInvalidArgumentNumber.New("JSON_EXTRACT", 2, len(args))
	}

	return &JSONExtract{args[0], args[1:]}, nil
}

// FunctionName implements sql.FunctionExpression
func (j *JSONExtract) FunctionName() string {
	return "json_extract"
}

// Resolved implements the sql.Expression interface.
func (j *JSONExtract) Resolved() bool {
	for _, p := range j.Paths {
		if !p.Resolved() {
			return false
		}
	}
	return j.JSON.Resolved()
}

// Type implements the sql.Expression interface.
func (j *JSONExtract) Type() sql.Type { return sql.JSON }

// Eval implements the sql.Expression interface.
func (j *JSONExtract) Eval(ctx *sql.Context, row sql.Row) (interface{}, error) {
	span, ctx := ctx.Span("function.JSONExtract")
	defer span.Finish()

	js, err := j.JSON.Eval(ctx, row)
	if err != nil {
		return nil, err
	}

	doc, err := unmarshalVal(js)
	if err != nil {
		return nil, err
	}

	var result = make([]interface{}, len(j.Paths))
	for i, p := range j.Paths {
		path, err := p.Eval(ctx, row)
		if err != nil {
			return nil, err
		}

		path, err = sql.LongText.Convert(path)
		if err != nil {
			return nil, err
		}

		c, err := jsonpath.Compile(path.(string))
		if err != nil {
			return nil, err
		}

		result[i], _ = c.Lookup(doc) // err ignored
	}

	if len(result) == 1 {
		return result[0], nil
	}

	return result, nil
}

func unmarshalVal(v interface{}) (interface{}, error) {
	v, err := sql.JSON.Convert(v)
	if err != nil {
		return nil, err
	}

	var doc interface{}
	if err := json.Unmarshal(v.([]byte), &doc); err != nil {
		return nil, err
	}

	return doc, nil
}

// IsNullable implements the sql.Expression interface.
func (j *JSONExtract) IsNullable() bool {
	for _, p := range j.Paths {
		if p.IsNullable() {
			return true
		}
	}
	return j.JSON.IsNullable()
}

// Children implements the sql.Expression interface.
func (j *JSONExtract) Children() []sql.Expression {
	return append([]sql.Expression{j.JSON}, j.Paths...)
}

// WithChildren implements the Expression interface.
func (j *JSONExtract) WithChildren(children ...sql.Expression) (sql.Expression, error) {
	return NewJSONExtract(children...)
}

func (j *JSONExtract) String() string {
	children := j.Children()
	var parts = make([]string, len(children))
	for i, c := range children {
		parts[i] = c.String()
	}
	return fmt.Sprintf("JSON_EXTRACT(%s)", strings.Join(parts, ", "))
}
