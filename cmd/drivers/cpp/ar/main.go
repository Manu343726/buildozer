package main

import (
	"github.com/Manu343726/buildozer/pkg/driver"
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/ar"
)

func main() {
	var d driver.Driver = ar.NewDriver()
	drivers.ExecuteDriver(d)
}
