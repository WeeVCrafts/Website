package components

import (
	"fmt"
	"strconv"
)

func RupeeInput(paise string) string {
	if paise == "" {
		return ""
	}
	value, err := strconv.ParseInt(paise, 10, 64)
	if err != nil || value < 0 {
		return ""
	}
	return fmt.Sprintf("%d.%02d", value/100, value%100)
}
