# `internal/infra/database/`

未来扩展点。

如果引入数据库（SQLite / Postgres / ...），把对应实现放在这里，
和 `queue/`、`storage/` 平级，向上对调用者暴露接口，注入到 service / handler 层。
保持 `infra/` 下都是"可替换的 IO / 异步后端"。

当前没有任何 Go 代码——占位文件仅用于目录占位。