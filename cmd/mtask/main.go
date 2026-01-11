package main

import (
	"kyri56xcaesar/pms-proj/internal/mtask"
)

func main() {
	mtask.InitAndServe("./config/task.container.env")
}
