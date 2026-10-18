package service

import (
	"sort"
	"strconv"
)

func itoa(i int64) string    { return strconv.FormatInt(i, 10) }
func itoaF(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func sortStrings(s []string) { sort.Strings(s) }
