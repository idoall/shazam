// Copyright 2019 The Gaea Authors. All Rights Reserved.
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

package plan

import (
	"fmt"
	"strings"

	"github.com/nooncall/shazam/backend"
	"github.com/nooncall/shazam/mysql"
	"github.com/nooncall/shazam/parser/ast"
	"github.com/nooncall/shazam/parser/format"
	"github.com/nooncall/shazam/proxy/router"
	"github.com/nooncall/shazam/proxy/sequence"
	"github.com/nooncall/shazam/util"
)

// constants of ShardType
const (
	ShardTypeUnshard = "unshard"
	ShardTypeShard   = "shard"
)

// ExplainPlan is the plan for explain statement
type ExplainPlan struct {
	shardType string
	sqls      map[string]map[string][]string
}

func buildExplainPlan(stmt *ast.ExplainStmt, db, sql string, r *router.Router, seq *sequence.SequenceManager) (*ExplainPlan, error) {
	stmtToExplain := stmt.Stmt
	if _, ok := stmtToExplain.(*ast.ExplainStmt); ok {
		return nil, fmt.Errorf("nested explain")
	}

	p, err := BuildPlan(stmtToExplain, db, sql, r, seq)
	if err != nil {
		return nil, fmt.Errorf("build plan to explain error: %v", err)
	}

	ep := &ExplainPlan{}

	switch pl := p.(type) {
	case *SelectPlan:
		ep.shardType = ShardTypeShard
		ep.sqls = pl.sqls
		return ep, nil
	case *DeletePlan:
		ep.shardType = ShardTypeShard
		ep.sqls = pl.sqls
		return ep, nil
	case *UpdatePlan:
		ep.shardType = ShardTypeShard
		ep.sqls = pl.sqls
		return ep, nil
	case *InsertPlan:
		ep.shardType = ShardTypeShard
		ep.sqls = pl.sqls
		return ep, nil
	case *UnshardPlan:
		ep.shardType = ShardTypeUnshard
		// 非分片SQL要用Restore从ast还原, 因为原SQL含有EXPLAIN
		ep.sqls = make(map[string]map[string][]string)
		dbSQLs := make(map[string][]string)
		s := &strings.Builder{}
		ctx := format.NewRestoreCtx(format.DefaultRestoreFlags, s)
		_ = pl.stmt.Restore(ctx)
		dbSQLs[pl.db] = []string{s.String()}
		ep.sqls[backend.DefaultSlice] = dbSQLs
		return ep, nil
	default:
		return nil, fmt.Errorf("unsupport plan to explain, type: %T", p)
	}
}

// ExecuteIn implement Plan
func (p *ExplainPlan) ExecuteIn(*util.RequestContext, Executor) (*mysql.Result, error) {
	return createExplainResult(p.shardType, p.sqls), nil
}

// Size implement Plan
func (p *ExplainPlan) Size() int {
	return 1
}

func createExplainResult(shardType string, sqls map[string]map[string][]string) *mysql.Result {
	var rows [][]interface{}
	var names = []string{"type", "slice", "db", "sql"}

	for slice, dbSQLs := range sqls {
		for db, tableSQLs := range dbSQLs {
			for _, sql := range tableSQLs {
				row := []interface{}{shardType, slice, db, sql}
				rows = append(rows, row)
			}
		}
	}

	r, _ := mysql.BuildResultset(nil, names, rows)
	ret := &mysql.Result{
		Resultset: r,
	}

	return ret
}
