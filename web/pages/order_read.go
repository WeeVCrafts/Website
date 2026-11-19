package pages

import "fmt"

func storefrontReadMoney(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
