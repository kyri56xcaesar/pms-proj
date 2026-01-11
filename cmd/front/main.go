package main

import (
	"kyri56xcaesar/pms-proj/internal/front"
)

func main() {

	front.InitAndServe("./config/front.container.env")
}
