package main

import (
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc"
)

func main() {
	drivers.ExecuteDriver(gcc.NewDriver())
}
