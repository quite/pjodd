package util

func Contains(ss []string, s string) bool {
	for _, i := range ss {
		if i == s {
			return true
		}
	}
	return false
}

func Last(ss []string) string {
	return ss[len(ss)-1]
}
