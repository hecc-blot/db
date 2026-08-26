package dbEnum

import "fmt"

type Value int

const (
	Mysql Value = iota + 1
	Postgres
)

// FromString 将配置中的默认库字符串转为枚举值，未匹配返回错误。
func FromString(s string) (Value, error) {
	switch s {
	case "mysql":
		return Mysql, nil
	case "postgres":
		return Postgres, nil
	default:
		return 0, fmt.Errorf("无效db类型:%s", s)
	}
}
