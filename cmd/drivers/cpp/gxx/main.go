package main

import (
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/gxx"
)

func main() {
	drivers.ExecuteDriver(gxx.NewDriver())
}
