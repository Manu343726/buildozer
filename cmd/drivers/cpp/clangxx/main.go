package main

import (
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/clangxx"
)

func main() {
	drivers.ExecuteDriver(clangxx.NewDriver())
}
