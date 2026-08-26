# hecc-blot-db

基于 GORM 的数据库组件：MySQL / PostgreSQL、链式查询、事务、多库动态切换，SQL 自动接入链路追踪。

## 安装

```bash
go get github.com/hecc-blot/db
```

## 接口定义

```go
import (
    dbContract "github.com/hecc-blot/db/contract"
    dbEnum "github.com/hecc-blot/db/enum/db"
)

type IDbFactory interface {
    Build(ctx context.Context, v ...dbEnum.Value) IDb
    SetDefault(t dbEnum.Value)
}

type IDbBase interface {
    GetInstance() any
}

type IDb interface {
    IDbBase
    Add(entry IDbModel) error
    Remove(entry IDbModel) error
    Query(entry IDbModel) IDb
    Save(entry IDbModel) error
    Count() (int64, error)
    Order(fields ...string) IDb
    Select(args ...interface{}) IDb
    Offset(v int) IDb
    Limit(v int) IDb
    Where(args ...interface{}) IDb
    Take(dst interface{}) error
    Find(dst interface{}) error
    WithContext(ctx context.Context) IDb
    Begin() IDb
    Commit() error
    Rollback()
}
```

## 初始化

**单库（推荐）：直接构造、注入 `IDb`，无需工厂。**

```go
import (
    db "github.com/hecc-blot/db/service"
)

mysqlDb, clearUp, err := db.NewMysql(&config.Db.Mysql, logSvc)
if err != nil {
    panic(err)
}
defer clearUp()

container.Set(new(dbContract.IDb), mysqlDb)
```

业务方直接注入 `IDb`，每个请求用 `WithContext(ctx)` 取副本：

```go
type ListApi struct {
    Db dbContract.IDb `inject:""`
}

func (a ListApi) Call(ctx *gin.Context) (interface{}, iCoreError.IError) {
    db := a.Db.WithContext(ctx)   // 返回绑定请求上下文的副本，并发安全
    data := new(make([]AccountModel, 0))
    if err := db.Where("id >= ?", 1).Find(data); err != nil {
        return nil, errorSvc.NewError(response.Fail, err)
    }
    return data, nil
}
```

**多库：用工厂按需切换（详见下文「多数据库切换」）。**

```go
dbFactory, clearUp, err := db.NewDbFactory(&config.Db, logSvc)
if err != nil {
    panic(err)
}
defer clearUp()

container.Set(new(dbContract.IDbFactory), dbFactory)
```

## Model 定义

Model 需嵌入 GORM Model 并实现 `IDbModel` 接口：

```go
type AccountModel struct {
    ID          int    `json:"id" gorm:"primaryKey"`
    AccountName string `json:"account_name"`
    Password    string `json:"password"`
    CreatedAt   int    `json:"created_at"`
    UpdatedAt   int    `json:"updated_at"`
    DeletedAt   int    `json:"deleted_at"`
}

func (a AccountModel) TableName() string {
    return "account"
}

func (a AccountModel) GetID() int {
    return a.ID
}
```

`GetID()` 用于框架获取主键，`TableName()` 指定表名。

## CRUD 操作

```go
// Add — 添加记录，newAccount.ID 会被自动填充
err := dbSvc.Add(&newAccount)

// Find — 查询多条
data := new(make([]AccountModel, 0))
err := dbSvc.Where("id >= ? AND id <= ?", 1, 8).Find(data)

// Find 分页
err = dbSvc.Where("id >= ?", 1).Offset(0).Limit(10).Find(data)

// Take — 查询单条
data := AccountModel{}
err := dbSvc.Where("id = ?", 1).Take(&data)

// Select — 指定字段
err = dbSvc.Select("id, account_name").Where("id = ?", 1).Take(&data)

// Save — 更新记录
updateData := AccountModel{AccountName: "updated"}
err = dbSvc.Where("id = ?", 1).Save(&updateData)

// Count — 统计
count, err := dbSvc.Query(&AccountModel{}).Where("id >= ?", 1).Count()

// Order — 排序
err = dbSvc.Select("id, account_name").Where("id >= ?", 1).Order("id DESC").Find(data)

// Remove — 删除
err = dbSvc.Where("id = ?", 1).Remove(&AccountModel{})
```

## 事务

`Begin()` 返回新的 `IDb` 事务实例，事务内所有操作都在该实例上执行：

