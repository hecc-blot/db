package db

// IDbBase 是所有数据库实例共有的基础能力。
// 只放「不依赖自身返回类型」的方法（GetInstance），
// 链式方法（如 WithContext）因需返回各自接口类型，留在各具体接口内。
type IDbBase interface {
	GetInstance() any
}
