package main

import (
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/clang"
)

func main() {
	drivers.ExecuteDriver(clang.NewDriver())
}