```go
func (a AddApi) Call(ctx *gin.Context) (interface{}, iCoreError.IError) {
    db := a.DbFactory.Build(ctx)

    tx := db.Begin()

    newAccount := AccountModel{AccountName: "test"}
    if err := tx.Add(&newAccount); err != nil {
        tx.Rollback()
        return nil, errorSvc.NewError(response.Fail, err)
    }

    updateData := AccountModel{Password: "new-password"}
    if err := tx.Where("id = ?", newAccount.ID).Save(&updateData); err != nil {
        tx.Rollback()
        return nil, errorSvc.NewError(response.Fail, err)
    }

    if err := tx.Commit(); err != nil {
        return nil, errorSvc.NewError(response.Fail, err)
    }

    return newAccount, nil
}
```

**注意事项：**

- `Begin()` 返回新实例，必须在返回的 `tx` 上操作，不能用原始的 `db`
- `Rollback()` 和 `Commit()` 只在事务实例上调用
- `Commit()` 或 `Rollback()` 后事务实例不应再使用
- 原始 `db` 实例不受事务影响

## 多数据库切换

同时配置 MySQL 和 PostgreSQL（按配置段是否存在判断启用）。**默认库在 `NewDbFactory` 时一次性确定**：

| 配置情况 | 行为 |
|---|---|
| 仅配置 1 个库 | 自动作为默认库 |
| 配置多个库 | 必须设置 `default`，否则 `NewDbFactory` 返回错误 |
| 未配置任何库 | `NewDbFactory` 返回错误 |

```go
dbFactory, clearUp, err := db.NewDbFactory(&config.Db, logSvc)

// 不带参数使用默认数据库（由 config.default 或单库自动决定）
db := dbFactory.Build(ctx)

// 运行时指定数据库类型
mysqlDB := dbFactory.Build(ctx, dbEnum.Mysql)
pgDB := dbFactory.Build(ctx, dbEnum.Postgres)

// 运行时切换默认库（可选）
dbFactory.SetDefault(dbEnum.Postgres)
```

## SQL 链路追踪

通过 GORM 的 OpenTelemetry 插件自动为 SQL 生成 span。

| 属性 | 说明 |
|------|------|
| `db.system` | 数据库类型（`mysql` / `postgresql`），自动从驱动探测 |
| `db.statement` | 完整 SQL 语句（变量已替换） |
| `db.operation` | 操作类型（`select` / `insert` / `update` / `delete`） |
| `db.query.summary` | 操作 + 表名（如 `select account`），同时作为 span 名称 |
| `db.rows_affected` | 影响行数 |

### 依赖与初始化顺序

db 模块**只依赖第三方** `gorm.io/plugin/opentelemetry`，通过全局 `TracerProvider` 生成 span，**不依赖 trace 模块**，避免扩展间耦合。

由于插件在创建时捕获全局 `TracerProvider`，**必须先初始化 trace 再初始化 db**：

```go
// ✅ 正确顺序
traceSvc, traceClearUp, err := trace.NewTraceSvc(&config.Trace)   // 1. 先设置全局 provider
dbFactory, dbClearUp, err := db.NewDbFactory(&config.Db, logSvc) // 2. 再初始化 db
```

未初始化 trace 时，全局 provider 为空操作（noop），SQL span 自动失效，无额外开销。

## 在 API 中使用

通过 IOC 注入 `IDbFactory`，每次请求从 Context 获取数据库实例：

```go
type ListApi struct {
    DbFactory dbContract.IDbFactory `inject:""`
}

func (a ListApi) Call(ctx *gin.Context) (interface{}, iCoreError.IError) {
    db := a.DbFactory.Build(ctx)
    data := new(make([]AccountModel, 0))
    if err := db.Where("id >= ?", 1).Find(data); err != nil {
        return nil, errorSvc.NewError(response.Fail, err)
    }
    return data, nil
}
```

## 配置

```yaml
db:
  default: mysql             # 默认库 "mysql"/"postgres"，仅配置一个库时可不填
  mysql:
    username: root
    password: "123456"
    ip: 127.0.0.1            # 配置了该段即启用，缺省整段则不启用
    port: 3306
    db_name: test
    max_idle_conn: 10        # 最大空闲连接数
    max_open_conn: 100       # 最大打开连接数
    conn_max_lifetime: 3600  # 连接最大生命周期（秒）
    slow_threshold: 200      # 慢查询阈值（毫秒），0 不记录
  postgres:
    # ... 同上
```

## 相关模块

| 模块 | 说明 |
|------|------|
| [framework](https://github.com/hecc-blot/framework) | IOC 注入 `IDbFactory`、统一错误 |
| [cache](https://github.com/hecc-blot/cache) | 缓存与数据库配合的读穿透模式 |
| [trace](https://github.com/hecc-blot/trace) | SQL span 依赖的全局 TracerProvider |
