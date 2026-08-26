package service

import (
	"fmt"

	dbConf "github.com/hecc-blot/db/config"
	dbContract "github.com/hecc-blot/db/contract"
	"github.com/hecc-blot/framework/contract/log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MysqlSvc struct {
	BaseDbSvc
}

// NewMysql 创建单个 MySQL 数据库实例（单库用户直接注入 IDb，无需工厂）
func NewMysql(config *dbConf.MysqlConfig, logger log.ILog) (dbContract.IDb, func(), error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&timeout=%ds",
		config.Username,
		config.Password,
		config.Ip,
		config.Port,
		config.DbName,
		config.ConnectTimeout,
	)

	mysqlDb, err := gorm.Open(mysql.Open(dsn), initGormConfig(logger, config.SlowThreshold))
	if err != nil {
		return nil, func() {}, err
	}

	// 注册 OpenTelemetry 追踪插件，SQL 执行自动生成 span
	useOtelPlugin(mysqlDb)

	sqlDb, err := mysqlDb.DB()
	if err != nil {
		return nil, func() {}, err
	}

	setSqlDbPool(sqlDb, config.MaxIdleConn, config.MaxOpenConn, config.ConnMaxLifetime)

	return &MysqlSvc{
			BaseDbSvc{db: mysqlDb},
		}, func() {
			sqlDb.Close()
		}, nil
}
