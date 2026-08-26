package service

import (
	"context"
	"fmt"
	"slices"

	dbConf "github.com/hecc-blot/db/config"
	dbContract "github.com/hecc-blot/db/contract"
	dbEnum "github.com/hecc-blot/db/enum/db"
	"github.com/hecc-blot/framework/contract/log"
)

type Factory struct {
	db        map[dbEnum.Value]dbContract.IDb
	defaultDb dbEnum.Value
}

func (f *Factory) Build(ctx context.Context, v ...dbEnum.Value) dbContract.IDb {
	t := f.defaultDb
	if len(v) > 0 {
		t = v[0]
	}

	dbSvc, ok := f.db[t]
	if !ok {
		panic(fmt.Sprintf("无效db类型:%v", v))
	}

	// WithContext 返回绑定请求上下文的副本，保证每请求隔离
	return dbSvc.WithContext(ctx)
}

func (f *Factory) SetDefault(t dbEnum.Value) {
	if _, ok := f.db[t]; !ok {
		panic(fmt.Sprintf("无效db类型:%v", t))
	}
	f.defaultDb = t
}

func NewDbFactory(config *dbConf.Config, logger log.ILog) (dbContract.IDbFactory, func(), error) {
	f := Factory{
		db: make(map[dbEnum.Value]dbContract.IDb),
	}
	var cleanupFuncs []func()

	if config.Mysql != nil {
		mysql, cleanup, err := NewMysql(config.Mysql, logger)
		if err != nil {
			return nil, func() {}, err
		}
		f.db[dbEnum.Mysql] = mysql
		cleanupFuncs = append(cleanupFuncs, cleanup)
	}

	if config.Postgres != nil {
		postgres, cleanup, err := NewPostgres(config.Postgres, logger)
		if err != nil {
			return nil, func() {}, err
		}
		f.db[dbEnum.Postgres] = postgres
		cleanupFuncs = append(cleanupFuncs, cleanup)
	}

	// 默认库解析：New 时一次性定死，成功后 defaultDb 恒有效，Build() 永不因配置歧义 panic
	configured := make([]dbEnum.Value, 0, len(f.db))
	for k := range f.db {
		configured = append(configured, k)
	}
	defaultDb, err := resolveDefault(configured, config.Default)
	if err != nil {
		return nil, func() {}, err
	}
	f.defaultDb = defaultDb

	return &f, func() {
		for _, fn := range cleanupFuncs {
			fn()
		}
	}, nil
}

// resolveDefault 解析默认库，New 时一次定死，返回 error 实现 fail-fast。
func resolveDefault(configured []dbEnum.Value, defaultName string) (dbEnum.Value, error) {
	if len(configured) == 0 {
		return 0, fmt.Errorf("未配置任何数据库")
	}

	// 显式声明默认库：必须指向已配置的库，否则 fail-fast
	if defaultName != "" {
		t, err := dbEnum.FromString(defaultName)
		if err != nil {
			return 0, err
		}
		if slices.Contains(configured, t) {
			return t, nil
		}
		return 0, fmt.Errorf("默认库 %s 未配置", defaultName)
	}

	// 未声明默认库：单库自动默认，多库必须显式声明
	if len(configured) == 1 {
		return configured[0], nil
	}
	return 0, fmt.Errorf("配置了多个数据库，必须设置 default 指定默认库")
}
