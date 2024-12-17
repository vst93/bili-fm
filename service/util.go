package service

import "strconv"

// NumberToString 任意类型数字转换为字符串(太长的数字会有精度问题)
func NumberToString(n interface{}) (string, bool) {
	r := ""
	ok := true
	switch n.(type) {
	case string:
		r = n.(string)
	case int:
		r = strconv.Itoa(n.(int))
	case int64:
		r = strconv.FormatInt(n.(int64), 10)
	case float64:
		r = strconv.FormatFloat(n.(float64), 'f', -1, 64)
	default:
		ok = false
	}
	return r, ok
}
