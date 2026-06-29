// Package database 提供基于 SQLite 的持久化，存放插件生成并同步过来的 AST design。
// 用纯 Go 的 modernc.org/sqlite 驱动（无需 CGO，兼容现有交叉编译），通过标准
// database/sql 使用。建表迁移随 Open 幂等执行，无需独立的迁移命令。
package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// schema 是幂等的建表语句：designs 存一条 AST 结果，created_at 用 unix 毫秒便于排序。
const schema = `
CREATE TABLE IF NOT EXISTS designs (
	id         TEXT    PRIMARY KEY,
	tag        TEXT    NOT NULL,
	ast        TEXT    NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_designs_created_at ON designs(created_at DESC);
`

// Open 打开（或首次创建）SQLite 数据库并执行建表迁移。
// SQLite 的写是串行的，把连接数限制为 1 可规避并发写下的 "database is locked"。
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}
	return db, nil
}
