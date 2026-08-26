package service

import (
	"context"
	"net/http"
	"testing"

	dbConf "github.com/hecc-blot/db/config"
	dbContract "github.com/hecc-blot/db/contract"
	dbEnum "github.com/hecc-blot/db/enum/db"
	logConf "github.com/hecc-blot/framework/config/log"
	log "github.com/hecc-blot/framework/service/log"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

var dbConfig = &dbConf.Config{
	Default:  "mysql",
	Mysql:    &mysqlConf,
	Postgres: &postgresConfig,
}

var mysqlConf = dbConf.MysqlConfig{
	Ip:              "127.0.0.1",
	Port:            3306,
	Username:        "root",
	Password:        "123456",
	DbName:          "core",
	ConnectTimeout:  3,
	MaxIdleConn:     10,
	MaxOpenConn:     200,
	ConnMaxLifetime: 3600,
	SlowThreshold:   200,
}

var postgresConfig = dbConf.PostgresConfig{
	Ip:              "127.0.0.1",
	Port:            5432,
	Username:        "admin",
	Password:        "123456",
	DbName:          "core",
	ConnectTimeout:  3,
	MaxIdleConn:     10,
	MaxOpenConn:     200,
	ConnMaxLifetime: 3600,
	SlowThreshold:   200,
}

var localConf = &logConf.LocalConfig{
	Enable:     true,
	RootDir:    "./runtime/logs",
	MaxSize:    1,
	MaxBackups: 3,
	MaxAge:     7,
	Compress:   false,
}

// Account 定义model
type Account struct {
	ID          int                   `json:"id" gorm:"primaryKey"`
	AccountName string                `json:"account_name"`
	Password    string                `json:"password"`
	CreatedAt   int64                 `json:"created_at"`
	UpdatedAt   int64                 `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"deleted_at"`
}

func (b Account) GetID() int {
	return b.ID
}

// TestBuildExtractsGinContext 验证 Build 会将 *gin.Context 提取为 Request.Context()。
// 若不提取，GORM otel 插件从 *gin.Context 读不到父 span（gin 的 Value 默认不委托到
// Request.Context()），SQL 会变成独立 trace。
func TestBuildExtractsGinContext(t *testing.T) {
	gormDB, err := gorm.Open(mysql.Open("root:123456@tcp(127.0.0.1:3306)/core"), &gorm.Config{})
	assert.NoError(t, err)

	f := Factory{
		db:        map[dbEnum.Value]dbContract.IDb{dbEnum.Mysql: &BaseDbSvc{db: gormDB}},
		defaultDb: dbEnum.Mysql,
	}

	// 模拟 trace 中间件：父 span 等信息存放在 Request.Context() 中
	req := (&http.Request{}).WithContext(context.WithValue(context.Background(), "trace.parent", "span"))
	ginCtx := &gin.Context{Request: req}

	clone := f.Build(ginCtx)

	stmtCtx := clone.GetInstance().(*gorm.DB).Statement.Context
	_, isGin := stmtCtx.(*gin.Context)
	assert.False(t, isGin)
	assert.Equal(t, "span", stmtCtx.Value("trace.parent"))
}

func TestFactory(t *testing.T) {
	logSvc, err := log.NewLogger(localConf)
	assert.NoError(t, err)
	assert.NotNil(t, logSvc)

	dbFactory, clearUp, err := NewDbFactory(dbConfig, logSvc)
	defer clearUp()
	assert.NoError(t, err)
	assert.NotNil(t, dbFactory)

	ctx := context.Background()
	t.Run("default mysql", func(t *testing.T) {
		newData := Account{
			AccountName: "mysql",
		}
		mysqlDB := dbFactory.Build(ctx)
		err = mysqlDB.Add(&newData)
		assert.NoError(t, err)

		data := Account{}
		err = mysqlDB.
			Where("id = ?", newData.ID).
			Take(&data)
		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, "mysql", data.AccountName)

		t.Logf("data: %+v", data)
	})

	t.Run("default with postgres", func(t *testing.T) {
		data := Account{}
		err = dbFactory.Build(ctx, dbEnum.Postgres).
			Where("id = ?", 1).
			Take(&data)
		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, "pg", data.AccountName)

		t.Logf("data: %+v", data)
	})

	t.Run("set postgres", func(t *testing.T) {
		dbFactory.SetDefault(dbEnum.Postgres)

		data := Account{}
		err = dbFactory.Build(ctx).
			Where("id = ?", 1).
			Take(&data)

		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, "pg", data.AccountName)

		t.Logf("data: %+v", data)
	})

	t.Run("set postgres with mysql", func(t *testing.T) {
		dbFactory.SetDefault(dbEnum.Postgres)

		newData := Account{
			AccountName: "mysql",
		}
		mysqlDB := dbFactory.Build(ctx, dbEnum.Mysql)
		err = mysqlDB.Add(&newData)
		assert.NoError(t, err)

		data := Account{}
		err = mysqlDB.
			Where("id = ?", newData.ID).
			Take(&data)

		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, "mysql", data.AccountName)

		t.Logf("data: %+v", data)
	})
}

// TestResolveDefault 验证默认库 fail-fast 规则（纯函数，无需数据库连接）
func TestResolveDefault(t *testing.T) {
	t.Run("未配置任何库", func(t *testing.T) {
		_, err := resolveDefault(nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "未配置任何数据库")
	})

	t.Run("单库自动默认", func(t *testing.T) {
		d, err := resolveDefault([]dbEnum.Value{dbEnum.Mysql}, "")
		assert.NoError(t, err)
		assert.Equal(t, dbEnum.Mysql, d)
	})

	t.Run("多库未设置默认库", func(t *testing.T) {
		_, err := resolveDefault([]dbEnum.Value{dbEnum.Mysql, dbEnum.Postgres}, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "必须设置 default")
	})

	t.Run("多库设置有效默认库", func(t *testing.T) {
		d, err := resolveDefault([]dbEnum.Value{dbEnum.Mysql, dbEnum.Postgres}, "postgres")
		assert.NoError(t, err)
		assert.Equal(t, dbEnum.Postgres, d)
	})

	t.Run("多库设置无效默认库", func(t *testing.T) {
		_, err := resolveDefault([]dbEnum.Value{dbEnum.Mysql, dbEnum.Postgres}, "oracle")
		assert.Error(t, err)
	})

	t.Run("多库设置未配置的默认库", func(t *testing.T) {
		_, err := resolveDefault([]dbEnum.Value{dbEnum.Mysql}, "postgres")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "未配置")
	})
}
