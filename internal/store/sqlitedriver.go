package store

// ============================================================================
// SQLite 驱动（单独一个文件，方便替换）
//
// 默认用纯 Go 的 modernc.org/sqlite：不需要 cgo / gcc，Termux、Android、
// Windows 都能直接编，交叉编译也不会因为 cgo 挂掉。
//
// 想换成 cgo 版（性能略好，但需要 C 编译器）：
//   1) 把下面的 import 换成：  import _ "github.com/mattn/go-sqlite3"
//   2) 把 SqliteDriverName 改成 "sqlite3"
//   3) go Get github.com/mattn/go-sqlite3
//
// 首次构建前需要拉依赖（二选一）：
//   go Get modernc.org/sqlite   然后 go mod tidy
// ============================================================================

import _ "modernc.org/sqlite"

const SqliteDriverName = "sqlite"
