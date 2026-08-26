package config

type Config struct {
	Default  string          `mapstructure:"default"` // 默认库 "mysql"/"postgres"，多库时必填
	Mysql    *MysqlConfig    `mapstructure:"mysql"`   // nil = 未启用
	Postgres *PostgresConfig `mapstructure:"postgres"`
}

type MysqlConfig struct {
	Ip              string `mapstructure:"ip"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	DbName          string `mapstructure:"db_name"`
	ConnectTimeout  int    `mapstructure:"connect_timeout"`
	MaxIdleConn     int    `mapstructure:"max_idle_conn"`
	MaxOpenConn     int    `mapstructure:"max_open_conn"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	SlowThreshold   int    `mapstructure:"slow_threshold"`
}

type PostgresConfig struct {
	Ip              string `mapstructure:"ip"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	DbName          string `mapstructure:"db_name"`
	ConnectTimeout  int    `mapstructure:"connect_timeout"`
	MaxIdleConn     int    `mapstructure:"max_idle_conn"`
	MaxOpenConn     int    `mapstructure:"max_open_conn"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	SlowThreshold   int    `mapstructure:"slow_threshold"`
}
