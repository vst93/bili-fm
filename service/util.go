package service

import "strconv"

// NumberToString 任意类型数字转换为字符串(太长的数字会有精度问题)
func NumberToString(n interface{}) (string, bool) {
	r := ""
	ok := true
	switch n := n.(type) {
	case string:
		r = n
	case int:
		r = strconv.Itoa(n)
	case int64:
		r = strconv.FormatInt(n, 10)
	case float64:
		r = strconv.FormatFloat(n, 'f', -1, 64)
	default:
		ok = false
	}
	return r, ok
}
