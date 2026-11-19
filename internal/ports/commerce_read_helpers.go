package ports

import "strconv"

func formatRecordID(id int64) string { return strconv.FormatInt(id, 10) }
